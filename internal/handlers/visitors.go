package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

// ListVisitors returns the visitors table page.
func (h *Handlers) ListVisitors(w http.ResponseWriter, r *http.Request) error {
	limit, offset := parsePage(r)
	q := r.URL.Query()
	f := repo.VisitorFilter{
		Query:      q.Get("q"),
		Online:     q.Get("online") == "1" || q.Get("online") == "true",
		Reseller:   q.Get("reseller"),
		DeviceType: q.Get("device"),
		Limit:      limit,
		Offset:     offset,
	}
	if b := q.Get("banned"); b != "" {
		v := b == "1" || b == "true"
		f.Banned = &v
	}
	visitors, total, err := h.Repo.Visitors.List(r.Context(), h.scopeProject(r), f)
	if err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusOK, newList(visitors, total, limit, offset))
	return nil
}

// GetVisitor returns one visitor with their conversation history.
func (h *Handlers) GetVisitor(w http.ResponseWriter, r *http.Request) error {
	v, err := h.Repo.Visitors.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("کاربر یافت نشد")
	}
	if err := h.checkProjectScope(r, v.ProjectID); err != nil {
		return err
	}
	convs, err := h.Repo.Conversations.ListByVisitor(r.Context(), v.ID, 20)
	if err != nil {
		return apierr.Internal(err)
	}
	online := h.Hub.IsVisitorOnline(v.ID)
	writeJSON(w, http.StatusOK, map[string]any{
		"visitor":       v,
		"conversations": convs,
		"online":        online,
	})
	return nil
}

// VisitorConversations lists the conversations of one visitor.
func (h *Handlers) VisitorConversations(w http.ResponseWriter, r *http.Request) error {
	v, err := h.Repo.Visitors.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("کاربر یافت نشد")
	}
	if err := h.checkProjectScope(r, v.ProjectID); err != nil {
		return err
	}
	limit, offset := parsePage(r)
	convs, total, err := h.Repo.Conversations.List(r.Context(), v.ProjectID, repo.ConvFilter{
		VisitorID: v.ID, Limit: limit, Offset: offset, Status: "all",
	})
	if err != nil {
		return apierr.Internal(err)
	}
	items, err := h.enrichConversations(r, convs)
	if err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusOK, newList(items, total, limit, offset))
	return nil
}

// BanVisitor blocks a visitor.
func (h *Handlers) BanVisitor(w http.ResponseWriter, r *http.Request) error {
	v, err := h.Repo.Visitors.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("کاربر یافت نشد")
	}
	if err := h.checkProjectScope(r, v.ProjectID); err != nil {
		return err
	}
	var in struct {
		Duration string `json:"duration"`
	}
	_ = bind(r, &in)
	var until *time.Time
	switch strings.ToLower(in.Duration) {
	case "", "permanent":
		until = nil
	case "1h", "1hour", "hour":
		t := time.Now().UTC().Add(time.Hour)
		until = &t
	case "24h", "1d", "day":
		t := time.Now().UTC().Add(24 * time.Hour)
		until = &t
	case "7d", "week":
		t := time.Now().UTC().AddDate(0, 0, 7)
		until = &t
	default:
		return apierr.BadRequest("مدت مسدودسازی نامعتبر است (1h، 24h یا permanent)")
	}
	if err := h.Repo.Visitors.Ban(r.Context(), v.ID, until); err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "banned_until": until})
	return nil
}

// UnbanVisitor lifts a ban.
func (h *Handlers) UnbanVisitor(w http.ResponseWriter, r *http.Request) error {
	v, err := h.Repo.Visitors.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("کاربر یافت نشد")
	}
	if err := h.checkProjectScope(r, v.ProjectID); err != nil {
		return err
	}
	if err := h.Repo.Visitors.Unban(r.Context(), v.ID); err != nil {
		return apierr.Internal(err)
	}
	return okMsg(w)
}

// ExportVisitor produces a GDPR export of everything stored about a visitor.
func (h *Handlers) ExportVisitor(w http.ResponseWriter, r *http.Request) error {
	v, err := h.Repo.Visitors.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("کاربر یافت نشد")
	}
	if err := h.checkProjectScope(r, v.ProjectID); err != nil {
		return err
	}
	convs, err := h.Repo.Conversations.ListByVisitor(r.Context(), v.ID, 100)
	if err != nil {
		return apierr.Internal(err)
	}
	type convExport struct {
		Conversation *models.Conversation `json:"conversation"`
		Messages     []*models.Message    `json:"messages"`
	}
	out := make([]convExport, 0, len(convs))
	for _, c := range convs {
		msgs, merr := h.Msgs.Latest(r.Context(), c.ID, 500)
		if merr != nil {
			continue
		}
		out = append(out, convExport{Conversation: c, Messages: msgs})
	}
	w.Header().Set("Content-Disposition", `attachment; filename="visitor-export.json"`)
	writeJSON(w, http.StatusOK, map[string]any{
		"visitor":       v,
		"conversations": out,
		"exported_at":   time.Now().UTC(),
	})
	return nil
}

// AnonymizeVisitor erases personal data (GDPR).
func (h *Handlers) AnonymizeVisitor(w http.ResponseWriter, r *http.Request) error {
	v, err := h.Repo.Visitors.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("کاربر یافت نشد")
	}
	if err := h.checkProjectScope(r, v.ProjectID); err != nil {
		return err
	}
	if err := h.Repo.Visitors.Anonymize(r.Context(), v.ID); err != nil {
		return apierr.Internal(err)
	}
	_ = h.Repo.Sessions.DeleteByVisitor(r.Context(), v.ID)
	return okMsg(w)
}

// UpdateVisitor edits visitor identity fields.
func (h *Handlers) UpdateVisitor(w http.ResponseWriter, r *http.Request) error {
	v, err := h.Repo.Visitors.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("کاربر یافت نشد")
	}
	if err := h.checkProjectScope(r, v.ProjectID); err != nil {
		return err
	}
	var in struct {
		Name     *string        `json:"name"`
		Email    *string        `json:"email"`
		Phone    *string        `json:"phone"`
		Metadata models.JSONMap `json:"metadata"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	if in.Name != nil {
		v.Name = in.Name
	}
	if in.Email != nil {
		v.Email = in.Email
	}
	if in.Phone != nil {
		v.Phone = in.Phone
	}
	if in.Metadata != nil {
		v.Metadata = in.Metadata
	}
	if err := h.Repo.Visitors.Update(r.Context(), v); err != nil {
		return apierr.Internal(err)
	}
	return ok(w, v)
}

// checkProjectScope forbids cross-project access for non superadmins.
func (h *Handlers) checkProjectScope(r *http.Request, projectID string) error {
	c := claimsOf(r)
	if c == nil {
		return apierr.Unauthorized("احراز هویت لازم است")
	}
	if c.Role == models.RoleSuperAdmin || c.ProjectID == projectID {
		return nil
	}
	return apierr.Forbidden("شما به این منبع دسترسی ندارید")
}

// parseIDList parses repeated ?id= query params.
func parseIDList(r *http.Request, key string) []string {
	vals := r.URL.Query()[key]
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
