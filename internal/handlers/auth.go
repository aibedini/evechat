package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/logx"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

type loginRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	ProjectKey string `json:"project_key"`
}

// dummyHash keeps failed logins as slow as successful ones (no user probing).
var dummyHash = []byte("$2a$12$C6UzMDM.H6dfI/f/IKcEe.6YQ0PYYf1o7QZgY8H1cQ7o5oUoYb1nq")

// Login authenticates an agent and returns a token pair.
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) error {
	var in loginRequest
	if err := bind(r, &in); err != nil {
		return err
	}
	email := strings.TrimSpace(strings.ToLower(in.Email))
	if email == "" || in.Password == "" {
		return apierr.BadRequest("ایمیل و رمز عبور لازم است")
	}

	var (
		agent   *models.Agent
		project *models.Project
	)
	ctx := r.Context()
	if key := strings.TrimSpace(in.ProjectKey); key != "" {
		p, err := h.Repo.Projects.GetByAPIKey(ctx, key)
		if err != nil {
			return apierr.Unauthorized("ایمیل یا رمز عبور نادرست است")
		}
		project = p
		a, err := h.Repo.Agents.GetByEmail(ctx, p.ID, email)
		if err == nil {
			agent = a
		}
	} else {
		candidates, err := h.Repo.Agents.GetByEmailAny(ctx, email)
		if err == nil && len(candidates) > 0 {
			for _, a := range candidates {
				if a.IsActive {
					agent = a
					break
				}
			}
			if agent == nil {
				agent = candidates[0]
			}
			if p, perr := h.Repo.Projects.GetByID(ctx, agent.ProjectID); perr == nil {
				project = p
			}
		}
	}

	if agent == nil {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(in.Password))
		return apierr.Unauthorized("ایمیل یا رمز عبور نادرست است")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(agent.PasswordHash), []byte(in.Password)); err != nil {
		return apierr.Unauthorized("ایمیل یا رمز عبور نادرست است")
	}
	if !agent.IsActive {
		return apierr.Forbidden("حساب شما غیرفعال است")
	}

	pair, err := h.Tokens.Issue(agent)
	if err != nil {
		return apierr.Internal(err)
	}
	_ = h.Repo.Agents.TouchSeen(ctx, agent.ID)
	logx.WithReq(ctx).Info().Str("agent_id", agent.ID).Msg("login succeeded")
	h.recordAudit(r, agent.ProjectID, "auth.login", "agent", agent.ID, map[string]any{"email": agent.Email})

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"token_type":    pair.TokenType,
		"expires_in":    pair.ExpiresIn,
		"agent":         agent,
		"project":       project,
	})
	return nil
}

// Refresh rotates a refresh token into a new pair.
func (h *Handlers) Refresh(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	if strings.TrimSpace(in.RefreshToken) == "" {
		return apierr.BadRequest("توکن تازه‌سازی لازم است")
	}
	claims, err := h.Tokens.ParseTyp(in.RefreshToken, "refresh")
	if err != nil {
		return apierr.Unauthorized("توکن تازه‌سازی معتبر نیست")
	}
	agent, err := h.Repo.Agents.GetByID(r.Context(), claims.Subject)
	if err != nil {
		return apierr.Unauthorized("حساب کاربری یافت نشد")
	}
	if !agent.IsActive {
		return apierr.Forbidden("حساب شما غیرفعال است")
	}
	pair, err := h.Tokens.Issue(agent)
	if err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"token_type":    pair.TokenType,
		"expires_in":    pair.ExpiresIn,
	})
	return nil
}

// Logout marks the agent offline.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) error {
	agent, err := h.callerAgent(r)
	if err != nil {
		return err
	}
	_ = h.Repo.Agents.SetOnline(r.Context(), agent.ID, false)
	h.Hub.ToProjectAgents(agent.ProjectID, envelopePresence(agent.ID, agent.Name, false))
	return okMsg(w)
}

// Me returns the current agent and project.
func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) error {
	agent, err := h.callerAgent(r)
	if err != nil {
		return err
	}
	var project *models.Project
	if p, perr := h.Repo.Projects.GetByID(r.Context(), agent.ProjectID); perr == nil {
		project = p
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"agent":   agent,
		"project": project,
		"presence": map[string]any{
			"online_agents": len(h.Hub.OnlineAgents(agent.ProjectID)),
		},
	})
	return nil
}

// ChangePassword updates the caller's password.
func (h *Handlers) ChangePassword(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	if len(in.NewPassword) < 8 {
		return apierr.BadRequest("رمز عبور جدید باید حداقل ۸ کاراکتر باشد")
	}
	agent, err := h.callerAgent(r)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(agent.PasswordHash), []byte(in.CurrentPassword)); err != nil {
		return apierr.Unauthorized("رمز عبور فعلی نادرست است")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), h.Cfg.Auth.BcryptCost)
	if err != nil {
		return apierr.Internal(err)
	}
	if err := h.Repo.Agents.UpdatePassword(r.Context(), agent.ID, string(hash)); err != nil {
		return apierr.Internal(err)
	}
	return okMsg(w)
}

// HashPassword is shared by the seed command and agent creation.
func HashPassword(pw string, cost int) (string, error) {
	if cost <= 0 || cost > 14 {
		cost = 12
	}
	b, err := bcrypt.GenerateFromPassword([]byte(pw), cost)
	return string(b), err
}

// EnsureProjectExists returns the project or a 404.
func (h *Handlers) projectOf(ctx context.Context, id string) (*models.Project, error) {
	p, err := h.Repo.Projects.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, apierr.NotFound("پروژه یافت نشد")
		}
		return nil, apierr.Internal(err)
	}
	return p, nil
}

// validateDay returns a UTC time or a 400.
func validateDay(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, apierr.BadRequest("تاریخ باید به شکل YYYY-MM-DD باشد")
	}
	return t.UTC(), nil
}

// ForgotPassword starts the reset flow. It always returns 200 so the endpoint
// cannot be used to enumerate accounts.
func (h *Handlers) ForgotPassword(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Email string `json:"email"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	if h.Reset != nil {
		_ = h.Reset.Request(r.Context(), in.Email)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "اگر این ایمیل ثبت شده باشد، لینک بازیابی ارسال می‌شود."})
	return nil
}

// ResetPassword consumes a reset token and sets a new password.
func (h *Handlers) ResetPassword(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	if h.Reset == nil {
		return apierr.BadRequest("بازیابی رمز فعال نیست")
	}
	agentID, err := h.Reset.Reset(r.Context(), in.Token, in.Password)
	if err != nil {
		return err
	}
	if agent, aerr := h.Repo.Agents.GetByID(r.Context(), agentID); aerr == nil {
		h.recordAudit(r, agent.ProjectID, "auth.password_reset", "agent", agent.ID, nil)
	}
	return okMsg(w)
}
