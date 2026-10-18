# Changelog

All notable changes to this project are documented here.
Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) ·
Versioning: [SemVer](https://semver.org/).

## [Unreleased]

### Planned
- Redis broker behind the `redis` build tag for multi-node deployments.
- Billing provider adapters (interface only today).
- Realtime translation of agent messages.

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
