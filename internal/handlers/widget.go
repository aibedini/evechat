package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/embedfs"
	"github.com/evechat/evechat/internal/middleware"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/security"
	"github.com/evechat/evechat/internal/services"
)

// WidgetJS serves the compiled widget bundle with ETag caching.
func (h *Handlers) WidgetJS(w http.ResponseWriter, r *http.Request) error {
	data, err := fs.ReadFile(embedfs.Widget(), "widget.js")
	if err != nil {
		return apierr.NotFound("باندل ویجت یافت نشد")
	}
	sum := sha256.Sum256(data)
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return nil
	}
	_, err = w.Write(data)
	return err
}

// WidgetConfig returns the runtime configuration of the widget.
func (h *Handlers) WidgetConfig(w http.ResponseWriter, r *http.Request) error {
	project, okp := middleware.ProjectFrom(r.Context())
	if !okp {
		return apierr.Unauthorized("کلید پروژه معتبر نیست")
	}
	online := len(h.Hub.OnlineAgents(project.ID)) > 0
	cfg := widgetConfig(project, online)
	writeJSON(w, http.StatusOK, cfg)
	return nil
}

func projectSettings(p *models.Project) models.JSONMap {
	if p.Settings == nil {
		return models.JSONMap{}
	}
	return p.Settings
}

func nested(m models.JSONMap, key string) models.JSONMap {
	if raw, ok := m[key]; ok {
		if mm, ok2 := raw.(map[string]any); ok2 {
			return models.JSONMap(mm)
		}
		if mm, ok2 := raw.(models.JSONMap); ok2 {
			return mm
		}
	}
	return models.JSONMap{}
}

func widgetConfig(p *models.Project, online bool) map[string]any {
	s := projectSettings(p)
	colors := nested(s, "colors")
	greeting := strOfAny(s["greeting"])
	if greeting == "" {
		greeting = "سلام 👋 چطور می‌توانیم کمک کنیم؟"
	}
	offline := strOfAny(s["offline_message"])
	if offline == "" {
		offline = "در حال حاضر آفلاین هستیم؛ پیام بگذارید تا در اولین فرصت پاسخ دهیم."
	}
	position := strOfAny(s["position"])
	if position == "" {
		position = "bottom-left"
	}
	locale := strOfAny(s["locale"])
	if locale == "" {
		locale = "fa"
	}
	if colors == nil || len(colors) == 0 {
		colors = models.JSONMap{"primary": "#2563eb", "text": "#0f172a", "background": "#ffffff", "launcher": "#2563eb"}
	}
	return map[string]any{
		"project_id":       p.ID,
		"name":             p.Name,
		"locale":           locale,
		"direction":        "rtl",
		"colors":           colors,
		"position":         position,
		"greeting":         greeting,
		"offline_message":  offline,
		"pre_chat_form":    s["pre_chat_form"],
		"business_hours":   s["business_hours"],
		"proactive":        s["proactive"],
		"widget_title":     strOfAny(s["widget_title"]),
		"show_branding":    true,
		"online":           online,
		"session_ttl_days": 30,
	}
}

