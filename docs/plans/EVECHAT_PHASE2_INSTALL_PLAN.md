# EveChat — Phase 2 Execution Pack: Install, Release, Operations & Typography

> **Repo:** `https://github.com/aibedini/evechat` (branch `main`)
> **Continues from:** `EVECHAT_BUILD_PLAN.md` TASK 01–17 (already implemented and merged)
> **This pack:** TASK 18 → TASK 34

---

## ⛔ RULES FOR THE EXECUTOR — READ ONCE, OBEY ALWAYS

1. You are an implementation engine. **Do not ask questions. Do not propose alternatives. Do not summarize what you are about to do.**
2. Output **files only**, in this exact format:
   ```
   === FILE: <exact/relative/path/from/repo/root> ===
   <complete content — no truncation, no "// unchanged", no "...">
   ```
   For a file you are **modifying**, output the **entire new file**, not a diff.
3. Never write `TODO`, `FIXME`, placeholder bodies, or `// implement later`.
4. Finish each TASK completely, print only `--- TASK NN DONE ---`, then start the next TASK immediately with no commentary in between.
5. Everything must keep working: `CGO_ENABLED=0 go build ./...`, `go vet ./...`, `go test ./...` must stay green, and the widget must stay **under 12 288 bytes gzip**.
6. All user-facing strings in CLI wizards, web wizard, dashboard and widget are **Persian (fa-IR)**. Log lines and code comments stay English.
7. Do not break existing config keys. New keys are **additive with safe defaults**.
8. No new Go dependency unless this document names it explicitly.

---

## PART 0 — GAP MATRIX (what was promised vs. what exists vs. which TASK fixes it)

| # | Requirement | Current state | Fixed by |
|---|---|---|---|
| 1 | Prebuilt binaries for download | ❌ Releases empty | TASK 18 |
| 2 | Published container image | ❌ Packages empty | TASK 18 |
| 3 | CI (build/test/lint on push) | ❌ none | TASK 18 |
| 4 | One-line installer | ❌ none | TASK 19 |
| 5 | Interactive CLI setup wizard with menu | ❌ none | TASK 20 |
| 6 | Browser-based first-run setup wizard | ❌ none | TASK 21 |
| 7 | Preflight / health diagnostics command | ❌ only `/api/v1/health` | TASK 22 |
| 8 | systemd service | ❌ none | TASK 23 |
| 9 | Automatic TLS (Caddy) | ❌ only manual nginx | TASK 23 |
| 10 | Backup / restore / update / uninstall tooling | ⚠️ prose in README only | TASK 24 |
| 11 | Vazirmatn font, self-hosted | ❌ system fonts | TASK 25 |
| 12 | Prometheus metrics | ❌ none | TASK 26 |
| 13 | Widget spam/abuse protection | ⚠️ rate-limit only | TASK 27 |
| 14 | SMTP / email notification channel | ❌ none | TASK 28 |
| 15 | Persian README + screenshots + demo | ❌ English only, no images | TASK 29 |
| 16 | SECURITY / CONTRIBUTING / CHANGELOG / templates | ❌ none | TASK 30 |
| 17 | Versioning, tags, release notes | ❌ 1 commit, no tag | TASK 30 |
| 18 | Dependency & vulnerability scanning | ❌ none | TASK 30 |
| 19 | Offline / air-gapped install bundle (Iran) | ❌ none | TASK 31 |
| 20 | Audit log of admin actions | ❌ none | TASK 32 |
| 21 | Dark mode + accessibility pass | ⚠️ unverified | TASK 33 |
| 22 | End-to-end install test in CI | ❌ none | TASK 34 |

---

# TASK 18 — RELEASE PIPELINE (highest priority — nothing else matters without this)

## 18.1 `.github/workflows/ci.yml`

Triggers: `push` to `main`, `pull_request`.

Jobs:

**`go`** (ubuntu-latest):
- `actions/checkout@v4`
- `actions/setup-go@v5` with `go-version: '1.23'`, `cache: true`
- `go vet ./...`
- `go test ./... -race -count=1`
- `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` (non-blocking: `continue-on-error: true`, but surfaced in the job summary)
- `gofmt -l .` → fail if output is non-empty

**`web`** (ubuntu-latest):
- `actions/setup-node@v4`, `node-version: '22'`, `cache: npm`, cache paths `web/widget/package-lock.json` and `web/dashboard/package-lock.json`
- build widget → assert gzip size `< 12288` → upload `widget.js` as an artifact
- build dashboard → upload `embed/dashboard` as an artifact
- `cd web/widget && npx vitest run`

**`docker`** (needs: go, web): build the image only (no push) to prove the Dockerfile works.

## 18.2 `.github/workflows/release.yml`

Trigger: `push` on tags matching `v*.*.*`, plus `workflow_dispatch`.

Steps:
1. Checkout with `fetch-depth: 0`.
2. Node build of widget + dashboard into `embed/` (release artifacts must be fully embedded).
3. Cross-compile matrix with **one** job using a loop (not a matrix of runners — keep it cheap):

| GOOS | GOARCH | output |
|---|---|---|
| linux | amd64 | `evechat_<ver>_linux_amd64` |
| linux | arm64 | `evechat_<ver>_linux_arm64` |
| linux | arm (GOARM=7) | `evechat_<ver>_linux_armv7` |
| darwin | amd64 | `evechat_<ver>_darwin_amd64` |
| darwin | arm64 | `evechat_<ver>_darwin_arm64` |
| windows | amd64 | `evechat_<ver>_windows_amd64.exe` |

Build flags, exactly:
```
CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${GITHUB_SHA::7} -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o dist/<name> ./cmd/evechat
```
4. For each binary: produce `<name>.tar.gz` (zip for windows) containing the binary + `evechat.example.yaml` + `LICENSE` + `README.md` + `README.fa.md`.
5. `sha256sum dist/* > checksums.txt`.
6. Create the GitHub Release with `softprops/action-gh-release@v2`, body generated from `CHANGELOG.md` section for that version, attach all archives + `checksums.txt` + the standalone `widget.js`.
7. Build and push a multi-arch image (`linux/amd64,linux/arm64`) with `docker/setup-buildx-action@v3` + `docker/build-push-action@v6` to `ghcr.io/aibedini/evechat`, tags: `<version>`, `<major>.<minor>`, `latest`. Login with `GITHUB_TOKEN`, `packages: write` permission.

