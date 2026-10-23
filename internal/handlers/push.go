package handlers

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

type pushDeviceInput struct {
	DeviceID    string `json:"device_id"`
	Platform    string `json:"platform"`
	Provider    string `json:"provider"`
	Token       string `json:"token"`
	AppVersion  string `json:"app_version"`
	AppBuild    int    `json:"app_build"`
	OSVersion   string `json:"os_version"`
	DeviceModel string `json:"device_model"`
	Locale      string `json:"locale"`
	Timezone    string `json:"timezone"`
}

// RegisterPushDevice upserts the caller's FCM registration.
func (h *Handlers) RegisterPushDevice(w http.ResponseWriter, r *http.Request) error {
	c := claimsOf(r)
	if c == nil || c.Subject == "" || c.ProjectID == "" {
		return apierr.Unauthorized("احراز هویت لازم است")
	}
	var in pushDeviceInput
	if err := bind(r, &in); err != nil {
		return err
	}

	deviceID := strings.TrimSpace(in.DeviceID)
	token := strings.TrimSpace(in.Token)
	if deviceID == "" || len(deviceID) > 191 {
		return apierr.BadRequest("شناسه دستگاه معتبر نیست")
	}
	if token == "" || len(token) > 4096 {
		return apierr.BadRequest("توکن FCM معتبر نیست")
	}
	provider := strings.ToLower(strings.TrimSpace(in.Provider))
	if provider == "" {
		provider = "fcm"
	}
	if provider != "fcm" {
		return apierr.BadRequest("provider پشتیبانی نمی‌شود")
	}
	platform := strings.ToLower(strings.TrimSpace(in.Platform))
	if platform == "" {
		platform = "android"
	}
	appBuild := in.AppBuild
	if appBuild < 0 {
		appBuild = 0
	}

	d := &models.PushDevice{
		DeviceID:    deviceID,
		AgentID:     c.Subject,
		ProjectID:   c.ProjectID,
		Token:       token,
		Platform:    clipText(platform, 32),
		Provider:    provider,
		AppVersion:  clipText(in.AppVersion, 64),
		AppBuild:    appBuild,
		OSVersion:   clipText(in.OSVersion, 128),
		DeviceModel: clipText(in.DeviceModel, 128),
		Locale:      clipText(in.Locale, 32),
		Timezone:    clipText(in.Timezone, 64),
	}
	created, err := h.Repo.Push.Upsert(r.Context(), d)
	if err != nil {
		if errors.Is(err, repo.ErrConflict) {
			return apierr.Conflict("این توکن قبلاً برای دستگاه دیگری ثبت شده است")
		}
		return apierr.Internal(err)
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, map[string]any{})
	return nil
}

// DeletePushDevice removes the caller's device, or any device in the caller's
// project when the caller is an admin/superadmin.
func (h *Handlers) DeletePushDevice(w http.ResponseWriter, r *http.Request) error {
	c := claimsOf(r)
	if c == nil || c.Subject == "" || c.ProjectID == "" {
		return apierr.Unauthorized("احراز هویت لازم است")
	}
	deviceID := strings.TrimSpace(chiURLParam(r, "device_id"))
	if deviceID == "" {
		return apierr.BadRequest("شناسه دستگاه لازم است")
	}
	d, err := h.Repo.Push.Get(r.Context(), deviceID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return apierr.NotFound("دستگاه یافت نشد")
		}
		return apierr.Internal(err)
	}
	if d.ProjectID != c.ProjectID && c.Role != models.RoleSuperAdmin {
		return apierr.NotFound("دستگاه یافت نشد")
	}
	if d.AgentID != c.Subject && !c.Role.CanManageTeam() {
		return apierr.Forbidden("این دستگاه به شما تعلق ندارد")
	}
	if err := h.Repo.Push.Delete(r.Context(), deviceID); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return apierr.NotFound("دستگاه یافت نشد")
		}
		return apierr.Internal(err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func clipText(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" || max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return strings.TrimSpace(string(runes[:max]))
}
