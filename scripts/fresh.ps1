# EveChat fresh instance for the setup wizard (Windows PowerShell 5.1+).
#
# Creates a throwaway data directory under %TEMP%, picks a free port, generates
# temporary secrets, starts `evechat serve`, prints the wizard URL and the setup
# token, then waits. Ctrl+C stops the server and deletes the temporary data.
# It never reads or writes ./data or ./evechat.yaml.
#
# Usage:
#   powershell -ExecutionPolicy Bypass -File scripts/fresh.ps1
#   make fresh
#
# For the Playwright wizard suite:
#   $env:EVECHAT_BASE = "http://127.0.0.1:<port>"
#   $env:EVECHAT_SETUP_TOKEN = "<printed token>"
#   cd web/e2e; npx playwright test wizard.spec.ts

param(
    [int]$Port = 0,
    [switch]$NoBuild
)

$ErrorActionPreference = "Stop"
$repo = Split-Path -Parent $PSScriptRoot
$bin = Join-Path $repo "evechat.exe"
$tempRoot = [System.IO.Path]::GetTempPath()
$tempDir = Join-Path $tempRoot ("evechat-fresh-" + [Guid]::NewGuid().ToString("N").Substring(0, 8))
$dataDir = Join-Path $tempDir "data"
$configPath = Join-Path $tempDir "evechat.yaml"
$dotenvPath = Join-Path $tempDir ".env"
$process = $null
$cleaned = $false

function New-RandomBase64([int]$bytes) {
    $buf = New-Object byte[] $bytes
    $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    try { $rng.GetBytes($buf) } finally { $rng.Dispose() }
    return [Convert]::ToBase64String($buf)
}

function Get-FreePort {
    $listener = New-Object System.Net.Sockets.TcpListener -ArgumentList ([System.Net.IPAddress]::Loopback), 0
    $listener.Start()
    try {
        return ([System.Net.IPEndPoint]$listener.LocalEndpoint).Port
    } finally {
        $listener.Stop()
    }
}

function Test-PortFree([int]$candidate) {
    $listener = New-Object System.Net.Sockets.TcpListener -ArgumentList ([System.Net.IPAddress]::Loopback), $candidate
    try {
        $listener.Start()
        return $true
    } catch {
        return $false
    } finally {
        try { $listener.Stop() } catch { }
    }
}

function Remove-FreshInstance {
    if ($script:cleaned) { return }
    $script:cleaned = $true
    if ($null -ne $script:process -and -not $script:process.HasExited) {
        try {
            $script:process.Kill()
            $script:process.WaitForExit(5000) | Out-Null
        } catch { }
    }
    if (Test-Path $script:tempDir) {
        try { Remove-Item -Recurse -Force $script:tempDir } catch { }
    }
}

try {
    # A safety net: never operate outside %TEMP%, and never under the repo.
    if (-not $tempDir.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase)) {
        throw "refusing to use a data directory outside %TEMP%"
    }
    if ($tempDir.StartsWith((Join-Path $repo "data"), [StringComparison]::OrdinalIgnoreCase)) {
        throw "refusing to use the repository data directory"
    }

    New-Item -ItemType Directory -Path $dataDir -Force | Out-Null

    if (-not $NoBuild) {
        $go = Get-Command go -ErrorAction SilentlyContinue
        if ($null -ne $go) {
            Write-Host "[fresh] building evechat.exe..." -ForegroundColor Cyan
            Push-Location $repo
            try {
                & go build -o $bin ./cmd/evechat
                if ($LASTEXITCODE -ne 0) { throw "go build failed with exit code $LASTEXITCODE" }
            } finally {
                Pop-Location
            }
        } elseif (-not (Test-Path $bin)) {
            throw "go is not installed and $bin does not exist"
        }
    }
    if (-not (Test-Path $bin)) { throw "binary not found: $bin" }

    if ($Port -eq 0) {
        $Port = Get-FreePort
    } elseif (-not (Test-PortFree $Port)) {
        throw "port $Port is already in use; omit -Port to pick a free one"
    }

    $jwt = New-RandomBase64 48
    $key = New-RandomBase64 32
    $dbPath = (Join-Path $dataDir "evechat.db") -replace "\\", "/"
    $uploadsPath = (Join-Path $dataDir "uploads") -replace "\\", "/"

    @(
        "EVECHAT_AUTH__JWT_SECRET=$jwt",
        "EVECHAT_ENCRYPTION_KEY_V1=$key"
    ) | Set-Content -Path $dotenvPath -Encoding ASCII

    @"
server:
  host: "127.0.0.1"
  port: $Port
  public_url: "http://127.0.0.1:$Port"
  # trusted for the X-Forwarded-For simulation used by wizard.spec.ts
  trust_proxy: true
  trusted_proxies:
    - "127.0.0.1/32"
    - "::1/128"
database:
  path: "$dbPath"
  busy_timeout_ms: 5000
storage:
  uploads_dir: "$uploadsPath"
encryption:
  current_version: 1
log:
  level: "warn"
"@ | Set-Content -Path $configPath -Encoding ASCII

    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $bin
    $psi.Arguments = "--config `"$configPath`" serve"
    $psi.WorkingDirectory = $tempDir
    $psi.UseShellExecute = $false
    $psi.CreateNoWindow = $true
    $script:process = [System.Diagnostics.Process]::Start($psi)

    $base = "http://127.0.0.1:$Port"
    $ready = $false
    for ($i = 0; $i -lt 80; $i++) {
        Start-Sleep -Milliseconds 250
        if ($process.HasExited) { break }
        try {
            $response = Invoke-WebRequest -UseBasicParsing -Uri "$base/api/v1/health" -TimeoutSec 2
            if ($response.StatusCode -eq 200) { $ready = $true; break }
        } catch { }
    }
    if (-not $ready) {
        throw "the server did not become healthy on $base (exit code: $($process.ExitCode))"
    }

    $tokenPath = Join-Path $dataDir "setup-token.txt"
    $token = ""
    if (Test-Path $tokenPath) {
        $token = (Get-Content -Path $tokenPath -Raw).Trim()
    }

    Write-Host ""
    Write-Host "  EveChat fresh instance is ready" -ForegroundColor Green
    Write-Host "  Wizard:       $base/setup"
    Write-Host "  Dashboard:    $base/"
    Write-Host "  Data:         $tempDir"
    if ($token) {
        Write-Host "  Setup token:  $token"
        Write-Host "  Token file:   $tokenPath"
        Write-Host ""
        Write-Host "  Playwright:"
        Write-Host "    `$env:EVECHAT_BASE = `"$base`""
        Write-Host "    `$env:EVECHAT_SETUP_TOKEN = `"$token`""
    }
    Write-Host ""
    Write-Host "  Press Ctrl+C to stop the server and delete the temporary data." -ForegroundColor Yellow
    Write-Host ""

    if ($process.HasExited) {
        throw "the server exited unexpectedly (exit code: $($process.ExitCode))"
    }
    Wait-Process -Id $process.Id
} finally {
    Remove-FreshInstance
}
