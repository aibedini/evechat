# Upgrade guide

## Before you upgrade

1. `evechat doctor --verbose` and fix anything red.
2. Back up: `evechat backup /backup/pre-upgrade --with-uploads`.
3. Store `.env` separately: without `EVECHAT_ENCRYPTION_KEY_V<n>` the messages
   cannot be decrypted after a disaster.
4. Read `CHANGELOG.md` for breaking changes (SemVer: major only).

## Standard upgrade

```bash
sudo evechat update --check
sudo evechat update
```

`evechat update` performs: GitHub release lookup → asset download → SHA-256
verification → pre-update backup → binary swap (`.prev` kept) → service restart
→ health poll. On a failed health check the previous binary is restored and the
service restarted again.

Manual alternative:

```bash
sudo systemctl stop evechat
sudo cp /opt/evechat/evechat /opt/evechat/evechat.prev
sudo install -m 0755 evechat /opt/evechat/evechat
sudo -u evechat /opt/evechat/evechat --config /opt/evechat/evechat.yaml migrate
sudo systemctl start evechat
curl -fsS localhost:3000/api/v1/health
```

## Database migrations

Migrations are embedded and applied automatically at startup (and by
`ExecStartPre=… migrate` in the systemd unit). They are forward-only and each
runs in a transaction; `schema_migrations` records applied versions. There is no
down-migration: restore a backup to roll back.

## Rolling back

```bash
sudo systemctl stop evechat
sudo mv /opt/evechat/evechat.prev /opt/evechat/evechat
sudo systemctl start evechat
```

If the newer version already ran migrations, also restore the pre-upgrade
backup: `evechat restore /backup/pre-upgrade.tar.gz`.

## 1.0.x → 1.1.x checklist

- New migrations `007_setup`, `008_audit`, `009_security` run automatically.
- `security.pow_difficulty` defaults to 16: older widgets (pre-1.1) cannot open
  sessions. Re-embed `/widget.js` or set the value to `0` temporarily.
- First-run gate: if `app_settings.setup_completed` is absent, the instance
  redirects to `/setup`. Existing installs are marked complete automatically by
  the seed/wizard; if you upgraded manually, run
  `evechat setup --non-interactive` once (it is idempotent).
- `Vazirmatn` fonts add ~217 KB to the binary; no API change.
- `/metrics` exists but is disabled unless `metrics.enabled=true`.
