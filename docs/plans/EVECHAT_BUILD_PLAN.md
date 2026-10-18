# EveChat — Complete Execution Plan (for DeepSeek)

> **READ THIS HEADER FIRST. IT IS NOT OPTIONAL.**
>
> You are an implementation engine, not a design partner.
> This document already contains every architectural decision.
> **Do not propose alternatives. Do not ask questions. Do not write explanations.**
> Produce **code files only**, in the exact order given, with the exact paths given.
>
> **Output format for every single file:**
> ```
> === FILE: <exact/relative/path> ===
> <complete file content, no truncation, no "// ... rest of code">
> ```
>
> **Hard rules:**
> 1. Never write `// TODO`, `// implement later`, `...`, or placeholder bodies. Every function is fully implemented.
> 2. Never ask "should I...?" — the answer is always: do what this document says.
> 3. If this document is silent on a micro-detail, pick the simplest idiomatic Go solution and move on. Do not stop.
> 4. Complete one TASK block fully before starting the next. At the end of each TASK, output only the line `--- TASK NN DONE ---` and continue immediately to the next TASK.
> 5. All Go code must compile with `CGO_ENABLED=0 go build ./...`. No CGO. Ever.
> 6. Comments in code: English only, short, only where logic is non-obvious.
> 7. Persian (fa-IR) is the default UI language for widget + dashboard, with RTL layout.

---

# PART 0 — PROJECT IDENTITY

| Key | Value |
|-----|-------|
| Project name | `evechat` |
| Go module path | `github.com/evechat/evechat` |
| Go version | `1.23` |
| Binary name | `evechat` |
| Default port | `3000` |
| DB file | `/data/evechat.db` |
| Target RAM idle | `< 20MB` |
| Target RAM @1000 WS | `< 60MB` |
| CGO | **disabled, permanently** |
| Build target | single static binary, Linux amd64 + arm64 |

---

# PART 1 — LOCKED DEPENDENCY LIST

Write `go.mod` with **exactly** these, no others. Do not add an ORM. Do not add Redis in Phase 1–2. Do not add a message queue.

```
module github.com/evechat/evechat

go 1.23

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/go-chi/cors v1.2.1
	github.com/coder/websocket v1.8.12
	modernc.org/sqlite v1.33.1
	github.com/knadh/koanf/v2 v2.1.1
	github.com/knadh/koanf/parsers/yaml v0.1.0
	github.com/knadh/koanf/providers/file v1.1.2
	github.com/knadh/koanf/providers/env v0.1.0
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/rs/zerolog v1.33.0
	github.com/oklog/ulid/v2 v2.1.0
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1
	golang.org/x/crypto v0.27.0
	golang.org/x/time v0.6.0
)
```

**ID generation:** ULID everywhere (`internal/id/id.go`). Never use auto-increment integers for public IDs.

**Phase 2 additions only (do not add earlier):**
```
github.com/anthropics/anthropic-sdk-go v0.2.0-alpha
github.com/disintegration/imaging v1.6.2
```

---

# PART 2 — FULL DIRECTORY TREE (create exactly this)

```
evechat/
├── go.mod
├── go.sum
├── sqlc.yaml
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── .dockerignore
├── .gitignore
├── evechat.example.yaml
├── README.md
├── cmd/evechat/main.go
├── internal/
│   ├── id/id.go
│   ├── config/config.go
│   ├── logx/logx.go
│   ├── apierr/apierr.go
│   ├── db/
│   │   ├── db.go
│   │   ├── migrate.go
│   │   └── migrations/
│   │       ├── 001_init.sql
│   │       ├── 002_tickets.sql
│   │       ├── 003_knowledge_base.sql
│   │       ├── 004_webhooks.sql
│   │       └── 005_fts.sql
│   ├── models/models.go
│   ├── repo/
│   │   ├── repo.go
│   │   ├── projects.go
│   │   ├── agents.go
│   │   ├── visitors.go
│   │   ├── sessions.go
│   │   ├── conversations.go
│   │   ├── messages.go
│   │   ├── routing_rules.go
│   │   ├── tickets.go
│   │   ├── knowledge_base.go
│   │   ├── webhooks.go
│   │   └── stats.go
│   ├── hub/
│   │   ├── hub.go
│   │   ├── client.go
│   │   ├── room.go
│   │   ├── protocol.go
│   │   └── msgbuf.go
│   ├── crypto/
│   │   ├── encryption.go
│   │   └── hmac.go
│   ├── services/
│   │   ├── conversation.go
│   │   ├── routing.go
│   │   ├── message.go
│   │   ├── visitor.go
│   │   ├── notification.go
│   │   ├── telegram.go
│   │   ├── webhook.go
│   │   ├── ticket.go
│   │   ├── search.go
│   │   └── ai.go            (Phase 2)
│   ├── integrations/
│   │   ├── eve/client.go
│   │   ├── gmweb/client.go
│   │   └── telegram/bot.go
│   ├── middleware/
│   │   ├── auth.go
│   │   ├── apikey.go
│   │   ├── cors.go
│   │   ├── ratelimit.go
│   │   ├── requestid.go
│   │   └── recover.go
│   ├── handlers/
│   │   ├── handlers.go
│   │   ├── auth.go
│   │   ├── ws_visitor.go
│   │   ├── ws_agent.go
│   │   ├── widget.go
│   │   ├── conversations.go
│   │   ├── messages.go
│   │   ├── visitors.go
│   │   ├── agents.go
│   │   ├── routing_rules.go
│   │   ├── tickets.go
│   │   ├── knowledge_base.go
│   │   ├── webhooks.go
│   │   ├── upload.go
│   │   ├── stats.go
│   │   └── integrations.go
│   ├── router/router.go
│   └── embedfs/embedfs.go
├── web/
│   ├── widget/
│   │   ├── package.json
│   │   ├── tsconfig.json
│   │   ├── rollup.config.js
│   │   └── src/
│   │       ├── index.ts
│   │       ├── types.ts
│   │       ├── config.ts
│   │       ├── transport/
│   │       │   ├── transport.ts
│   │       │   ├── ws.ts
│   │       │   ├── sse.ts
│   │       │   └── poll.ts
│   │       ├── queue.ts
│   │       ├── store.ts
│   │       ├── ui/
│   │       │   ├── launcher.ts
│   │       │   ├── panel.ts
│   │       │   ├── messages.ts
│   │       │   ├── composer.ts
│   │       │   └── styles.ts
│   │       └── i18n.ts
│   └── dashboard/
│       ├── package.json
│       ├── vite.config.ts
│       ├── tsconfig.json
│       ├── tailwind.config.js
│       ├── index.html
│       └── src/
│           ├── main.tsx
│           ├── App.tsx
│           ├── lib/{api.ts,ws.ts,utils.ts,auth.ts}
│           ├── components/ui/*        (shadcn primitives)
│           ├── components/{Sidebar.tsx,ConversationList.tsx,ChatPanel.tsx,VisitorPanel.tsx,Composer.tsx}
│           ├── hooks/{useWebSocket.ts,useConversations.ts,useAuth.ts}
│           └── pages/
│               ├── Login.tsx
│               ├── Inbox.tsx
│               ├── Dashboard.tsx
│               ├── Visitors.tsx
│               ├── Tickets.tsx
│               ├── KnowledgeBase.tsx
│               └── settings/{Widget.tsx,Routing.tsx,Team.tsx,Integrations.tsx,Webhooks.tsx}
├── embed/
│   ├── dashboard/.gitkeep
│   └── widget/.gitkeep
└── scripts/
    ├── build.sh
    ├── dev.sh
    └── seed.go
```

---

# PART 3 — WORK ORDER (execute in this exact sequence)

Each TASK is self-contained. Finish it, emit `--- TASK NN DONE ---`, start the next.

