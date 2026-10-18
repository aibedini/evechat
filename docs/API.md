# EveChat HTTP & WebSocket API

Base URL: `https://chat.example.com`. All REST bodies are JSON, all field names
are `snake_case`, timestamps are RFC3339 UTC. Errors use:

```json
{ "code": "invalid_request", "message": "متن فارسی", "details": null }
```

Codes: `invalid_request` (400), `unauthorized` (401), `forbidden` (403),
`not_found` (404), `conflict` (409), `payload_too_large` (413), `rate_limited`
(429), `internal` (500), `setup_required` (409 — instance not configured),
`pow_required` (403 — widget must solve the challenge).

## Widget (public)

| Method | Path | Notes |
|---|---|---|
| GET | `/widget.js` | embedded bundle, `If-None-Match` supported |
| GET | `/api/widget/v1/config?project_key=` | widget settings |
| GET | `/api/widget/v1/challenge?project_key=&token=` | `{challenge, difficulty, expires_at}` |
| POST | `/api/widget/v1/session?project_key=` | `{token?, challenge?, nonce?, visitor{}, page{}}` → `{token, visitor_id, conversation?, messages?}` |
| GET | `/api/widget/v1/events?token=` | SSE; `retry: 3000`, keepalive every 20s |
| GET | `/api/widget/v1/poll?token=&since=` | long-poll up to 25s → `{events, now}` |
| POST | `/api/widget/v1/conversations/:id/messages?token=` | `{content, client_message_id}` |
| GET | `/api/widget/v1/conversations/:id/sync?token=&since=` | ring buffer or DB replay |
| POST | `/api/widget/v1/csat?token=` | `{conversation_id?, score 1..5, comment?}` |

## Auth

```
POST /api/v1/auth/login      {email, password, project_key?} → tokens + agent + project
POST /api/v1/auth/refresh    {refresh_token}
POST /api/v1/auth/forgot     {email}          # always 200
POST /api/v1/auth/reset      {token, password}
POST /api/v1/auth/logout     (Bearer)
GET  /api/v1/auth/me         (Bearer)
PUT  /api/v1/auth/password   {current_password, new_password}
```

## Conversations

```
GET    /api/v1/conversations?status=&agent=me|<id>&channel=&q=&from=&to=&limit=&offset=
GET    /api/v1/conversations/export?format=json|csv&from=&to=
GET    /api/v1/conversations/:id
POST   /api/v1/conversations/:id/assign          {agent_id}
POST   /api/v1/conversations/:id/transfer        {agent_id, note}
POST   /api/v1/conversations/:id/resolve
POST   /api/v1/conversations/:id/reopen
POST   /api/v1/conversations/:id/convert-ticket  {title, priority, tags, due_at}
GET    /api/v1/conversations/:id/messages?since=&limit=
POST   /api/v1/conversations/:id/messages        {content, content_type, client_message_id, attachments, is_internal}
POST   /api/v1/conversations/:id/note            {content}
POST   /api/v1/conversations/:id/read            {up_to_message_id}
```

Authorization: `superadmin` sees everything, `admin` sees the project, `agent`
sees assigned + pending conversations only (enforced in the SQL filter).

## Visitors, agents, rules, tickets, KB, webhooks

```
GET/PUT      /api/v1/visitors[/:id]              POST /api/v1/visitors/:id/ban {duration}
DELETE       /api/v1/visitors/:id/ban            POST /api/v1/visitors/:id/anonymize
GET          /api/v1/visitors/:id/export         GET  /api/v1/visitors/:id/conversations
CRUD         /api/v1/agents  (+ PUT /api/v1/agents/:id/password)
CRUD         /api/v1/routing-rules  (+ POST /routing-rules/reorder, POST /routing-rules/:id/test)
CRUD         /api/v1/canned-responses
CRUD         /api/v1/tickets  (+ POST /tickets/:id/status, /tickets/:id/notes)
CRUD         /api/v1/knowledge-base  (+ GET /knowledge-base/search?q=)
CRUD         /api/v1/webhooks  (+ GET /:id/deliveries, POST /:id/test)
GET          /api/v1/search/messages?q=
```

## Stats & operations

```
GET  /api/v1/stats/overview|series|agents
GET  /api/v1/health                  # public: status, version, commit, uptime_seconds, db, hub
GET  /api/v1/diagnostics             # admin+: full doctor report (?only=id,id)
GET  /api/v1/audit?action=&actor=&from=&to=&format=csv
GET  /api/v1/security/blocklist      PUT /api/v1/security/blocklist
GET  /api/v1/config                  # public runtime info
GET  /api/v1/settings/project        PUT /api/v1/settings/project
POST /api/v1/settings/project/rotate-key
POST /api/v1/upload                  # multipart field "file"
GET  /api/v1/integrations/status
POST /api/v1/integrations/telegram/webhook     # X-Telegram-Bot-Api-Secret-Token
POST /api/v1/integrations/gmweb/inbound        # X-Webhook-Secret
GET  /api/v1/admin/{projects,plans,usage}      # superadmin
GET  /metrics                                  # metrics.enabled + token/loopback
GET  /api/openapi.yaml · /api/docs
```

## WebSocket protocol

`GET /ws/visitor?token=<session>&project=<id>` and `GET /ws/agent?token=<JWT>`
(browsers cannot set headers; the query token is the primary form).

Envelope: `{"type","id"?,"data"?,"ts"}` (server `ts` is unix millis).

Visitor → server: `conversation.start`, `message.send`, `typing.start|stop`,
`read.ack`, `sync.request`, `csat.submit`, `ping`.

Server → visitor: `conversation.created`, `message.sent` (ack `{id, client_id,
created_at}`), `message.received`, `agent.assigned`, `agent.typing[.stop]`,
`conversation.resolved`, `conversation.updated`, `sync.response`, `csat.request`,
`error`, `pong`.

Agent → server: `message.send`, `note.add`, `typing.start|stop`,
`conversation.assign|transfer|resolve|reopen`, `read.ack`, `presence.set`, `ping`.

Server → agent: `conversation.new`, `conversation.updated`, `message.received`,
`visitor.typing[.stop]`, `visitor.online|offline`, `presence.update`,
`stats.tick`, `note.added`, `sync.response`, `error`, `pong`.

`sync.request {since: <unix_ms>}` is answered from the in-memory ring (last 50
envelopes per conversation) or the database when the ring no longer covers it.
