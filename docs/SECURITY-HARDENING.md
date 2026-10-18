# Security hardening

This is the operator checklist behind [SECURITY.md](../SECURITY.md). Every item
maps to code in this repository.

## Transport & deployment

- Terminate TLS with Caddy/nginx/Traefik (`deploy/`); set
  `server.trust_proxy: true` so rate limits key on the real IP.
- The dashboard sends `Content-Security-Policy`, `X-Content-Type-Options`,
  `Referrer-Policy` and `Permissions-Policy` (`middleware/security.go`).
  The widget endpoint deliberately omits `X-Frame-Options` so it can be embedded.
- Uploads are served with `Content-Disposition: attachment` and a
  `Content-Security-Policy: default-src 'none'` on the asset path.

## Secrets

- JWT secret, encryption keys and integration credentials come from the
  environment only (`EVECHAT_*`); `.env` is `0600` and never committed.
- Integration settings saved from the wizard are encrypted with AES-256-GCM in
  `app_settings`; the key id prefix allows rotation.
- Audit diffs redact anything containing `password`, `secret`, `token`, `key`,
  `authorization` or `api_key` recursively.
- Logs never include tokens; `logx` drops `Authorization` and `Set-Cookie`.

## Authentication & sessions

- JWT HS256 access tokens (24h) + refresh tokens (30 days) with separate `typ`;
  refresh rotates and the old one is revoked.
- bcrypt cost 12 for agents; password reset tokens are single-use, hashed and
  expire after 30 minutes; requesting a reset never reveals whether the email
  exists.
- Roles: `superadmin` → all projects, `admin` → own project, `agent` → assigned +
  pending conversations (enforced in SQL, not only in the UI).
- Login rate limit per IP+email; repeated failures are logged.

## Abuse protection

- Proof of work per session (SHA-256, 16 leading zero bits by default, adaptive
  +2/+4/+6 for busy IPs, +4 for low-reputation visitors) with a 2-minute TTL and
  single-use challenges. `security.pow_difficulty: 0` disables it.
- Honeypot field `visitor.website` silently rejects bots.
- Per-visitor conversation limits, per-conversation messages/minute limits and
  widget/IP rate limits.
- Visitor reputation decreases on ban; runtime blocklists accept IPs, CIDRs and
  user-agent substrings and are hot-reloaded (no restart).
- Uploads: max 10 MB, magic-byte sniffing (not the client MIME), image
  re-encoding through `imaging` (strips EXIF and polyglots), attachment
  filenames sanitised, zip/PDF stored as opaque attachments.

## Data protection

- Messages are AES-256-GCM encrypted at rest with a random nonce; the key version
  is stored per row so rotation is safe (`evechat key rotate`).
- Backup archives exclude the encryption key by design; restoring without it is
  impossible, which is documented in the backup output.
- CSAT comments and visitor metadata are covered by the retention janitor
  (`audit.retention_days`, visitor anonymisation endpoint).
- `evechat backup --encrypt` adds AES-256-GCM with an operator passphrase.

## Container & host

- Image runs as non-root `65532`, read-only root filesystem, `no-new-privileges`,
  all capabilities dropped, `/data` the only writable mount
  (`docker-compose.yml`).
- systemd unit: `ProtectSystem=strict`, `ProtectHome`, `PrivateTmp`,
  `NoNewPrivileges`, `SystemCallFilter=@system-service`, `LimitNOFILE=65535`.
- SQLite is never placed on NFS/SMB (`database.path` check in doctor).

## Verification

```bash
evechat doctor --verbose          # includes env permissions, key versions, offsets
curl -s localhost:3000/metrics | grep rate_limited
node scripts/contrast-check.mjs   # UI contrast budget
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```
