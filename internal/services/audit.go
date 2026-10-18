package services

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/repo"
)

// AuditService records administrative actions with secrets redacted.
type AuditService struct {
	repo *repo.Repo
	log  zerolog.Logger
}

// NewAuditService builds the audit service.
func NewAuditService(r *repo.Repo, log zerolog.Logger) *AuditService {
	return &AuditService{repo: r, log: log}
}

// AuditInput is one audit entry to record.
type AuditInput struct {
	ProjectID  string
	ActorType  string // agent | system | api
	ActorID    string
	ActorName  string
	Action     string
	TargetType string
	TargetID   string
	IP         string
	UserAgent  string
	Detail     map[string]any
}

// Record writes an audit entry. Errors are logged, never returned: auditing
// must not break the operation it describes.
func (s *AuditService) Record(ctx context.Context, in AuditInput) {
	if s == nil || s.repo == nil || s.repo.Audit == nil {
		return
	}
	detail := "{}"
	if in.Detail != nil {
		if b, err := json.Marshal(redact(in.Detail)); err == nil {
			detail = string(b)
		}
	}
	entry := &repo.AuditEntry{
		ID:         id.NewPrefixed("aud"),
		ProjectID:  in.ProjectID,
		ActorType:  firstNonEmptyStr(in.ActorType, "system"),
		ActorID:    in.ActorID,
		ActorName:  in.ActorName,
		Action:     in.Action,
		TargetType: in.TargetType,
		TargetID:   in.TargetID,
		IP:         in.IP,
		UserAgent:  in.UserAgent,
		Detail:     detail,
	}
	if err := s.repo.Audit.Create(ctx, entry); err != nil {
		s.log.Warn().Err(err).Str("action", in.Action).Msg("audit write failed")
	}
}

// Audit records via the package-level accessor used by handlers.
func (s *AuditService) Audit(ctx context.Context, in AuditInput) { s.Record(ctx, in) }

func redact(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		lk := strings.ToLower(k)
		if strings.Contains(lk, "password") || strings.Contains(lk, "secret") ||
			strings.Contains(lk, "token") || strings.Contains(lk, "key") ||
			lk == "authorization" || lk == "api_key" {
			out[k] = "***"
			continue
		}
		if nested, ok := v.(map[string]any); ok {
			out[k] = redact(nested)
			continue
		}
		out[k] = v
	}
	return out
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