| TASK | Scope | Est. files |
|------|-------|-----------|
| 01 | Bootstrap: go.mod, config, logx, id, apierr, main.go skeleton | 7 |
| 02 | Database layer: db.go, migrate.go, all 5 migration SQL files | 8 |
| 03 | Models + repo layer (all tables, hand-written SQL, no ORM) | 13 |
| 04 | Crypto: AES-256-GCM + HMAC | 2 |
| 05 | Hub: protocol, client, room, msgbuf, hub | 5 |
| 06 | Middleware: auth, apikey, cors, ratelimit, requestid, recover | 6 |
| 07 | Services: visitor, conversation, routing, message, notification | 5 |
| 08 | Integrations: telegram bot, eve client, gmweb client, notification wiring | 4 |
| 09 | Handlers: auth, widget, ws_visitor, ws_agent | 4 |
| 10 | Handlers: CRUD (conversations, messages, visitors, agents, routing, stats) | 7 |
| 11 | Router wiring + embedfs + main.go final | 3 |
| 12 | Widget (TypeScript): transport chain, queue, UI, build config | 16 |
| 13 | Dashboard (React): auth, inbox, WS, settings pages | ~30 |
| 14 | Build system: Dockerfile, compose, Makefile, build.sh, example.yaml, README | 7 |
| 15 | Phase 2: tickets, KB + FTS, webhooks, upload, AI, search, canned responses | ~15 |
| 16 | Phase 3 scaffolding: Redis adapter interface, metering, tenant admin | ~8 |
| 17 | Phase 4: WordPress plugin | ~6 |

---

# TASK 01 — BOOTSTRAP

### `internal/id/id.go`
```go
package id

// New returns a lowercase ULID string, monotonic within a process.
func New() string
// NewPrefixed returns prefix + "_" + New(), e.g. "conv_01J...".
func NewPrefixed(prefix string) string
// Token returns n crypto/rand bytes as base64url (no padding).
func Token(n int) string
```
Prefixes used across the codebase: `proj_`, `agt_`, `vis_`, `conv_`, `msg_`, `rule_`, `tkt_`, `kb_`, `wh_`, `whd_`.

### `internal/config/config.go`
Full struct, loaded by koanf in this precedence: **defaults → yaml file → env (`EVECHAT_` prefix, `__` = nesting)**.

```go
type Config struct {
	Server struct {
		Host            string `koanf:"host"`              // default "0.0.0.0"
		Port            int    `koanf:"port"`              // default 3000
		PublicURL       string `koanf:"public_url"`        // e.g. https://chat.example.com
		ReadTimeout     string `koanf:"read_timeout"`      // "15s"
		WriteTimeout    string `koanf:"write_timeout"`     // "15s"
		ShutdownTimeout string `koanf:"shutdown_timeout"`  // "20s"
		TLS struct {
			Enabled    bool   `koanf:"enabled"`
			CertFile   string `koanf:"cert_file"`
			KeyFile    string `koanf:"key_file"`
			ACMEDomain string `koanf:"acme_domain"`
			ACMECache  string `koanf:"acme_cache"`        // "/data/acme"
		} `koanf:"tls"`
	} `koanf:"server"`

	Database struct {
		Path            string `koanf:"path"`              // "/data/evechat.db"
		MaxOpenConns    int    `koanf:"max_open_conns"`    // 1  (writer)
		MaxReadConns    int    `koanf:"max_read_conns"`    // 4
		BusyTimeoutMS   int    `koanf:"busy_timeout_ms"`   // 5000
	} `koanf:"database"`

	Storage struct {
		UploadsDir    string `koanf:"uploads_dir"`         // "/data/uploads"
		MaxFileSizeMB int    `koanf:"max_file_size_mb"`    // 10
	} `koanf:"storage"`

	Auth struct {
		JWTSecret        string `koanf:"jwt_secret"`
		AccessExpiry     string `koanf:"access_expiry"`    // "24h"
		RefreshExpiry    string `koanf:"refresh_expiry"`   // "720h"
		BcryptCost       int    `koanf:"bcrypt_cost"`      // 12
	} `koanf:"auth"`

	Encryption struct {
		CurrentVersion int `koanf:"current_version"`       // 1
		// keys NEVER in yaml: env EVECHAT_ENCRYPTION_KEY_V1..V9, base64 of 32 bytes
	} `koanf:"encryption"`

	Hub struct {
		PingIntervalSeconds int `koanf:"ping_interval_seconds"` // 30
		PongWaitSeconds     int `koanf:"pong_wait_seconds"`     // 90
		SendBufferSize      int `koanf:"send_buffer_size"`      // 256
		RingBufferSize      int `koanf:"ring_buffer_size"`      // 50
		MaxMessageBytes     int `koanf:"max_message_bytes"`     // 32768
	} `koanf:"hub"`

	RateLimit struct {
		WidgetSessionPerMin int `koanf:"widget_session_per_min"` // 10
		MessagePerMin       int `koanf:"message_per_min"`        // 60
		LoginPerMin         int `koanf:"login_per_min"`          // 5
	} `koanf:"ratelimit"`

	Telegram struct {
		BotToken   string `koanf:"bot_token"`
		WebhookURL string `koanf:"webhook_url"`
		Secret     string `koanf:"secret"`   // X-Telegram-Bot-Api-Secret-Token
		Enabled    bool   `koanf:"enabled"`
	} `koanf:"telegram"`

	Integrations struct {
		EVE   struct{ BaseURL, APIKey, HMACSecret string } `koanf:"eve"`
		GMweb struct{ BaseURL, APIKey string }             `koanf:"gmweb"`
	} `koanf:"integrations"`

	AI struct {
		Enabled      bool   `koanf:"enabled"`
		ClaudeAPIKey string `koanf:"claude_api_key"`
		Model        string `koanf:"model"`         // "claude-haiku-4-5"
		MaxTokens    int    `koanf:"max_tokens"`    // 512
		HandoffAfter int    `koanf:"handoff_after"` // 3 failed answers -> human
	} `koanf:"ai"`

	Log struct {
		Level  string `koanf:"level"`  // "info"
		Pretty bool   `koanf:"pretty"` // false
	} `koanf:"log"`
}

func Load(path string) (*Config, error)
func (c *Config) Validate() error        // fails if JWTSecret=="" or =="CHANGE_ME"
func (c *Config) EncryptionKeys() (map[int][]byte, error)  // reads EVECHAT_ENCRYPTION_KEY_V<n>
```
`Validate()` must hard-fail at startup if: jwt_secret missing/default, no encryption key for current_version, database dir not writable.

### `internal/logx/logx.go`
zerolog global setup; `logx.Init(level string, pretty bool)`; `logx.L()` returns `*zerolog.Logger`; `logx.WithReq(ctx)` pulls request_id into the logger.

### `internal/apierr/apierr.go`
```go
type Error struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`     // "invalid_request", "unauthorized", "not_found", "rate_limited", "internal"
	Message string `json:"message"`  // Persian, user-safe
	Details any    `json:"details,omitempty"`
}
func New(status int, code, msg string) *Error
func BadRequest(msg string) *Error
func Unauthorized(msg string) *Error
func Forbidden(msg string) *Error
func NotFound(msg string) *Error
func RateLimited(msg string) *Error
func Internal(err error) *Error
func Write(w http.ResponseWriter, err error)   // marshals; unknown errors -> 500 generic
```
All handlers return `error`; a wrapper `handlers.H(fn)` converts to HTTP. Implement `H` in `handlers/handlers.go`.

### `cmd/evechat/main.go` (final version written in TASK 11, skeleton now)
Flags: `--config` (default `./evechat.yaml`), `--migrate-only`, `--seed`, `--version`.
Flow: parse flags → load config → validate → logx init → open DB → run migrations → build encryption service → build repos → build hub (`go hub.Run(ctx)`) → build services → build router → `http.Server` with timeouts → graceful shutdown on SIGINT/SIGTERM (drain hub, close WS with code 1001, close DB).

`--- TASK 01 DONE ---`

---

# TASK 02 — DATABASE LAYER

### `internal/db/db.go`
- Open with `modernc.org/sqlite`, DSN:
  `file:<path>?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)&_pragma=cache_size(-8000)&_pragma=temp_store(MEMORY)`
- **Two pools**: `Writer` (`SetMaxOpenConns(1)`) and `Reader` (`SetMaxOpenConns(cfg.MaxReadConns)`). Expose `type DB struct{ W, R *sql.DB }`.
- `func Open(cfg) (*DB, error)`, `func (d *DB) Close() error`, `func (d *DB) Tx(ctx, fn func(*sql.Tx) error) error` (uses W, `BEGIN IMMEDIATE`, rollback on error).
- Run `PRAGMA optimize` on close and every 6h via a ticker goroutine.

### `internal/db/migrate.go`
- `//go:embed migrations/*.sql`
- Table `schema_migrations(version INTEGER PRIMARY KEY, applied_at DATETIME)`.
- Apply in filename order inside one transaction each. Idempotent. Log each applied version.