## 18.3 `cmd/evechat/version.go`
```go
var (version = "dev"; commit = "none"; buildDate = "unknown")
func versionString() string  // "evechat v1.0.0 (a1b2c3d, 2026-10-18T12:00:00Z, go1.23.2, linux/amd64)"
```
`--version` prints it and exits 0. `/api/v1/health` returns `version`, `commit`, `build_date`, `uptime_seconds`, `go_version`.

## 18.4 `Makefile` additions
```
release-local:   # builds the full matrix locally into dist/ without pushing
checksums:
```

**Acceptance:** push tag `v1.0.0` → Release page shows 6 archives + `checksums.txt` + `widget.js`, and `docker pull ghcr.io/aibedini/evechat:1.0.0` works.

`--- TASK 18 DONE ---`

---

# TASK 19 — ONE-LINE INSTALLER

## 19.1 `install.sh` (repo root, also served raw from GitHub)

Usage, all of these must work:
```bash
curl -fsSL https://raw.githubusercontent.com/aibedini/evechat/main/install.sh | bash
curl -fsSL .../install.sh | bash -s -- --version v1.0.0 --dir /opt/evechat --port 8080
curl -fsSL .../install.sh | bash -s -- --no-systemd
curl -fsSL .../install.sh | bash -s -- --uninstall
```

Behaviour, in order:
1. `set -euo pipefail`; define `info/warn/err/ok` colored output helpers (disable color when not a TTY or when `NO_COLOR` is set).
2. Detect OS (`linux`/`darwin`) and ARCH (`x86_64→amd64`, `aarch64|arm64→arm64`, `armv7l→armv7`). Unsupported → clear error naming what was detected.
3. Require: `curl` or `wget`, `tar`, `sha256sum` or `shasum`. Missing → print the exact apt/dnf/brew command to install it.
4. Resolve version: `--version` flag, else the latest tag from `https://api.github.com/repos/aibedini/evechat/releases/latest`. If the API is unreachable (common in IR), fall back to `--version` requirement with a clear message and a mirror hint (`--base-url`).
5. Support `--base-url <url>` so the whole download can come from a mirror or a local HTTP server (offline install, see TASK 31).
6. Download the archive + `checksums.txt`, **verify the sha256**, abort loudly on mismatch.
7. Install to `--dir` (default `/opt/evechat`): binary at `<dir>/evechat` (mode 0755), `evechat.example.yaml` at `<dir>/`.
8. Create a dedicated system user `evechat` (`useradd --system --no-create-home --shell /usr/sbin/nologin`) unless `--user` is given or the OS is macOS. Create `<dir>/data` owned by that user, mode 0750.
9. Generate secrets if `<dir>/.env` does not exist:
   ```
   EVECHAT_AUTH__JWT_SECRET=<openssl rand -base64 48, fallback: head -c48 /dev/urandom | base64>
   EVECHAT_ENCRYPTION_KEY_V1=<32 random bytes, base64>
   ```
   File mode **0600**, owned by the service user. If `.env` exists, never overwrite — print "existing .env kept".
10. Write `<dir>/evechat.yaml` from the example with `database.path`, `storage.uploads_dir`, `server.port` and `server.public_url` substituted.
11. Install the systemd unit (TASK 23) unless `--no-systemd` or systemd is absent; `systemctl daemon-reload && enable --now evechat`.
12. Wait up to 20s for `/api/v1/health` to return ok; on failure, print the last 30 journal lines and exit 1.
13. Run `<dir>/evechat --config <dir>/evechat.yaml setup --non-interactive --print-credentials` (TASK 20) to create the first project + superadmin, and print the final box:
    ```
    ╔══════════════════════════════════════════════╗
    ║  EveChat نصب شد ✓                            ║
    ╠══════════════════════════════════════════════╣
    ║  پنل:        http://<ip>:3000                ║
    ║  ایمیل:      admin@evechat.local             ║
    ║  رمز عبور:   ••••••••••••                    ║
    ║  کلید پروژه: pk_...                          ║
    ╚══════════════════════════════════════════════╝
    کد embed در فایل <dir>/embed-snippet.html ذخیره شد.
    ```
14. `--uninstall`: stop + disable the service, remove the unit, remove the binary, **keep `data/` and `.env`** and print where they are (never delete user data silently). `--purge` additionally deletes them after an explicit `yes` typed on stdin.

Also provide `install.ps1` for Windows (download, verify, extract, create a Scheduled Task or print manual run instructions). Keep it short — Windows is not a primary target.

## 19.2 `docker-compose.quick.yml`
A compose file that pulls the published image (no build), reads `.env`, and includes a commented Caddy service for automatic TLS. Referenced in README as the zero-build path.

**Acceptance:** on a clean Ubuntu 24.04 VM, a single `curl | bash` yields a running, reachable dashboard with credentials printed, with **no Go and no Node installed**.

`--- TASK 19 DONE ---`

---

# TASK 20 — INTERACTIVE CLI SETUP WIZARD (`evechat setup`)

## 20.1 Command layout
Migrate `cmd/evechat` from bare flags to subcommands (hand-rolled with `flag.NewFlagSet`, **no cobra**):
```
evechat                      → run the server (default, back-compat with current flags)
evechat serve                → explicit run
evechat setup                → interactive wizard (this task)
evechat doctor               → diagnostics (TASK 22)
evechat migrate              → run migrations and exit
evechat backup <path>        → online backup (TASK 24)
evechat restore <path>       → restore (TASK 24)
evechat user add|list|passwd|disable
evechat project add|list|key-rotate
evechat key rotate|status    → encryption key ops
evechat healthcheck          → exit 0/1 (container healthcheck)
evechat version
```
Unknown subcommand → print the menu and exit 2. `evechat --help` prints a Persian help block.

## 20.2 `internal/tui/` — a tiny terminal UI (no dependency)
Files: `tui.go`, `prompt.go`, `menu.go`, `box.go`, `color.go`, `spinner.go`.

