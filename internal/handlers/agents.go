package handlers

import (
	"net/http"
	"strings"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/models"
)

// ListAgents returns the project team.
func (h *Handlers) ListAgents(w http.ResponseWriter, r *http.Request) error {
	limit, offset := parsePage(r)
	agents, err := h.Repo.Agents.List(r.Context(), h.scopeProject(r))
	if err != nil {
		return apierr.Internal(err)
	}
	total := len(agents)
	if offset > total {
		agents = nil
	} else {
		end := offset + limit
		if end > total {
			end = total
		}
		agents = agents[offset:end]
	}
	writeJSON(w, http.StatusOK, newList(agents, total, limit, offset))
	return nil
}

// GetAgent returns one team member.
func (h *Handlers) GetAgent(w http.ResponseWriter, r *http.Request) error {
	a, err := h.Repo.Agents.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("کارشناس یافت نشد")
	}
	if err := h.checkProjectScope(r, a.ProjectID); err != nil {
		return err
	}
	return ok(w, a)
}

type agentInput struct {
	Email            string  `json:"email"`
	Password         string  `json:"password"`
	Name             string  `json:"name"`
	Role             string  `json:"role"`
	TelegramID       *string `json:"telegram_id"`
	AvatarURL        *string `json:"avatar_url"`
	MaxConversations int     `json:"max_conversations"`
	IsActive         *bool   `json:"is_active"`
}

// CreateAgent adds a team member.
func (h *Handlers) CreateAgent(w http.ResponseWriter, r *http.Request) error {
	c := claimsOf(r)
	if c.Role != models.RoleAdmin && c.Role != models.RoleSuperAdmin {
		return apierr.Forbidden("فقط مدیران می‌توانند کارشناس اضافه کنند")
	}
	var in agentInput
	if err := bind(r, &in); err != nil {
		return err
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email == "" || !strings.Contains(email, "@") {
		return apierr.BadRequest("ایمیل معتبر لازم است")
	}
	if len(in.Password) < 8 {
		return apierr.BadRequest("رمز عبور باید حداقل ۸ کاراکتر باشد")
	}
	role := models.Role(in.Role)
	if role != models.RoleAgent && role != models.RoleAdmin && role != models.RoleSuperAdmin {
		role = models.RoleAgent
	}
	if role == models.RoleSuperAdmin && c.Role != models.RoleSuperAdmin {
		return apierr.Forbidden("فقط سوپرادمین می‌تواند سوپرادمین بسازد")
	}
	hash, err := HashPassword(in.Password, h.Cfg.Auth.BcryptCost)
	if err != nil {
		return apierr.Internal(err)
	}
	maxConv := in.MaxConversations
	if maxConv <= 0 {
		maxConv = 10
	}
	a := &models.Agent{
		ID:               id.NewPrefixed(id.PrefixAgent),
		ProjectID:        h.scopeProject(r),
		Email:            email,
		PasswordHash:     hash,
		Name:             strings.TrimSpace(in.Name),
		AvatarURL:        in.AvatarURL,
		Role:             role,
		TelegramID:       in.TelegramID,
		MaxConversations: maxConv,
		IsActive:         true,
	}
	if a.Name == "" {
		a.Name = email
	}
	if err := h.Repo.Agents.Create(r.Context(), a); err != nil {
		if err.Error() == "duplicate" {
			return apierr.Conflict("این ایمیل قبلاً ثبت شده است")
		}
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusCreated, a)
	return nil
}

// UpdateAgent edits a team member.
func (h *Handlers) UpdateAgent(w http.ResponseWriter, r *http.Request) error {
	c := claimsOf(r)
	if c.Role != models.RoleAdmin && c.Role != models.RoleSuperAdmin && c.Subject != chiURLParam(r, "id") {
		return apierr.Forbidden("دسترسی کافی ندارید")
	}
	a, err := h.Repo.Agents.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("کارشناس یافت نشد")
	}
	if err := h.checkProjectScope(r, a.ProjectID); err != nil {
		return err
	}
	var in agentInput
	if err := bind(r, &in); err != nil {
		return err
	}
	if in.Name != "" {
		a.Name = strings.TrimSpace(in.Name)
	}
	if in.Email != "" {
		a.Email = strings.ToLower(strings.TrimSpace(in.Email))
	}
	if in.TelegramID != nil {
		a.TelegramID = in.TelegramID
	}
	if in.AvatarURL != nil {
		a.AvatarURL = in.AvatarURL
	}
	if in.MaxConversations > 0 {
		a.MaxConversations = in.MaxConversations
	}
	if in.IsActive != nil {
		a.IsActive = *in.IsActive
	}
	if in.Role != "" {
		role := models.Role(in.Role)
		if role != models.RoleAgent && role != models.RoleAdmin && role != models.RoleSuperAdmin {
			return apierr.BadRequest("نقش نامعتبر است")
		}
		if role == models.RoleSuperAdmin && c.Role != models.RoleSuperAdmin {
			return apierr.Forbidden("فقط سوپرادمین می‌تواند سوپرادمین بسازد")
		}
		a.Role = role
	}
	if err := h.Repo.Agents.Update(r.Context(), a); err != nil {
		return apierr.Internal(err)
	}
	return ok(w, a)
}