### `internal/db/migrations/001_init.sql` — WRITE IN FULL
```sql
CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    api_key     TEXT NOT NULL UNIQUE,
    secret_key  TEXT NOT NULL,
    domains     TEXT NOT NULL DEFAULT '[]',
    settings    TEXT NOT NULL DEFAULT '{}',
    is_active   INTEGER NOT NULL DEFAULT 1,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS agents (
    id                TEXT PRIMARY KEY,
    project_id        TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    email             TEXT NOT NULL,
    password_hash     TEXT NOT NULL,
    name              TEXT NOT NULL,
    avatar_url        TEXT,
    role              TEXT NOT NULL DEFAULT 'agent',
    telegram_id       TEXT,
    max_conversations INTEGER NOT NULL DEFAULT 10,
    is_online         INTEGER NOT NULL DEFAULT 0,
    is_active         INTEGER NOT NULL DEFAULT 1,
    last_seen_at      DATETIME,
    created_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_project_email ON agents(project_id, email);
CREATE INDEX IF NOT EXISTS idx_agents_online ON agents(project_id, is_online);

CREATE TABLE IF NOT EXISTS visitors (
    id                     TEXT PRIMARY KEY,
    project_id             TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name                   TEXT,
    email                  TEXT,
    phone                  TEXT,
    ip                     TEXT,
    country                TEXT,
    device_type            TEXT,
    os                     TEXT,
    browser                TEXT,
    user_agent             TEXT,
    language               TEXT,
    eve_mobile             TEXT,
    eve_user_id            TEXT,
    eve_reseller_id        TEXT,
    eve_subscription_info  TEXT NOT NULL DEFAULT '{}',
    metadata               TEXT NOT NULL DEFAULT '{}',
    is_banned              INTEGER NOT NULL DEFAULT 0,
    banned_until           DATETIME,
    first_seen_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_visitors_project ON visitors(project_id, last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_visitors_reseller ON visitors(project_id, eve_reseller_id);
CREATE INDEX IF NOT EXISTS idx_visitors_mobile ON visitors(project_id, eve_mobile);

CREATE TABLE IF NOT EXISTS routing_rules (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    priority    INTEGER NOT NULL DEFAULT 0,
    conditions  TEXT NOT NULL DEFAULT '{}',
    route_to    TEXT NOT NULL DEFAULT '{}',
    auto_reply  TEXT,
    is_active   INTEGER NOT NULL DEFAULT 1,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_rules_project_prio ON routing_rules(project_id, is_active, priority DESC);

CREATE TABLE IF NOT EXISTS conversations (
    id                 TEXT PRIMARY KEY,
    project_id         TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    visitor_id         TEXT NOT NULL REFERENCES visitors(id) ON DELETE CASCADE,
    assigned_agent_id  TEXT REFERENCES agents(id) ON DELETE SET NULL,
    routing_rule_id    TEXT REFERENCES routing_rules(id) ON DELETE SET NULL,
    status             TEXT NOT NULL DEFAULT 'pending',
    channel            TEXT NOT NULL DEFAULT 'web',
    subject            TEXT,
    page_url           TEXT,
    page_title         TEXT,
    referrer           TEXT,
    routed_telegram_id TEXT,
    routed_phone       TEXT,
    ticket_id          TEXT,
    unread_agent       INTEGER NOT NULL DEFAULT 0,
    unread_visitor     INTEGER NOT NULL DEFAULT 0,
    last_message_at    DATETIME,
    first_response_at  DATETIME,
    resolved_at        DATETIME,
    csat_score         INTEGER,
    csat_comment       TEXT,
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_conv_project_status ON conversations(project_id, status, last_message_at DESC);
CREATE INDEX IF NOT EXISTS idx_conv_agent ON conversations(assigned_agent_id, status);
CREATE INDEX IF NOT EXISTS idx_conv_visitor ON conversations(visitor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_conv_tg ON conversations(routed_telegram_id) WHERE routed_telegram_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_conv_phone ON conversations(routed_phone) WHERE routed_phone IS NOT NULL;

CREATE TABLE IF NOT EXISTS messages (
    id                  TEXT PRIMARY KEY,
    conversation_id     TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    client_message_id   TEXT,
    sender_type         TEXT NOT NULL,
    sender_id           TEXT,
    sender_name         TEXT,
    content_type        TEXT NOT NULL DEFAULT 'text',
    content_encrypted   TEXT NOT NULL,
    content_iv          TEXT NOT NULL,
    key_version         INTEGER NOT NULL DEFAULT 1,
    attachments         TEXT NOT NULL DEFAULT '[]',
    telegram_message_id INTEGER,
    is_internal         INTEGER NOT NULL DEFAULT 0,
    is_read             INTEGER NOT NULL DEFAULT 0,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_msg_conv ON messages(conversation_id, created_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_msg_dedup ON messages(conversation_id, client_message_id) WHERE client_message_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_msg_tg ON messages(telegram_message_id) WHERE telegram_message_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS visitor_sessions (
    token          TEXT PRIMARY KEY,
    visitor_id     TEXT NOT NULL REFERENCES visitors(id) ON DELETE CASCADE,
    project_id     TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    active_conv_id TEXT REFERENCES conversations(id) ON DELETE SET NULL,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at     DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_visitor ON visitor_sessions(visitor_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON visitor_sessions(expires_at);
```

### `002_tickets.sql`, `003_knowledge_base.sql`, `004_webhooks.sql`
Exactly as in the architecture doc, plus: `tickets` gets `due_at DATETIME`, `updated_at`, index `idx_tickets_project_status`. `webhook_deliveries` gets `next_retry_at DATETIME`, `error TEXT`, index on `(webhook_id, created_at DESC)`.

Add also in 004:
```sql
CREATE TABLE IF NOT EXISTS canned_responses (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    shortcut TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_canned_shortcut ON canned_responses(project_id, shortcut);
```

### `005_fts.sql`
FTS5 for `knowledge_base` (title, content) with external-content config + 3 triggers (ai/ad/au).
**Note:** messages are encrypted so they are NOT in FTS. Message search is implemented in `services/search.go` by decrypting a bounded window (max 2000 recent messages per conversation scope) — document this limitation in a code comment.

`--- TASK 02 DONE ---`

---

# TASK 03 — MODELS + REPOSITORY LAYER

**Rule:** no ORM, no sqlc for now — hand-written `database/sql` with explicit column lists. (sqlc.yaml is still produced in TASK 14 for future use.)

### `internal/models/models.go`
One struct per table, with `json` tags, `time.Time` for dates, `*string` for nullables, and `JSONField` helpers:
```go
type JSONMap map[string]any
func (j *JSONMap) Scan(src any) error
func (j JSONMap) Value() (driver.Value, error)

type StringList []string   // same Scan/Value over a JSON array
```
Enums as typed string constants:
```go
type ConvStatus string
const (StatusPending ConvStatus="pending"; StatusActive="active"; StatusResolved="resolved"; StatusClosed="closed")
type SenderType string // visitor, agent, bot, system
type Role string       // superadmin, admin, agent
```

