# Contributing to EveChat

Thanks for helping! This project values small binaries, predictable behaviour and
calm operations over clever abstractions.

## Development setup

```bash
git clone https://github.com/aibedini/evechat.git
cd evechat

cp evechat.example.yaml evechat.yaml
export EVECHAT_AUTH__JWT_SECRET="$(openssl rand -base64 48)"
export EVECHAT_ENCRYPTION_KEY_V1="$(openssl rand -base64 32)"

make dev          # Go API on :3000, dashboard on :5173, widget watcher
make test         # Go tests
make test-widget  # vitest
```

Go 1.26+, Node 22+ and npm are required. Docker is optional.

## Non-negotiables

1. `CGO_ENABLED=0 go build ./...`, `go vet ./...` and `go test ./...` must pass.
2. The widget stays **under 16,384 bytes gzip**; the font is never bundled.
3. No new runtime dependency on a CDN, font host or third-party API.
4. Secrets never go into YAML, logs, backups by default, or audit diffs.
5. The hub goroutine never blocks and never takes a mutex.
6. `@font-face` is never declared inside a shadow root.
7. User data is never deleted without a typed confirmation.
8. Persian strings live in `internal/i18n`, `web/*/src/i18n.ts` or
   `internal/i18n/email`, never inline in logic.

## Code style

- `gofmt` clean; comments in English; Persian only in user-facing strings.
- Every IO function takes `context.Context` first and wraps errors with `%w`.
- Handlers return `error`; `handlers.H` converts them to HTTP.
- Prefer explicit SQL over an ORM; reads use `db.R`, writes use `db.W`.
- Front-end: TypeScript strict for the setup wizard and dashboard; the widget
  uses a relaxed config to keep the bundle small.

## Commits

Conventional Commits: `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`,
`perf:`, `build:`, `ci:`. Scope where useful, e.g. `fix(widget): …`.

## Pull requests

- One logical change per PR; include tests when behaviour changes.
- Update `CHANGELOG.md` under `## [Unreleased]`.
- Paste `evechat doctor --json` output for operational changes.
- Make sure `make build` passes locally (it enforces all size budgets).

## Restricted networks

```bash
export GOPROXY=https://goproxy.cn,https://goproxy.io,direct
export GOSUMDB=off            # only if sum.golang.org is unreachable; this
                              # removes checksum verification for new modules
npm config set registry https://registry.npmmirror.com
```

Prefer `scripts/fetch-fonts.sh --mirror <url>` over npm when the registry is
blocked; it verifies pinned SHA-256 sums.

## Security

Please report vulnerabilities privately — see [SECURITY.md](SECURITY.md).
