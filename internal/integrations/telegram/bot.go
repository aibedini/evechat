// Package telegram implements the Telegram relay bot in webhook mode only.
package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog"
	"golang.org/x/time/rate"

	"github.com/evechat/evechat/internal/i18n"
)

// Config configures the bot.
type Config struct {
	Token        string
	WebhookURL   string
	Secret       string
	DashboardURL string
}

// InboundFunc receives a reply typed by a support agent inside Telegram.
type InboundFunc func(ctx context.Context, chatID int64, convID, text string) error

// ActionFunc handles the inline keyboard callbacks.
type ActionFunc func(ctx context.Context, convID string) error

// Bot wraps the Telegram Bot API with rate limiting and message correlation.
type Bot struct {
	api       *tgbotapi.BotAPI
	cfg       Config
	log       zerolog.Logger
	limiter   *rate.Limiter
	dashboard string

	mu      sync.Mutex
	mapping map[int64]mappingEntry

	inbound  InboundFunc
	resolve  ActionFunc
	escalate ActionFunc
}

type mappingEntry struct {
	convID string
	exp    time.Time
}

// New builds the bot. It performs a getMe request, so a bad token fails here
// instead of silently dropping notifications later.
func New(cfg Config, log zerolog.Logger) (*Bot, error) {
	if cfg.Token == "" {
		return nil, errors.New("telegram: empty bot token")
	}
	api, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("telegram: creating bot: %w", err)
	}
	api.Debug = false
	b := &Bot{
		api:       api,
		cfg:       cfg,
		log:       log,
		limiter:   rate.NewLimiter(rate.Limit(25), 25),
		dashboard: strings.TrimRight(cfg.DashboardURL, "/"),
		mapping:   make(map[int64]mappingEntry, 256),
	}
	log.Info().Str("username", api.Self.UserName).Msg("telegram bot ready")
	return b, nil
}

// SetHandlers wires the server-side callbacks.
func (b *Bot) SetHandlers(inbound InboundFunc, resolve, escalate ActionFunc) {
	b.inbound = inbound
	b.resolve = resolve
	b.escalate = escalate
}

// Username returns the bot handle.
func (b *Bot) Username() string { return b.api.Self.UserName }

// SetWebhook registers the webhook URL with the secret token.
func (b *Bot) SetWebhook(ctx context.Context) error {
	if b.cfg.WebhookURL == "" {
		return errors.New("telegram: empty webhook url")
	}
	params := tgbotapi.Params{}
	params["url"] = b.cfg.WebhookURL
	params["secret_token"] = b.cfg.Secret
	params["max_connections"] = "40"
	allowed, _ := json.Marshal([]string{"message", "edited_message", "callback_query"})
	params["allowed_updates"] = string(allowed)
	if _, err := b.api.MakeRequest("setWebhook", params); err != nil {
		return fmt.Errorf("telegram: setWebhook: %w", err)
	}
	b.log.Info().Str("url", b.cfg.WebhookURL).Msg("telegram webhook registered")
	return nil
}

// ConversationFor resolves a Telegram message id to a conversation id.
func (b *Bot) ConversationFor(tgMsgID int64) (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	e, ok := b.mapping[tgMsgID]
	if !ok || time.Now().After(e.exp) {
		delete(b.mapping, tgMsgID)
		return "", false
	}
	return e.convID, true
}

// Remember links an outbound Telegram message id to a conversation for 72h.
func (b *Bot) Remember(tgMsgID int64, convID string) {
	if tgMsgID == 0 || convID == "" {
		return
	}
	b.mu.Lock()
	if len(b.mapping) > 5000 {
		now := time.Now()
		for k, v := range b.mapping {
			if now.After(v.exp) {
				delete(b.mapping, k)
			}
		}
	}
	b.mapping[tgMsgID] = mappingEntry{convID: convID, exp: time.Now().Add(72 * time.Hour)}
	b.mu.Unlock()
}

