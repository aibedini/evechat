// Package i18n holds every Persian string the server sends to users: auto
// replies, Telegram templates, notification texts and system messages.
// No Persian text may be hard-coded inside logic files.
package i18n

import "fmt"

// FA is the default server-side locale.
const FA = "fa"

const (
	// AutoReplyWelcome greets the visitor when a routing rule auto-replies.
	AutoReplyWelcome = "سلام 👋\nبه پشتیبانی خوش آمدید. پیام شما ثبت شد و همکاران ما در اسرع وقت پاسخ می‌دهند."
	// AutoReplyOffline is sent outside business hours.
	AutoReplyOffline = "در حال حاضر خارج از ساعات کاری هستیم. پیام شما ثبت شد و اولین فرصت پاسخ می‌دهیم."
	// AutoReplyNoAgent is sent when nobody can take the chat right now.
	AutoReplyNoAgent = "پیام شما ثبت شد. در حال حاضر همکاران ما آنلاین نیستند و در اولین فرصت پاسخ می‌دهیم."
	// AssignedToHuman tells the visitor an agent joined.
	AssignedToHuman = "یک کارشناس به گفتگو پیوست."
	// ConversationResolved is the closing system message.
	ConversationResolved = "گفتگو بسته شد. از اینکه با ما صحبت کردید سپاسگزاریم."
	// ConversationReopened is the reopening system message.
	ConversationReopened = "گفتگو دوباره باز شد."
	// CSATRequest asks for a satisfaction score.
	CSATRequest = "لطفاً تجربه خود از این گفتگو را ارزیابی کنید."
	// CSATThanks thanks the visitor after scoring.
	CSATThanks = "از بازخورد شما سپاسگزاریم. 🙏"
	// TransferNote is the system message on transfer.
	TransferNote = "گفتگو به کارشناس دیگری ارجاع شد."
	// AITookOver announces the assistant answer.
	AITookOver = "پاسخ خودکار دستیار هوشمند"
	// AIHandoff announces the escalation to a human.
	AIHandoff = "این گفتگو برای پاسخ دقیق‌تر به کارشناس انسانی ارجاع شد."
)

// TelegramNewConversation builds the Telegram notification header for a new chat.
func TelegramNewConversation(name, device, osName, browser, pageTitle, plan, expiry, message string) string {
	if name == "" {
		name = "ناشناس"
	}
	return fmt.Sprintf(`🔔 چت جدید
👤 %s
📱 %s · %s · %s
🔗 %s
💳 پلن: %s | انقضا: %s
━━━━━━━━━━
%s

برای پاسخ، روی همین پیام Reply کنید.`, name, fallback(device), fallback(osName), fallback(browser),
		fallback(pageTitle), fallback(plan), fallback(expiry), message)
}

// TelegramMessage builds a relay message for an existing conversation.
func TelegramMessage(name, message string) string {
	if name == "" {
		name = "کاربر"
	}
	return fmt.Sprintf(`💬 %s
━━━━━━━━━━
%s`, name, message)
}

// TelegramNewTicket builds the ticket-created notification.
func TelegramNewTicket(title, priority, description string) string {
	return fmt.Sprintf(`🎫 تیکت جدید
عنوان: %s
اولویت: %s
━━━━━━━━━━
%s`, title, priority, description)
}

const (
	// TelegramButtonDashboard opens the conversation in the panel.
	TelegramButtonDashboard = "مشاهده در پنل"
	// TelegramButtonResolve closes the conversation.
	TelegramButtonResolve = "بستن گفتگو"
	// TelegramButtonEscalate assigns it to a human.
	TelegramButtonEscalate = "ارجاع به پشتیبان"
	// TelegramResolvedAck confirms the callback.
	TelegramResolvedAck = "گفتگو بسته شد."
	// TelegramEscalatedAck confirms the escalation.
	TelegramEscalatedAck = "به پشتیبان انسانی ارجاع شد."
	// SMSTicketCreated is the SMS body for a new ticket.
	SMSTicketCreated = "تیکت جدید در پشتیبانی: %s"
	// SMSChatStarted is the SMS body when a routed phone gets a new chat.
	SMSChatStarted = "چت جدید از %s در پشتیبانی شما"
)

func fallback(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
