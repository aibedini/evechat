package handlers

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/integrations/gmweb"
	"github.com/evechat/evechat/internal/logx"
)

// IntegrationStatus reports which integrations are configured.
func (h *Handlers) IntegrationStatus(w http.ResponseWriter, r *http.Request) error {
	status := map[string]any{
		"eve": map[string]any{
			"configured": h.EVE != nil && h.EVE.Enabled(),
			"signature":  h.EVE != nil && h.EVE.RequireSignature(),
		},
		"telegram": map[string]any{
			"enabled":  h.Telegram != nil,
			"username": telegramUsername(h),
		},
		"gmweb": map[string]any{
			"configured": h.GMweb != nil && h.GMweb.Enabled(),
		},
		"ai": map[string]any{
			"enabled": h.Cfg.AI.Enabled,
			"model":   h.Cfg.AI.Model,
		},
	}
	return ok(w, status)
}

func telegramUsername(h *Handlers) string {
	if h.Telegram == nil {
		return ""
	}
	return h.Telegram.Username()
}

// TelegramWebhook receives updates from Telegram.
func (h *Handlers) TelegramWebhook(w http.ResponseWriter, r *http.Request) error {
	if h.Telegram == nil {
		return apierr.NotFound("تلگرام فعال نیست")
	}
	secret := strings.TrimSpace(h.Cfg.Telegram.Secret)
	if secret != "" {
		got := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
		if subtle.ConstantTimeCompare([]byte(got), []byte(secret)) != 1 {
			return apierr.Unauthorized("امضای تلگرام نامعتبر است")
		}
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return apierr.BadRequest("بدنه نامعتبر است")
	}
	var update tgbotapi.Update
	if err := json.Unmarshal(body, &update); err != nil {
		return apierr.BadRequest("به‌روزرسانی تلگرام نامعتبر است")
	}
	if err := h.Telegram.HandleUpdate(r.Context(), update); err != nil {
		logx.WithReq(r.Context()).Warn().Err(err).Msg("telegram update failed")
	}
	// Always 200: Telegram retries non-2xx responses aggressively.
	return okMsg(w)
}

// SetTelegramWebhook registers the webhook with Telegram.
func (h *Handlers) SetTelegramWebhook(w http.ResponseWriter, r *http.Request) error {
	if err := h.requireManager(r); err != nil {
		return err
	}
	if h.Telegram == nil {
		return apierr.BadRequest("توکن ربات تلگرام تنظیم نشده است")
	}
	if err := h.Telegram.SetWebhook(r.Context()); err != nil {
		return apierr.Internal(err)
	}
	return okMsg(w)
}

// TestTelegram sends a test message to the caller's chat id.
func (h *Handlers) TestTelegram(w http.ResponseWriter, r *http.Request) error {
	agent, err := h.callerAgent(r)
	if err != nil {
		return err
	}
	if h.Telegram == nil {
		return apierr.BadRequest("تلگرام فعال نیست")
	}
	if agent.TelegramID == nil || *agent.TelegramID == "" {
		return apierr.BadRequest("ابتدا شناسه تلگرام خود را در تنظیمات تیم ثبت کنید")
	}
	if _, err := h.Telegram.SendText(r.Context(), parseChatID(*agent.TelegramID), "",
		"✅ اتصال EveChat به تلگرام برقرار است.", 0); err != nil {
		return apierr.Internal(err)
	}
	return okMsg(w)
}

// EVEUser proxies an EVE profile lookup by mobile.
func (h *Handlers) EVEUser(w http.ResponseWriter, r *http.Request) error {
	mobile := strings.TrimSpace(r.URL.Query().Get("mobile"))
	if mobile == "" {
		return apierr.BadRequest("شماره موبایل لازم است")
	}
	if h.EVE == nil || !h.EVE.Enabled() {
		return apierr.BadRequest("ادغام EVE تنظیم نشده است")
	}
	u, err := h.EVE.GetUserByMobile(r.Context(), mobile)
	if err != nil {
		return apierr.NotFound("کاربر در EVE یافت نشد")
	}
	return ok(w, u)
}

// GMwebInbound receives SMS replies from the gateway.
func (h *Handlers) GMwebInbound(w http.ResponseWriter, r *http.Request) error {
	if h.GMweb == nil || !h.GMweb.Enabled() {
		return apierr.NotFound("درگاه پیامک فعال نیست")
	}
	secret := strings.TrimSpace(h.Cfg.Integrations.GMweb.WebhookSecret)
	if secret != "" {
		got := r.Header.Get("X-Webhook-Secret")
		if subtle.ConstantTimeCompare([]byte(got), []byte(secret)) != 1 {
			return apierr.Unauthorized("کلید وب‌هوک نامعتبر است")
		}
	}
	in, err := gmwebParse(r)
	if err != nil {
		return apierr.BadRequest("داده پیامک نامعتبر است")
	}
	if err := h.smsInbound(r, in.From, in.Text); err != nil {
		logx.WithReq(r.Context()).Warn().Err(err).Str("from", mask(in.From)).Msg("sms inbound failed")
	}
	return okMsg(w)
}

// TestSMS sends a test SMS (admin only).
func (h *Handlers) TestSMS(w http.ResponseWriter, r *http.Request) error {
	if err := h.requireManager(r); err != nil {
		return err
	}
	if h.GMweb == nil || !h.GMweb.Enabled() {
		return apierr.BadRequest("درگاه پیامک تنظیم نشده است")
	}
	var in struct {
		Phone string `json:"phone"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	if in.Phone == "" {
		return apierr.BadRequest("شماره موبایل لازم است")
	}
	ctx, cancel := contextWithTimeout(r, 15*time.Second)
	defer cancel()
	if err := h.GMweb.SendSMS(ctx, in.Phone, "پیام آزمایشی EveChat"); err != nil {
		return apierr.Internal(err)
	}
	return okMsg(w)
}

func mask(p string) string {
	if len(p) <= 6 {
		return "***"
	}
	return p[:4] + "***" + p[len(p)-3:]
}

func gmwebParse(r *http.Request) (gmweb.InboundMessage, error) { return gmweb.ParseInbound(r) }

func (h *Handlers) smsInbound(r *http.Request, from, text string) error {
	if h.SMS == nil {
		return errors.New("sms channel is not configured")
	}
	return h.SMS.HandleInbound(r.Context(), from, text)
}

func parseChatID(raw string) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func contextWithTimeout(r *http.Request, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), d)
}