```go
func Title(s string)                                  // boxed header
func Section(s string)
func Ask(label, def string, validate func(string) error) string
func AskSecret(label string) string                   // golang.org/x/term, no echo
func AskYesNo(label string, def bool) bool
func Menu(title string, items []MenuItem) int         // ↑/↓ + Enter + number keys
func MultiSelect(title string, items []string, preselected []int) []int
func Box(title string, lines []string)
func Step(n, total int, title string)                 // "گام ۲ از ۷ — پایگاه داده"
func OK(s string); func Warn(s string); func Fail(s string)
func Spinner(label string) func()                     // returns stop func
```
Rules:
- Raw-mode arrow-key menu via `golang.org/x/term` (**the only new dependency allowed in this task**). If the terminal is not interactive, fall back to numbered input on stdin.
- RTL-safe: Persian labels printed as-is; do **not** try to reverse strings. Keep box borders ASCII-safe and compute width with rune count, not byte length.
- Honour `NO_COLOR` and non-TTY (plain output, no escape codes).
- Persian digits for step counters, Latin digits for values the user copies (ports, keys).

## 20.3 Wizard flow (`evechat setup`)

```
╔═══════════════════════════════════════╗
║        نصب و راه‌اندازی EveChat        ║
╚═══════════════════════════════════════╝
```

**گام ۱ از ۸ — بررسی پیش‌نیازها**
Runs the same checks as `doctor` (TASK 22). A hard failure stops the wizard with an actionable message.

**گام ۲ از ۸ — مسیر داده‌ها**
Ask for `database.path` dir (default `./data`), uploads dir. Create them, verify writability, show free disk space.

**گام ۳ از ۸ — آدرس و پورت**
Ask `server.port` (validate 1–65535, warn if already bound), `server.public_url` (validate URL; warn if it is `localhost` while the host has a public IP), `server.trust_proxy` (yes if behind nginx/Caddy).

**گام ۴ از ۸ — کلیدهای امنیتی**
Offer a menu:
```
۱) تولید خودکار کلیدها (پیشنهادی)
۲) وارد کردن کلیدهای موجود
```
Generate `jwt_secret` (48 bytes) and `EVECHAT_ENCRYPTION_KEY_V1` (32 bytes). Write them to `.env` (0600) **never** to the YAML. Display them once inside a box with the warning: «این کلیدها را جای امنی نگه دارید. با گم شدن کلید رمزنگاری، پیام‌های قدیمی غیرقابل بازیابی می‌شوند.»

**گام ۵ از ۸ — حساب مدیر**
Ask name, email (validate), password (`AskSecret` twice, min 10 chars, reject the 20 most common passwords from a small embedded list) or offer auto-generation of a 16-char password.

**گام ۶ از ۸ — اولین پروژه**
Ask project name, allowed domains (comma-separated, validate each as a hostname; empty = dev mode with a printed warning). Create the project, generate `api_key` + `secret_key`.

**گام ۷ از ۸ — یکپارچه‌سازی‌ها (اختیاری)**
`MultiSelect`:
```
[ ] تلگرام (ربات پشتیبانی)
[ ] EVE (شناسایی کاربر)
[ ] GMweb (پیامک)
[ ] دستیار هوش مصنوعی
[ ] ایمیل (SMTP)
```
For each selected item ask its fields, then **test the connection live** with a spinner:
- Telegram → `getMe`, print the bot username; offer to set the webhook now.
- EVE → `GET {base}/api/user/0000000000` expecting 200/404 (not a connection error).
- GMweb → a lightweight ping to the base URL.
- AI → a 1-token Claude request.
- SMTP → connect + STARTTLS + AUTH, no mail sent.
A failed test is a warning, not a stop: «ذخیره شد ولی تست ناموفق بود — بعداً در تنظیمات اصلاح کنید.»

**گام ۸ از ۸ — پایان**
Write `evechat.yaml`, run migrations, print a summary box with: dashboard URL, email, password, project key, the ready-to-paste embed snippet, and the next command (`systemctl start evechat` or `./evechat serve`). Also write `embed-snippet.html` and `setup-summary.txt` (0600) next to the config.

## 20.4 Non-interactive mode
`evechat setup --non-interactive` with flags `--admin-email --admin-password --admin-name --project-name --domains --port --public-url --data-dir --print-credentials --json`.
Missing password → generate one. `--json` prints a machine-readable result for `install.sh` to parse. Idempotent: if a superadmin already exists, exit 0 with `already_configured: true` and change nothing.

## 20.5 Re-running
If `evechat.yaml` already exists, the wizard opens a menu instead:
```
۱) تغییر تنظیمات سرور
۲) افزودن کاربر پشتیبان
۳) افزودن پروژه
۴) پیکربندی یکپارچه‌سازی‌ها
۵) تولید مجدد کلیدها (چرخش کلید)
۶) تست سلامت سیستم
۷) نمایش کد embed
۰) خروج
```

`--- TASK 20 DONE ---`

---

# TASK 21 — WEB FIRST-RUN SETUP WIZARD

For users who will never open a terminal.

## 21.1 Server side

New table in `006_setup.sql`:
```sql
CREATE TABLE IF NOT EXISTS app_settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```
Keys used: `setup_completed` (`"1"`), `setup_completed_at`, `instance_id`.

**Gate middleware** (`middleware/setup.go`): if `setup_completed` is unset, every non-asset request outside `/setup*`, `/api/setup/*`, `/api/v1/health` redirects (HTML) or returns `409 {"code":"setup_required"}` (JSON). Once set, `/setup*` returns 404 and `/api/setup/*` returns 410.

**Anti-hijack:** on boot with setup incomplete, generate a 32-byte `setup_token`, print it in the startup banner and write it to `<data_dir>/setup-token.txt` (0600). `/api/setup/*` requires `X-Setup-Token` **unless** the request comes from a loopback address. Document this clearly: whoever can read the server's stdout or filesystem is the legitimate installer.

**Endpoints:**
```
GET  /api/setup/status      → {setup_required, version, checks:[...]}
POST /api/setup/check       → re-run preflight checks, returns the doctor report
POST /api/setup/admin       → {name, email, password}            → creates superadmin
POST /api/setup/project     → {name, domains[]}                  → creates project, returns api_key + snippet
POST /api/setup/integrations→ {telegram?, eve?, gmweb?, ai?, smtp?} → saves + tests each
POST /api/setup/test/:kind  → live connection test, returns {ok, message}
POST /api/setup/complete    → sets setup_completed, returns {access_token, redirect:"/inbox"}
```
Integration secrets go to `app_settings` **encrypted** with the AES service (they are not in the env flow); the config loader merges DB-stored integration settings over YAML at startup and on change.