### `internal/repo/repo.go`
```go
type Repo struct {
	db *db.DB
	Projects *ProjectRepo
	Agents *AgentRepo
	Visitors *VisitorRepo
	Sessions *SessionRepo
	Conversations *ConversationRepo
	Messages *MessageRepo
	Rules *RuleRepo
	Tickets *TicketRepo
	KB *KBRepo
	Webhooks *WebhookRepo
	Stats *StatsRepo
}
func New(d *db.DB) *Repo
```

Each sub-repo implements, at minimum:

**ProjectRepo:** `Create`, `GetByID`, `GetByAPIKey`, `List`, `Update`, `Delete`.
**AgentRepo:** `Create`, `GetByID`, `GetByEmail(projectID,email)`, `List(projectID)`, `Update`, `SetOnline(id,bool)`, `ActiveLoad(projectID) map[agentID]int` (open conv count), `Delete`.
**VisitorRepo:** `Upsert(v *Visitor)`, `GetByID`, `List(projectID, filter, limit, offset)`, `UpdateLastSeen`, `Ban(id, until)`, `IsBanned(id) bool`.
**SessionRepo:** `Create(visitorID, projectID, ttl)`, `Get(token)`, `Touch(token)`, `SetActiveConv(token, convID)`, `DeleteExpired()` (called hourly by a janitor goroutine).
**ConversationRepo:** `Create`, `GetByID`, `List(projectID, ConvFilter{Status, AgentID, VisitorID, Channel, From, To, Limit, Offset})`, `Assign(convID, agentID)`, `SetStatus`, `SetRouting(convID, ruleID, tgID, phone)`, `TouchLastMessage`, `IncUnread(convID, side)`, `ClearUnread`, `GetByTelegramID(tgID) (latest open conv)`, `GetByPhone(phone)`, `SetCSAT`.
**MessageRepo:** `Create(msg) (id, error)` returning `ErrDuplicate` on the dedup unique index, `ListByConversation(convID, since time.Time, limit)`, `LatestN(convID, n)`, `MarkRead(convID, upToID, side)`, `GetByTelegramMsgID(int)`, `SetTelegramMsgID(msgID, tgID)`.
**RuleRepo:** `Create`, `List(projectID, activeOnly)`, `Update`, `Reorder([]idPriority)`, `Delete`.
**StatsRepo:** `Overview(projectID, from, to) (OverviewStats)` with: total conversations, pending, active, resolved, avg first response seconds, avg resolution minutes, CSAT avg, messages count, unique visitors; `Series(projectID, from, to, bucket string)` returning daily/hourly buckets; `AgentStats(projectID, from, to)`.

**Every repo method takes `ctx context.Context` first.** Reads use `r.db.R`, writes use `r.db.W`.

`--- TASK 03 DONE ---`

---

# TASK 04 — CRYPTO

### `internal/crypto/encryption.go`
```go
type Service struct { keys map[int][]byte; current int }
func New(keys map[int][]byte, current int) (*Service, error)  // validates 32-byte keys
func (s *Service) Encrypt(plain string) (ct string, iv string, version int, err error)
func (s *Service) Decrypt(ct, iv string, version int) (string, error)
func (s *Service) ReEncrypt(ct, iv string, oldVer int) (string, string, int, error) // for key rotation job
```
AES-256-GCM, 12-byte random nonce per message, base64 std encoding for both fields. Unknown `version` → `ErrUnknownKeyVersion`. Never log plaintext or keys.

### `internal/crypto/hmac.go`
```go
func SignHex(secret string, body []byte) string                 // HMAC-SHA256 hex
func VerifyHex(secret string, body []byte, sig string) bool     // hmac.Equal, constant-time
func SignEVEContext(secret string, payload []byte, ts int64) string
func VerifyEVEContext(secret string, payload []byte, ts int64, sig string, maxSkew time.Duration) bool
```
Webhook signature header: `X-EveChat-Signature: sha256=<hex>` plus `X-EveChat-Timestamp`. Signed string is `"<timestamp>.<body>"`.

`--- TASK 04 DONE ---`

---

# TASK 05 — HUB (the heart — get this exactly right)

### Design, non-negotiable
- **One** `Hub` goroutine owns all maps. No mutexes on hub state. Everything goes through `cmds chan command`.
- Each client has: `send chan []byte` (buffered, size from config), a **read** goroutine, a **write** goroutine.
- A slow client whose `send` buffer is full is **dropped immediately** (non-blocking select with `default:` → `hub.unregister`). Never block the hub goroutine.
- Ping every `ping_interval`, expect pong within `pong_wait`; otherwise close.

### `internal/hub/protocol.go`
```go
type Envelope struct {
	Type string          `json:"type"`
	ID   string          `json:"id,omitempty"`    // client-generated UUID for ack correlation
	Data json.RawMessage `json:"data,omitempty"`
	TS   int64           `json:"ts,omitempty"`    // server unix millis, set on outbound
}
```
**Inbound from visitor:** `conversation.start`, `message.send`, `typing.start`, `typing.stop`, `sync.request`, `read.ack`, `csat.submit`, `ping`.
**Outbound to visitor:** `conversation.created`, `message.sent` (ack: `{id, client_id, created_at}`), `message.received`, `agent.assigned`, `agent.typing`, `conversation.resolved`, `sync.response`, `error`, `pong`.
**Inbound from agent:** `message.send`, `typing.start/stop`, `conversation.assign`, `conversation.resolve`, `conversation.transfer`, `note.add`, `read.ack`, `presence.set`, `ping`.
**Outbound to agent:** `conversation.new`, `conversation.updated`, `message.received`, `visitor.typing`, `visitor.online`, `visitor.offline`, `presence.update`, `stats.tick` (every 30s, counts only), `error`, `pong`.

Error envelope data: `{"code":"...","message":"..."}`.

### `internal/hub/msgbuf.go`
Fixed-size ring buffer per conversation holding the last N (`cfg.Hub.RingBufferSize`) **already-decrypted-for-transport** outbound envelopes with their server timestamps, so `sync.request?since=<ts>` is answered from memory with zero DB reads. Evict a conversation's ring 30 minutes after its last activity. Guarded by its own `sync.RWMutex` (it is read from client goroutines, not the hub goroutine).

### `internal/hub/client.go`
```go
type ClientKind int // KindVisitor, KindAgent
type Client struct {
	ID         string      // visitorID or agentID
	Kind       ClientKind
	ProjectID  string
	ConnID     string      // ULID, unique per socket (multi-tab support)
	ConvIDs    map[string]struct{} // rooms this client is in
	send       chan []byte
	conn       *websocket.Conn
	hub        *Hub
	closeOnce  sync.Once
}
func (c *Client) readLoop(ctx)   // reads, enforces MaxMessageBytes, dispatches to hub.OnMessage
func (c *Client) writeLoop(ctx)  // drains send, writes ping on ticker
func (c *Client) Send(e Envelope) // marshal + non-blocking enqueue; drop client if full
func (c *Client) Close(code websocket.StatusCode, reason string)
```
Multiple sockets per identity are allowed (multi-tab, phone+desktop). The hub stores `map[identityID]map[connID]*Client`.

### `internal/hub/room.go`
A room = one conversation. Holds the set of connIDs subscribed (visitor sockets + assigned agent sockets + any admin watching). `Broadcast(e Envelope, exceptConnID string)`.

