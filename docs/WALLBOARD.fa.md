# صفحه‌ی نمایش (Wallboard)

یک صفحه‌ی فقط‌خواندنی برای تلویزیون اتاق پشتیبانی. هیچ محتوای پیام یا اطلاعات
شناسایی‌کننده‌ی بازدیدکننده در آن نیست.

## ساخت لینک

از **تنظیمات ← ویجت ← صفحه‌ی نمایش** دکمه‌ی «ساخت لینک» را بزنید، یا:

```bash
curl -X POST http://127.0.0.1:3000/api/v1/settings/project/wallboard-token \
  -H "Authorization: Bearer <TOKEN>"
```

پاسخ:

```json
{"token":"...","url":"http://127.0.0.1:3000/wallboard?token=..."}
```

توکن فقط یک بار نمایش داده می‌شود؛ اگر گم شد، توکن جدید بسازید (توکن قبلی باطل
می‌شود). برای لغو:

```bash
curl -X DELETE http://127.0.0.1:3000/api/v1/settings/project/wallboard-token \
  -H "Authorization: Bearer <TOKEN>"
```

## داده‌ها

`GET /api/v1/wallboard?token=...` (عمومی، بدون JWT؛ توکن اعتبارنامه است و با
مقایسه‌ی constant-time بررسی می‌شود):

```json
{
  "project": "EVE",
  "generated_at": "2026-10-18T13:20:00Z",
  "window": "24h",
  "agents": {"online": 2, "total": 5},
  "pending": 7,
  "active": 3,
  "resolved": 21,
  "csat_avg": 4.6,
  "first_response_seconds": 38
}
```

هیچ پیام، نام بازدیدکننده، ایمیل، شماره یا IP در پاسخ نیست. صفحه هر ۱۵ ثانیه
خودش را به‌روز می‌کند.

## امنیت

- توکن ۲۴ بایتی تصادفی، ذخیره‌شده در `projects.settings.wallboard_token`.
- توکن نامعتبر → `401`؛ بدون توکن → `401`.
- مسیر در فهرست مسیرهای عمومی JWT است، پس نیازی به نشست پنل ندارد؛ در عوض
  هیچ عملیات نوشتنی روی آن وجود ندارد.
- چرخش/لغو توکن در گزارش فعالیت ثبت می‌شود (`settings.wallboard_token_rotated`).
