package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/middleware"
	"github.com/evechat/evechat/internal/repo"
	"github.com/evechat/evechat/internal/security"
	"github.com/evechat/evechat/internal/services"
)

// channelInput is the editable surface of a channel.
type channelInput struct {
	Name            string            `json:"name"`
	Kind            string            `json:"kind"`
	OutboundURL     string            `json:"outbound_url"`
	OutboundSecret  string            `json:"outbound_secret"`
	OutboundHeaders map[string]string `json:"outbound_headers"`
	SessionTTL      *int              `json:"session_ttl_seconds"`
	Mapping         json.RawMessage   `json:"mapping"`
	IsActive        *bool             `json:"is_active"`
}

var channelKinds = map[string]bool{"webhook": true, "telegram": true, "sms": true, "custom": true}

// ListChannels returns the project channels.
func (h *Handlers) ListChannels(w http.ResponseWriter, r *http.Request) error {
	items, err := h.Repo.Channels.List(r.Context(), h.scopeProject(r))
	if err != nil {
		return apierr.Internal(err)
	}
	out := make([]map[string]any, 0, len(items))
	for _, c := range items {
		out = append(out, c.Public())
	}
	writeJSON(w, http.StatusOK, newList(out, len(out), len(out), 0))
	return nil
}

// CreateChannel registers a channel and returns its inbound secret once.
func (h *Handlers) CreateChannel(w http.ResponseWriter, r *http.Request) error {
	if err := h.requireManager(r); err != nil {
		return err
	}
	validated, err := h.validateChannelInput(r)
	if err != nil {
		return err
	}
	ch := &repo.Channel{
		ID:            id.NewPrefixed(id.PrefixChannel),
		ProjectID:     h.scopeProject(r),
		Name:          validated.Name,
		Kind:          validated.Kind,
		InboundSecret: id.Token(24),
		Mapping:       validated.Mapping,
		IsActive:      validated.Active,
		SessionTTL:    validated.SessionTTL,
	}
	headers, err := mergeChannelHeaders(validated.OutboundHeaders, nil)
	if err != nil {
		return err
	}
	ch.OutboundHeaders = headers
	if validated.OutboundURL != "" {
		ch.OutboundURL = &validated.OutboundURL
	}
	if validated.OutboundSecret != "" {
		ch.OutboundSecret = &validated.OutboundSecret
	}
	if err := h.Repo.Channels.Create(r.Context(), ch); err != nil {
		return apierr.Internal(err)
	}
	payload := ch.Public()
	payload["inbound_secret"] = ch.InboundSecret
	payload["inbound_url"] = h.Cfg.BaseURL() + "/api/channels/" + ch.ID + "/inbound"
	writeJSON(w, http.StatusCreated, payload)
	return nil
}

// GetChannel returns one channel (without secrets).
func (h *Handlers) GetChannel(w http.ResponseWriter, r *http.Request) error {
	ch, err := h.channelOf(r)
	if err != nil {
		return err
	}
	return ok(w, ch.Public())
}

// UpdateChannel edits a channel.
func (h *Handlers) UpdateChannel(w http.ResponseWriter, r *http.Request) error {
	if err := h.requireManager(r); err != nil {
		return err
	}
	ch, err := h.channelOf(r)
	if err != nil {
		return err
	}
	validated, err := h.validateChannelInput(r)
	if err != nil {
		return err
	}
	ch.Name = validated.Name
	ch.Kind = validated.Kind
	ch.Mapping = validated.Mapping
	ch.IsActive = validated.Active
	headers, err := mergeChannelHeaders(validated.OutboundHeaders, ch.OutboundHeaders)
	if err != nil {
		return err
	}
	ch.OutboundHeaders = headers
	if validated.SessionTTLSet {
		ch.SessionTTL = validated.SessionTTL
	}
	if validated.OutboundURL == "" {
		ch.OutboundURL = nil
	} else {
		ch.OutboundURL = &validated.OutboundURL
	}
	if validated.OutboundSecret != "" {
		ch.OutboundSecret = &validated.OutboundSecret
	}
	if err := h.Repo.Channels.Update(r.Context(), ch); err != nil {
		return apierr.Internal(err)
	}
	return ok(w, ch.Public())
}

// DeleteChannel removes a channel.
func (h *Handlers) DeleteChannel(w http.ResponseWriter, r *http.Request) error {
	if err := h.requireManager(r); err != nil {
		return err
	}
	ch, err := h.channelOf(r)
	if err != nil {
		return err
	}
	if err := h.Repo.Channels.Delete(r.Context(), ch.ID); err != nil {
		return apierr.Internal(err)
	}
	return okMsg(w)
}

// RotateChannelSecret issues a new inbound secret and returns it once.
func (h *Handlers) RotateChannelSecret(w http.ResponseWriter, r *http.Request) error {
	if err := h.requireManager(r); err != nil {
		return err
	}
	ch, err := h.channelOf(r)
	if err != nil {
		return err
	}
	secret := id.Token(24)
	if err := h.Repo.Channels.RotateSecret(r.Context(), ch.ID, secret); err != nil {
		return apierr.Internal(err)
	}
	return ok(w, map[string]any{"inbound_secret": secret})
}