### `internal/hub/hub.go`
```go
type Hub struct {
	cmds       chan command
	visitors   map[string]map[string]*Client
	agents     map[string]map[string]*Client
	rooms      map[string]*Room
	agentsByProject map[string]map[string]struct{}
	bufs       *MsgBuffers
	handler    MessageHandler   // injected: services layer
	cfg        HubConfig
	log        zerolog.Logger
}

type MessageHandler interface {
	OnVisitorMessage(ctx context.Context, c *Client, e Envelope) error
	OnAgentMessage(ctx context.Context, c *Client, e Envelope) error
	OnVisitorConnect(ctx context.Context, c *Client) error
	OnVisitorDisconnect(ctx context.Context, c *Client)
	OnAgentConnect(ctx context.Context, c *Client) error
	OnAgentDisconnect(ctx context.Context, c *Client)
}

func New(cfg HubConfig, log zerolog.Logger) *Hub
func (h *Hub) SetHandler(MessageHandler)
func (h *Hub) Run(ctx context.Context)        // the single goroutine; select over cmds + 30s stats ticker + 5m GC ticker
func (h *Hub) Register(c *Client)
func (h *Hub) Unregister(c *Client)
func (h *Hub) JoinRoom(convID string, c *Client)
func (h *Hub) LeaveRoom(convID string, c *Client)
func (h *Hub) ToConversation(convID string, e Envelope, exceptConnID string)
func (h *Hub) ToVisitor(visitorID string, e Envelope)
func (h *Hub) ToAgent(agentID string, e Envelope)
func (h *Hub) ToProjectAgents(projectID string, e Envelope)
func (h *Hub) OnlineAgents(projectID string) []string
func (h *Hub) IsVisitorOnline(visitorID string) bool
func (h *Hub) Stats() HubStats  // connections, rooms, dropped, goroutines
func (h *Hub) Shutdown(ctx context.Context)  // close all with 1001 "server shutting down"
```
All exported methods **must** be safe from any goroutine: they only push a `command` onto `h.cmds` (with a `ctx.Done()` escape so they never deadlock at shutdown). Query-style methods (`OnlineAgents`, `IsVisitorOnline`, `Stats`) push a command carrying a reply channel.

**Write a unit test** `internal/hub/hub_test.go`: 500 fake clients, 10k broadcasts, assert no deadlock, no goroutine leak (`goleak`-style manual count), and that a stalled client gets dropped rather than blocking others.

`--- TASK 05 DONE ---`

---

# TASK 06 — MIDDLEWARE

- `requestid.go` — ULID into context + `X-Request-ID` response header.
- `recover.go` — recover panics, log stack, return 500 `apierr.Internal`.
- `cors.go` — two configs: **open** (`*`) for `/widget.js`, `/api/widget/v1/*`, `/ws/visitor`; **strict** (dashboard origin from `server.public_url`) for `/api/v1/*`.
- `apikey.go` — reads `X-Project-Key` (or `?project_key=` for `widget.js`), loads project from a 60s in-memory cache (`sync.Map` + expiry), rejects inactive, verifies the `Origin`/`Referer` host is in `project.domains` **unless** domains is empty (dev mode). Puts `*models.Project` in context.
- `auth.go` — Bearer JWT HS256. Claims: `sub` (agentID), `pid` (projectID), `role`, `exp`, `iat`, `typ` ("access"|"refresh"). Helpers `CurrentAgent(ctx)`, `RequireRole(roles ...Role)`.
- `ratelimit.go` — token bucket via `golang.org/x/time/rate`, keyed by `project+IP` for widget routes and by `agentID` for dashboard routes, stored in a sharded map with 10-minute idle eviction. **No Redis.** Returns 429 with `Retry-After`.

Client IP resolution: trust `X-Forwarded-For` first hop only if `server.trust_proxy` is true (add this bool to config), else `RemoteAddr`.

`--- TASK 06 DONE ---`

---

# TASK 07 — SERVICES

### `services/visitor.go`
- `IdentifyOrCreate(ctx, projectID string, req SessionRequest) (*Visitor, *Session, error)`
- Device parsing from User-Agent **without a dependency**: hand-written matcher covering device_type (mobile/tablet/desktop), os (Android, iOS, Windows, macOS, Linux), browser (Chrome, Safari, Firefox, Edge, Samsung, Opera). ~80 lines of regex-free `strings.Contains` checks, ordered correctly (Edge before Chrome, Chrome before Safari).
- Merge `window.EveChat.visitor` fields into the visitor row; never overwrite a non-empty stored value with an empty incoming one.

### `services/routing.go`
```go
type Decision struct {
	RuleID      string
	AgentID     string
	TelegramID  string
	Phone       string
	AutoReply   string
	Fallback    bool
}
func (s *RoutingService) Decide(ctx, conv *Conversation, v *Visitor) (Decision, error)
```
Matching order: rules sorted by `priority DESC, created_at ASC`; first rule whose **all** present conditions match wins.
Supported condition keys inside `conditions` JSON:
```json
{
  "domains": ["eve.ir"],
  "pages": ["/subscription*", "*/billing"],     // glob, * only
  "eve_reseller_id": ["reseller_abc"],
  "device_types": ["mobile"],
  "languages": ["fa"],
  "has_subscription": true,
  "business_hours": true,
  "variables": {"utm_source": ["google"]}
}
```
`route_to` JSON: `{"agent_id":"...","telegram_id":"...","phone":"...","team":"..."}`.
Fallback chain when no rule matches: least-loaded online agent (respecting `max_conversations`) → any online agent → project default telegram → mark `pending` and notify all agents.

### `services/conversation.go`
```go
Start(ctx, sess *Session, in StartInput) (*Conversation, *Message, error)
Assign(ctx, convID, agentID, byAgentID string) error
Transfer(ctx, convID, toAgentID, note string) error
Resolve(ctx, convID, byAgentID string) error
Reopen(ctx, convID string) error
ConvertToTicket(ctx, convID string, in TicketInput) (*Ticket, error)
SubmitCSAT(ctx, convID string, score int, comment string) error
```
`Start` is transactional: create conversation → routing decision → persist routing → insert first message → emit `conversation.created` to visitor, `conversation.new` to project agents → fire notification (Telegram/SMS) **asynchronously** via a buffered worker channel, never inline in the request path.

### `services/message.go`
```go
Send(ctx, in SendInput) (*Message, error)
```
Steps, in order, every time:
1. Validate: non-empty after trim, ≤ 4000 runes, conversation exists and is not `closed`, sender authorized for this conversation, visitor not banned.
2. Dedup: if `client_message_id` already exists for this conversation → return the existing message with `Duplicate: true` (idempotent; the widget retries freely).
3. Encrypt content.
4. Insert row; update `conversations.last_message_at`, bump unread on the other side, set `first_response_at` if sender is an agent and it is unset.
5. Push into the conversation ring buffer.
6. Broadcast to room (visitor sockets + agent sockets), plus `conversation.updated` to all project agents for inbox ordering.
7. Enqueue notifications (Telegram relay, webhook fan-out) on the async worker.
8. Return the message with its ack.

### `services/notification.go`
Abstraction so channels are pluggable:
```go
type Channel interface {
	Name() string
	Notify(ctx context.Context, ev Event) error
}
type Service struct { channels map[string]Channel; queue chan Event; workers int }
func (s *Service) Start(ctx) // N=4 workers, each draining queue with per-event timeout 10s
func (s *Service) Enqueue(ev Event) // non-blocking; drop + warn-log if queue full (size 1024)
```
Events: `chat.started`, `message.received`, `chat.assigned`, `chat.resolved`, `ticket.created`, `csat.received`.

`--- TASK 07 DONE ---`

---

# TASK 08 — INTEGRATIONS

### `integrations/telegram/bot.go`
- Webhook mode only (no long polling): `POST /api/v1/integrations/telegram/webhook`, verified by `X-Telegram-Bot-Api-Secret-Token`.
- `SetWebhook` on startup when `telegram.enabled`.
- Outbound new-conversation message (Persian, HTML parse mode):
```
🔔 چت جدید
👤 {visitor_name|شماره|ناشناس}
📱 {device} · {os} · {browser}
🔗 {page_title}
💳 پلن: {plan} | انقضا: {expiry}
━━━━━━━━━━
{message}

برای پاسخ، روی همین پیام Reply کنید.
```
With inline keyboard: `[مشاهده در پنل]` (URL to dashboard conversation), `[بستن گفتگو]` (callback `resolve:<convID>`), `[ارجاع به پشتیبان]` (callback `escalate:<convID>`).
- **Correlation:** persist `messages.telegram_message_id` on every outbound message AND keep an in-memory `map[int64]string` (tgMsgID → convID) with 72h TTL for fast lookup. On `message.reply_to_message`, resolve convID from memory, fall back to DB.
- Handle `callback_query` for the two buttons; answer the callback so the spinner stops.
- Rate-limit outbound to 25 msg/sec globally (Telegram limit), with a simple token bucket and a retry on HTTP 429 honoring `retry_after`.

