package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/repo"
)

// PasswordResetService issues and consumes single-use reset tokens.
type PasswordResetService struct {
	repo  *repo.Repo
	email *EmailChannel
	base  string
	ttl   time.Duration
	cost  int
	log   zerolog.Logger
}

// NewPasswordResetService builds the service.
func NewPasswordResetService(r *repo.Repo, emailCh *EmailChannel, baseURL string, bcryptCost int, log zerolog.Logger) *PasswordResetService {
	if bcryptCost <= 0 {
		bcryptCost = 12
	}
	return &PasswordResetService{
		repo: r, email: emailCh, base: strings.TrimRight(baseURL, "/"),
		ttl: 30 * time.Minute, cost: bcryptCost, log: log,
	}
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Request always succeeds from the caller's point of view (no account
// enumeration). When the email belongs to an agent, a reset link is sent.
func (s *PasswordResetService) Request(ctx context.Context, emailAddr string) error {
	emailAddr = strings.ToLower(strings.TrimSpace(emailAddr))
	if emailAddr == "" {
		return nil
	}
	agents, err := s.repo.Agents.GetByEmailAny(ctx, emailAddr)
	if err != nil || len(agents) == 0 {
		return nil
	}
	agent := agents[0]
	token := id.Token(32)
	if err := s.repo.Agents.CreateResetToken(ctx, agent.ID, hashToken(token), time.Now().UTC().Add(s.ttl)); err != nil {
		s.log.Warn().Err(err).Msg("storing reset token failed")
		return nil
	}
	link := fmt.Sprintf("%s/reset?token=%s", s.base, token)
	if s.email == nil {
		s.log.Warn().Str("agent_id", agent.ID).Msg("reset requested but email channel is disabled")
		return nil
	}
	if err := s.email.SendPasswordReset(ctx, agent.Email, agent.Name, link, int(s.ttl.Minutes())); err != nil {
		s.log.Warn().Err(err).Msg("password reset email failed")
	}
	return nil
}

// Reset consumes a token and sets the new password. It returns the id of the
// agent whose password changed so the caller can audit the action.
func (s *PasswordResetService) Reset(ctx context.Context, token, newPassword string) (string, error) {
	if len(newPassword) < 8 {
		return "", apierr.BadRequest("رمز عبور جدید باید حداقل ۸ کاراکتر باشد")
	}
	agentID, err := s.repo.Agents.ConsumeResetToken(ctx, hashToken(token))
	if err != nil {
		return "", apierr.BadRequest("لینک بازیابی نامعتبر یا منقضی شده است")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.cost)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}
	if err := s.repo.Agents.UpdatePassword(ctx, agentID, string(hash)); err != nil {
		return "", fmt.Errorf("updating password: %w", err)
	}
	return agentID, nil
}

var errResetExpired = errors.New("reset token expired")