## 21.2 Client side — `web/setup/`
A **separate, tiny** Vite bundle (not part of the dashboard, so a broken dashboard build never blocks setup). Embedded at `embed/setup/`.

Five full-screen steps with a progress bar, RTL, Vazirmatn (TASK 25), dark-mode aware:

1. **خوش آمدید** — version, a green/red checklist from `/api/setup/check` (disk writable, port, keys present, clock skew, outbound internet), a «بررسی مجدد» button, «شروع» disabled while any hard check fails.
2. **حساب مدیر** — name, email, password with a strength meter and a «تولید رمز قوی» button that reveals + copies.
3. **اولین پروژه** — project name, domain chips input (Enter adds, × removes), a live widget preview panel on the side.
4. **یکپارچه‌سازی‌ها** — accordion cards, each with fields, a «تست اتصال» button showing ✓/✗ inline, and a «رد کردن» link. Nothing here is mandatory.
5. **پایان** — big success state, credentials box with copy buttons, the embed snippet in a syntax-highlighted `<pre>` with a copy button, a «دانلود خلاصه» button (txt), and «ورود به پنل» which uses the returned token to land directly in `/inbox`.

Rules: every step validates before advancing, Back preserves state, a refresh resumes at the right step (server state is the source of truth), and all errors render as Persian inline messages, never `alert()`.

**Acceptance:** `docker run ghcr.io/aibedini/evechat` → open `http://localhost:3000` → wizard → finish → logged into the inbox, with the widget snippet copied. Zero terminal commands beyond `docker run`.

`--- TASK 21 DONE ---`

---

# TASK 22 — `evechat doctor`

`internal/doctor/doctor.go`:
```go
type Check struct {
	ID       string
	Name     string   // Persian
	Level    Level    // Fatal, Warn, Info
	Run      func(ctx context.Context) Result
}
type Result struct { OK bool; Message string; Fix string; Detail string }
func RunAll(ctx, cfg) Report
func (r Report) Print(w io.Writer, verbose bool)
func (r Report) JSON() []byte
func (r Report) ExitCode() int   // 0 all ok, 1 fatal, 2 warnings only
```

Checks, each with a Persian `Fix` string telling the user exactly what to run:

| ID | Check |
|---|---|
| `config.file` | config readable and parseable |
| `config.jwt` | jwt_secret present, ≥ 24 chars, not the default |
| `config.keys` | encryption key for `current_version` present, exactly 32 bytes after base64 decode; **and** every `key_version` referenced in `messages` has a loaded key |
| `fs.data` | DB dir exists, writable, ≥ 200MB free |
| `fs.uploads` | uploads dir writable |
| `fs.perms` | `.env` is 0600 and not world-readable |
| `db.open` | DB opens, `PRAGMA integrity_check` returns ok |
| `db.wal` | journal_mode is WAL |
| `db.migrations` | schema version == latest embedded |
| `net.port` | `server.port` bindable (or already bound by us) |
| `net.outbound` | HTTPS reachability of `api.telegram.org` (only when telegram enabled) |
| `net.publicurl` | `public_url` resolves and, if reachable, returns our own `/api/v1/health` with a matching `instance_id` |
| `time.skew` | system clock within 60s of an HTTP `Date` header (JWT and Telegram break on skew) |
| `tls.cert` | when `tls.enabled`, cert parses, matches the key, not expired, warns under 21 days |
| `integration.telegram` | `getMe` + current webhook matches the configured URL |
| `integration.eve` | base URL reachable, auth accepted |
| `integration.gmweb` | base URL reachable |
| `integration.ai` | key accepted (1-token probe) |
| `integration.smtp` | connect + auth |
| `runtime.memory` | current RSS, warn above 150MB |
| `runtime.fd` | open-file limit, warn below 4096 — `Fix` names the `LimitNOFILE` line |
| `widget.size` | embedded widget present and under 12KB gzip |
| `assets.dashboard` | embedded dashboard has `index.html` |
| `assets.font` | Vazirmatn woff2 present in the embedded assets |

Flags: `--json`, `--verbose`, `--only <id,id>`, `--fix` (performs only the **safe** fixes: create missing dirs, chmod `.env` to 0600, run pending migrations, re-set the Telegram webhook — never touches keys or data).

Wire the same report into `GET /api/v1/diagnostics` (superadmin only) and render it in the dashboard under Settings → سلامت سیستم with ✓/⚠/✗ rows and the Fix hint.

`--- TASK 22 DONE ---`

---

# TASK 23 — SERVICE UNITS & REVERSE PROXIES

## 23.1 `deploy/systemd/evechat.service`
```ini
[Unit]
Description=EveChat live support server
Documentation=https://github.com/aibedini/evechat
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=evechat
Group=evechat
WorkingDirectory=/opt/evechat
EnvironmentFile=/opt/evechat/.env
ExecStartPre=/opt/evechat/evechat --config /opt/evechat/evechat.yaml migrate
ExecStart=/opt/evechat/evechat --config /opt/evechat/evechat.yaml serve
Restart=always
RestartSec=3
TimeoutStopSec=30
KillSignal=SIGTERM
LimitNOFILE=65535

# hardening
NoNewPrivileges=true
PrivateTmp=true
PrivateDevices=true
ProtectSystem=strict
ProtectHome=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
RestrictNamespaces=true
LockPersonality=true
MemoryDenyWriteExecute=true
SystemCallFilter=@system-service
SystemCallErrorNumber=EPERM
ReadWritePaths=/opt/evechat/data

[Install]
WantedBy=multi-user.target
```
Also ship `deploy/systemd/evechat-backup.service` + `evechat-backup.timer` (daily 03:00, `Persistent=true`).

