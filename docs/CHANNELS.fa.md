# کانال‌ها (وبهوک دوطرفه)

کانال یعنی یک سامانه‌ی بیرونی (CRM، تلگرام، درگاه پیامک یا یک اسکریپت) که هم
می‌تواند وارد گفتگو شود و هم پاسخ دریافت کند. هر کانال یک رکورد در جدول
`channels` است و به یک پروژه تعلق دارد.

## جدول‌ها

| جدول | کار |
|---|---|
| `channels` | تنظیمات کانال: نام، نوع، کلید ورودی، آدرس بازگشت، نگاشت JSONPath |
| `channel_identities` | نگاشت `external_id` هر سامانه به یک `visitor_id`؛ هر هویت یک بازدیدکننده |
| `webhook_inbox` | هر درخواست ورودی برای بازرس؛ ۴۸ ساعت نگه‌داری می‌شود |

## مسیرهای API

| متد | مسیر | کار |
|---|---|---|
| GET | `/api/v1/channels` | فهرست کانال‌ها |
| POST | `/api/v1/channels` | ساخت کانال (کلید ورودی یک‌بار برمی‌گردد) |
| GET | `/api/v1/channels/{id}` | یک کانال |
| PUT | `/api/v1/channels/{id}` | ویرایش |
| DELETE | `/api/v1/channels/{id}` | حذف |
| POST | `/api/v1/channels/{id}/rotate-secret` | چرخش کلید ورودی |
| POST | `/api/v1/channels/{id}/test` | ارسال یک پیام آزمایشی از مسیر واقعی ingest |
| POST | `/api/v1/channels/{id}/preview` | پیش‌نمایش زنده‌ی نگاشت روی یک بدنه |
| GET | `/api/v1/channels/{id}/inbox` | بازرس: آخرین درخواست‌های ورودی |
| GET | `/api/v1/channels/{id}/curl` | تولیدکننده‌ی دستور curl |
| POST | `/api/channels/{id}/inbound` | **عمومی**: ورودی کانال (اعتبارنامه = `X-Channel-Secret`) |

مسیر عمومی JWT نمی‌خواهد و از `MutationHook` بیرون است؛ به‌جایش کلید کانال با
مقایسه‌ی constant-time بررسی می‌شود.

## نگاشت ورودی (JSONPath)

فیلد `mapping` یک شیء JSON است:

```json
{
  "external_id": "$.user.id",
  "name": "$.user.name",
  "email": "$.user.email",
  "message": "$.message.text",
  "metadata": "$.user"
}
```

زیرمجموعه‌ی پشتیبانی‌شده: `$.a.b`، `$.items[0].x`، `$['a b']`. وایلدکارد،
فیلتر و recursive descent **پشتیبانی نمی‌شود** و مسیر نامعتبر باعث رد درخواست
می‌شود؛ نگاشت، کانفیگ است و نباید بتواند مثل یک زبان برنامه‌نویسی رفتار کند.
پیش‌فرض‌ها: `external_id → $.external_id` و `message → $.message`.

## پردازش ورودی — ده گام به همین ترتیب

1. کانال وجود دارد و `is_active` است (وگرنه 404).
2. کلید `X-Channel-Secret` (یا `?secret=`) با constant-time مقایسه می‌شود.
3. محدودیت نرخ per-channel+IP (پیش‌فرض ۱۲۰ در دقیقه) اعمال می‌شود.
4. بدنه حداکثر ۲۵۶KB خوانده و به‌عنوان شیء JSON پارس می‌شود.
5. نگاشت JSONPath هویت، نام، ایمیل، متن و متادیتا را بیرون می‌کشد.
6. `channel_identities` هویت را به بازدیدکننده تبدیل می‌کند؛ اولین بار هر دو ساخته می‌شوند.
7. گفتگوی باز همان بازدیدکننده استفاده می‌شود، وگرنه گفتگوی تازه با `channel = kind` ساخته می‌شود.
8. پیام رمزنگاری و ذخیره می‌شود و در اتاق گفتگو broadcast می‌شود.
9. اگر `outbound_url` تنظیم شده باشد، رسید با نردبان retry ارسال می‌شود.
10. نتیجه در `webhook_inbox` ثبت می‌شود و به فراخواننده برگردانده می‌شود.

خطا در هر گام، رکورد بازرس را با `status=rejected` و متن خطا ثبت می‌کند.

## خروجی و نردبان retry

بدنه‌ی خروجی:

```json
{"event":"message.received","channel_id":"ch_...","conversation_id":"conv_...","message_id":"msg_...","text":"...","timestamp":1700000000}
```

امضا: `X-EveChat-Signature` = HMAC-SHA256 روی `timestamp + body` با
`outbound_secret` (همان قرارداد وبهوک‌های خروجی). نردبان تلاش:
`0s, 30s, 2m, 10m, 1h, 6h`. بعد از تلاش آخر، یک **پیام سیستمی** در همان
گفتگو نوشته می‌شود تا تیم متوجه قطعی کانال شود. آدرس بازگشت از فیلتر SSRF
عبور می‌کند (`security.ValidateOutboundURL`).

## بازرس و نگه‌داری

هر درخواست یک رکورد در `webhook_inbox` دارد: متد، IP، وضعیت
(`received|processed|rejected`)، خطا، بدنه‌ی پاک‌سازی‌شده و خروجی نگاشت.
بدنه قبل از ذخیره با همان قواعد audit پاک‌سازی می‌شود (`password`، `token`،
`secret`، `key` → `***`). رکوردها هر ساعت پاک‌سازی و بعد از **۴۸ ساعت** حذف
می‌شوند.

## نمونه‌ی curl

```bash
curl -X POST http://127.0.0.1:3000/api/channels/ch_xxx/inbound \
  -H 'Content-Type: application/json' \
  -H 'X-Channel-Secret: <INBOUND_SECRET>' \
  -d '{"external_id":"user-42","name":"Test User","message":"hello"}'
```

## محدودیت نسخه‌ی فعلی

ارسال پیام **کارشناس** به سامانه‌ی بیرونی هنوز پیاده نشده است؛ خروجی فعلی
رسیدِ `message.received` است. برای این کار باید کانال به لایه‌ی notification
وصل شود (پیگیری در TASK 36 phase 2).
