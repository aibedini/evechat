<div dir="rtl">

# اجرای EveChat کنار سرور EVE

EveChat یک سرویس جدا با پورت، پایگاه داده و کاربر سیستمی خودش است. به EVE دست
نمی‌زند؛ تنها چیزی که به صفحه‌ی EVE اضافه می‌شود یک `<script async>` است. این
سند همان مسیری است که روی سرور واقعی جواب می‌دهد و در هر مرحله می‌گوید چه
چیزی باید ببینید.

> پیش‌فرض‌ها: پورت `3000`، دامنه‌ی `chat.example.com`، نصب در `/opt/evechat`.
> اگر سرور شما پورت دیگری دارد، همان را در دستورها بگذارید.

## ۱. پورت‌ها را ببینید

```bash
ss -ltnp | grep -E ':(3000|3030|5000|8080)'
```

انتظار دارید: یا هیچ خطی نبینید، یا ببینید کدام سرویس روی کدام پورت است.
پورت آزاد را انتخاب کنید. اگر ۳۰۰۰ در اختیار سرویس دیگری است، مثلاً ۳۱۰۰
بردارید. نصب‌کننده هم قبل از هر تغییری همین بررسی را انجام می‌دهد و اگر پورت
پیش‌فرض پر باشد، اولین پورت آزاد از ۳۱۰۰ به بعد را پیشنهاد می‌دهد و می‌گوید چه
چیزی پورت را گرفته است.

## ۲. زیردامنه بسازید

یک رکورد `A` (یا `AAAA`) برای `chat.example.com` به همان سرور EVE اشاره کنید.

انتظار دارید:

```bash
dig +short chat.example.com
# همان IP سرور EVE
```

اگر صفحه‌ی EVE روی HTTPS است، EveChat هم باید HTTPS باشد؛ مرورگرها ویجت
`http://` را در صفحه‌ی `https://` بلاک می‌کنند (mixed content).

## ۳. نصب

```bash
sudo ./install.sh --port 3100 --behind-proxy --domain chat.example.com
```

`--behind-proxy` این دو کار را می‌کند:

- `server.host` را روی `127.0.0.1` می‌گذارد تا سرور فقط از پشت پروکسی در دسترس
  باشد.
- `server.trust_proxy: true` و `server.trusted_proxies` را تنظیم می‌کند تا
  محدودیت نرخ و فهرست مسدودها IP واقعی مشتری را ببینند، نه IP پروکسی.

انتظار دارید در خروجی ببینید: پورت آزاد (یا پورت جایگزین)، گزارش منابع، ساخت
کاربر `evechat`، نوشتن `.env`، نصب سرویس systemd، پاسخ سلامت، و در پایان جدول
«آنچه روی این سرور تغییر کرد». اگر ۸۰/۴۴۳ در اختیار nginx/apache/Caddy است،
نصب‌کننده Caddy را نصب یا اجرا نمی‌کند و همین را در گزارش می‌نویسد.

## ۴. nginx

اگر nginx روی همان سرور است، ساده‌ترین راه:

```bash
sudo ./install.sh --port 3100 --behind-proxy --domain chat.example.com --write-nginx
```

این دستور بلوک `/etc/nginx/conf.d/evechat.conf` را می‌نویسد، **اول** `nginx -t`
را اجرا می‌کند و فقط اگر موفق بود reload می‌کند. اگر `nginx -t` خطا داد، فایل
تولیدشده حذف می‌شود (یا نسخه‌ی قبلی برمی‌گردد) و nginx reload **نمی‌شود**.

اگر خودتان بلوک را می‌نویسید، از `deploy/nginx/evechat.conf` شروع کنید و چهار
نکته را رعایت کنید:

1. `proxy_set_header X-Forwarded-For $remote_addr;` — نه
   `$proxy_add_x_forwarded_for`؛ نوع دوم هدر ارسالی کلاینت را نگه می‌دارد.
2. برای `/api/widget/v1/events` و `/api/widget/v1/poll`:
   `proxy_buffering off;` و `proxy_cache off;`.
3. برای `/ws/`: هدرهای `Upgrade` و `Connection` و
   `proxy_read_timeout 3600s;`.
4. قبل از هر reload، `nginx -t`.

```bash
sudo nginx -t
sudo systemctl reload nginx
```

انتظار دارید:

```text
nginx: the configuration file /etc/nginx/nginx.conf syntax is ok
nginx: configuration file /etc/nginx/nginx.conf test is successful
```

گواهی TLS:

```bash
sudo certbot --nginx -d chat.example.com
```

اگر گواهی هنوز وجود ندارد، `--write-nginx` یک بلوک HTTP-only می‌نویسد تا
`nginx -t` شکست نخورد؛ بعد از certbot همان دستور را دوباره اجرا کنید تا بلوک
HTTPS نوشته شود.

## ۵. دامنه‌ی مجاز ویجت

