# Changelog

All notable changes to this project are documented here.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) ·
Versioning: [SemVer](https://semver.org/).

## [Unreleased]

### Planned
- Redis broker behind the `redis` build tag for multi-node deployments.
- Billing provider adapters (interface only today).
- Realtime translation of agent messages.


## [1.2.5] - 2026-10-19

### Added
- **Channel outbound headers and continuation window.** Each channel accepts up
  to 20 custom outbound headers (`X-API-Key`, `X-Tenant`, …) and a
  `session_ttl_seconds` value (default 24h, `0` = always a new conversation).
  Sensitive values are returned as `••••••`, never logged or audited, and the
  edit form sends the mask back so the stored value survives a save.
- **Channel test delivery inspector.** «ارسال آزمایشی» sends a real callback
  with the configured headers and records an `OUTBOUND TEST` row (masked
  headers + status) that the inspector expands.
- **SMTP card.** `/api/v1/integrations/status` now reports
  `smtp.configured`, and the integrations page shows the email card.
- **Exact quick-start count.** `overview.agent_messages` counts agent-authored
  messages, so step 3 no longer approximates with “conversations and messages”.

### Changed
- **PoW is cooperative and lazy.** The widget solves the challenge only on the
  first panel open (or explicit send), yields every 8ms, and gives up softly
  after 15s with «در حال آمادهسازی…» and a tap-to-retry banner. The host page
  stays interactive; the bundle remains under the 16,384-byte gzip cap.
- E2E keeps PoW enabled at difficulty 14 and asserts no main-thread task over
  50ms; loopback outbound is allowlisted for the channel test.

### Fixed
- **BUG-4 detail fetch storm.** `conversation.updated` now patches the open
  conversation cache instead of invalidating it; a live message no longer
  refetches the panel (`behavior.spec.ts` test 9).
- **BUG-1 regression coverage.** `behavior.spec.ts` test 8 asserts the same
  conversation id after a reload.

## [1.2.4] - 2026-10-19

> **هشدار امنیتی:** رفع جعل IP؛ نسخه‌های 1.2.2 و 1.2.3 آسیب‌پذیرند و ارتقا توصیه می‌شود.

### Added
- **Proxy trust is explicit.** `server.trusted_proxies` is a CIDR list
  (loopback by default). Forwarding headers are read only when the socket peer
  is in that list; `X-Forwarded-For` is walked from the right and trusted hops
  are skipped (`internal/middleware/clientip_test.go`).
- **Five more doctor checks** — `net.ports`, `net.bind`, `net.https`,
  `net.proxy` and `widget.domains` — for 29 checks in total, with unit tests.
- **Deployment beside EVE:** `install.sh --behind-proxy --domain …
  --write-nginx` (nginx `-t` before any reload; the file is removed or restored
  on failure), a preflight that suggests a free port and refuses to install
  Caddy when 80/443 are busy, and `docs/DEPLOY_WITH_EVE.fa.md`.
- **UX writing catalogue:** `web/dashboard/src/lib/copy.fa.ts` plus
  `PageHeader`, `EmptyState` (cta required), `HelpTip`, `FieldHelp` and
  `Callout`; every page now has an intro, a three-part empty state and field
  help. The sidebar label «کاربران» is now «مشتریان» (URL unchanged). A
  four-step quick-start card is driven by real data, and the nine webhook
  events use the agreed Persian labels. `scripts/check-copy.mjs` blocks Persian
  text outside the catalogue and enforces intro length/empty-state rules in CI.
- **Wizard polish:** the setup-token gate for non-loopback callers, the
  TASK 45 copy in the web and CLI wizards, `scripts/fresh.ps1` / `make fresh`
  for a throwaway instance, and `wizard.spec.ts` (loopback, wrong/correct
  token, 375/1440px, 44px targets, post-completion 404) in CI.

### Changed
- `server.trusted_proxies` replaces the 1.2.3 loopback/private heuristic. The
  nginx, Caddy and Traefik examples overwrite `X-Forwarded-For`; setting
  `trust_proxy: true` on a public interface now warns in `doctor`.
- Go 1.26 is the toolchain in `go.mod`, the Dockerfile, CI and the release
  matrix. `golang.org/x/image`, `go-viper/mapstructure` and `golang-jwt/jwt`
  were updated; `govulncheck` now reports zero called vulnerabilities.

### Fixed
- A client could spoof its own IP (and bypass rate limits, the IP blocklist and
  the setup gate) by sending `X-Forwarded-For`; forwarding headers are now only
  honoured from a trusted proxy.
- The web wizard works on a remote server without a terminal: the setup token
  can be entered in the wizard, with the Persian error from the plan.


## [1.2.3] - 2026-10-19

### Fixed
- Client IP resolution no longer takes the left-most `X-Forwarded-For` entry;
  the initial fix required a loopback/private peer and walked the header from
  the right. Superseded by `server.trusted_proxies` in 1.2.4.


## [1.2.2] - 2026-10-19

### Added
- Webhook event catalogue API, nine events with Persian labels, and a guided
  empty state on the webhooks page.

### Fixed
- The release workflow YAML (duplicate `name` keys) was repaired and
  actionlint runs in CI.


## [1.2.1] - 2026-10-19

### Changed
- Widget budget is now a single source of truth (`scripts/widget-budget.txt`,
  16,384 bytes gzip); the release workflow enforces it without
  `continue-on-error`, and every stale 12,288 reference in the docs was updated.
- Channel inbound docs and OpenAPI use the id-based route
  `POST /api/channels/{id}/inbound` (slug is only unique per project).
- `evechat version` falls back to `debug.ReadBuildInfo` VCS metadata, so a plain
  `go build` reports `dev-<commit>[-dirty]` instead of `dev (none, unknown)`.
## [1.2.0] - 2026-10-18

### Added
- **Channels (two-way webhooks)**: `channels`, `channel_identities` and
  `webhook_inbox`; ten REST routes plus the public
  `POST /api/channels/{id}/inbound`; JSONPath input mapping with a live
  preview; ten-step ingestion; outbound acks on the `0s/30s/2m/10m/1h/6h`
  ladder with a system message when it gives up; a channels page with a curl
  generator and an inspector; bodies are scrubbed and kept 48 hours
  (`docs/CHANNELS.fa.md`, `docs/WEBHOOKS.fa.md`).
- **Responsive pass**: `breakpoints.css` (360/480/768/1024/1440) as the single
  source of truth; the widget handles the mobile keyboard with
  `visualViewport`, `dvh`, safe-area insets and body scroll lock; the dashboard
  gets an off-canvas sidebar, a single-column inbox, table→cards and
  kanban→tabs; the setup wizard gets a mobile pass (`docs/RESPONSIVE.md`).
- **/wallboard**: read-only big screen behind a rotating project token with an
  aggregates-only payload (`docs/WALLBOARD.fa.md`).
- **Demo pages**: `/demo`, `/demo/test`, `/demo/split`, `/demo/devices`,
  `/demo/events`, local-only, with a control bar, network simulations and a
  hostile-CSS isolation test (`docs/DEMO.fa.md`).
- **Tests**: 46 Playwright tests (behaviour + responsive + demo) in CI; Go tests
  for channels, JSONPath, claim/activation, resume, SSRF, audit redaction, PII
  masking, backup/restore decryption and update rollback.

### Fixed
- **Security**: `chimw.RealIP` trusted `X-Forwarded-For` for every caller, which
  let anyone choose their own IP (rate limits, IP blocklist, the setup wizard's
  loopback check, `/demo*`). Removed; `middleware.ClientIP` honours proxy
  headers only with `trust_proxy`, first hop only.
- **BUG-1**: a resolved conversation swallowed the next visitor message, and
  `visitor_sessions.active_conv_id` was never set for WebSocket-created
  conversations.
- **BUG-2**: assignment and activation were the same thing, so routing's
  capacity fallback moved every new conversation to `active`. `Assign` no
  longer activates; new atomic `claim` route; the first agent reply activates.
- **BUG-3/4**: agents were not in the room of a pending conversation, so live
  messages and typing indicators never reached the dashboard; typing now has a
  server-side TTL; the dashboard patches the query cache instead of refetching
  on every frame and no longer refetches on window focus.
- **BUG-0**: the widget survives blocked `localStorage`/`indexedDB` with an
  in-memory fallback; the manual test page moved to `/demo/test`.
- The widget panel rendered **transparent** when a project palette omitted a
  colour (an explicit `undefined` overrode the defaults).
- **doctor**: the missing 24th check was `net.outbound`; `net.publicurl` now
  compares the health `instance_id` with the local installation.

### Changed
- Widget gzip budget raised from 12 KB to 16 KB. The 12 KB figure was arbitrary;
  the widget ships three transports, an offline outbox, a shadow-DOM UI, two
  locales and now keyboard/responsive handling. Terser was tightened first
  (`compact` output, extra compress passes) which freed 52 bytes. The cap is
  enforced — only the number changed — in the Makefile, `scripts/build.sh`, CI
  and `doctor`.


## [1.1.3] - 2026-10-18

### Fixed
- Release matrix builds linux/armv7 through `GOARCH=arm` + `GOARM=7` (Go rejects
  `GOARCH=armv7`) and exports the variable instead of prefixing the command, so
  the archive now contains a 5.8 MB binary instead of metadata only.
- Removed the `|| true` that silently published an empty armv7 artifact.

## [1.1.2] - 2026-10-18

### Fixed
- `install-offline.sh` and the operational shell scripts are executable in the
  repository, so the offline tarball can run the installer directly.
- E2E reads `evechat doctor --json` instead of piping it, because doctor exits 2
  when it reports warnings even with zero failures.
- Offline E2E installs under sudo, matching the installer contract.

## [1.1.1] - 2026-10-18

### Fixed
- Dockerfile cross-compiles with `TARGETOS`/`TARGETARCH`, so the arm64 image
  contains an arm64 binary, and builds the setup wizard in the image.
- Flags written after positional arguments are honoured (`evechat backup /path
  --with-uploads`, `evechat user create --email …`).
- `install.sh` restarts the service after the CLI wizard so the cached setup gate
  does not leave a fresh install on `/setup`.

### Added
- Scheduled email jobs: `evechat job missed|digest` with systemd timers.
- Generic audit hook records every authenticated mutation.
## [1.1.0] - 2026-10-18

Phase 2: install, release, operations, typography and hardening.

### Added
- **Release pipeline**: CI (build, vet, race tests, govulncheck, gofmt, widget
  budget), tag-driven release workflow with six platform archives, checksums,
  GitHub Release and a multi-arch image at `ghcr.io/aibedini/evechat`.
- **One-line installer** (`install.sh`) with checksum verification, mirror
  support (`--base-url`), systemd unit, secret generation, health polling,
  non-interactive setup and safe uninstall (`--purge` needs a typed confirmation).
- **`install.ps1`** for Windows and `docker-compose.quick.yml` for the
  zero-build image path.
- **CLI subcommands**: `serve`, `setup`, `doctor`, `migrate`, `backup`,
  `restore`, `update`, `user`, `project`, `key`, `healthcheck`, `version`.
- **Interactive setup wizard** (`evechat setup`) with arrow-key menus, live
  integration probes and an idempotent non-interactive mode (`--json`).
- **Web first-run wizard** at `/setup` (separate bundle, five steps, RTL,
  anti-hijack token for remote installs).
- **`evechat doctor`** with 24 checks, Persian fix hints, `--json`, `--verbose`,
  `--only`, `--fix`, plus a dashboard health page.
- **Backup/restore/update**: WAL-safe `VACUUM INTO` backups with manifests and
  optional uploads, checksum-verified restores with a pre-restore copy, and an
  updater with automatic rollback.
- **Vazirmatn typography** self-hosted (variable + static woff2, OFL notice),
  served from `/assets/fonts/*` and injected into the widget on first open.
- **Prometheus metrics** at `/metrics` (hand-written exposition, token or
  loopback protection) with Prometheus/Grafana examples.
- **Abuse protection**: proof-of-work challenges with adaptive difficulty,
  honeypot field, per-conversation and per-visitor throttles, visitor
  reputation, runtime IP/CIDR/UA blocklists, upload hardening and security
  headers.
- **SMTP channel** with Persian RTL templates and a password-reset flow
  (`/forgot`, `/reset`).
- **Audit log** (`audit_log`, `/api/v1/audit`, dashboard activity page) with
  secret redaction and retention pruning; every authenticated mutation is
  audited automatically through an auth-middleware hook.
- **Dark mode**, keyboard/focus/screen-reader pass, reduced-motion support and a
  contrast budget check in CI.
- **Scheduled email jobs**: `evechat job missed|digest` with systemd timers for
  unanswered-chat reminders and a daily support summary.
- **Offline bundle** (`make bundle` + `install-offline.sh`), mirrored installs
  and an external-URL audit that fails the build if any CDN dependency appears.
- **Documentation**: Persian README, install/config/API/widget/routing/telegram/
  upgrade guides, OpenAPI 3.1 description and `/api/docs`.
- **Governance**: CONTRIBUTING, SECURITY, CODE_OF_CONDUCT, NOTICE, issue
  templates, PR template, Dependabot, CodeQL and stale bot.

### Changed
- `--config` is accepted by every subcommand; `<dir>/.env` is loaded
  automatically so systemd/CLI behaviour matches.
- Version metadata (`version`, `commit`, `build_date`, Go version) is exposed by
  `evechat version` and `/api/v1/health`.
- Widget now ships with PoW solving, a lazily injected font and theme support,
  while staying under the 12 KB gzip budget (11.3 KB).

### Security
- Widget session creation requires a proof-of-work solution (configurable,
  `security.pow_difficulty`, 0 disables).
- Message content stays AES-256-GCM encrypted at rest; audit diffs redact
  passwords, keys, tokens and secrets.

## [1.0.0] - 2026-10-18

Initial implementation of `EVECHAT_BUILD_PLAN.md` TASK 01–17.

### Added
- Go 1.23 backend: chi router, `coder/websocket` hub with rooms and ring-buffer
  replay, SQLite WAL with writer/reader pools, embedded migrations, ULID ids.
- AES-256-GCM message encryption with key versions and HMAC-signed webhooks.
- Services: visitors, routing rules with a condition DSL, conversations,
  idempotent messages, notifications, tickets, FTS5 knowledge base, AI answers
  from KB, webhooks with retry, CSAT scheduling.
- Integrations: Telegram relay (webhook, reply correlation, inline keyboard),
  EVE user lookup, GMweb SMS.
- Widget: vanilla TypeScript, Shadow DOM, WS → SSE → long-poll fallback,
  IndexedDB outbox, Persian RTL UI, < 12 KB gzip.
- Dashboard: React 18 + Vite + Tailwind, RTL Persian, Jalali dates, inbox,
  visitors, tickets Kanban, knowledge base and settings pages.
- Dockerfile (distroless), compose, Makefile, scripts, WordPress plugin.
- Tests: crypto, routing (23 cases), repo dedup, hub concurrency, widget
  transport state machine.