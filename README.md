# ⚡ EveChat

Self-hosted, real-time customer-support chat for the EVE ecosystem.
One static Go binary, embedded SQLite, an embeddable widget and a Persian (RTL) dashboard.

- **Backend:** Go 1.23, `chi`, `coder/websocket`, `modernc.org/sqlite` (CGO-free), ULIDs
- **Widget:** vanilla TypeScript, Shadow DOM, < 12 KB gzip, WS → SSE → long-poll fallback, IndexedDB outbox
- **Dashboard:** React 18 + Vite + TypeScript + Tailwind, fa-IR / RTL, TanStack Query
- **Storage:** SQLite WAL, AES-256-GCM message encryption, HMAC-signed webhooks
- **Idle footprint:** < 20 MB; 1000 websockets < 60 MB

---

## Quick start

```bash
git clone https://github.com/<you>/evechat.git
cd evechat

# 1. secrets (never stored in YAML)
cp evechat.example.yaml evechat.yaml
export EVECHAT_AUTH__JWT_SECRET="$(openssl rand -base64 48)"
export EVECHAT_ENCRYPTION_KEY_V1="$(openssl rand -base64 32)"

# 2. run (migrations + demo project + superadmin)
go build -o evechat ./cmd/evechat
./evechat --config evechat.yaml --seed
```

The seed prints the dashboard URL, the superadmin password, the project `api_key`
and a ready-to-paste embed snippet. The dashboard is served at `/`
(the inbox lives at `/inbox`).

### Docker

```bash
export EVECHAT_AUTH__JWT_SECRET=... EVECHAT_ENCRYPTION_KEY_V1=...
docker compose up -d --build
```

The image is distroless (no shell, non-root) and ships its own
`docker compose`-compatible healthcheck through `evechat --healthcheck`.

### Full build from source

```bash
make build            # widget + dashboard + binary, fails if the widget exceeds 12 KB gzip
make test             # Go test suite
cd web/widget && npx vitest run   # transport state-machine tests
```

---

## Embedding the widget

```html
<script>
  window.EveChat = {
    projectKey: "pk_...",
    apiUrl: "https://chat.example.com",
    visitor: {
      name: "مریم",
      mobile: "09121234567",
      metadata: { eve_user_id: "u_123", eve_reseller_id: "reseller_abc" },
      subscription: { plan: "pro", expiry: "2027-01-01" }
    },
    locale: "fa",
    position: "bottom-left",
    autoOpen: false
  };
</script>
<script async src="https://chat.example.com/widget.js"></script>
```

Queued-command API (works before the bundle loads):

```js
window.ec = window.ec || function () { (ec.q = ec.q || []).push(arguments); };
ec("identify", { name: "علی", mobile: "09120000000" });
ec("setVariable", "utm_source", "google");
ec("open");
ec("on", "message", function (msg) { console.log(msg); });
```

---

## Configuration

Precedence: **defaults → `evechat.yaml` → environment**.
Environment variables use the `EVECHAT_` prefix and `__` for nesting:
`EVECHAT_SERVER__PORT=3000`, `EVECHAT_TELEGRAM__BOT_TOKEN=...`.

| Key | Default | Description |
|---|---|---|
| `server.host` / `server.port` | `0.0.0.0` / `3000` | listen address |
| `server.public_url` | `http://localhost:3000` | dashboard origin, embed URLs, CORS allow-list |
| `server.trust_proxy` | `false` | honour `X-Forwarded-For` (set true behind nginx/Caddy) |
| `server.tls.enabled` | `false` | serve TLS directly (cert/key) |
| `database.path` | `/data/evechat.db` | SQLite file |
| `database.max_read_conns` | `4` | WAL reader pool size |
| `storage.uploads_dir` | `/data/uploads` | attachments root |
| `storage.max_file_size_mb` | `10` | upload limit |
| `auth.jwt_secret` | — | **required**, ≥ 24 chars, set via env |
| `auth.access_expiry` / `refresh_expiry` | `24h` / `720h` | JWT lifetimes |
| `encryption.current_version` | `1` | active AES key version |
| `hub.ping_interval_seconds` | `30` | websocket ping cadence |
| `hub.pong_wait_seconds` | `90` | dead-socket timeout |
| `hub.ring_buffer_size` | `50` | per-conversation replay ring |
| `hub.max_message_bytes` | `32768` | frame limit |
| `ratelimit.*` | 10 / 60 / 5 | widget sessions, messages, logins per minute |
| `telegram.enabled` | `false` | relay bot |
| `integrations.eve.*` | — | EVE user lookup + signed context |
| `integrations.gmweb.*` | — | SMS gateway |
| `ai.enabled` | `false` | knowledge-base assistant |
| `log.level` / `log.pretty` | `info` / `false` | zerolog |

Encryption keys are read **only** from the environment:
`EVECHAT_ENCRYPTION_KEY_V1` … `EVECHAT_ENCRYPTION_KEY_V9`, each base64 of 32 bytes.

---

## Telegram relay

