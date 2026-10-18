# وبهوک‌ها

EveChat دو جهت دارد:

## ۱) وبهوک خروجی (webhooks)

رویدادهای پروژه به آدرسی که مدیر تنظیم می‌کند POST می‌شوند.

- مسیرها: `GET/POST /api/v1/webhooks`، `GET/PUT/DELETE /api/v1/webhooks/{id}`،
  `GET /api/v1/webhooks/{id}/deliveries`، `POST /api/v1/webhooks/{id}/test`.
- رویدادها: `chat.started`، `message.received`، `chat.assigned`،
  `chat.resolved`، `ticket.created`، `csat.received` یا `*`.
- امضا: `X-EveChat-Signature` = HMAC-SHA256 روی `timestamp + body` با کلید امضای
  همان وبهوک؛ `X-EveChat-Timestamp` برای جلوگیری از replay.
- نردبان تلاش: `0s, 30s, 2m, 10m, 1h, 6h`؛ هر تلاش در
  `webhook_deliveries` ثبت می‌شود و در پنل قابل مشاهده است.
- آدرس باید از فیلتر SSRF عبور کند: فقط http/https، و هر آدرسی که به loopback،
  شبکه‌ی خصوصی، link-local، CGNAT یا metadata برسد رد می‌شود. برای مقاصد داخلی
  `security.outbound_allow_hosts` را تنظیم کنید.

## ۲) وبهوک ورودی (channels)

برای گرفتن پیام از سامانه‌های بیرونی از **کانال** استفاده کنید؛ جزئیات کامل در
`docs/CHANNELS.fa.md`. خلاصه:

```bash
curl -X POST http://127.0.0.1:3000/api/channels/ch_xxx/inbound \
  -H 'Content-Type: application/json' \
  -H 'X-Channel-Secret: <INBOUND_SECRET>' \
  -d '{"external_id":"user-42","name":"Test User","message":"hello"}'
```

- کلید ورودی در بدنه یا هدر نمی‌آید مگر در هدر `X-Channel-Secret` یا
  `?secret=`؛ پاسخ 401 برای کلید نامعتبر.
- هویت‌ها نگاشت می‌شوند (`channel_identities`)، پیام در گفتگوی همان بازدیدکننده
  می‌نشیند و در پنل بلافاصله دیده می‌شود.
- هر درخواست در `webhook_inbox` برای بازرس ثبت و پس از ۴۸ ساعت حذف می‌شود.

## تفاوت را در یک نگاه

| | وبهوک خروجی | کانال (ورودی) |
|---|---|---|
| جهت | EveChat → سامانه شما | سامانه شما → EveChat |
| اعتبارنامه | HMAC با کلید وبهوک | `X-Channel-Secret` |
| ذخیره‌سازی | `webhook_deliveries` | `webhook_inbox` (۴۸ ساعت) |
| نگاشت | ثابت (JSON استاندارد) | JSONPath قابل تنظیم |
