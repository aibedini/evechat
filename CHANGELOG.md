# Changelog

All notable changes to this project are documented here.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) ·
Versioning: [SemVer](https://semver.org/).

## [Unreleased]

### Added
- **Channels (two-way webhooks)**: `channels`, `channel_identities` and
  `webhook_inbox` tables; ten REST routes plus the public
  `POST /api/channels/{id}/inbound`; JSONPath input mapping with a live
  preview; ten-step ingestion (secret, rate limit, parse, map, identity,
  conversation, encrypted message, room broadcast, outbound ack, inspector);
  outbound delivery with the `0s/30s/2m/10m/1h/6h` ladder and a system message
  when it gives up; a channels page with the curl generator and inbox
  inspector. Bodies are scrubbed and kept 48 hours. See `docs/CHANNELS.fa.md`.
- **Behaviour suite**: `web/e2e` with seven Playwright scenarios covering
  BUG-0…BUG-4, run by the `behavior` job in `.github/workflows/e2e.yml`.

### Changed
- Widget gzip budget raised from 12 KB to 16 KB. The 12 KB figure came from the
  first plan and was arbitrary: the widget now ships three transports, an
  offline outbox, a shadow-DOM UI and two locales, and the responsiveness work
  (visualViewport keyboard handling, safe-area insets) needs several hundred
  more bytes. The build, the Makefile, the release workflow and the
  `doctor` widget.size check all enforce 16384; the assert is not relaxed,
  only the number. Terser was tightened in the same change (`compact` output,
  extra compress passes) which freed 52 bytes of headroom.

### Planned
- Redis broker behind the `redis` build tag for multi-node deployments.
- Billing provider adapters (interface only today).
- Realtime translation of agent messages.

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