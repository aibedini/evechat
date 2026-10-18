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

// Redacted is what replaces any secret value in an audit entry.
const Redacted = "***"

// secretKeyParts are matched case-insensitively anywhere in a key. Over-matching
// is deliberate: losing a detail from the audit log is cheaper than storing a
// credential in it.
var secretKeyParts = []string{
	"password", "passwd", "pwd", "secret", "token", "key",
	"authorization", "cookie", "credential", "signature", "session",
}

func isSecretKey(k string) bool {
	lk := strings.ToLower(k)
	for _, part := range secretKeyParts {
		if strings.Contains(lk, part) {
			return true
		}
	}
	return false
}

// redact returns a copy of in with every secret-looking key replaced, recursing
// into nested maps and slices so a secret cannot slip through inside a list.
func redact(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		if isSecretKey(k) {
			out[k] = Redacted
			continue
		}
		out[k] = redactValue(v)
	}
	return out
}

func redactValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return redact(val)
	case map[string]string:
		out := make(map[string]string, len(val))
		for k, s := range val {
			if isSecretKey(k) {
				out[k] = Redacted
				continue
			}
			out[k] = s
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = redactValue(item)
		}
		return out
	case []map[string]any:
		out := make([]map[string]any, len(val))
		for i, item := range val {
			out[i] = redact(item)
		}
		return out
	default:
		return v
	}
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
