# Security Policy

## Supported versions

| Version | Supported |
|---------|-----------|
| 1.1.x   | ✅ |
| 1.0.x   | security fixes only |
| < 1.0   | ❌ |

## Reporting a vulnerability

Please **do not open a public issue**. Use GitHub's private vulnerability
reporting (Security → Report a vulnerability) or email the maintainers listed
in the repository profile.

Include: affected version (`evechat version`), installation method,
reproduction steps, impact and any logs (redact tokens and message content).

You can expect an acknowledgement within 72 hours. We aim to ship a fix within
30 days for high-severity issues; the full disclosure timeline is 90 days after
the report, coordinated with you.

## Security model (read before reporting)

- **Message content is encrypted at rest** with AES-256-GCM using keys supplied
  through environment variables (`EVECHAT_ENCRYPTION_KEY_V<n>`). The operator
  holds the keys: anyone with root access to the host and the `.env` file can
  read messages. This is by design for a self-hosted product.
- The JWT secret, encryption keys and integration tokens are never stored in
  YAML, the database (integration secrets are encrypted), logs, audit diffs or
  backups by default.
- Visitor sessions are bearer tokens scoped to one project and one visitor.
- Dashboard access is role-based (`superadmin`, `admin`, `agent`); plain agents
  only see their own and unassigned conversations.
- The widget is protected by a project key, a domain allow-list, per-IP rate
  limits, a proof-of-work challenge with adaptive difficulty and a honeypot.
- Webhooks are signed with HMAC-SHA256 (`X-EveChat-Signature:
  sha256=<hex>` over `<timestamp>.<body>`).
- Outbound HTTP clients all have explicit timeouts; inbound uploads are limited
  by size and sniffed content type, and images are re-encoded to strip metadata.

## Hardening checklist for operators

1. Set a unique 48-character JWT secret and 32-byte encryption key, stored in a
   `0600` `.env` (the installer does this).
2. Put EveChat behind a TLS terminator and set `server.trust_proxy: true`.
3. Keep `security.pow_difficulty` ≥ 16 on public instances.
4. Restrict `/metrics` with `metrics.token` or a loopback-only listener.
5. Enable the systemd hardening unit (`deploy/systemd/evechat.service`).
6. Back up `.env` separately from database backups; without the key, messages
   are unrecoverable.
7. Review Settings → گزارش فعالیت for unexpected admin actions.