// HandleUpdate processes one webhook update.
func (b *Bot) HandleUpdate(ctx context.Context, u tgbotapi.Update) error {
	switch {
	case u.CallbackQuery != nil:
		return b.handleCallback(ctx, u.CallbackQuery)
	case u.Message != nil:
		return b.handleMessage(ctx, u.Message)
	}
	return nil
}

func (b *Bot) handleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) error {
	data := cb.Data
	convID := ""
	action := ""
	if i := strings.IndexByte(data, ':'); i > 0 {
		action, convID = data[:i], data[i+1:]
	}
	ack := i18n.TelegramResolvedAck
	var err error
	switch action {
	case "resolve":
		if b.resolve != nil {
			err = b.resolve(ctx, convID)
		}
	case "escalate":
		if b.escalate != nil {
			err = b.escalate(ctx, convID)
		}
		ack = i18n.TelegramEscalatedAck
	default:
		ack = "درخواست نامشخص"
	}
	if err != nil {
		ack = "خطا: " + err.Error()
	}
	if _, aerr := b.api.Request(tgbotapi.NewCallback(cb.ID, ack)); aerr != nil {
		b.log.Debug().Err(aerr).Msg("telegram answerCallbackQuery failed")
	}
	return err
}

func (b *Bot) handleMessage(ctx context.Context, msg *tgbotapi.Message) error {
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return nil
	}
	convID := ""
	if msg.ReplyToMessage != nil {
		if id, ok := b.ConversationFor(int64(msg.ReplyToMessage.MessageID)); ok {
			convID = id
		}
	}
	if b.inbound == nil {
		return nil
	}
	if err := b.inbound(ctx, msg.Chat.ID, convID, text); err != nil {
		b.log.Warn().Err(err).Int64("chat_id", msg.Chat.ID).Msg("telegram inbound failed")
		return err
	}
	return nil
}

// SendStart sends the new-conversation card with the inline keyboard.
func (b *Bot) SendStart(ctx context.Context, chatID int64, convID, text string) (int64, error) {
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL(i18n.TelegramButtonDashboard, b.dashboard+"/inbox?c="+convID),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(i18n.TelegramButtonResolve, "resolve:"+convID),
			tgbotapi.NewInlineKeyboardButtonData(i18n.TelegramButtonEscalate, "escalate:"+convID),
		),
	)
	m := tgbotapi.NewMessage(chatID, text)
	m.ParseMode = tgbotapi.ModeHTML
	m.ReplyMarkup = keyboard
	sent, err := b.send(ctx, m)
	if err != nil {
		return 0, err
	}
	b.Remember(int64(sent.MessageID), convID)
	return int64(sent.MessageID), nil
}

// SendText relays a message to a Telegram chat, optionally as a reply.
func (b *Bot) SendText(ctx context.Context, chatID int64, convID, text string, replyTo int64) (int64, error) {
	m := tgbotapi.NewMessage(chatID, text)
	m.ParseMode = tgbotapi.ModeHTML
	if replyTo > 0 {
		m.ReplyToMessageID = int(replyTo)
	}
	sent, err := b.send(ctx, m)
	if err != nil {
		return 0, err
	}
	b.Remember(int64(sent.MessageID), convID)
	return int64(sent.MessageID), nil
}

func (b *Bot) send(ctx context.Context, m tgbotapi.MessageConfig) (tgbotapi.Message, error) {
	if err := b.limiter.Wait(ctx); err != nil {
		return tgbotapi.Message{}, err
	}
	sent, err := b.api.Send(m)
	if err == nil {
		return sent, nil
	}
	var tgErr tgbotapi.Error
	if errors.As(err, &tgErr) && tgErr.RetryAfter > 0 {
		select {
		case <-time.After(time.Duration(tgErr.RetryAfter) * time.Second):
		case <-ctx.Done():
			return tgbotapi.Message{}, ctx.Err()
		}
		return b.api.Send(m)
	}
	return sent, err
}

// ChatID parses a stored telegram id.
func ChatID(raw string) (int64, bool) {
	v, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || v == 0 {
		return 0, false
	}
	return v, true
}
