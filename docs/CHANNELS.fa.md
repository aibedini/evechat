# کانال‌ها (وبهوک دوطرفه)

کانال یعنی یک سامانه‌ی بیرونی (CRM، تلگرام، درگاه پیامک یا یک اسکریپت) که هم
می‌تواند وارد گفتگو شود و هم پاسخ دریافت کند. هر کانال یک رکورد در جدول
`channels` است و به یک پروژه تعلق دارد.

## جدول‌ها

| جدول | کار |
|---|---|
| `channels` | تنظیمات کانال: نام، نوع، کلید ورودی، آدرس بازگشت، هدرهای خروجی، مدت ادامه‌ی گفتگو و نگاشت JSONPath |
| `channel_identities` | نگاشت `external_id` هر سامانه به یک `visitor_id`؛ هر هویت یک بازدیدکننده |
| `webhook_inbox` | هر درخواست ورودی و هر ارسال آزمایشی خروجی برای بازرس؛ ۴۸ ساعت نگه‌داری می‌شود |

## مسیرهای API

| متد | مسیر | کار |
|---|---|---|
| GET | `/api/v1/channels` | فهرست کانال‌ها |
| POST | `/api/v1/channels` | ساخت کانال (کلید ورودی یک‌بار برمی‌گردد) |
| GET | `/api/v1/channels/{id}` | یک کانال |
| PUT | `/api/v1/channels/{id}` | ویرایش |
| DELETE | `/api/v1/channels/{id}` | حذف |
| POST | `/api/v1/channels/{id}/rotate-secret` | چرخش کلید ورودی |
| POST | `/api/v1/channels/{id}/test` | ارسال آزمایشی واقعی به آدرس بازگشت با هدرهای تنظیم‌شده و ثبت نتیجه در بازرس |
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
7. اگر گفتگوی باز همان بازدیدکننده هست و آخرین پیامش از `session_ttl_seconds` قدیمی‌تر نیست، همان گفتگو ادامه می‌یابد؛ وگرنه گفتگوی تازه با `channel = kind` ساخته می‌شود. مقدار `0` یعنی همیشه گفتگوی تازه.
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

## هدرهای خروجی

هر کانال می‌تواند تا ۲۰ هدر دلخواه برای درخواست‌های خروجی داشته باشد؛ مثلاً:

```json
{"X-API-Key": "<secret>", "X-Tenant": "acme"}
```

هدرهای `Host`، `Content-Length`، `Transfer-Encoding`، `Connection` و
هدرهای مشابه را خود `net/http` تنظیم می‌کند و پذیرفته نمی‌شوند. هدرهای
`Content-Type`، `User-Agent`، `X-EveChat-Channel`، `X-EveChat-Timestamp` و
`X-EveChat-Signature` بعد از هدرهای کاربر ست می‌شوند تا قابل بازنویسی نباشند.

مقدار هدرهای حساس (هر نامی که شامل `key`، `secret`، `token`، `auth`،
`password`، `cookie` یا `signature` باشد) در پاسخ API با `••••••` پوشانده
می‌شود. رابط کاربری همان ماسک را در ویرایش برمی‌گرداند و سرور مقدار ذخیره‌شده
را نگه می‌دارد؛ مقدار خام هرگز در لاگ، audit یا بدنه‌ی بازرس نوشته نمی‌شود.

دکمه‌ی «ارسال آزمایشی» یک درخواست واقعی با همین هدرها به `outbound_url`
می‌فرستد و یک رکورد `OUTBOUND TEST` با هدرهای ماسک‌شده و کد وضعیت در بازرس
ثبت می‌کند. برای تست محلی با مقصد loopback باید میزبان در
`security.outbound_allow_hosts` باشد.

## بازرس و نگه‌داری

هر درخواست یک رکورد در `webhook_inbox` دارد: متد، IP، وضعیت
(`received|processed|rejected|sent|failed`)، خطا، بدنه‌ی پاک‌سازی‌شده و خروجی
نگاشت. رکوردهای `OUTBOUND TEST` هدرهای ماسک‌شده را در بدنه دارند. بدنه قبل از
ذخیره با همان قواعد audit پاک‌سازی می‌شود (`password`، `token`، `secret`،
`key` → `***`). رکوردها هر ساعت پاک‌سازی و بعد از **۴۸ ساعت** حذف می‌شوند.

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

## اختلاف مسیرهای پلن ۳۶.۳ با router

| مسیر پلن | وضعیت | تصمیم |
|---|---|---|
| `POST /channels/:id/inbound` | پیاده‌شده (id-based) | slug مبهم است؛ id می‌ماند |
| `POST /channels/:id/status` | **پیاده نشد** | فعلاً وضعیت تحویل از پاسخ خروجی ثبت می‌شود؛ افزودن آن نیاز به جدول delivery دارد |
| `POST /channels/:id/heartbeat` | **پیاده نشد** | سلامت کانال از آخرین inbound/outbound محاسبه می‌شود |
| `GET /channels` و CRUD | پیاده‌شده | — |
| `POST /channels/:id/test` | پیاده‌شده | — |
| `POST /channels/:id/rotate-secret` | پیاده‌شده | — |
| `GET /channels/:id/inbox` | پیاده‌شده (فهرست ورودی) | — |
| `POST /webhooks/inbox/:id/replay` | **پیاده نشد** | بازپخش نیاز به بازسازی امضا و idempotency دارد |
| بازرس `/demo/webhooks` | **پیاده نشد** | خارج از دامنه‌ی v1.2.1 |