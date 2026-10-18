package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/hub"
	"github.com/evechat/evechat/internal/i18n"
	"github.com/evechat/evechat/internal/integrations/telegram"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

// TelegramChannel relays chats and messages to Telegram chat ids.
type TelegramChannel struct {
	bot       *telegram.Bot
	repo      *repo.Repo
	msgs      *MessageService
	convs     *ConversationService
	hub       *hub.Hub
	dashboard string
	log       zerolog.Logger
}

// NewTelegramChannel builds the channel.
func NewTelegramChannel(bot *telegram.Bot, r *repo.Repo, msgs *MessageService, convs *ConversationService,
	h *hub.Hub, dashboard string, log zerolog.Logger) *TelegramChannel {
	return &TelegramChannel{bot: bot, repo: r, msgs: msgs, convs: convs, hub: h, dashboard: dashboard, log: log}
}

// Name implements Channel.
func (c *TelegramChannel) Name() string { return "telegram" }

// Notify sends the relevant event to Telegram.
func (c *TelegramChannel) Notify(ctx context.Context, ev Event) error {
	tgID, ok := telegram.ChatID(ev.Str("telegram_id"))
	if !ok {
		return nil
	}
	switch ev.Name {
	case EventChatStarted:
		plan, expiry := subscriptionOf(ev.Data["subscription"])
		text := i18n.TelegramNewConversation(
			ev.Str("visitor_name"), ev.Str("device"), ev.Str("os"), ev.Str("browser"),
			ev.Str("page_title"), plan, expiry, ev.Str("message"))
		_, err := c.bot.SendStart(ctx, tgID, ev.ConversationID, text)
		return err
	case EventMessageReceived:
		if ev.Str("sender_type") == string(models.SenderBot) || ev.Str("sender_type") == string(models.SenderSystem) {
			return nil
		}
		text := i18n.TelegramMessage(ev.Str("sender_name"), ev.Str("content"))
		if _, err := c.bot.SendText(ctx, tgID, ev.ConversationID, text, 0); err != nil {
			return err
		}
		if mid := ev.Str("message_id"); mid != "" {
			// The relay message id is stored by the caller through the repo.
			c.log.Debug().Str("message_id", mid).Msg("telegram relay sent")
		}
		return nil
	case EventChatResolved:
		_, err := c.bot.SendText(ctx, tgID, ev.ConversationID, "✅ گفتگو بسته شد.", 0)
		return err
	case EventTicketCreated:
		_, err := c.bot.SendText(ctx, tgID, ev.ConversationID,
			i18n.TelegramNewTicket(ev.Str("title"), ev.Str("priority"), ev.Str("description")), 0)
		return err
	}
	return nil
}

func subscriptionOf(raw any) (string, string) {
	m, ok := raw.(models.JSONMap)
	if !ok {
		if mm, ok2 := raw.(map[string]any); ok2 {
			m = models.JSONMap(mm)
		} else {
			return "", ""
		}
	}
	plan := m.Str("plan")
	if plan == "" {
		plan = m.Str("subscription_plan")
	}
	expiry := m.Str("expiry")
	if expiry == "" {
		expiry = m.Str("subscription_expiry")
	}
	return plan, expiry
}

// HandleInbound posts a Telegram reply into the originating conversation.
func (c *TelegramChannel) HandleInbound(ctx context.Context, chatID int64, convID, text string) error {
	if convID == "" {
		conv, err := c.repo.Conversations.GetByTelegramID(ctx, fmt.Sprint(chatID))
		if err != nil {
			return fmt.Errorf("no conversation for telegram chat %d", chatID)
		}
		convID = conv.ID
	}
	_, _, err := c.msgs.Send(ctx, SendInput{
		ConversationID:    convID,
		SenderType:        models.SenderAgent,
		SenderName:        "پشتیبان (تلگرام)",
		Content:           strings.TrimSpace(text),
		SkipAuthorization: true,
	})
	return err
}

// HandleResolve closes the conversation from the inline keyboard.
func (c *TelegramChannel) HandleResolve(ctx context.Context, convID string) error {
	return c.convs.Resolve(ctx, convID, "")
}

// HandleEscalate returns the conversation to the pending queue.
func (c *TelegramChannel) HandleEscalate(ctx context.Context, convID string) error {
	conv, err := c.repo.Conversations.GetByID(ctx, convID)
	if err != nil {
		return err
	}
	if err := c.repo.Conversations.Unassign(ctx, convID); err != nil {
		return err
	}
	if err := c.repo.Conversations.SetStatus(ctx, convID, models.StatusPending); err != nil {
		return err
	}
	c.hub.ToProjectAgents(conv.ProjectID, hub.NewEnvelope(hub.TypeConversationUpdated, map[string]any{
		"conversation_id": convID,
		"status":          models.StatusPending,
		"escalated":       true,
	}))
	_, _ = c.msgs.SystemMessage(ctx, convID, "گفتگو برای پاسخ به تیم پشتیبانی ارجاع شد.")
	return nil
}
