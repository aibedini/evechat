package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/config"
	"github.com/evechat/evechat/internal/crypto"
	"github.com/evechat/evechat/internal/hub"
	"github.com/evechat/evechat/internal/integrations/eve"
	"github.com/evechat/evechat/internal/integrations/gmweb"
	"github.com/evechat/evechat/internal/integrations/telegram"
	"github.com/evechat/evechat/internal/logx"
	"github.com/evechat/evechat/internal/middleware"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
	"github.com/evechat/evechat/internal/services"
)

// Handlers holds every HTTP dependency. One instance serves the whole app.
type Handlers struct {
	Cfg      *config.Config
	Log      zerolog.Logger
	Repo     *repo.Repo
	Hub      *hub.Hub
	Tokens   *middleware.TokenManager
	Enc      *crypto.Service
	Visitors *services.VisitorService
	Convs    *services.ConversationService
	Msgs     *services.MessageService
	Routing  *services.RoutingService
	Search   *services.SearchService
	Tickets  *services.TicketService
	Webhooks *services.WebhookService
	Realtime *services.Realtime
	Notif    *services.NotificationService
	AI       *services.AIService
	SMS      *services.SMSChannel
	EVE      *eve.Client
	GMweb    *gmweb.Client
	Telegram *telegram.Bot

	StartedAt time.Time
	Version   string
}

// H adapts a handler returning error into an http.HandlerFunc.
func (h *Handlers) H(fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			var ae *apierr.Error
			if !errors.As(err, &ae) {
				logx.WithReq(r.Context()).Error().Err(err).Str("path", r.URL.Path).Str("method", r.Method).Msg("handler error")
			}
			apierr.Write(w, err)
		}
	}
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// listResponse is the standard paginated envelope.
type listResponse struct {
	Data   any `json:"data"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

func newList(data any, total, limit, offset int) listResponse {
	if data == nil {
		data = []any{}
	}
	return listResponse{Data: data, Total: total, Limit: limit, Offset: offset}
}

func parsePage(r *http.Request) (int, int) {
	limit := 25
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// bind decodes a JSON body with a 1MB limit.
func bind(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return apierr.BadRequest("بدنه درخواست خالی است")
		}
		return apierr.BadRequest("داده‌های ارسالی نامعتبر است")
	}
	return nil
}

// claimsOf returns the authenticated dashboard claims.
func claimsOf(r *http.Request) *middleware.Claims {
	c, _ := middleware.CurrentAgent(r.Context())
	return c
}

// callerAgent returns the full agent row of the caller.
func (h *Handlers) callerAgent(r *http.Request) (*models.Agent, error) {
	c := claimsOf(r)
	if c == nil {
		return nil, apierr.Unauthorized("احراز هویت لازم است")
	}
	a, err := h.Repo.Agents.GetByID(r.Context(), c.Subject)
	if err != nil {
		return nil, apierr.Unauthorized("حساب کاربری یافت نشد")
	}
	return a, nil
}

// scopeProject returns the caller's project id, honouring superadmin overrides.
func (h *Handlers) scopeProject(r *http.Request) string {
	c := claimsOf(r)
	if c == nil {
		return ""
	}
	if override := r.URL.Query().Get("project_id"); override != "" && c.Role == models.RoleSuperAdmin {
		return override
	}
	return c.ProjectID
}

func parseTimeQuery(r *http.Request, key string) (time.Time, error) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		if t2, err2 := time.Parse("2006-01-02", v); err2 == nil {
			return t2.UTC(), nil
		}
		return time.Time{}, apierr.BadRequest("قالب تاریخ نامعتبر است (RFC3339 یا YYYY-MM-DD)")
	}
	return t.UTC(), nil
}

func defaultWindow(r *http.Request) (time.Time, time.Time, error) {
	from, err := parseTimeQuery(r, "from")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	to, err := parseTimeQuery(r, "to")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	now := time.Now().UTC()
	if to.IsZero() {
		to = now
	}
	if from.IsZero() {
		from = to.AddDate(0, 0, -14)
	}
	return from, to, nil
}

// ok writes the standard success envelope.
func ok(w http.ResponseWriter, v any) error {
	writeJSON(w, http.StatusOK, v)
	return nil
}

// okMsg writes {"ok":true}.
func okMsg(w http.ResponseWriter) error {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	return nil
}

// chiURLParam reads a path parameter.
func chiURLParam(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}

// timeNowH is the injectable clock used by handler helpers.
func timeNowH() time.Time { return time.Now().UTC() }

// envelopePresence builds a presence.update envelope.
func envelopePresence(agentID, name string, online bool) hub.Envelope {
	return hub.NewEnvelope(hub.TypePresenceUpdate, map[string]any{
		"agent_id": agentID,
		"name":     name,
		"online":   online,
	})
}