// UpdateAgentPassword resets another agent's password (admin only).
func (h *Handlers) UpdateAgentPassword(w http.ResponseWriter, r *http.Request) error {
	c := claimsOf(r)
	if c.Role != models.RoleAdmin && c.Role != models.RoleSuperAdmin {
		return apierr.Forbidden("دسترسی کافی ندارید")
	}
	a, err := h.Repo.Agents.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("کارشناس یافت نشد")
	}
	if err := h.checkProjectScope(r, a.ProjectID); err != nil {
		return err
	}
	var in struct {
		NewPassword string `json:"new_password"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	if len(in.NewPassword) < 8 {
		return apierr.BadRequest("رمز عبور باید حداقل ۸ کاراکتر باشد")
	}
	hash, err := HashPassword(in.NewPassword, h.Cfg.Auth.BcryptCost)
	if err != nil {
		return apierr.Internal(err)
	}
	if err := h.Repo.Agents.UpdatePassword(r.Context(), a.ID, hash); err != nil {
		return apierr.Internal(err)
	}
	return okMsg(w)
}

// DeleteAgent removes a team member.
func (h *Handlers) DeleteAgent(w http.ResponseWriter, r *http.Request) error {
	c := claimsOf(r)
	if c.Role != models.RoleAdmin && c.Role != models.RoleSuperAdmin {
		return apierr.Forbidden("دسترسی کافی ندارید")
	}
	a, err := h.Repo.Agents.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("کارشناس یافت نشد")
	}
	if err := h.checkProjectScope(r, a.ProjectID); err != nil {
		return err
	}
	if a.ID == c.Subject {
		return apierr.BadRequest("حذف حساب خودتان ممکن نیست")
	}
	if a.Role == models.RoleAdmin || a.Role == models.RoleSuperAdmin {
		n, cerr := h.Repo.Agents.CountByRole(r.Context(), a.ProjectID, models.RoleAdmin)
		if cerr == nil && n <= 1 {
			if sa, err2 := h.Repo.Agents.CountByRole(r.Context(), a.ProjectID, models.RoleSuperAdmin); err2 == nil && sa <= 1 && a.Role == models.RoleSuperAdmin {
				return apierr.BadRequest("حداقل یک مدیر باید باقی بماند")
			}
			if a.Role == models.RoleAdmin {
				return apierr.BadRequest("حداقل یک مدیر باید باقی بماند")
			}
		}
	}
	if err := h.Repo.Agents.Delete(r.Context(), a.ID); err != nil {
		return apierr.Internal(err)
	}
	return okMsg(w)
}
