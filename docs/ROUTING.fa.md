<div dir="rtl">

# دستور پخت قوانین مسیریابی

هر قانون یک شیء `conditions` و یک `route_to` دارد. همه شرط‌های موجود باید برقرار باشند (AND). قانون‌ها به ترتیب `priority` نزولی بررسی می‌شوند و **اولین تطبیق** برنده است؛ اگر هیچ قانونی تطبیق نکند، موتور ظرفیت (کم‌بارترین کارشناس آنلاین با رعایت `max_conversations`) و سپس `default_telegram_id` پروژه را انتخاب می‌کند.

## شرط‌ها

| کلید | نوع | مثال |
|---|---|---|
| `domains` | لیست دامنه | `["eve.ir"]` یا `["ir"]` برای همه زیردامنه‌ها |
| `pages` | glob (فقط `*`) | `["*/billing*", "/subscription/*"]` |
| `device_types` | `mobile`/`tablet`/`desktop` | `["mobile"]` |
| `languages` | پیشوند زبان | `["fa"]` (با `fa-IR` هم می‌خواند) |
| `eve_reseller_id` | لیست شناسه | `["reseller_abc"]` |
| `has_subscription` | بولین | `true` = اشتراک فعال دارد |
| `business_hours` | بولین | `false` = خارج از ساعت کاری |
| `variables` | متغیر متادیتا | `{"utm_source": ["google","bing"]}` |

## مقصدها (`route_to`)

```json
{ "agent_id": "agt_...", "telegram_id": "123456789", "phone": "09120000000" }
```

- `agent_id`: کارشناس مشخص (باید در همان پروژه باشد).
- `telegram_id`: چت پشتیبانی؛ ربات پیام را با دکمه‌های بستن/ارجاع می‌فرستد.
- `phone`: ارسال پیامک اطلاع‌رسانی از طریق GMweb.

## نمونه‌ها

```json
{
  "name": "صفحه پرداخت → کارشناس فروش",
  "priority": 100,
  "conditions": { "pages": ["*/billing*", "*/subscription*", "*/payment*"] },
  "route_to": { "agent_id": "agt_01..." },
  "auto_reply": "سلام 👋 برای سؤال درباره پرداخت، همکاران فروش در خدمت شما هستند."
}
```

```json
{
  "name": "شب و خارج از ساعت کاری → تلگرام",
  "priority": 50,
  "conditions": { "business_hours": false },
  "route_to": { "telegram_id": "123456789" },
  "auto_reply": "پیام شما ثبت شد؛ خارج از ساعات کاری در اولین فرصت پاسخ می‌دهیم."
}
```

```json
{
  "name": "نمایندگی طلایی → تیم ارشد",
  "priority": 80,
  "conditions": {
    "eve_reseller_id": ["reseller_gold"],
    "has_subscription": true,
    "device_types": ["mobile", "tablet"]
  },
  "route_to": { "agent_id": "agt_..." }
}
```

## تست قانون

- در پنل: تنظیمات ← مسیریابی ← «تست قانون»، نمونه کاربر را پر کنید.
- در API: `POST /api/v1/routing-rules/:id/test` با `{visitor_sample:{device_type, language, eve_reseller_id, metadata, subscription}, page:{url,title}}`.
- پاسخ شامل `matched_rule`، `decision` و `business_hours` است.

## نکته‌ها

- ترتیب را با کشیدن ردیف‌ها تغییر دهید؛ `priority` به‌صورت ۱۰۰، ۹۰، ۸۰… بازنویسی می‌شود.
- `business_hours` از تنظیمات ویجت خوانده می‌شود: `enabled`، `timezone`، `days` (۰=یکشنبه)، `start`، `end`. بازه‌های شبانه (۲۱:۰۰ تا ۰۳:۰۰) پشتیبانی می‌شوند. اگر فعال نباشد «همیشه باز» فرض می‌شود.
- `auto_reply` پس از ایجاد گفتگو ارسال می‌شود (پیام ربات، نه کارشناس).
- اگر هیچ کارشناسی آنلاین نباشد و مقصد تلگرام هم تعیین نشده باشد، گفتگو در صف `pending` می‌ماند و به همه کارشناسان اعلان می‌شود.

</div>
