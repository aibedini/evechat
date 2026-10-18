<div dir="rtl">

# مرجع پیکربندی

ترتیب اولویت: پیش‌فرض‌ها → `evechat.yaml` → محیط. متغیر محیطی: `EVECHAT_` + مسیر با `__` (مثلاً `EVECHAT_SERVER__PORT`).

## server

| کلید | پیش‌فرض | توضیح |
|---|---|---|
| `host` | `0.0.0.0` | آدرس شنود |
| `port` | `3000` | پورت |
| `public_url` | `http://localhost:3000` | آدرس عمومی؛ CORS، لینک‌ها و کد embed از آن ساخته می‌شود |
| `trust_proxy` | `false` | اعتماد به `X-Forwarded-For` (پشت پروکسی `true`) |
| `read_timeout` | `15s` | تایم‌اوت خواندن |
| `shutdown_timeout` | `20s` | مهلت خاموشی نرم |
| `tls.enabled` | `false` | سرو TLS مستقیم |

توجه: `WriteTimeout` عمداً صفر است چون SSE و long-poll باید باز بمانند؛ محدودیت را در پروکسی اعمال کنید.

## database

| کلید | پیش‌فرض | توضیح |
|---|---|---|
| `path` | `/data/evechat.db` | مسیر فایل؛ روی دیسک محلی باشد |
| `max_open_conns` | `1` | pool نوشتن (SQLite تک‌نویسنده) |
| `max_read_conns` | `4` | pool خواندن زیر WAL |
| `busy_timeout_ms` | `5000` | مهلت قفل |

## storage

| کلید | پیش‌فرض |
|---|---|
| `uploads_dir` | `/data/uploads` |
| `max_file_size_mb` | `10` |

## auth / encryption

- `auth.jwt_secret` **الزامی** (حداقل ۲۴ کاراکتر) — فقط از محیط.
- `auth.access_expiry` = `24h`، `refresh_expiry` = `720h`، `bcrypt_cost` = `12`.
- `encryption.current_version` = `1`؛ کلیدها فقط از `EVECHAT_ENCRYPTION_KEY_V1..V9`.

## hub

`ping_interval_seconds=30`، `pong_wait_seconds=90`، `send_buffer_size=256`، `ring_buffer_size=50`، `max_message_bytes=32768`.

## ratelimit

`widget_session_per_min=10`، `message_per_min=60`، `login_per_min=5`.

## security

| کلید | پیش‌فرض | توضیح |
|---|---|---|
| `pow_difficulty` | `16` | بیت‌های صفر پیشرو؛ ۰ = غیرفعال |
| `messages_per_conversation_per_min` | `30` | ضد اسپم پیام |
| `conversations_per_visitor_per_hour` | `5` | سقف گفتگوی جدید |
| `blocked_ips` / `blocked_cidrs` / `blocked_user_agents` | — | لیست سیاه اولیه (از پنل هم قابل تغییر) |

## metrics / audit / log

- `metrics.enabled=false`، `metrics.token` اختیاری (در غیر این صورت فقط loopback).
- `audit.retention_days=180`.
- `log.level=info`، `log.pretty=false`، `log.file` خالی، `log.max_size_mb=50`، `log.max_backups=5`.

## telegram / smtp / ai / integrations

```yaml
telegram:
  enabled: true
  webhook_url: "https://chat.example.com/api/v1/integrations/telegram/webhook"
  # bot_token و secret از محیط

smtp:
  enabled: true
  host: "smtp.example.com"
  port: 587
  username: "support@example.com"
  from_email: "support@example.com"
  from_name: "پشتیبانی EVE"
  tls_mode: "starttls"     # plain | starttls | implicit

ai:
  enabled: true
  model: "claude-haiku-4-5"
  max_tokens: 512
  handoff_after: 3
  # claude_api_key از محیط

integrations:
  eve:
    base_url: "https://api.eve.ir"
    require_signature: true
  gmweb:
    base_url: "https://sms.example.ir"
```

کلیدهای حساس در `.env` (mode 0600) قرار می‌گیرند و توسط `evechat setup` یا ویزارد وب مدیریت می‌شوند. اسرار یکپارچه‌سازی‌های ثبت‌شده در ویزارد وب، در `app_settings` **رمزنگاری‌شده** ذخیره می‌شوند و هنگام بالا آمدن روی YAML اعمال می‌گردند.

</div>