func strOfAny(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

type widgetSessionRequest struct {
	Token     string `json:"token"`
	Locale    string `json:"locale"`
	Challenge string `json:"challenge"`
	Nonce     string `json:"nonce"`
	Visitor   struct {
		Name          string         `json:"name"`
		Website       string         `json:"website"`
		Email         string         `json:"email"`
		Mobile        string         `json:"mobile"`
		EVEUserID     string         `json:"eve_user_id"`
		EVEResellerID string         `json:"eve_reseller_id"`
		EVEMobile     string         `json:"eve_mobile"`
		Metadata      models.JSONMap `json:"metadata"`
		Subscription  models.JSONMap `json:"subscription"`
	} `json:"visitor"`
	Page struct {
		URL      string `json:"url"`
		Title    string `json:"title"`
		Referrer string `json:"referrer"`
	} `json:"page"`
}

// WidgetSession creates or resumes a visitor session.
func (h *Handlers) WidgetSession(w http.ResponseWriter, r *http.Request) error {
	project, okp := middleware.ProjectFrom(r.Context())
	if !okp {
		return apierr.Unauthorized("کلید پروژه معتبر نیست")
	}
	var in widgetSessionRequest
	if err := bind(r, &in); err != nil {
		return err
	}
	// Honeypot: real visitors never fill the hidden "website" field.
	if strings.TrimSpace(in.Visitor.Website) != "" {
		return apierr.BadRequest("درخواست نامعتبر است")
	}
	if base := h.State.POWDifficulty(h.Cfg.Security.POWDifficulty); base > 0 {
		if in.Challenge == "" || !h.State.Pow().Verify(in.Challenge, in.Nonce) {
			return apierr.New(http.StatusForbidden, "pow_required", "برای ادامه، چالش امنیتی را حل کنید")
		}
	}
	ip := middleware.ClientIP(r, h.Cfg.Server.TrustProxy)
	h.State.Throttle().Hit(ip)

	v, sess, err := h.Visitors.IdentifyOrCreate(r.Context(), services.SessionRequest{
		ProjectID: project.ID,
		Token:     strings.TrimSpace(in.Token),
		Page:      services.PageInfo{URL: in.Page.URL, Title: in.Page.Title, Referrer: in.Page.Referrer},
		Visitor: services.VisitorInput{
			Name:          in.Visitor.Name,
			Email:         in.Visitor.Email,
			Mobile:        in.Visitor.Mobile,
			EVEUserID:     in.Visitor.EVEUserID,
			EVEResellerID: in.Visitor.EVEResellerID,
			EVEMobile:     in.Visitor.EVEMobile,
			Metadata:      in.Visitor.Metadata,
			Subscription:  in.Visitor.Subscription,
		},
		IP:        ip,
		UserAgent: r.UserAgent(),
		Language:  in.Locale,
	})
	if err != nil {
		return apierr.Internal(err)
	}

	// Best-effort EVE enrichment for the visitor profile.
	h.enrichFromEVE(r.Context(), v)

	resp := map[string]any{
		"token":      sess.Token,
		"visitor_id": v.ID,
		"expires_at": sess.ExpiresAt,
		"visitor":    v,
		"online":     len(h.Hub.OnlineAgents(project.ID)) > 0,
	}
	if conv, err := h.Repo.Conversations.OpenForVisitor(r.Context(), v.ID); err == nil {
		_ = h.Repo.Sessions.SetActiveConv(r.Context(), sess.Token, conv.ID)
		msgs, _ := h.Msgs.Latest(r.Context(), conv.ID, 50)
		resp["conversation"] = conv
		resp["messages"] = msgs
	}
	writeJSON(w, http.StatusOK, resp)
	return nil
}

func (h *Handlers) enrichFromEVE(ctx context.Context, v *models.Visitor) {
	if h.EVE == nil || !h.EVE.Enabled() || v == nil {
		return
	}
	mobile := ""
	if v.EVEMobile != nil {
		mobile = *v.EVEMobile
	}
	if mobile == "" && v.Phone != nil {
		mobile = *v.Phone
	}
	if mobile == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	u, err := h.EVE.GetUserByMobile(ctx, mobile)
	if err != nil || u == nil {
		return
	}
	changed := false
	if u.UserID != "" && (v.EVEUserID == nil || *v.EVEUserID != u.UserID) {
		uid := u.UserID
		v.EVEUserID = &uid
		changed = true
	}
	if u.ResellerID != "" && (v.EVEResellerID == nil || *v.EVEResellerID != u.ResellerID) {
		rid := u.ResellerID
		v.EVEResellerID = &rid
		changed = true
	}
	if len(u.Subscription) > 0 {
		if v.SubscriptionInfo == nil {
			v.SubscriptionInfo = models.JSONMap{}
		}
		for k, val := range u.Subscription {
			v.SubscriptionInfo[k] = val
		}
		changed = true
	}
	if changed {
		_ = h.Repo.Visitors.Upsert(ctx, v)
	}
}

// widgetSession authenticates a widget request by session token.
func (h *Handlers) widgetSession(r *http.Request) (*models.VisitorSession, error) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		token = strings.TrimSpace(r.Header.Get("X-Widget-Token"))
	}
	if token == "" {
		return nil, apierr.Unauthorized("نشست ویجت یافت نشد")
	}
	sess, err := h.Repo.Sessions.Get(r.Context(), token)
	if err != nil {
		return nil, apierr.Unauthorized("نشست شما منقضی شده است")
	}
	return sess, nil
}

