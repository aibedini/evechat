package handlers

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/models"
)

// wallboardSetting is the project setting holding the read-only wallboard token.
const wallboardSetting = "wallboard_token"

// WallboardToken issues (or rotates) the read-only token of the project.
func (h *Handlers) WallboardToken(w http.ResponseWriter, r *http.Request) error {
	if err := h.requireManager(r); err != nil {
		return err
	}
	project, err := h.projectOf(r.Context(), h.scopeProject(r))
	if err != nil {
		return err
	}
	if project.Settings == nil {
		project.Settings = models.JSONMap{}
	}
	token := id.Token(24)
	project.Settings[wallboardSetting] = token
	if err := h.Repo.Projects.Update(r.Context(), project); err != nil {
		return apierr.Internal(err)
	}
	h.recordAudit(r, project.ID, "settings.wallboard_token_rotated", "project", project.ID, nil)
	return ok(w, map[string]any{"token": token, "url": h.Cfg.BaseURL() + "/wallboard?token=" + token})
}

// RevokeWallboardToken disables the current wallboard link.
func (h *Handlers) RevokeWallboardToken(w http.ResponseWriter, r *http.Request) error {
	if err := h.requireManager(r); err != nil {
		return err
	}
	project, err := h.projectOf(r.Context(), h.scopeProject(r))
	if err != nil {
		return err
	}
	if project.Settings != nil {
		delete(project.Settings, wallboardSetting)
		if err := h.Repo.Projects.Update(r.Context(), project); err != nil {
			return apierr.Internal(err)
		}
	}
	return okMsg(w)
}

// Wallboard is the public, read-only endpoint behind /wallboard. The token is
// the only credential and the payload deliberately contains aggregates only:
// no message bodies, no visitor identities, no email addresses.
func (h *Handlers) Wallboard(w http.ResponseWriter, r *http.Request) error {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		return apierr.Unauthorized("توکن لازم است")
	}
	projects, err := h.Repo.Projects.List(r.Context())
	if err != nil {
		return apierr.Internal(err)
	}
	var project *models.Project
	for _, p := range projects {
		candidate, _ := p.Settings[wallboardSetting].(string)
		if candidate != "" && subtle.ConstantTimeCompare([]byte(candidate), []byte(token)) == 1 {
			project = p
			break
		}
	}
	if project == nil {
		return apierr.Unauthorized("توکن نامعتبر است")
	}
	now := time.Now().UTC()
	stats, err := h.Repo.Stats.Overview(r.Context(), project.ID, now.Add(-24*time.Hour), now)
	if err != nil {
		return apierr.Internal(err)
	}
	agents, _ := h.Repo.Agents.List(r.Context(), project.ID)
	online := 0
	for _, a := range agents {
		if a.IsOnline && a.IsActive {
			online++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"project":                project.Name,
		"generated_at":           now,
		"window":                 "24h",
		"agents":                 map[string]any{"online": online, "total": len(agents)},
		"pending":                stats.Pending,
		"active":                 stats.Active,
		"resolved":               stats.Resolved,
		"csat_avg":               stats.CSATAvg,
		"first_response_seconds": stats.AvgFirstResponseSeconds,
	})
	return nil
}