### `integrations/eve/client.go`
- `GetUserByMobile(ctx, mobile) (EVEUser, error)` → `GET {base}/api/user/{mobile}` with `Authorization: Bearer {api_key}`, 5s timeout, 1 retry.
- `VerifySignedContext(payload []byte, ts int64, sig string) bool` using `eve.hmac_secret` (Phase 2 path — implement now, enable by config flag `integrations.eve.require_signature`).

### `integrations/gmweb/client.go`
- `SendSMS(ctx, phone, text string) error` → `POST {base}/api/send`, body `{"to":...,"message":...}`, bearer api_key, 8s timeout, 2 retries with backoff.
- Inbound handler maps `from` phone → conversation via `conversations.routed_phone` plus a 24h in-memory map; posts as an agent message.

### Notification channels
`TelegramChannel`, `SMSChannel`, `WebhookChannel` all implement `services.Channel` and are registered in `main.go` based on config.

`--- TASK 08 DONE ---`

---

# TASK 09 — AUTH, WIDGET & WEBSOCKET HANDLERS

### `handlers/auth.go`
- `POST /api/v1/auth/login` → `{email, password, project_key?}`; bcrypt compare; returns `{access_token, refresh_token, agent:{...}, project:{...}}`. Rate-limited 5/min per IP.
- `POST /api/v1/auth/refresh` → rotate.
- `POST /api/v1/auth/logout` → sets agent offline.
- `GET /api/v1/auth/me`.

### `handlers/widget.go`
- `GET /widget.js` — serves embedded, minified widget from `embed/widget`, `Cache-Control: public, max-age=300`, `ETag`. Injects nothing; config is fetched at runtime.
- `GET /api/widget/v1/config?project_key=...&domain=...` — returns `{project_id, name, locale:"fa", direction:"rtl", colors:{...}, position, greeting, offline_message, pre_chat_form, business_hours, online: bool}`.
- `POST /api/widget/v1/session` — body: `{project_key, visitor:{name,email,mobile,metadata}, page:{url,title,referrer}, token?}`. If `token` is valid and unexpired → resume (return same visitor + `active_conversation` + last 50 messages). Else create. Returns `{token, visitor_id, expires_at, conversation?, messages?}`.
- Fallback transport endpoints:
  - `GET /api/widget/v1/events?token=...` — SSE. Sends `retry: 3000`, a `: keepalive` comment every 20s, and the same envelopes as WS.
  - `GET /api/widget/v1/poll?token=...&since=<unix_ms>` — long-poll, holds up to 25s, returns `{events:[...], now: <ms>}`.
  - `POST /api/widget/v1/conversations/:id/messages` — REST send (used when transport is SSE/poll).
  - `GET /api/widget/v1/conversations/:id/sync?since=<ms>` — ring-buffer-backed catch-up, DB fallback if `since` is older than the ring.

### `handlers/ws_visitor.go`
`GET /ws/visitor?token=<session>&project=<id>` → validate session, `websocket.Accept` with `InsecureSkipVerify: true` for origin (CORS is handled by the project-domain check instead — document this), `conn.SetReadLimit(cfg.MaxMessageBytes)`, build `Client{Kind: KindVisitor}`, register, join active conversation room, send `sync.response` with everything since `?since=`, then run read/write loops.