func (h *Handlers) activeConversation(r *http.Request, sess *models.VisitorSession) *models.Conversation {
	ctx := r.Context()
	if sess.ActiveConvID != nil {
		if conv, err := h.Repo.Conversations.GetByID(ctx, *sess.ActiveConvID); err == nil && conv.VisitorID == sess.VisitorID {
			if conv.Status == models.StatusPending || conv.Status == models.StatusActive {
				return conv
			}
		}
	}
	conv, err := h.Repo.Conversations.OpenForVisitor(ctx, sess.VisitorID)
	if err != nil {
		return nil
	}
	_ = h.Repo.Sessions.SetActiveConv(ctx, sess.Token, conv.ID)
	return conv
}

// WidgetEvents streams envelopes over Server-Sent Events.
func (h *Handlers) WidgetEvents(w http.ResponseWriter, r *http.Request) error {
	sess, err := h.widgetSession(r)
	if err != nil {
		return err
	}
	flusher, okf := w.(http.Flusher)
	if !okf {
		return apierr.Internal(nil)
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, "retry: 3000\n\n")
	flusher.Flush()

	// Replay the last minute so a reconnect does not lose a frame.
	since := time.Now().UnixMilli() - 60_000
	convID := ""
	if conv := h.activeConversation(r, sess); conv != nil {
		convID = conv.ID
	}
	keepalive := time.NewTicker(20 * time.Second)
	defer keepalive.Stop()
	poll := time.NewTicker(time.Second)
	defer poll.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-keepalive.C:
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return nil
			}
			flusher.Flush()
		case <-poll.C:
			if convID == "" {
				if conv := h.activeConversation(r, sess); conv != nil {
					convID = conv.ID
					since = time.Now().UnixMilli() - 60_000
				}
				continue
			}
			events, ok := h.Hub.SyncBuffer(convID, since)
			if !ok || len(events) == 0 {
				continue
			}
			for _, e := range events {
				b, merr := json.Marshal(e)
				if merr != nil {
					continue
				}
				if _, werr := fmt.Fprintf(w, "data: %s\n\n", b); werr != nil {
					return nil
				}
				since = e.TS
			}
			flusher.Flush()
		}
	}
}

