# Email channel (SMTP)

EveChat can send Persian, RTL transactional email for four events:

| Event | Recipients | Template |
|---|---|---|
| New conversation | assigned agent / project inbox | `NewConversation` |
| Missed conversation digest | project inbox | `DailyDigest` |
| Password reset | the requesting agent | `PasswordReset` |
| New CSAT response | assigned agent | `CSATReceived` |

## Configuration

YAML (recommended) plus the password from the environment:

```yaml
smtp:
  enabled: true
  host: "smtp.example.com"
  port: 587
  username: "support@example.com"
  from_email: "support@example.com"
  from_name: "پشتیبانی EVE"
  tls_mode: "starttls"      # plain | starttls | implicit
  timeout: "15s"
```

```bash
EVECHAT_SMTP__PASSWORD=...
```

Ports: 587 + `starttls`, 465 + `implicit`, 25 + `plain` (only inside a trusted
network). The client always verifies the server certificate; there is no
"skip verification" switch on purpose.

## Testing

```bash
evechat doctor --only integration.smtp --verbose
```

or Settings → integrations → SMTP → «تست اتصال» (the web wizard also probes it
before saving). A failed probe returns the SMTP error verbatim in Persian.

## Templates

`internal/i18n/email/templates.go` builds inline-CSS HTML with a Vazirmatn
`@font-face` pointing at `/assets/fonts/vazirmatn-variable.woff2` and a Tahoma
fallback (Outlook ignores webfonts). All strings are Persian and RTL; every
message has a plain-text alternative. To customise, edit the template functions
— they are embedded, so rebuild the binary.

## Password reset flow

1. `POST /api/v1/auth/forgot {email}` → always `200` (no account enumeration).
2. A single-use token (32 random bytes, SHA-256 hashed at rest) is stored with a
   30-minute TTL and emailed as `https://<public_url>/reset?token=...`.
3. `POST /api/v1/auth/reset {token, password}` consumes it and revokes all
   refresh tokens for that agent.
4. Expired tokens are pruned hourly; only the hash is ever stored.

If SMTP is disabled the endpoint still returns `200` but logs a warning; the
operator can reset a password locally with `evechat user password`.

## Scheduled jobs

The two background emails run as one-shot systemd units, not inside the server:

```bash
sudo cp deploy/systemd/evechat@.service /etc/systemd/system/
sudo cp deploy/systemd/evechat-digest.timer deploy/systemd/evechat-missed.timer /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now evechat-digest.timer evechat-missed.timer
```

| Job | Schedule | Command |
|---|---|---|
| missed chat reminder | every 15 min | `evechat job missed --older-than 10m` |
| daily digest | 08:00 local | `evechat job digest` |

Both read recipients from `notification_email`, `alert_email` or
`support_email` project settings and fall back to active admins/superadmins.
Without a configured SMTP host they log and exit 0, so a timer never fails
noisily on a mail-less install. `--quiet` keeps cron logs clean.