## 23.2 `deploy/caddy/Caddyfile`
```
chat.example.com {
	encode zstd gzip
	reverse_proxy 127.0.0.1:3000 {
		flush_interval -1          # required for SSE
		header_up X-Forwarded-For {remote_host}
		header_up X-Forwarded-Proto {scheme}
	}
}
```
With a header comment explaining: automatic Let's Encrypt, `flush_interval -1` is mandatory for SSE, and that `server.trust_proxy: true` must be set. Add a variant block for serving the widget with long cache headers.

## 23.3 `deploy/nginx/evechat.conf`
The existing README snippet, hardened: `proxy_read_timeout 3600s`, `proxy_send_timeout 3600s`, `proxy_buffering off` on `/api/widget/v1/events`, `client_max_body_size` matching `storage.max_file_size_mb`, gzip for `/widget.js`, and a `location /ws/` block with upgrade headers.

## 23.4 `deploy/traefik/dynamic.yml`
Minimal labels/config for users already on Traefik.

## 23.5 `deploy/docker/compose.caddy.yml`
compose with `evechat` + `caddy` services, a shared network, volumes for caddy data/config, and `.env` driven domain.

`--- TASK 23 DONE ---`

---

# TASK 24 — BACKUP, RESTORE, UPDATE, UNINSTALL

## 24.1 `evechat backup <dest>`
- Online, WAL-safe: `VACUUM INTO '<dest>.db'` (preferred, atomic) with a fallback to the backup API.
- `--with-uploads` → produce `<dest>.tar.gz` containing `evechat.db`, `uploads/`, and a `manifest.json` (`version`, `schema_version`, `created_at`, `instance_id`, `db_sha256`, `counts:{projects,agents,conversations,messages}`).
- `--encrypt` → age-style symmetric encryption using the AES service and a passphrase prompt. **Never** silently include `.env` in a backup; print a reminder that the encryption key is stored separately and is required to read messages.
- Retention: `--keep N` prunes the oldest archives in the destination directory.

## 24.2 `evechat restore <archive>`
- Refuses to run while the server is serving (lock file `<data>/evechat.lock` held by the running process).
- Verifies `manifest.json`, checksum, and that `schema_version` ≤ current; runs migrations after restore.
- Writes the current DB to `<data>/evechat.pre-restore-<ts>.db` before overwriting. Never deletes anything.
- `--dry-run` prints what would change.

## 24.3 `scripts/backup.sh`
Cron/timer-friendly wrapper: runs `evechat backup`, rotates by `KEEP_DAYS`, optional `rclone`/`rsync` offsite hook, logs to syslog, exits non-zero on failure so the timer surfaces it.

## 24.4 `evechat update` (and `scripts/update.sh`)
1. Read current version, query the latest release (or `--version`).
2. Download + verify checksum into a temp dir.
3. `evechat backup` automatically before touching anything.
4. Replace the binary atomically (`rename` over the old one), `systemctl restart evechat`.
5. Health-poll for 20s; on failure, roll back to the saved previous binary and restart, then report.
6. `--check` only reports whether an update is available.

## 24.5 Uninstall
`scripts/uninstall.sh` — stop/disable service, remove unit + binary, **keep data by default**, `--purge` with typed confirmation.

`--- TASK 24 DONE ---`

---

# TASK 25 — VAZIRMATN TYPOGRAPHY (self-hosted, offline-safe)

Source: `https://github.com/rastikerdar/vazirmatn` — SIL Open Font License 1.1. Pin **v33.003**.

> Why self-host: jsDelivr/Google Fonts are unreliable or blocked on Iranian networks. The font must ship inside the binary so a fully offline install still renders correctly.

## 25.1 Vendoring
- Add `vazirmatn@33.3.0` as a **devDependency** of `web/dashboard` and of `web/widget`.
- A build step copies only these files into `embed/assets/fonts/`:
  - `Vazirmatn[wght].woff2` → rename to `vazirmatn-variable.woff2` (variable, weights 100–900, **single file, ~70–90KB**)
  - `Vazirmatn-Regular.woff2` and `Vazirmatn-Bold.woff2` as static fallbacks for old browsers
- Copy `OFL.txt` to `embed/assets/fonts/OFL.txt` and add a `licenses/` note in the repo root `NOTICE` file. **The OFL requires the license file to travel with the font — do not omit it.**
- Add a `scripts/fetch-fonts.sh` that can fetch them directly from the GitHub release tag when npm is unavailable (Iran fallback), verifying sha256 against a pinned list in `scripts/fonts.sha256`.

## 25.2 Serving
New route, registered **before** the SPA fallback:
```
GET /assets/fonts/*   → embedded, Cache-Control: public, max-age=31536000, immutable
                        Content-Type: font/woff2
                        Access-Control-Allow-Origin: *      (needed: the widget loads it cross-origin)
                        ETag + 304 support
```
Also expose `GET /assets/fonts/vazirmatn.css` containing the `@font-face` block, so a host page can link it directly.

## 25.3 `@font-face` definition (single source of truth, `web/shared/font.css`)
```css
@font-face {
  font-family: 'Vazirmatn';
  src: url('/assets/fonts/vazirmatn-variable.woff2') format('woff2-variations'),
       url('/assets/fonts/vazirmatn-variable.woff2') format('woff2');
  font-weight: 100 900;
  font-style: normal;
  font-display: swap;
  unicode-range: U+0600-06FF, U+200C-200D, U+FB50-FDFF, U+FE70-FEFF, U+0000-00FF, U+0131, U+2000-206F, U+2190-21BB;
}
```
Font stack everywhere:
```css
--ec-font-sans: 'Vazirmatn', system-ui, -apple-system, 'Segoe UI', Tahoma, sans-serif;
```
Numerals: use `font-feature-settings: 'ss01'` is **not** needed — instead set `font-variant-numeric: tabular-nums` on numeric columns (stats tables, timers) so Jalali dates and counters don't jitter.

## 25.4 Dashboard
- `index.html`: `<link rel="preload" as="font" type="font/woff2" href="/assets/fonts/vazirmatn-variable.woff2" crossorigin>` before the stylesheet.
- Tailwind: `theme.extend.fontFamily.sans = ['Vazirmatn', ...defaultTheme.fontFamily.sans]`.
- Remove any previous `font-family` declarations that bypass the token.
- Type scale, applied consistently: 12/13/14/16/20/24/32px with line-heights 1.5 (body) and 1.3 (headings); Persian needs more leading than Latin — do not go below 1.5 for body text.
- Weight usage: 400 body, 500 labels, 600 headings, 700 only for the main page title. The variable font makes all of these free.

