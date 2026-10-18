# EveChat installer for Windows (PowerShell 5.1+).
#   irm https://raw.githubusercontent.com/aibedini/evechat/main/install.ps1 | iex
param(
    [string]$Version = "",
    [string]$Dir = "$env:ProgramData\evechat",
    [int]$Port = 3000,
    [string]$PublicUrl = "",
    [string]$BaseUrl = "https://github.com/aibedini/evechat/releases/download"
)

$ErrorActionPreference = "Stop"
$repo = "aibedini/evechat"

function Write-Step($msg) { Write-Host "▸ $msg" -ForegroundColor Cyan }
function Write-Ok($msg) { Write-Host "✓ $msg" -ForegroundColor Green }
function Write-Warn($msg) { Write-Host "! $msg" -ForegroundColor Yellow }

if (-not $Version) {
    Write-Step "دریافت آخرین نسخه از GitHub…"
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest" -TimeoutSec 20
    $Version = $release.tag_name
}
Write-Ok "نسخه: $Version"

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { "amd64" }
    default { throw "معماری پشتیبانی نمی‌شود: $env:PROCESSOR_ARCHITECTURE" }
}
$verNoV = $Version.TrimStart("v")
$archive = "evechat_${verNoV}_windows_${arch}.zip"
$url = "$BaseUrl/$Version/$archive"
$tmp = Join-Path $env:TEMP ("evechat-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null

Write-Step "دانلود $archive"
Invoke-WebRequest -Uri $url -OutFile (Join-Path $tmp $archive) -UseBasicParsing -TimeoutSec 120

try {
    $sums = Invoke-WebRequest -Uri "$BaseUrl/$Version/checksums.txt" -UseBasicParsing -TimeoutSec 30
    $expected = ($sums.Content -split "`n" | Where-Object { $_ -match [regex]::Escape($archive) } | ForEach-Object { ($_ -split "\s+")[0] } | Select-Object -First 1)
    $actual = (Get-FileHash (Join-Path $tmp $archive) -Algorithm SHA256).Hash.ToLower()
    if ($expected -and $expected.ToLower() -ne $actual) { throw "checksum مطابقت ندارد" }
    Write-Ok "checksum تأیید شد"
} catch {
    Write-Warn "بررسی checksum انجام نشد: $($_.Exception.Message)"
}

Expand-Archive -Path (Join-Path $tmp $archive) -DestinationPath $tmp -Force
New-Item -ItemType Directory -Force -Path $Dir | Out-Null
Copy-Item (Join-Path $tmp "evechat.exe") (Join-Path $Dir "evechat.exe") -Force
Write-Ok "باینری در $Dir\evechat.exe نصب شد"

$envFile = Join-Path $Dir ".env"
if (-not (Test-Path $envFile)) {
    $jwt = [Convert]::ToBase64String((1..48 | ForEach-Object { Get-Random -Maximum 256 }))
    $key = [Convert]::ToBase64String((1..32 | ForEach-Object { Get-Random -Maximum 256 }))
    @(
        "# EveChat — generated $(Get-Date -Format o)",
        "EVECHAT_AUTH__JWT_SECRET=$jwt",
        "EVECHAT_ENCRYPTION_KEY_V1=$key"
    ) | Set-Content -Path $envFile -Encoding ASCII
    Write-Ok "کلیدهای امنیتی ساخته شدند: $envFile"
}

if (-not $PublicUrl) { $PublicUrl = "http://localhost:$Port" }
$cfg = Join-Path $Dir "evechat.yaml"
if (-not (Test-Path $cfg)) {
    @"
server:
  host: "0.0.0.0"
  port: $Port
  public_url: "$PublicUrl"
database:
  path: "$($Dir -replace '\\','/')/evechat.db"
storage:
  uploads_dir: "$($Dir -replace '\\','/')/uploads"
encryption:
  current_version: 1
log:
  level: "info"
"@ | Set-Content -Path $cfg -Encoding UTF8
    Write-Ok "تنظیمات نوشته شد: $cfg"
}

# Load secrets into this process and complete the first-run setup.
Get-Content $envFile | ForEach-Object {
    if ($_ -match "^(?<k>[A-Z0-9_]+)=(?<v>.*)$") { [Environment]::SetEnvironmentVariable($Matches.k, $Matches.v, "Process") }
}
Write-Step "اجرای ویزارد نصب"
& (Join-Path $Dir "evechat.exe") --config $cfg setup --non-interactive --print-credentials

Write-Warn "برای اجرای خودکار، یک Scheduled Task بسازید یا دستی اجرا کنید:"
Write-Host "  $Dir\evechat.exe --config $cfg serve" -ForegroundColor Gray
Write-Warn "برای اجرای دائمی:"
Write-Host "  schtasks /create /tn EveChat /tr `"$Dir\evechat.exe --config $cfg serve`" /sc onstart /ru SYSTEM" -ForegroundColor Gray
