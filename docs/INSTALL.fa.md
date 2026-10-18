<div dir="rtl">

# نصب EveChat

## نصب‌کننده یک‌خطی

```bash
curl -fsSL https://raw.githubusercontent.com/aibedini/evechat/main/install.sh | bash
```

گام‌هایی که انجام می‌شود: تشخیص سیستم → بررسی ابزارها → دریافت آخرین نسخه → دانلود و **بررسی sha256** → ساخت کاربر `evechat` → نصب در `/opt/evechat` → ساخت `.env` و `evechat.yaml` → نصب systemd → انتظار برای سلامت → اجرای ویزارد و چاپ اطلاعات ورود.

### پرچم‌ها

| پرچم | کاربرد |
|---|---|
| `--version v1.1.0` | نسخه مشخص |
| `--dir /opt/evechat` | مسیر نصب |
| `--port 8080` | پورت سرویس |
| `--public-url https://chat.example.com` | آدرس عمومی |
| `--base-url https://mirror.example.ir/evechat` | آینه دانلود |
| `--no-systemd` | نصب بدون سرویس |
| `--skip-setup` | فقط باینری |
| `--uninstall` / `--purge` | حذف نصب (پیش‌فرض داده‌ها حفظ) |

## داکر

```bash
cp .env.example .env
# EVECHAT_AUTH__JWT_SECRET و EVECHAT_ENCRYPTION_KEY_V1 را پر کنید
docker compose -f docker-compose.quick.yml up -d
```

ایمیج منتشرشده: `ghcr.io/aibedini/evechat` (linux/amd64 و arm64). برای TLS خودکار فایل `deploy/docker/compose.caddy.yml` را با متغیر `EVECHAT_DOMAIN` اجرا کنید.

## نصب آفلاین

```bash
make bundle
tar xzf evechat-offline-v1.1.0-linux-amd64.tar.gz
cd evechat-offline-v1.1.0-linux-amd64
sudo ./install-offline.sh --port 3000
```

همه فایل‌ها (باینری، unit، Caddyfile، فونت، README) داخل بسته است و هیچ درخواست شبکه‌ای انجام نمی‌شود.

## میزبانی آینه

کافی است فایل‌های هر نسخه را در ساختار زیر بگذارید و `nginx autoindex on` فعال کنید:

```
/mirror/v1.1.0/evechat_v1.1.0_linux_amd64.tar.gz
/mirror/v1.1.0/checksums.txt
```

سپس: `install.sh --base-url https://mirror.example.ir/evechat --version v1.1.0`.

## TLS و پروکسی

- **Caddy:** `deploy/caddy/Caddyfile` (خودکار Let's Encrypt؛ `flush_interval -1` برای SSE الزامی است).
- **nginx:** `deploy/nginx/evechat.conf`.
- **Traefik:** `deploy/traefik/dynamic.yml`.
- در همه حالت‌ها `server.trust_proxy: true` را تنظیم کنید.

## سرویس

```bash
sudo systemctl status evechat
sudo journalctl -u evechat -f
sudo systemctl restart evechat
```

فایل unit: `deploy/systemd/evechat.service` (سخت‌سازی‌شده: `ProtectSystem=strict`، `NoNewPrivileges`، `LimitNOFILE=65535`).

## پشتیبان خودکار

```bash
sudo cp deploy/systemd/evechat-backup.* /etc/systemd/system/
sudo systemctl enable --now evechat-backup.timer
```

</div>