## 25.5 Widget — **critical technical detail**
`@font-face` declared **inside** a Shadow DOM is ignored by Chrome and Safari. Therefore:
- On init, the widget injects a single `<style id="evechat-font">` into `document.head` containing **only** the `@font-face` rule, with the family named `EveChatVazirmatn` (namespaced so it can never collide with or override a host-page `Vazirmatn`).
- Inside the shadow root, use `font-family: 'EveChatVazirmatn', system-ui, sans-serif`.
- Guard with `if (!document.getElementById('evechat-font'))` for multiple inits.
- The font file is **not** bundled into `widget.js` (that would blow the 12KB budget); it is fetched from `apiUrl + '/assets/fonts/...'`.
- New config option:
  ```js
  window.EveChat = { font: "vazirmatn" | "inherit" | "none" }   // default "vazirmatn"
  ```
  `inherit` → use the host page's font, inject nothing. `none` → system stack only.
- Lazy-load: inject the font `<style>` only when the chat panel is **first opened**, not on page load, so EveChat costs the host page zero font bytes until used.
- Add `font-display: swap` and a 100ms FOUT guard: panel text starts in the system stack and swaps in — never block rendering.

## 25.6 Setup wizard & error pages
`web/setup/` and the server-rendered fallback pages (`setup_required`, 404, 500) use the same font route.

## 25.7 Size budget check
`scripts/build.sh` asserts:
- `widget.js` gzip < 12 288 bytes (unchanged — the font is external)
- `embed/assets/fonts/` total < 400KB
- binary size after embedding < 45MB

**Acceptance:** disconnect the machine from the internet entirely, load the dashboard and a page with the widget → Persian text renders in Vazirmatn with no network requests outside the EveChat origin (verify in DevTools → Network, filtered to `font`).

`--- TASK 25 DONE ---`

---

# TASK 26 — METRICS & OBSERVABILITY

- `GET /metrics` in Prometheus text format, **hand-written** (no `prometheus/client_golang` — keep the dependency list small), guarded by `metrics.enabled` and either a bearer token (`metrics.token`) or a loopback-only bind.
- Metrics:
  ```
  evechat_build_info{version,commit,go_version} 1
  evechat_uptime_seconds
  evechat_ws_connections{kind="visitor|agent"}
  evechat_ws_messages_total{direction="in|out"}
  evechat_ws_clients_dropped_total{reason="slow|timeout|error"}
  evechat_conversations_total{status}
  evechat_messages_total{sender_type}
  evechat_http_requests_total{route,method,status}
  evechat_http_request_duration_seconds_bucket{route,le}   # 7 buckets: .005 .01 .05 .1 .5 1 5
  evechat_db_query_duration_seconds_bucket{op,le}
  evechat_notifications_total{channel,result}
  evechat_webhook_deliveries_total{result}
  evechat_ai_tokens_total{project,kind="input|output"}
  evechat_rate_limited_total{scope}
  evechat_goroutines
  evechat_memory_bytes{kind="heap|sys"}
  ```
- `deploy/monitoring/prometheus.yml` + `deploy/monitoring/grafana-dashboard.json` (one dashboard: connections, message rate, p95 latency, memory, error rate, queue depth).
- Structured log sampling: identical log lines rate-limited to 1/sec to keep disks sane during incidents.
- `log.file` + `log.max_size_mb` + `log.max_backups` config with built-in rotation (hand-rolled, ~80 lines — no lumberjack dependency) so a systemd-less install still rotates.

`--- TASK 26 DONE ---`

---

# TASK 27 — ABUSE & SPAM PROTECTION

The widget endpoint can currently be hammered to create unlimited visitors.

1. **Proof-of-work on session creation.** `GET /api/widget/v1/challenge` returns `{challenge, difficulty, expires_at}`; the client finds a nonce where `sha256(challenge+nonce)` has `difficulty` leading zero bits (default 16 ≈ 50ms on a phone, configurable `security.pow_difficulty`, `0` disables). `POST /api/widget/v1/session` requires the solved nonce. Challenges are single-use, stored in memory with a 2-minute TTL. Implemented in ~40 lines of TS on the widget side using a plain loop (no worker needed at difficulty 16).
2. **Adaptive difficulty:** raise difficulty automatically for an IP that created more than N sessions in 10 minutes.
3. **Content guards:** max 4000 runes per message (already), max 30 messages per conversation per minute, max 5 conversations per visitor per hour, configurable.
4. **Visitor reputation:** a simple score decremented on ban/spam signals, stored on the visitor; below a threshold, require PoW at higher difficulty.
5. **Blocklists:** `security.blocked_ips`, `security.blocked_cidrs`, `security.blocked_user_agents` (substring match), reloadable from `app_settings` without restart, editable in Settings.
6. **Honeypot field** in the pre-chat form, silently discarding submissions that fill it.
7. **Upload hardening:** re-verify magic bytes after write, strip EXIF from images, reject archives containing path traversal, serve uploads with `X-Content-Type-Options: nosniff` and `Content-Security-Policy: sandbox`.
8. **Security headers middleware** on all HTML responses: `X-Frame-Options: SAMEORIGIN` (dashboard only — the widget must stay embeddable), `Referrer-Policy: strict-origin-when-cross-origin`, `X-Content-Type-Options: nosniff`, and a dashboard CSP that allows only self + the font route.

`--- TASK 27 DONE ---`

---

# TASK 28 — SMTP / EMAIL CHANNEL

- `internal/integrations/smtp/client.go` using `net/smtp` + `crypto/tls` (no dependency). Supports plain/STARTTLS/implicit TLS, auth optional.
- Config block `smtp: {enabled, host, port, username, password, from_name, from_email, tls_mode, timeout}`; password via env.
- `services/notification.go` gains `EmailChannel`.
- Templates in `internal/i18n/email/` (Persian, RTL HTML + plain-text alternative), using Vazirmatn via `@font-face` with a **system-font fallback** (many mail clients ignore web fonts — the layout must survive Tahoma):
  - `new_conversation.html` — to the assigned/duty agent
  - `missed_conversation.html` — nobody answered within N minutes
  - `daily_digest.html` — counts, CSAT, busiest hours
  - `password_reset.html`
  - `csat_received.html`
