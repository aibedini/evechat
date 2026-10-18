# ⚡ EveChat

**Persian-first, self-hosted customer support chat.** One static Go binary, embedded SQLite, an embeddable widget and an RTL dashboard.

🇮🇷 [مستندات کامل فارسی](README.fa.md) · 📦 [Releases](https://github.com/aibedini/evechat/releases) · 📖 [API](docs/API.md) · 🔒 [Security](SECURITY.md)

```bash
curl -fsSL https://raw.githubusercontent.com/aibedini/evechat/main/install.sh | bash
```

The installer verifies checksums, creates a system user, generates secrets, installs a hardened systemd unit and prints the dashboard URL with credentials. Open the URL and the five-step web wizard finishes the setup.

## What it is

- **Backend:** Go 1.26, chi, coder/websocket, SQLite WAL (CGO-free), AES-256-GCM message encryption, ULIDs, FTS5 knowledge base.
- **Widget:** vanilla TypeScript, Shadow DOM, WebSocket → SSE → long-poll fallback, IndexedDB outbox, 12.2 KB gzip (budget 16 KB), self-hosted Vazirmatn injected on first open.
- **Dashboard:** React 18 + Vite + Tailwind, Persian RTL, Jalali dates, realtime inbox, visitors, tickets Kanban, knowledge base, audit/health/security settings, dark mode.
- **Ops:** `evechat setup|doctor|backup|restore|update|key|user|project`, Prometheus metrics, proof-of-work abuse protection, SMTP + password reset, audit log, offline install bundle, multi-arch container image.

## Install paths

| Method | Command |
|---|---|
| Binary (Linux/macOS) | `curl -fsSL …/install.sh \| bash` |
| Windows | `irm …/install.ps1 \| iex` |
| Docker | `docker compose -f docker-compose.quick.yml up -d` |
| Air-gapped | `make bundle`, then `./install-offline.sh` |
| From source | `make build` |

## After install

```bash
evechat doctor --verbose          # 24 checks with Persian fix hints
evechat backup /backup/now --with-uploads
evechat update --check
curl -s localhost:3000/metrics | head
```

## Documentation

- [docs/INSTALL.fa.md](docs/INSTALL.fa.md) — نصب، سرویس، TLS، آفلاین
- [docs/CONFIG.fa.md](docs/CONFIG.fa.md) — همه کلیدهای پیکربندی
- [docs/API.md](docs/API.md) — REST/WS endpoints
- [docs/WIDGET.md](docs/WIDGET.md) — JS API and events
- [docs/ROUTING.fa.md](docs/ROUTING.fa.md) — rule cookbook
- [docs/TELEGRAM.fa.md](docs/TELEGRAM.fa.md) — bot setup
- [docs/UPGRADE.md](docs/UPGRADE.md) and [docs/RELEASING.md](docs/RELEASING.md)
- [docs/AUDIT.md](docs/AUDIT.md), [docs/MONITORING.md](docs/MONITORING.md), [docs/EMAIL.md](docs/EMAIL.md)
- [docs/OFFLINE.md](docs/OFFLINE.md), [docs/ACCESSIBILITY.md](docs/ACCESSIBILITY.md), [docs/SECURITY-HARDENING.md](docs/SECURITY-HARDENING.md)

OpenAPI: served at `/api/openapi.yaml`, browsable at `/api/docs`.

## Architecture

```
widget / dashboard / REST / SSE
             │
         chi router ── middleware: request id, recover, CORS, API key, JWT, rate limit,
             │                    security headers, blocklist, setup gate, metrics
         handlers ── services (visitor, conversation, routing, message, notification,
             │                  ticket, search, webhook, ai, csat, audit, email)
             │
      hub (one goroutine, command channel)     repositories (hand-written SQL)
             │                                        │
         websockets                            SQLite WAL (writer + reader pools)
```

## Tests

```bash
go test ./... -race
cd web/widget && npx vitest run
node scripts/smoke.mjs http://127.0.0.1:3000 pk_... admin@evechat.local <password>
k6 run scripts/load_ws.js
```

## License

MIT — see [LICENSE](LICENSE). Vazirmatn is SIL OFL 1.1, see [NOTICE](NOTICE).