1. Create a bot with [@BotFather](https://t.me/BotFather) and copy the token.
2. Set `EVECHAT_TELEGRAM__BOT_TOKEN`, `EVECHAT_TELEGRAM__ENABLED=true`,
   `EVECHAT_TELEGRAM__WEBHOOK_URL=https://chat.example.com/api/v1/integrations/telegram/webhook`
   and a random `EVECHAT_TELEGRAM__SECRET`.
3. Restart — the server registers the webhook automatically (or call
   `POST /api/v1/integrations/telegram/set-webhook`).
4. Add your numeric Telegram id to an agent in **Settings → Team** to receive
   routed chats and to use **Send test**.

Replies inside Telegram (`Reply`) are posted back into the conversation; the
inline keyboard closes or escalates the chat. Outbound messages are capped at
25/sec and HTTP 429 `retry_after` is honoured.

---

## Reverse proxy (nginx)

```nginx
server {
    listen 443 ssl http2;
    server_name chat.example.com;
    # ssl_certificate ...;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 3600s;
        proxy_buffering off;   # required for SSE
    }
}
```

Set `server.trust_proxy: true` so rate limiting sees the real client IP.

---

## Backup & restore

```bash
# online, consistent backup (WAL safe)
sqlite3 /data/evechat.db ".backup '/backup/evechat-$(date +%F).db'"
# or copy after a checkpoint
sqlite3 /data/evechat.db "PRAGMA wal_checkpoint(TRUNCATE);"
cp /data/evechat.db /backup/
```

Restore by stopping the container, replacing `evechat.db` (delete `-wal`/`-shm`)
and starting again. Uploads live in `storage.uploads_dir` and must be copied too.

---

## Key rotation runbook

1. Generate a new key: `openssl rand -base64 32`.
2. Export it as `EVECHAT_ENCRYPTION_KEY_V2` **in addition** to the old `V1`.
3. Deploy. New messages are written with version 2; old ones still decrypt with V1.
4. Re-encrypt historical rows with a small maintenance job using
   `crypto.Service.ReEncrypt` (messages store `key_version` per row).
5. Only after every row reports version 2, drop `EVECHAT_ENCRYPTION_KEY_V1`.
6. Never delete a key while rows still reference its version — they become unreadable.

---

## Architecture

```
widget / dashboard / REST / SSE
            │
        chi router ── middleware: request id, recover, CORS, API key, JWT, rate limit
            │
        handlers ── services (visitor, conversation, routing, message, notification,
            │                  ticket, search, webhook, ai, csat)
            │
     hub (one goroutine, command channel)     repositories (hand-written SQL)
            │                                        │
        websockets                            SQLite WAL (writer + reader pools)
```

Non-negotiables baked into the code:

- the hub owns all connection state; every mutation goes through `cmds chan command`
- slow clients are dropped, never allowed to block the hub
- messages are encrypted at rest; `conversations.last_message_at` drives ordering
- `client_message_id` is a unique per-conversation index → idempotent retries
- webhooks are signed `X-EveChat-Signature: sha256=<hex>` over `<ts>.<body>`

## API surface (short)

```
POST /api/widget/v1/session            GET  /api/widget/v1/config
WS   /ws/visitor?token=                GET  /api/widget/v1/events (SSE)
GET  /api/widget/v1/poll               GET  /api/widget/v1/conversations/:id/sync
POST /api/widget/v1/conversations/:id/messages

POST /api/v1/auth/login | refresh | logout       GET /api/v1/auth/me
WS   /ws/agent?token=
GET  /api/v1/conversations[/:id[/messages]]
POST /api/v1/conversations/:id/{assign,transfer,resolve,reopen,convert-ticket,note,read}
GET  /api/v1/visitors[/:id[/conversations|/export]]
POST /api/v1/visitors/:id/{ban,anonymize}
CRUD /api/v1/agents, /api/v1/routing-rules, /api/v1/canned-responses,
     /api/v1/knowledge-base, /api/v1/tickets, /api/v1/webhooks
GET  /api/v1/stats/{overview,series,agents}   GET /api/v1/health
POST /api/v1/upload                          GET /api/v1/search/messages
GET  /api/v1/integrations/status
```

## Repository layout

```
cmd/evechat            entrypoint (flags, wiring, graceful shutdown)
internal/hub           websocket hub, rooms, ring buffers, protocol
internal/services      business logic + realtime handler
internal/repo          hand-written SQL repositories
internal/db            pools, migrations (embedded)
internal/handlers      HTTP/WS handlers
internal/router        route table + SPA
web/widget             embeddable widget (rollup)
web/dashboard          agent dashboard (vite)
embed                  compiled bundles, embedded into the binary
wordpress/evechat      WordPress plugin
scripts                build/dev/smoke/load/seed helpers
```

## Testing

```bash
go test ./...                       # crypto, routing (20 cases), repo dedup, hub concurrency
cd web/widget && npx vitest run     # transport state machine (8 cases)
node scripts/smoke.mjs http://127.0.0.1:3000 pk_... admin@evechat.local <password>
k6 run scripts/load_ws.js           # 1000 concurrent websockets
```

## License

MIT — see [LICENSE](LICENSE).