// TestChannel sends a real outbound test callback and records a masked row in
// the inspector. It is the only way to verify that configured headers (for
// example X-API-Key) actually reach the Android bridge.
func (h *Handlers) TestChannel(w http.ResponseWriter, r *http.Request) error {
	ch, err := h.channelOf(r)
	if err != nil {
		return err
	}
	if h.Channels == nil {
		return apierr.BadRequest("کانال‌ها فعال نیستند")
	}
	if ch.OutboundURL == nil || strings.TrimSpace(*ch.OutboundURL) == "" {
		return apierr.BadRequest("برای ارسال آزمایشی، آدرس بازگشت کانال را تنظیم کنید")
	}
	return ok(w, h.Channels.TestOutbound(r.Context(), ch))
}

// PreviewChannelMapping resolves a sample payload against a mapping without
// persisting anything; the channels page uses it for the live preview.
func (h *Handlers) PreviewChannelMapping(w http.ResponseWriter, r *http.Request) error {
	ch, err := h.channelOf(r)
	if err != nil {
		return err
	}
	var in struct {
		Payload json.RawMessage `json:"payload"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	var m struct {
		ExternalID string `json:"external_id"`
		Name       string `json:"name"`
		Message    string `json:"message"`
		Metadata   string `json:"metadata"`
	}
	_ = json.Unmarshal(ch.Mapping, &m)
	preview := services.PreviewMapping(m.ExternalID, m.Name, m.Message, m.Metadata, in.Payload)
	return ok(w, preview)
}

// ListChannelInbox returns the inspector rows of a channel.
func (h *Handlers) ListChannelInbox(w http.ResponseWriter, r *http.Request) error {
	ch, err := h.channelOf(r)
	if err != nil {
		return err
	}
	limit := atoiDefault(r.URL.Query().Get("limit"), 50)
	items, err := h.Repo.Channels.ListInbox(r.Context(), ch.ProjectID, ch.ID, limit)
	if err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusOK, newList(items, len(items), len(items), 0))
	return nil
}

// ChannelCurl renders a ready-to-run curl command for the channel.
func (h *Handlers) ChannelCurl(w http.ResponseWriter, r *http.Request) error {
	ch, err := h.channelOf(r)
	if err != nil {
		return err
	}
	curl := services.ChannelCurl(h.Cfg.BaseURL(), ch)
	return ok(w, map[string]any{"curl": curl})
}

// ChannelInbound is the public ingest endpoint: no JWT, the channel secret is
// the credential.
func (h *Handlers) ChannelInbound(w http.ResponseWriter, r *http.Request) error {
	if h.Channels == nil {
		return apierr.NotFound("کانال‌ها فعال نیستند")
	}
	ch, err := h.Repo.Channels.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil || !ch.IsActive {
		return apierr.NotFound("کانال یافت نشد")
	}
	secret := strings.TrimSpace(r.Header.Get("X-Channel-Secret"))
	if secret == "" {
		secret = strings.TrimSpace(r.URL.Query().Get("secret"))
	}
	if !h.Channels.VerifySecret(ch, secret) {
		return apierr.Unauthorized("کلید کانال نامعتبر است")
	}
	body, rerr := io.ReadAll(io.LimitReader(r.Body, 256<<10))
	if rerr != nil {
		return apierr.BadRequest("بدنه درخواست خوانده نشد")
	}
	ip := middleware.ClientIPWithProxies(r, h.Cfg.Server.TrustProxy, h.Cfg.Server.TrustedProxies)
	inbox := &repo.InboxEntry{
		ID: id.NewPrefixed(id.PrefixInbox), ChannelID: &ch.ID, ProjectID: &ch.ProjectID,
		Method: r.Method, RemoteIP: ip, Status: "received", Body: services.ScrubBody(body),
	}
	_ = h.Repo.Channels.CreateInbox(r.Context(), inbox)
	res, ierr := h.Channels.Ingest(r.Context(), services.IngestInput{
		Channel: ch, Body: body, Method: r.Method, RemoteIP: ip,
	})
	if ierr != nil {
		msg := ierr.Error()
		var ae *apierr.Error
		if errors.As(ierr, &ae) {
			msg = ae.Message
		}
		_ = h.Repo.Channels.UpdateInbox(r.Context(), inbox.ID, "rejected", &msg, "", "")
		return ierr
	}
	mapped := ""
	if b, err := json.Marshal(res); err == nil {
		mapped = string(b)
	}
	_ = h.Repo.Channels.UpdateInbox(r.Context(), inbox.ID, "processed", nil, mapped, res.ConversationID)
	writeJSON(w, http.StatusOK, res)
	return nil
}

// ---- helpers ----

type validatedChannel struct {
	Name, Kind, OutboundURL, OutboundSecret string
	OutboundHeaders                         map[string]string
	SessionTTL                              int
	SessionTTLSet                           bool
	Mapping                                 json.RawMessage
	Active                                  bool
}

const (
	defaultSessionTTL = 86400
	maxSessionTTL     = 30 * 24 * 60 * 60
)

func (h *Handlers) validateChannelInput(r *http.Request) (*validatedChannel, error) {
	var in channelInput
	if err := bind(r, &in); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, apierr.BadRequest("نام کانال لازم است")
	}
	kind := strings.TrimSpace(in.Kind)
	if kind == "" {
		kind = "webhook"
	}
	if !channelKinds[kind] {
		return nil, apierr.BadRequest("نوع کانال نامعتبر است")
	}
	mapping := in.Mapping
	if len(mapping) == 0 {
		mapping = json.RawMessage("{}")
	}
	var probe map[string]any
	if err := json.Unmarshal(mapping, &probe); err != nil {
		return nil, apierr.BadRequest("نگاشت باید یک شیء JSON باشد")
	}
	outbound := strings.TrimSpace(in.OutboundURL)
	if outbound != "" {
		if err := security.ValidateOutboundURL(outbound); err != nil {
			return nil, apierr.BadRequest("آدرس بازگشت کانال به مقصد مجاز اشاره نمی‌کند")
		}
	}
	headers, err := validateChannelHeaders(in.OutboundHeaders)
	if err != nil {
		return nil, err
	}
	ttl := defaultSessionTTL
	ttlSet := false
	if in.SessionTTL != nil {
		ttl = *in.SessionTTL
		ttlSet = true
		if ttl < 0 || ttl > maxSessionTTL {
			return nil, apierr.BadRequest(fmt.Sprintf("مدت ادامه‌ی گفتگو باید بین ۰ و %d ثانیه باشد", maxSessionTTL))
		}
	}
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	return &validatedChannel{
		Name: name, Kind: kind, OutboundURL: outbound, OutboundSecret: strings.TrimSpace(in.OutboundSecret),
		OutboundHeaders: headers, SessionTTL: ttl, SessionTTLSet: ttlSet,
		Mapping: mapping, Active: active,
	}, nil
}

// forbiddenChannelHeaders are set by net/http itself and cannot be overridden
// by a channel.
var forbiddenChannelHeaders = map[string]bool{
	"Host": true, "Content-Length": true, "Transfer-Encoding": true,
	"Connection": true, "Upgrade": true, "Keep-Alive": true,
	"Proxy-Connection": true, "Trailer": true, "Te": true,
}

func validHeaderName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	const extra = "!#$%&'*+-.^_`|~"
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			continue
		}
		if !strings.ContainsRune(extra, rune(c)) {
			return false
		}
	}
	return true
}