### `handlers/ws_agent.go`
`GET /ws/agent` with `Authorization: Bearer <jwt>` **or** `?token=<jwt>` (browsers can't set WS headers — support both, prefer the query param and document it). On connect: mark agent online, broadcast `presence.update`, join all rooms of conversations assigned to this agent + the project "lobby" for `conversation.new`.

`--- TASK 09 DONE ---`

---

# TASK 10 — REST CRUD HANDLERS

Implement every endpoint below. All list endpoints: `?limit=` (default 25, max 100), `?offset=`, and return `{data:[...], total, limit, offset}`. All responses JSON, `Content-Type: application/json; charset=utf-8`.

```
GET    /api/v1/conversations?status=&agent=me|<id>&channel=&q=&from=&to=
GET    /api/v1/conversations/:id
POST   /api/v1/conversations/:id/assign        {agent_id}
POST   /api/v1/conversations/:id/transfer      {agent_id, note}
POST   /api/v1/conversations/:id/resolve
POST   /api/v1/conversations/:id/reopen
POST   /api/v1/conversations/:id/convert-ticket {title, priority, tags}
GET    /api/v1/conversations/:id/messages?since=&limit=
POST   /api/v1/conversations/:id/messages      {content, content_type, client_message_id}
POST   /api/v1/conversations/:id/note          {content}
POST   /api/v1/conversations/:id/read          {up_to_message_id}
GET    /api/v1/conversations/export?format=json|csv&from=&to=

GET    /api/v1/visitors?q=&online=&reseller=
GET    /api/v1/visitors/:id
GET    /api/v1/visitors/:id/conversations
POST   /api/v1/visitors/:id/ban                {duration}  // 1h|24h|permanent
DELETE /api/v1/visitors/:id/ban
GET    /api/v1/visitors/:id/export             // GDPR
POST   /api/v1/visitors/:id/anonymize

GET/POST/PUT/DELETE /api/v1/agents  (+ PUT /api/v1/agents/:id/password)
GET/POST/PUT/DELETE /api/v1/routing-rules
POST   /api/v1/routing-rules/reorder            {order:[{id,priority}]}
POST   /api/v1/routing-rules/:id/test           {visitor_sample} → which rule would match
GET/POST/PUT/DELETE /api/v1/canned-responses
GET    /api/v1/stats/overview?from=&to=
GET    /api/v1/stats/series?from=&to=&bucket=day|hour
GET    /api/v1/stats/agents?from=&to=
GET    /api/v1/health            // {status, version, uptime, db:"ok", hub:{conns,rooms}}
GET    /api/v1/settings/project  + PUT
```
Authorization matrix: `agent` sees only conversations assigned to them or `pending`; `admin` sees all in project; `superadmin` sees all projects. Enforce in the repo filter, not only in the handler.

`--- TASK 10 DONE ---`

---

# TASK 11 — ROUTER + EMBED + MAIN

### `internal/embedfs/embedfs.go`
```go
//go:embed all:dashboard
var dashboardFS embed.FS
//go:embed all:widget
var widgetFS embed.FS
func Dashboard() fs.FS
func Widget() fs.FS
```
Build tags: when `embed/dashboard` is empty in dev, serve a plain "dashboard not built" page instead of failing compilation — ship `.gitkeep` and an `index.html` stub so `go:embed` always has a file.

### `internal/router/router.go`
Group order:
1. Global: `recover`, `requestid`, `realip`, zerolog request logging (skip `/api/v1/health`).
2. `/widget.js`, `/api/widget/v1/*`, `/ws/visitor` → open CORS + apikey (where applicable) + widget rate limit.
3. `/api/v1/auth/*` → strict CORS + login rate limit.
4. `/api/v1/*` → strict CORS + JWT.
5. `/ws/agent` → JWT from query or header.
6. `/api/v1/integrations/*` → shared-secret middleware (per-integration).
7. `/uploads/*` → `http.FileServer` over `storage.uploads_dir`, with `Content-Disposition: inline` only for images, `attachment` otherwise, and no directory listing.
8. `/*` → SPA handler: serve embedded dashboard, fall back to `index.html` for any unknown path that doesn't start with `/api` or `/ws`.

### `cmd/evechat/main.go` — final
Wire everything, print a startup banner with version/port/db path/hub config, and `--seed` which creates a demo project + superadmin (`admin@evechat.local` / printed random password) and prints the project api_key.

**Acceptance for TASK 11:**
```bash
CGO_ENABLED=0 go build -o evechat ./cmd/evechat   # must succeed
./evechat --config evechat.yaml --seed
curl -s localhost:3000/api/v1/health | jq .
```

`--- TASK 11 DONE ---`

---

# TASK 12 — WIDGET (vanilla TypeScript, < 12KB gzip)

### Public API (what EVE embeds)
```html
<script>
window.EveChat = {
  projectKey: "pk_...",
  apiUrl: "https://chat.example.com",
  visitor: { name:"...", mobile:"...", metadata:{ eve_user_id, eve_reseller_id, subscription_plan, subscription_expiry } },
  locale: "fa",
  position: "bottom-left",
  autoOpen: false
};
</script>
<script async src="https://chat.example.com/widget.js"></script>
```
Also support the queued-command pattern:
```js
window.ec = window.ec || function(){ (ec.q = ec.q || []).push(arguments) };
ec('setVariable','utm_source','google');
ec('open'); ec('close'); ec('identify', {...}); ec('on','message', cb);
```

### `transport/transport.ts` — the resilience chain (critical file)
```
State machine: CONNECTING → OPEN → RECONNECTING → DEGRADED → OFFLINE
Attempt order: WS → (2 failures) → SSE+REST → (2 failures) → LongPoll+REST
Every 60s while degraded, try to upgrade back to WS once.
Backoff: 1s, 2s, 4s, 8s, 15s, 30s cap, with ±25% jitter.
Heartbeat: send {"type":"ping"} every 25s; if no pong within 20s, force reconnect.
On any reconnect: send sync.request with since = lastServerTs, reconcile by message id.
```
Expose one uniform interface so UI code never knows which transport is active:
```ts
interface Transport {
  connect(): void; close(): void;
  send(e: Envelope): Promise<void>;   // resolves when server acks or queued
  on(type: string, cb: (data:any)=>void): void;
  state: TransportState;
}
```

### `queue.ts` — IndexedDB offline queue
- DB `evechat`, store `outbox` keyed by `client_message_id`.
- Each entry: `{client_message_id, conversation_id, content, content_type, created_at, attempts, status}`.
- On transport OPEN: flush in `created_at` order, max 5 concurrent, exponential retry, give up after 10 attempts → mark `failed` and show a retry button in the UI.
- Server dedup makes double-send harmless; never dedupe optimistically on the client beyond the id.
- Fallback to `localStorage` if IndexedDB is unavailable (private mode).

### UI requirements
- RTL by default (`dir="rtl"`), Persian strings in `i18n.ts` with an `en` fallback map.
- Shadow DOM root so host-page CSS cannot leak in. All styles injected as one `<style>` inside the shadow root.
- Launcher: 56px circle, unread badge, subtle pulse on new message.
- Panel: 380×560 desktop; full-screen on `max-width: 480px`; header (agent name + status), message list (virtualized above 200 messages), composer (textarea, Enter sends, Shift+Enter newline, attachment button in Phase 2), "در حال تایپ…" indicator, connection banner when DEGRADED/OFFLINE ("اتصال ضعیف — پیام‌ها پس از وصل شدن ارسال می‌شوند").
- Message states rendered as: ⏳ pending, ✓ sent, ✓✓ read, ⚠ failed(tap to retry).
- Persist open/closed state and draft text in `localStorage`.
- No external fonts, no network calls other than to `apiUrl`. Total bundle **must** be < 12KB gzip — verify with `gzip -c dist/widget.js | wc -c` in the build script and fail the build if exceeded.

### Build
Rollup + `@rollup/plugin-typescript` + terser (`compress: {passes:2}`, `mangle: {properties: {regex: /^_/}}`), output IIFE, no polyfills, target ES2018.

`--- TASK 12 DONE ---`

---

# TASK 13 — DASHBOARD (React 18 + Vite + TS + Tailwind + shadcn/ui)

**Locale:** fa-IR, RTL (`dir="rtl"` on `<html>`, Tailwind `rtl` plugin or logical properties). Jalali dates via a tiny local converter (no heavy dep) — write `lib/jalali.ts` (~60 lines, Gregorian↔Jalali).

**State:** TanStack Query v5 for REST, a single `useWebSocket` hook for realtime that invalidates/patches the query cache (optimistic updates for sends).

**Pages, with required behaviour:**

1. **Login** — email/password, remember-me, error toasts.
2. **Inbox** (the main screen, 3 columns):
   - Left: filter tabs (در انتظار / فعال من / همه / حل‌شده), search, conversation list with avatar, last message preview, unread badge, relative time, live reordering on `conversation.updated`.
   - Center: message thread (grouped by day, Jalali date dividers), typing indicator, composer with `/` canned-response dropdown, internal-note toggle (yellow background), file attach (Phase 2), keyboard shortcuts: `Ctrl+Enter` send, `Ctrl+K` search, `Ctrl+Shift+R` resolve.
   - Right: visitor panel — name/mobile, device/os/browser icons, current page URL + title, EVE block (user id, reseller id, plan, expiry, days remaining), past conversations list, tags, "تبدیل به تیکت" / "ارجاع" / "مسدود کردن" buttons.
   - Desktop notification + sound on new conversation (ask permission once, respect a mute toggle).
3. **Dashboard** — overview cards (گفتگوهای امروز، در انتظار، میانگین پاسخ اول، CSAT), a 14-day line chart (lightweight: inline SVG, no chart lib), agent leaderboard table.
4. **Visitors** — table with filters (online, reseller, device), click → drawer with history.
5. **Tickets** (Phase 2) — Kanban with drag-and-drop between status columns.
6. **Knowledge Base** (Phase 2) — list + markdown editor + FTS search preview.
7. **Settings/Widget** — live preview pane, colors, position, greeting, pre-chat form builder, business hours grid, embed-code copy box.
8. **Settings/Routing** — drag-to-reorder rule list, rule editor (conditions builder + route target), "تست قانون" with a sample visitor form.
9. **Settings/Team** — agents CRUD, role selector, telegram id, max conversations.
10. **Settings/Integrations** — EVE, Telegram, GMweb, AI toggles + connection test buttons.
11. **Settings/Webhooks** (Phase 2) — URL, event checkboxes, secret, delivery log table, "ارسال آزمایشی".

**`lib/ws.ts`** — reconnecting agent socket with the same backoff policy as the widget; on reconnect, refetch the conversation list and the open thread.

**Build output** must land in `embed/dashboard/` (Vite `build.outDir`), `base: "/"`, assets hashed.

`--- TASK 13 DONE ---`

---

# TASK 14 — BUILD, DEPLOY, DOCS

### `scripts/build.sh`
```bash
set -euo pipefail
# 1. widget
cd web/widget && npm ci && npm run build
SIZE=$(gzip -c dist/widget.js | wc -c)
[ "$SIZE" -lt 12288 ] || { echo "widget too big: $SIZE"; exit 1; }
cp dist/widget.js ../../embed/widget/
# 2. dashboard
cd ../dashboard && npm ci && npm run build   # outDir=../../embed/dashboard
# 3. binary
cd ../.. && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION:-dev}" -o evechat ./cmd/evechat
```

### `Dockerfile` — multi-stage
Stage 1 `node:22-alpine` builds widget + dashboard. Stage 2 `golang:1.23-alpine` builds the static binary. Stage 3 `gcr.io/distroless/static-debian12:nonroot` with only the binary + `/data` volume. Target image ≤ 20MB. `HEALTHCHECK` is not available in distroless — instead document using compose `healthcheck` with `wget` disabled; use a Go `--healthcheck` flag on the binary itself that curls localhost and exits 0/1.

### `docker-compose.yml`
As in the architecture doc, plus `healthcheck: ["CMD","/evechat","--healthcheck"]`, `deploy.resources.limits.memory: 128M`, and a commented `caddy`/`nginx` TLS terminator block.

### `Makefile`
`make dev` (air-style reload via `go run` + vite dev + rollup watch, 3 processes), `make build`, `make test`, `make lint`, `make migrate`, `make seed`, `make docker`.

### `README.md`
Install, config reference table, EVE embed snippet, Telegram bot setup steps, reverse-proxy sample (nginx with `proxy_read_timeout 3600s` and WS upgrade headers), backup instructions (`sqlite3 .backup` or copy with WAL checkpoint), key-rotation runbook.

`--- TASK 14 DONE ---`

---

# TASK 15 — PHASE 2 FEATURES

Implement in this order, each fully:
1. **Upload** — `POST /api/v1/upload` multipart, max size from config, allowed MIME: `image/png|jpeg|webp|gif`, `application/pdf`, `text/plain`, `application/zip`. Store as `/data/uploads/<projectID>/<yyyy>/<mm>/<ulid><ext>`. Reject by **sniffing content**, not extension. Images > 1600px → resize with `disintegration/imaging`. Return `{url, name, size, mime, width, height}`. Widget shows inline previews for images.
2. **Tickets** — repo + service + handlers + Kanban UI; auto-link to the originating conversation; internal notes array; `due_at`; status transitions logged in `internal_notes`.
3. **Knowledge Base + FTS5** — CRUD, `GET /api/v1/knowledge-base/search?q=` using `bm25()` ranking, Persian-friendly tokenizer config (`tokenize='unicode61 remove_diacritics 2'`).
4. **AI Q&A** (`services/ai.go`) — on a visitor message when `ai.enabled` and the conversation is unassigned: FTS-retrieve top 4 KB articles → Claude Haiku with a system prompt that **must** answer only from the provided articles in Persian and must reply with the exact token `NEED_HUMAN` when unsure → if `NEED_HUMAN` or after `handoff_after` consecutive AI turns, escalate to routing. Log token usage per project for metering. Timeout 12s; on any error, escalate silently.
5. **Webhooks** — fan-out worker with retry schedule `[0s, 30s, 2m, 10m, 1h, 6h]`, HMAC signature headers, delivery log, `POST /:id/test`.
6. **CSAT** — scheduled 2 minutes after resolve (in-memory timer wheel + DB-backed recovery on restart), widget renders 1–5 stars + optional comment.
7. **Canned responses**, **business hours + auto-reply**, **pre-chat form**, **visitor ban**, **conversation transfer**, **export JSON/CSV**, **proactive triggers** (`{after_seconds, page_pattern, message}` in project settings, evaluated client-side).
8. **GMweb/SMS** inbound+outbound wiring end-to-end.

`--- TASK 15 DONE ---`

---

# TASK 16 — PHASE 3 SCAFFOLDING (SaaS)

- `internal/hub/broker.go`: `type Broker interface { Publish(topic string, b []byte) error; Subscribe(topic string) (<-chan []byte, func(), error) }`; `InProcBroker` (default) + `RedisBroker` behind build tag `redis`. Hub publishes cross-node events (`room:<convID>`, `project:<id>`) through the broker; local delivery stays direct.
- Tenant plans table + usage metering (conversations/month, AI tokens, agents seats) + per-plan rate limits.
- Custom widget domains (CNAME) with autocert `HostPolicy` reading allowed domains from the DB.
- SMTP notification channel.
- Billing hooks (interface only, no provider lock-in): `BillingProvider{CreateCustomer, CreateSubscription, Webhook}`.
- Tenant admin panel routes under `/api/v1/admin/*`, `superadmin` only.

`--- TASK 16 DONE ---`

---

# TASK 17 — PHASE 4 WORDPRESS PLUGIN

`wordpress/evechat/` with `evechat.php` (header, settings page, `wp_enqueue_script` of `widget.js` with project key), `includes/settings.php`, `includes/woocommerce.php` (passes `order_id`, `customer_id`, LTV into `visitor.metadata`), `includes/shortcode.php` (`[evechat]`), `readme.txt`, and uninstall cleanup. PHP 7.4+ compatible, no Composer deps.

`--- TASK 17 DONE ---`

---

# PART 4 — GLOBAL CODING STANDARDS (apply to every file you write)

1. **Context first:** every function that touches IO takes `ctx context.Context` as its first parameter and honours cancellation.
2. **Errors:** wrap with `fmt.Errorf("doing X: %w", err)`. Sentinel errors in the package that owns them (`repo.ErrNotFound`, `repo.ErrDuplicate`, `crypto.ErrUnknownKeyVersion`). Never `panic` outside `main` and middleware recovery.
3. **No globals** except the zerolog logger and the embedded FS.
4. **Dependency injection via constructors.** No service locator, no `init()` side effects.
5. **Timeouts everywhere:** every outbound HTTP client has an explicit `Timeout`; every DB query inherits the request context; the hub never blocks.
6. **Logging:** structured fields only (`log.Info().Str("conv_id", id).Msg("assigned")`). Never log message content, tokens, keys, or full phone numbers (mask to `+98912***6789`).
7. **JSON:** all API field names `snake_case`. All timestamps RFC3339 UTC in REST, unix-millis in WS envelopes.
8. **Tests:** write table-driven tests for `crypto`, `services/routing` (at least 12 rule-matching cases), `repo/messages` dedup, `hub` concurrency, and the widget transport state machine (vitest). Nothing else is required.
9. **Persian strings** live in one place per layer (`i18n.ts` for widget, `lib/i18n.ts` for dashboard, `internal/i18n/fa.go` for server-sent texts like auto-replies and Telegram templates). No hard-coded Persian inside logic files.

---

# PART 5 — ACCEPTANCE CHECKLIST (run before declaring Phase 1 done)

```bash
# Build
CGO_ENABLED=0 go build ./... && go vet ./... && go test ./...
./scripts/build.sh

# Boot
./evechat --config evechat.yaml --seed
curl -s localhost:3000/api/v1/health | jq .     # status ok, db ok

# Widget served and small
curl -s localhost:3000/widget.js | wc -c
curl -s localhost:3000/widget.js | gzip | wc -c  # < 12288

# Session + WS round trip
TOKEN=$(curl -s -X POST localhost:3000/api/widget/v1/session \
  -H 'Content-Type: application/json' \
  -d '{"project_key":"pk_...","visitor":{"name":"Test"},"page":{"url":"http://x/test"}}' | jq -r .token)
wscat -c "ws://localhost:3000/ws/visitor?token=$TOKEN"
> {"type":"conversation.start","id":"c1","data":{"message":"سلام"}}
# expect conversation.created then message.sent

# Agent side
JWT=$(curl -s -X POST localhost:3000/api/v1/auth/login -d '{"email":"admin@evechat.local","password":"..."}' | jq -r .access_token)
wscat -c "ws://localhost:3000/ws/agent?token=$JWT"
# expect conversation.new for the conversation above; reply and verify the visitor socket receives message.received

# Dedup
# send the same client_message_id twice -> exactly one row in messages

# Offline resilience
# kill the server mid-chat, send 3 messages from the widget, restart -> all 3 arrive exactly once

# Memory
docker stats evechat --no-stream   # < 30MB idle

# Load
k6 run scripts/load_ws.js   # 1000 concurrent WS, < 60MB RSS, p95 message latency < 50ms
```

**Phase 1 is "done" only when every line above passes.**

---

# PART 6 — WHAT YOU MUST NOT DO

- Do not introduce an ORM (gorm, ent, bun).
- Do not introduce Redis, NATS, Kafka, or any broker before TASK 16.
- Do not use `gorilla/websocket` — it is `coder/websocket`.
- Do not use CGO-based SQLite (`mattn/go-sqlite3`).
- Do not store encryption keys in YAML.
- Do not put mutexes on hub state — the command channel is the only path.
- Do not block the hub goroutine on a slow client.
- Do not log plaintext message content.
- Do not ship a widget larger than 12KB gzip.
- Do not ask the user clarifying questions. Build it.

---

**BEGIN WITH TASK 01 NOW. Output files only.**