- Password reset flow: `POST /api/v1/auth/forgot` → single-use token, 30-minute TTL, stored hashed; `POST /api/v1/auth/reset`. Dashboard pages for both. Rate-limited 3/hour per email. Always returns 200 regardless of whether the email exists.

`--- TASK 28 DONE ---`

---

# TASK 29 — DOCUMENTATION & README

## 29.1 `README.fa.md` — the primary document for the target audience
Full Persian, RTL-aware markdown, structured as:
1. یک پاراگراف معرفی + بج‌ها (release, license, CI status, image size)
2. اسکرین‌شات‌ها (see 29.3)
3. **نصب در ۶۰ ثانیه** — the `curl | bash` line first, Docker second, build-from-source last
4. راه‌اندازی اولیه (ویزارد وب + ویزارد CLI, with screenshots)
5. افزودن ویجت به سایت (snippet + WordPress + EVE)
6. پیکربندی (همان جدول، فارسی)
7. تلگرام، EVE، GMweb، AI، ایمیل
8. پشتیبان‌گیری و بازیابی
9. به‌روزرسانی
10. عیب‌یابی — a table of the 15 most likely problems with symptom → cause → fix (port in use, 403 from domain mismatch, WS fails behind proxy, SSE buffered by nginx, clock skew breaking JWT, missing encryption key, telegram webhook not set, widget not appearing, CORS error, disk full, permission denied on data dir, font not loading, dashboard blank after update, SQLite locked, memory growth)
11. سؤالات متداول
12. مشارکت و لایسنس

## 29.2 `README.md` (English) — trimmed to: what it is, install, docs links, architecture, license. Link `README.fa.md` at the very top.

## 29.3 `docs/` and assets
- `docs/img/` — screenshots: inbox, widget open on a page, setup wizard steps 1/3/5, routing rules editor, Telegram relay in action, dashboard stats. Generate them **programmatically** with the existing `tools/ui-check` Playwright harness so they can be regenerated: add `scripts/screenshots.mjs` that seeds demo data, opens each page at 1440×900 and 390×844, and writes PNGs.
- `docs/img/demo.gif` — a 15-second capture: visitor sends a message → agent answers → Telegram relay. Produced by `scripts/record-demo.mjs` (Playwright video → ffmpeg → gif, ≤ 3MB).
- `docs/INSTALL.fa.md`, `docs/CONFIG.fa.md`, `docs/API.md` (full endpoint reference with request/response examples), `docs/WIDGET.md` (JS API reference), `docs/ROUTING.fa.md` (rule cookbook with 8 worked examples), `docs/TELEGRAM.fa.md`, `docs/UPGRADE.md`.
- `docs/openapi.yaml` — a complete OpenAPI 3.1 spec generated by hand from the route table; serve it at `/api/openapi.yaml` and mount a tiny Redoc/Scalar page at `/api/docs` (vendored, not CDN).

`--- TASK 29 DONE ---`

---

# TASK 30 — PROJECT GOVERNANCE & SUPPLY CHAIN

- `CHANGELOG.md` — Keep a Changelog format, seeded with `## [1.0.0] - 2026-10-18` describing TASK 01–17, and `## [Unreleased]`.
- `CONTRIBUTING.md` — dev setup, `make dev`, code standards (the non-negotiables from the first plan), commit convention (Conventional Commits), PR checklist.
- `SECURITY.md` — supported versions, private reporting instructions, 90-day disclosure, explicit statement that message content is encrypted at rest but the operator holds the key.
- `CODE_OF_CONDUCT.md` — Contributor Covenant 2.1.
- `.github/ISSUE_TEMPLATE/{bug_report.yml,feature_request.yml,question.yml}` — bilingual labels, required fields: version (`evechat version` output), install method, `evechat doctor --json` output.
- `.github/PULL_REQUEST_TEMPLATE.md`.
- `.github/dependabot.yml` — weekly for `gomod`, `npm` (both web dirs), `github-actions`, `docker`.
- `.github/workflows/codeql.yml` — Go + JavaScript.
- `.github/workflows/stale.yml` — 60 days.
- `NOTICE` — third-party attributions including **Vazirmatn (OFL-1.1)** and DejaVu/Roboto lineage.
- Tag `v1.0.0` instructions in `docs/RELEASING.md`: bump `CHANGELOG`, tag, push, verify the release workflow, smoke-test the published image.

`--- TASK 30 DONE ---`

---

# TASK 31 — OFFLINE / RESTRICTED-NETWORK INSTALL (Iran-specific)

This is a first-class scenario, not an afterthought.

1. **`make bundle`** → produces `evechat-offline-<version>-linux-amd64.tar.gz` containing: the static binary, `evechat.example.yaml`, `install-offline.sh`, the systemd unit, Caddyfile, fonts, `README.fa.md`, `checksums.txt`. Everything needed; nothing downloaded at install time.
2. **`install-offline.sh`** — same flow as `install.sh` but sourcing files from the extracted directory; no network calls at all. Must complete successfully on a machine with no route to the internet.
3. **Mirror support** — `install.sh --base-url https://mirror.example.ir/evechat` downloads release assets from any HTTP mirror; document how to host one with a single `nginx autoindex` directory.
4. **Build-time mirrors** documented in `CONTRIBUTING.md` and honoured by `scripts/dev.sh` if set:
   ```
   export GOPROXY=https://goproxy.cn,https://goproxy.io,direct
   export GOSUMDB=off          # only when the sum DB is unreachable; document the tradeoff honestly
   npm config set registry https://registry.npmmirror.com
   ```
5. **Docker image export/import** — `make image-bundle` → `docker save` to `evechat-<version>.image.tar.gz` plus `docker load` instructions, for hosts that cannot reach ghcr.io.
6. **No runtime dependency on any external host.** Audit and assert in CI: grep the built dashboard + widget + setup bundles for `http://` / `https://` literals and fail the build if anything outside the configured `apiUrl` appears (allow-list: the embed snippet example, documentation links). This is what guarantees the product keeps working during an internet disruption.