// WidgetPoll long-polls for envelopes newer than ?since=.
func (h *Handlers) WidgetPoll(w http.ResponseWriter, r *http.Request) error {
	sess, err := h.widgetSession(r)
	if err != nil {
		return err
	}
	since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
	convID := ""
	if conv := h.activeConversation(r, sess); conv != nil {
		convID = conv.ID
	}
	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		if err := r.Context().Err(); err != nil {
			return nil
		}
		if convID != "" {
			if events, ok := h.Hub.SyncBuffer(convID, since); ok && len(events) > 0 {
				writeJSON(w, http.StatusOK, map[string]any{
					"events": events,
					"now":    events[len(events)-1].TS,
				})
				return nil
			}
		}
		select {
		case <-r.Context().Done():
			return nil
		case <-time.After(400 * time.Millisecond):
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": []any{}, "now": time.Now().UnixMilli()})
	return nil
}

// WidgetPostMessage is the REST send path used by SSE/poll transports.
func (h *Handlers) WidgetPostMessage(w http.ResponseWriter, r *http.Request) error {
	sess, err := h.widgetSession(r)
	if err != nil {
		return err
	}
	convID := chiURLParam(r, "id")
	conv, err := h.Repo.Conversations.GetByID(r.Context(), convID)
	if err != nil || conv.VisitorID != sess.VisitorID {
		return apierr.Forbidden("شما به این گفتگو دسترسی ندارید")
	}
	var in struct {
		Content         string `json:"content"`
		ContentType     string `json:"content_type"`
		ClientMessageID string `json:"client_message_id"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	msg, _, err := h.Msgs.Send(r.Context(), services.SendInput{
		ConversationID:  convID,
		SenderType:      models.SenderVisitor,
		SenderID:        sess.VisitorID,
		Content:         in.Content,
		ContentType:     in.ContentType,
		ClientMessageID: in.ClientMessageID,
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, msg)
	return nil
}

// WidgetCSAT stores a satisfaction score from the widget.
func (h *Handlers) WidgetCSAT(w http.ResponseWriter, r *http.Request) error {
	sess, err := h.widgetSession(r)
	if err != nil {
		return err
	}
	var in struct {
		ConversationID string `json:"conversation_id"`
		Score          int    `json:"score"`
		Comment        string `json:"comment"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	convID := in.ConversationID
	if convID == "" {
		if conv := h.activeConversation(r, sess); conv != nil {
			convID = conv.ID
		}
	}
	conv, err := h.Repo.Conversations.GetByID(r.Context(), convID)
	if err != nil || conv.VisitorID != sess.VisitorID {
		return apierr.Forbidden("شما به این گفتگو دسترسی ندارید")
	}
	if err := h.Convs.SubmitCSAT(r.Context(), convID, in.Score, in.Comment); err != nil {
		return err
	}
	return okMsg(w)
}

// WidgetSync returns the reconciliation payload for a conversation.
func (h *Handlers) WidgetSync(w http.ResponseWriter, r *http.Request) error {
	sess, err := h.widgetSession(r)
	if err != nil {
		return err
	}
	convID := chiURLParam(r, "id")
	conv, err := h.Repo.Conversations.GetByID(r.Context(), convID)
	if err != nil || conv.VisitorID != sess.VisitorID {
		return apierr.Forbidden("شما به این گفتگو دسترسی ندارید")
	}
	since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
	res, err := h.Realtime.Sync(r.Context(), convID, since)
	if err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusOK, res)
	return nil
}

// WidgetChallenge issues a single-use proof-of-work challenge. Difficulty is
// adaptive: repeat offenders from the same IP (and low-reputation visitors)
// receive a harder challenge.
func (h *Handlers) WidgetChallenge(w http.ResponseWriter, r *http.Request) error {
	base := h.State.POWDifficulty(h.Cfg.Security.POWDifficulty)
	if base <= 0 {
		writeJSON(w, http.StatusOK, map[string]any{"challenge": "", "difficulty": 0, "expires_at": 0})
		return nil
	}
	ip := middleware.ClientIP(r, h.Cfg.Server.TrustProxy)
	difficulty := security.DifficultyFor(base, h.State.Throttle().Count(ip))
	if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" {
		if sess, err := h.Repo.Sessions.Get(r.Context(), token); err == nil {
			if v, verr := h.Repo.Visitors.GetByID(r.Context(), sess.VisitorID); verr == nil && v.Reputation < 50 {
				difficulty += 4
			}
		}
	}
	challenge, expires := h.State.Pow().Issue(difficulty)
	writeJSON(w, http.StatusOK, map[string]any{
		"challenge":  challenge,
		"difficulty": difficulty,
		"expires_at": expires,
	})
	return nil
}
