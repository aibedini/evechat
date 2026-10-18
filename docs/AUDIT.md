# Audit log

Every sensitive action is appended to `audit_log` with actor, IP, user agent and
a JSON detail diff. Passwords, tokens, secrets and API keys are replaced by
`***` before the row is written (`services.redact`), recursively.

## What is recorded

| Action | Source |
|---|---|
| `setup.admin_created`, `setup.project_created`, `setup.completed`, `setup.integration_tested` | setup wizard |
| `security.blocklist_updated` | Settings → security |
| `settings.project.update`, `security.api_key_rotated`, `settings.rotate-key` | generic hook |
| `agents.create/update/delete`, `agents.password` | generic hook |
| `routing-rules.*`, `webhooks.*`, `knowledge-base.*`, `tickets.*` | generic hook |
| `visitors.ban`, `visitors.anonymize` | generic hook |
| `conversations.assign|transfer|resolve|reopen|convert-ticket` | generic hook |

The generic hook (`handlers.AuditMutation`) runs inside the auth middleware for
every authenticated `POST/PUT/PATCH/DELETE` that returns 2xx, so new endpoints
are covered automatically. Readings (`GET`) are never audited.

## Storage & retention

- Table: `audit_log` (migrations `008_audit`), indexed by project, action and
  `created_at`.
- Retention: `audit.retention_days` (default 180). The janitor deletes older
  rows every hour; a manual `evechat migrate` run also prunes once.
- No message content is stored; only ids and metadata.

## Reading it

- Dashboard → تنظیمات → گزارش فعالیت (filters, detail modal, CSV export).
- `GET /api/v1/audit?action=&actor=&from=&to=&limit=&format=csv` (admin+).
- SQL: `SELECT created_at, action, actor_name, ip FROM audit_log ORDER BY created_at DESC LIMIT 50;`

## Responding to an incident

1. Filter by `actor` or `ip` around the incident window and export CSV.
2. Rotate the API key (Settings → Widget) and integration secrets.
3. Force password resets for affected agents (`PUT /api/v1/agents/:id/password`
   or the login "forgot password" flow).
4. Keep the CSV with your incident notes; it is the authoritative trail because
   rows are append-only (the application never updates or deletes single rows).