`--- TASK 31 DONE ---`

---

# TASK 32 — AUDIT LOG

Migration `007_audit.sql`:
```sql
CREATE TABLE IF NOT EXISTS audit_log (
    id         TEXT PRIMARY KEY,
    project_id TEXT,
    actor_type TEXT NOT NULL,   -- agent, system, api
    actor_id   TEXT,
    actor_name TEXT,
    action     TEXT NOT NULL,   -- agent.created, agent.deleted, rule.updated, visitor.banned,
                                -- conversation.exported, settings.changed, key.rotated, login.failed,
                                -- login.success, project.key_rotated, backup.created, setup.completed
    target_type TEXT,
    target_id   TEXT,
    ip          TEXT,
    user_agent  TEXT,
    detail      TEXT NOT NULL DEFAULT '{}',   -- JSON diff {before,after}, secrets redacted
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_audit_project_time ON audit_log(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_log(actor_id, created_at DESC);
```
- `services/audit.go` with `Record(ctx, entry)`, called from every mutating handler. Secrets (`password`, `*_key`, `*_secret`, `token`) are replaced with `"***"` before storing the diff.
- `GET /api/v1/audit?action=&actor=&from=&to=` (admin+), paginated, exportable to CSV.
- Dashboard page Settings → گزارش فعالیت with filters and a detail drawer showing the before/after diff.
- Retention: `audit.retention_days` (default 180), pruned by the hourly janitor.

`--- TASK 32 DONE ---`

---

# TASK 33 — DARK MODE & ACCESSIBILITY PASS

- Full dark theme driven by CSS custom properties on `:root[data-theme]`, honouring `prefers-color-scheme` with a manual override persisted in `localStorage`. Applies to dashboard, widget, and the setup wizard.
- Widget: expose `theme: "auto" | "light" | "dark"` and a `colors` override in `window.EveChat`; the dark palette must be derived from the configured brand color (compute contrast-safe variants at runtime, ~30 lines).
- Contrast: every text/background pair ≥ 4.5:1 (3:1 for ≥ 18.66px bold). Write `scripts/contrast-check.mjs` asserting this over the token set and run it in CI.
- Keyboard: full tab navigation in the widget and the dashboard inbox, visible focus rings (never `outline: none` without a replacement), `Esc` closes the widget panel and dashboard modals, focus trap inside open dialogs, focus returns to the trigger on close.
- Screen readers: `role="log"` + `aria-live="polite"` on the message list, `aria-label` on all icon-only buttons (Persian), `lang="fa"` + `dir="rtl"` on the shadow root, the launcher announced as «باز کردن گفتگوی پشتیبانی» with an unread count in `aria-label`.
- `prefers-reduced-motion`: disable the launcher pulse and panel slide animation.
- Zoom: layout must survive 200% browser zoom and a 320px-wide viewport without horizontal scroll.

`--- TASK 33 DONE ---`

---

# TASK 34 — END-TO-END INSTALL TEST IN CI

`.github/workflows/e2e.yml`, triggered on `push` to `main` and nightly:

1. **Fresh-install job:** spin up a container of `ubuntu:24.04`, copy the built release archive in, run `install.sh --base-url file:///artifacts --no-systemd`, start the binary, poll `/api/v1/health`, run `evechat setup --non-interactive --json`, then run `scripts/smoke.mjs` against it. Assert exit 0.
2. **Docker job:** `docker run` the built image with a temp volume, hit the web setup wizard through Playwright (fill all 5 steps), assert it lands on `/inbox` and that a second visit to `/setup` returns 404.
3. **Upgrade job:** install `v1.0.0` from the previous release, seed 50 conversations, run `evechat update --version <new>`, assert the data survived (counts match, a message from before still decrypts) and health is ok.
4. **Offline job:** build the offline bundle, run it in a container started with `--network none`, assert install + boot + dashboard HTML + font file all succeed.
5. **Backup/restore job:** seed data → backup → wipe → restore → assert counts and that messages decrypt.

Each job uploads logs on failure. A red E2E blocks the release workflow.

`--- TASK 34 DONE ---`

---

# PART 99 — FINAL ACCEPTANCE (run all of this before tagging v1.1.0)

```bash
# code health
CGO_ENABLED=0 go build ./... && go vet ./... && gofmt -l . && go test ./... -race

# budgets
gzip -c embed/widget/widget.js | wc -c          # < 12288
du -sb embed/assets/fonts                       # < 400000
ls -l evechat                                   # < 45MB

# wizards
./evechat setup --non-interactive --json | jq .
./evechat doctor --json | jq '.summary'
./evechat doctor --fix

# install paths (each on a clean VM/container)
curl -fsSL .../install.sh | bash                            # binary install
docker compose -f docker-compose.quick.yml up -d            # published image
tar xzf evechat-offline-*.tar.gz && ./install-offline.sh     # air-gapped

# web wizard
# open http://host:3000 -> 5 steps -> lands on /inbox ; revisit /setup -> 404

# font, offline
# disconnect network -> dashboard + widget render in Vazirmatn, zero external requests

# ops
./evechat backup /tmp/b --with-uploads && ./evechat restore /tmp/b.tar.gz --dry-run
./evechat update --check
curl -s localhost:3000/metrics | head -20
curl -s localhost:3000/api/v1/health | jq '.version, .uptime_seconds'

# abuse
# 100 rapid POSTs to /api/widget/v1/session without PoW -> all rejected
```

---

# PART 100 — WHAT YOU MUST NOT DO (still binding)

- No cobra, no viper, no prometheus/client_golang, no lumberjack. The only new Go dependency permitted in this pack is `golang.org/x/term` (TASK 20).
- Do not load fonts, scripts, styles or anything else from a CDN at runtime. Everything ships in the binary.
- Do not put secrets in YAML, in the DB unencrypted, in logs, in backups by default, or in audit diffs.
- Do not delete user data in any uninstall, purge, restore or update path without a typed confirmation.
- Do not exceed the widget size budget to add the font — the font is served separately, always.
- Do not declare `@font-face` inside the Shadow DOM.
- Do not break backwards compatibility of existing config keys or API routes.
- Do not ask the user anything. Build it.

**BEGIN WITH TASK 18 NOW. Output files only.**
