package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/security"
)

// ListWebhooks returns the project webhooks.
func (h *Handlers) ListWebhooks(w http.ResponseWriter, r *http.Request) error {
	hooks, err := h.Repo.Webhooks.List(r.Context(), h.scopeProject(r), false)
	if err != nil {
		return apierr.Internal(err)
	}
	for _, hook := range hooks {
		hook.Secret = ""
	}
	writeJSON(w, http.StatusOK, newList(hooks, len(hooks), len(hooks), 0))
	return nil
}

type webhookInput struct {
	URL         string   `json:"url"`
	Events      []string `json:"events"`
	Description *string  `json:"description"`
	IsActive    *bool    `json:"is_active"`
}

// CreateWebhook registers a webhook and returns its signing secret once.
func (h *Handlers) CreateWebhook(w http.ResponseWriter, r *http.Request) error {
	if err := h.requireManager(r); err != nil {
		return err
	}
	var in webhookInput
	if err := bind(r, &in); err != nil {
		return err
	}
	if !strings.HasPrefix(in.URL, "http://") && !strings.HasPrefix(in.URL, "https://") {
		return apierr.BadRequest("آدرس webhook باید با http یا https شروع شود")
	}
	if err := security.ValidateOutboundURL(in.URL); err != nil {
		return apierr.BadRequest("آدرس webhook به مقصد مجاز اشاره نمی‌کند")
	}
	secret := randomHex(24)
	active := true
	if in.IsActive != nil {
		active = *in.IsActive
	}
	hook := &models.Webhook{
		ID:          id.NewPrefixed(id.PrefixWebhook),
		ProjectID:   h.scopeProject(r),
		URL:         in.URL,
		Events:      models.StringList(in.Events),
		Secret:      secret,
		Description: in.Description,
		IsActive:    active,
	}
	if len(hook.Events) == 0 {
		hook.Events = models.StringList{"*"}
	}
	if err := h.Repo.Webhooks.Create(r.Context(), hook); err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusCreated, hook)
	return nil
}

// GetWebhook returns one webhook.
func (h *Handlers) GetWebhook(w http.ResponseWriter, r *http.Request) error {
	hook, err := h.Repo.Webhooks.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("webhook یافت نشد")
	}
	if err := h.checkProjectScope(r, hook.ProjectID); err != nil {
		return err
	}
	hook.Secret = ""
	return ok(w, hook)
}

// UpdateWebhook edits a webhook.
func (h *Handlers) UpdateWebhook(w http.ResponseWriter, r *http.Request) error {
	if err := h.requireManager(r); err != nil {
		return err
	}
	hook, err := h.Repo.Webhooks.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("webhook یافت نشد")
	}
	if err := h.checkProjectScope(r, hook.ProjectID); err != nil {
		return err
	}
	var in webhookInput
	if err := bind(r, &in); err != nil {
		return err
	}
	if in.URL != "" {
		if err := security.ValidateOutboundURL(in.URL); err != nil {
			return apierr.BadRequest("آدرس webhook به مقصد مجاز اشاره نمی‌کند")
		}
		hook.URL = in.URL
	}
	if in.Events != nil {
		hook.Events = models.StringList(in.Events)
	}
	if in.Description != nil {
		hook.Description = in.Description
	}
	if in.IsActive != nil {
		hook.IsActive = *in.IsActive
	}
	if err := h.Repo.Webhooks.Update(r.Context(), hook); err != nil {
		return apierr.Internal(err)
	}
	hook.Secret = ""
	return ok(w, hook)
}

// DeleteWebhook removes a webhook.
func (h *Handlers) DeleteWebhook(w http.ResponseWriter, r *http.Request) error {
	if err := h.requireManager(r); err != nil {
		return err
	}
	hook, err := h.Repo.Webhooks.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("webhook یافت نشد")
	}
	if err := h.checkProjectScope(r, hook.ProjectID); err != nil {
		return err
	}
	if err := h.Repo.Webhooks.Delete(r.Context(), hook.ID); err != nil {
		return apierr.Internal(err)
	}
	return okMsg(w)
}

