// Package email holds Persian, RTL-aware transactional email templates.
// Mail clients ignore webfonts often enough that the layout must survive with
// Tahoma as the fallback.
package email

import (
	"fmt"
	"html"
	"strings"
)

const shell = `<!doctype html>
<html lang="fa" dir="rtl"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>
@font-face{font-family:'Vazirmatn';src:url('/assets/fonts/vazirmatn-variable.woff2') format('woff2');font-weight:100 900;font-display:swap}
body{margin:0;background:#f1f5f9;font-family:'Vazirmatn',Tahoma,'Segoe UI',sans-serif;color:#0f172a;line-height:1.8}
.wrap{max-width:560px;margin:24px auto;background:#fff;border-radius:16px;overflow:hidden;border:1px solid #e2e8f0}
.head{background:#2563eb;color:#fff;padding:18px 22px;font-size:16px;font-weight:700}
.body{padding:22px;font-size:14px}
.row{display:flex;justify-content:space-between;border-bottom:1px solid #f1f5f9;padding:8px 0;font-size:13px}
.row span:first-child{color:#64748b}
.btn{display:inline-block;margin-top:16px;background:#2563eb;color:#fff !important;text-decoration:none;padding:10px 18px;border-radius:10px;font-size:13px}
.foot{padding:14px 22px;background:#f8fafc;color:#94a3b8;font-size:11px;text-align:center}
pre{background:#0f172a;color:#e2e8f0;padding:10px;border-radius:8px;direction:ltr;text-align:left;font-size:11px;overflow-x:auto}
</style></head>
<body><div class="wrap"><div class="head">%s</div><div class="body">%s</div>
<div class="foot">این ایمیل توسط EveChat ارسال شده است.</div></div></body></html>`

func htmlShell(title, body string) string {
	return fmt.Sprintf(shell, html.EscapeString(title), body)
}

func row(label, value string) string {
	return fmt.Sprintf(`<div class="row"><span>%s</span><span>%s</span></div>`, html.EscapeString(label), html.EscapeString(value))
}

// Rendered is a ready-to-send message.
type Rendered struct {
	Subject string
	HTML    string
	Text    string
}

// NewConversation tells an agent that a chat started.
func NewConversation(visitor, page, message, dashboardURL, conversationID string) Rendered {
	subject := "چت جدید از " + visitor
	body := "<p>یک گفتگوی جدید آغاز شد.</p>" +
		row("کاربر", visitor) + row("صفحه", page) + row("پیام", message) +
		fmt.Sprintf(`<a class="btn" href="%s/inbox?c=%s">مشاهده در پنل</a>`,
			html.EscapeString(strings.TrimRight(dashboardURL, "/")), html.EscapeString(conversationID))
	text := fmt.Sprintf("چت جدید از %s\nصفحه: %s\nپیام: %s\n%s/inbox?c=%s\n", visitor, page, message, dashboardURL, conversationID)
	return Rendered{Subject: subject, HTML: htmlShell("چت جدید", body), Text: text}
}

// MissedConversation warns that nobody answered in time.
func MissedConversation(visitor, waiting, dashboardURL, conversationID string) Rendered {
	body := "<p>گفتگویی بدون پاسخ مانده است.</p>" +
		row("کاربر", visitor) + row("مدت انتظار", waiting) +
		fmt.Sprintf(`<a class="btn" href="%s/inbox?c=%s">پاسخ دادن</a>`,
			html.EscapeString(strings.TrimRight(dashboardURL, "/")), html.EscapeString(conversationID))
	text := fmt.Sprintf("گفتگوی بدون پاسخ از %s (انتظار: %s)\n%s/inbox?c=%s\n", visitor, waiting, dashboardURL, conversationID)
	return Rendered{Subject: "گفتگوی بدون پاسخ — " + visitor, HTML: htmlShell("گفتگوی بدون پاسخ", body), Text: text}
}

// DailyDigest is the operator summary.
func DailyDigest(date string, conversations, resolved, messages int, csat float64, busiest string) Rendered {
	body := "<p>خلاصه روز " + html.EscapeString(date) + "</p>" +
		row("گفتگوها", fmt.Sprint(conversations)) +
		row("حل‌شده", fmt.Sprint(resolved)) +
		row("پیام‌ها", fmt.Sprint(messages)) +
		row("میانگین رضایت", fmt.Sprintf("%.1f از ۵", csat)) +
		row("ساعات پرترافیک", busiest)
	text := fmt.Sprintf("خلاصه %s\nگفتگو: %d\nحل‌شده: %d\nپیام: %d\nCSAT: %.1f\n", date, conversations, resolved, messages, csat)
	return Rendered{Subject: "خلاصه روزانه EveChat — " + date, HTML: htmlShell("خلاصه روزانه", body), Text: text}
}

// PasswordReset sends the single-use reset link.
func PasswordReset(name, link string, minutes int) Rendered {
	body := "<p>" + html.EscapeString(name) + " عزیز،</p><p>برای تعیین رمز عبور جدید روی دکمه زیر بزنید. این لینک " +
		fmt.Sprint(minutes) + " دقیقه اعتبار دارد و فقط یک بار قابل استفاده است.</p>" +
		fmt.Sprintf(`<a class="btn" href="%s">تعیین رمز عبور جدید</a>`, html.EscapeString(link)) +
		"<p style=\"font-size:12px;color:#64748b\">اگر این درخواست را شما نداده‌اید، این ایمیل را نادیده بگیرید.</p>"
	text := fmt.Sprintf("%s عزیز،\nبرای تعیین رمز جدید: %s\nاعتبار: %d دقیقه\n", name, link, minutes)
	return Rendered{Subject: "بازیابی رمز عبور EveChat", HTML: htmlShell("بازیابی رمز عبور", body), Text: text}
}

// CSATReceived reports a new satisfaction score.
func CSATReceived(visitor string, score int, comment string) Rendered {
	stars := strings.Repeat("★", score) + strings.Repeat("☆", 5-score)
	body := "<p>بازخورد جدید ثبت شد.</p>" + row("کاربر", visitor) + row("امتیاز", stars) + row("نظر", comment)
	text := fmt.Sprintf("بازخورد %s: %d/5 — %s\n", visitor, score, comment)
	return Rendered{Subject: fmt.Sprintf("بازخورد %d ستاره از %s", score, visitor), HTML: htmlShell("بازخورد مشتری", body), Text: text}
}