// validateChannelHeaders canonicalizes names and checks values. A masked value
// is allowed here; mergeChannelHeaders resolves it against the stored map.
func validateChannelHeaders(in map[string]string) (map[string]string, error) {
	if len(in) > 20 {
		return nil, apierr.BadRequest("حداکثر ۲۰ هدر خروجی مجاز است")
	}
	out := make(map[string]string, len(in))
	for rawName, rawValue := range in {
		rawName = strings.TrimSpace(rawName)
		if rawName == "" {
			continue
		}
		name := textproto.CanonicalMIMEHeaderKey(rawName)
		if !validHeaderName(name) {
			return nil, apierr.BadRequest("نام هدر نامعتبر است: " + rawName)
		}
		if forbiddenChannelHeaders[name] {
			return nil, apierr.BadRequest("این هدر را خود سرور تنظیم می‌کند: " + name)
		}
		value := strings.TrimSpace(rawValue)
		if strings.ContainsAny(value, "\r\n") {
			return nil, apierr.BadRequest("مقدار هدر نباید خط جدید داشته باشد")
		}
		if len(value) > 1024 {
			return nil, apierr.BadRequest("مقدار هدر بیش از حد طولانی است: " + name)
		}
		if value == "" {
			continue
		}
		out[name] = value
	}
	return out, nil
}

// mergeChannelHeaders keeps stored values when the UI sends the mask back.
func mergeChannelHeaders(in, existing map[string]string) (map[string]string, error) {
	clean, err := validateChannelHeaders(in)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(clean))
	for name, value := range clean {
		if value != repo.MaskedHeader {
			out[name] = value
			continue
		}
		stored := ""
		for k, v := range existing {
			if strings.EqualFold(k, name) {
				stored = v
				break
			}
		}
		if stored == "" {
			return nil, apierr.BadRequest("مقدار هدر حساس را دوباره وارد کنید: " + name)
		}
		out[name] = stored
	}
	return out, nil
}

func (h *Handlers) channelOf(r *http.Request) (*repo.Channel, error) {
	ch, err := h.Repo.Channels.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return nil, apierr.NotFound("کانال یافت نشد")
	}
	if err := h.checkProjectScope(r, ch.ProjectID); err != nil {
		return nil, err
	}
	return ch, nil
}