// ListWebhookDeliveries returns the delivery log.
func (h *Handlers) ListWebhookDeliveries(w http.ResponseWriter, r *http.Request) error {
	hook, err := h.Repo.Webhooks.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("webhook یافت نشد")
	}
	if err := h.checkProjectScope(r, hook.ProjectID); err != nil {
		return err
	}
	deliveries, err := h.Repo.Webhooks.ListDeliveries(r.Context(), hook.ID, atoiDefault(r.URL.Query().Get("limit"), 50))
	if err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusOK, newList(deliveries, len(deliveries), len(deliveries), 0))
	return nil
}

// TestWebhook sends a sample delivery.
func (h *Handlers) TestWebhook(w http.ResponseWriter, r *http.Request) error {
	hook, err := h.Repo.Webhooks.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("webhook یافت نشد")
	}
	if err := h.checkProjectScope(r, hook.ProjectID); err != nil {
		return err
	}
	if err := h.Webhooks.Test(r.Context(), hook); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return nil
	}
	return okMsg(w)
}

// ListCanned returns the saved replies.
func (h *Handlers) ListCanned(w http.ResponseWriter, r *http.Request) error {
	items, err := h.Repo.Canned.List(r.Context(), h.scopeProject(r))
	if err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusOK, newList(items, len(items), len(items), 0))
	return nil
}

type cannedInput struct {
	Shortcut string `json:"shortcut"`
	Title    string `json:"title"`
	Body     string `json:"body"`
}

// CreateCanned adds a saved reply.
func (h *Handlers) CreateCanned(w http.ResponseWriter, r *http.Request) error {
	var in cannedInput
	if err := bind(r, &in); err != nil {
		return err
	}
	in.Shortcut = strings.TrimSpace(in.Shortcut)
	if in.Shortcut == "" || in.Title == "" || in.Body == "" {
		return apierr.BadRequest("نام کوتاه، عنوان و متن لازم است")
	}
	c := &models.CannedResponse{
		ProjectID: h.scopeProject(r),
		Shortcut:  in.Shortcut,
		Title:     in.Title,
		Body:      in.Body,
	}
	if err := h.Repo.Canned.Create(r.Context(), c); err != nil {
		if err.Error() == "duplicate" {
			return apierr.Conflict("این نام کوتاه قبلاً استفاده شده است")
		}
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusCreated, c)
	return nil
}

// UpdateCanned edits a saved reply.
func (h *Handlers) UpdateCanned(w http.ResponseWriter, r *http.Request) error {
	c, err := h.Repo.Canned.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("پاسخ آماده یافت نشد")
	}
	if err := h.checkProjectScope(r, c.ProjectID); err != nil {
		return err
	}
	var in cannedInput
	if err := bind(r, &in); err != nil {
		return err
	}
	if in.Shortcut != "" {
		c.Shortcut = in.Shortcut
	}
	if in.Title != "" {
		c.Title = in.Title
	}
	if in.Body != "" {
		c.Body = in.Body
	}
	if err := h.Repo.Canned.Update(r.Context(), c); err != nil {
		return apierr.Internal(err)
	}
	return ok(w, c)
}

// DeleteCanned removes a saved reply.
func (h *Handlers) DeleteCanned(w http.ResponseWriter, r *http.Request) error {
	c, err := h.Repo.Canned.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("پاسخ آماده یافت نشد")
	}
	if err := h.checkProjectScope(r, c.ProjectID); err != nil {
		return err
	}
	if err := h.Repo.Canned.Delete(r.Context(), c.ID); err != nil {
		return apierr.Internal(err)
	}
	return okMsg(w)
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return id.New() + id.New()
	}
	return hex.EncodeToString(b)
}