در پنل: **تنظیمات ← ویجت**، دامنه‌ی صفحه‌ی اشتراک EVE را اضافه کنید
(مثلاً `eve.ir` یا `panel.eve.ir`). اگر این فهرست خالی بماند، ویجت روی هر سایتی
کار می‌کند و فقط برای آزمایش مناسب است. `evechat doctor` این حالت را با هشدار
`widget.domains` گزارش می‌کند.

## ۶. کد در EVE

در مدل `OnlineChatScript` (طبق پلن اولیه در `panel/models/core.py`) این کد را
بگذارید. `async` حتماً باشد تا اگر EveChat در دسترس نبود، صفحه‌ی EVE خراب نشود.

```html
<script>
window.EveChat = {
  projectKey: "pk_...",
  apiUrl: "https://chat.example.com",
  visitor: {
    name: "{{ current_user.full_name }}",
    mobile: "{{ current_user.mobile }}",
    metadata: {
      eve_user_id: "{{ current_user.id }}",
      eve_reseller_id: "{{ current_user.reseller_id }}",
      subscription_plan: "{{ active_subscription.plan_name }}",
      subscription_expiry: "{{ active_subscription.expire_at.isoformat() }}"
    }
  }
};
</script>
<script async src="https://chat.example.com/widget.js"></script>
```

انتظار دارید: بعد از باز کردن صفحه‌ی اشتراک، دکمه‌ی چت پایین صفحه ظاهر شود.
اگر ظاهر نشد، اول کنسول مرورگر را ببینید؛ پیام خطا معمولاً علت را می‌گوید.

## ۷. تست

1. صفحه‌ی اشتراک EVE را باز کنید؛ ویجت باید ظاهر شود.
2. پیام بفرستید؛ گفتگو باید در «صندوق گفتگو»ی پنل دیده شود.
3. از پنل جواب بدهید؛ پیام باید در ویجت برگردد.
4. `evechat doctor --verbose` را بزنید؛ چک‌های `net.ports`، `net.bind`،
   `net.https`، `net.proxy` و `widget.domains` باید سبز یا با هشدار قابل‌فهم
   باشند.

برای بررسی جعل IP:

```bash
curl -s -o /dev/null -w '%{http_code}\n' \
  -H 'X-Forwarded-For: 1.2.3.4' \
  http://127.0.0.1:3100/api/setup/status
```

اگر از یک اتصال غیرمعتمد (بیرون پروکسی) این کار را بکنید، هدر نادیده گرفته
می‌شود و IP ثبت‌شده همان IP واقعی اتصال است، نه `1.2.3.4`. اگر از loopback و
پشت پروکسی معتمد بفرستید، EveChat هدر را از راست می‌خواند و اولین آدرس
غیرمعتمد را برمی‌دارد؛ یعنی مقدار جعلی سمت چپ هرگز برنده نمی‌شود.

## عیب‌یابی

| علامت | علت | راه‌حل |
|---|---|---|
| ویجت ظاهر نمی‌شود، کنسول: mixed content | EveChat روی http است ولی EVE روی https | دامنه + گواهی |
| ۴۰۳ از `/session` | دامنه‌ی EVE در فهرست مجاز نیست | افزودن در تنظیمات ← ویجت |
| ویجت بالا می‌آید ولی وصل نمی‌شود | پروکسی WebSocket را رد نمی‌کند | هدرهای Upgrade و `proxy_read_timeout` |
| کنسول: Refused to load / violates Content-Security-Policy | EVE هدر CSP دارد | افزودن دامنه‌ی چت به `script-src`، `connect-src` (شامل `wss://`)، `style-src` و `font-src` |
| همه‌ی مشتری‌ها یک IP می‌شوند، محدودیت نرخ زود فعال می‌شود | `trust_proxy` خاموش یا `trusted_proxies` غلط | بخش ۴.۳ پلن؛ `trusted_proxies` باید آدرس پروکسی را داشته باشد |
| ۵۰۲ از nginx | سرویس EveChat بالا نیست یا پورت اشتباه است | `systemctl status evechat` و `server.port` |
| اعلان تلگرام نمی‌رسد | اگر سرور در ایران است، دسترسی به `api.telegram.org` ممکن است مسدود باشد | `doctor` را بزنید؛ نیاز به پراکسی یا رله‌ی خارج |
| اطلاعات پلن کنار گفتگو نیست | `GET /api/user/:mobile` در EVE وجود ندارد یا کلید غلط است | یکپارچه‌سازی EVE ← تست اتصال |
| nginx بعد از تغییر بلوک بالا نمی‌آید | `nginx -t` رد شده ولی reload انجام شده | `sudo nginx -t`؛ فایل تولیدشده را حذف یا نسخه‌ی `.bak` را برگردانید |

## چه چیزی تغییر کرد؟

در پایان نصب، install.sh جدولی چاپ می‌کند: کاربر سرویس، پوشه‌ی نصب، سرویس
systemd، فایل nginx (اگر نوشته شد) و پورت/آدرس عمومی. هیچ سرویس همسایه‌ای
(Redis، Chrome، Docker) نصب، متوقف یا بازتنظیم نمی‌شود.

</div>
