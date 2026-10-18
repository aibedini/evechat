package handlers

import (
	"net/http"
	"strings"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/models"
)

// GetProjectSettings returns the project with its widget settings.
func (h *Handlers) GetProjectSettings(w http.ResponseWriter, r *http.Request) error {
	project, err := h.projectOf(r.Context(), h.scopeProject(r))
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"project": project,
		"widget":  widgetConfig(project, len(h.Hub.OnlineAgents(project.ID)) > 0),
		"embed_code": `<script>window.EveChat={projectKey:"` + project.APIKey + `",apiUrl:"` + h.Cfg.BaseURL() + `"};</script>
<script async src="` + h.Cfg.BaseURL() + `/widget.js"></script>`,
	})
	return nil
}

// UpdateProjectSettings merges new settings into the project.
func (h *Handlers) UpdateProjectSettings(w http.ResponseWriter, r *http.Request) error {
	if err := h.requireManager(r); err != nil {
		return err
	}
	project, err := h.projectOf(r.Context(), h.scopeProject(r))
	if err != nil {
		return err
	}
	var in struct {
		Name     *string        `json:"name"`
		Domains  []string       `json:"domains"`
		Settings models.JSONMap `json:"settings"`
		IsActive *bool          `json:"is_active"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	if in.Name != nil && strings.TrimSpace(*in.Name) != "" {
		project.Name = strings.TrimSpace(*in.Name)
	}
	if in.Domains != nil {
		project.Domains = models.StringList(in.Domains)
	}
	if in.IsActive != nil {
		project.IsActive = *in.IsActive
	}
	if in.Settings != nil {
		if project.Settings == nil {
			project.Settings = models.JSONMap{}
		}
		mergeJSON(project.Settings, in.Settings)
	}
	if err := h.Repo.Projects.Update(r.Context(), project); err != nil {
		return apierr.Internal(err)
	}
	return ok(w, project)
}

// mergeJSON recursively merges src into dst.
func mergeJSON(dst, src models.JSONMap) {
	for k, v := range src {
		if sub, ok := v.(map[string]any); ok {
			if existing, ok2 := dst[k].(map[string]any); ok2 {
				mergeJSON(existing, sub)
				continue
			}
			dst[k] = sub
			continue
		}
		dst[k] = v
	}
}

// RotateAPIKey regenerates the public widget key.
func (h *Handlers) RotateAPIKey(w http.ResponseWriter, r *http.Request) error {
	if err := h.requireManager(r); err != nil {
		return err
	}
	project, err := h.projectOf(r.Context(), h.scopeProject(r))
	if err != nil {
		return err
	}
	newKey := id.NewPrefixed("pk")
	if err := h.Repo.Projects.RotateAPIKey(r.Context(), project.ID, newKey); err != nil {
		return apierr.Internal(err)
	}
	project.APIKey = newKey
	return ok(w, project)
}
