package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/evechat/evechat/internal/db"
)

// Sentinel errors returned by every repository.
var (
	ErrNotFound  = errors.New("not found")
	ErrDuplicate = errors.New("duplicate")
	ErrConflict  = errors.New("conflict")
)

// Repo aggregates all table repositories.
type Repo struct {
	db *db.DB

	Projects      *ProjectRepo
	Agents        *AgentRepo
	Visitors      *VisitorRepo
	Sessions      *SessionRepo
	Conversations *ConversationRepo
	Messages      *MessageRepo
	Rules         *RuleRepo
	Tickets       *TicketRepo
	KB            *KBRepo
	Webhooks      *WebhookRepo
	Canned        *CannedRepo
	Stats         *StatsRepo
	SaaS          *SaaSRepo
}

// New wires every sub-repository against one database handle.
func New(d *db.DB) *Repo {
	return &Repo{
		db:            d,
		Projects:      &ProjectRepo{db: d},
		Agents:        &AgentRepo{db: d},
		Visitors:      &VisitorRepo{db: d},
		Sessions:      &SessionRepo{db: d},
		Conversations: &ConversationRepo{db: d},
		Messages:      &MessageRepo{db: d},
		Rules:         &RuleRepo{db: d},
		Tickets:       &TicketRepo{db: d},
		KB:            &KBRepo{db: d},
		Webhooks:      &WebhookRepo{db: d},
		Canned:        &CannedRepo{db: d},
		Stats:         &StatsRepo{db: d},
		SaaS:          &SaaSRepo{db: d},
	}
}

// DB exposes the underlying handle (used by services that need a transaction).
func (r *Repo) DB() *db.DB { return r.db }

// Tx runs fn in a write transaction.
func (r *Repo) Tx(ctx context.Context, fn func(*sql.Tx) error) error {
	return r.db.Tx(ctx, fn)
}

// ---- scan helpers ----

type scanner interface {
	Scan(dest ...any) error
}

func str(ns sql.NullString) *string {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	s := ns.String
	return &s
}

func strVal(ns sql.NullString) string {
	if !ns.Valid {
		return ""
	}
	return ns.String
}

func toNullStr(p *string) sql.NullString {
	if p == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *p, Valid: true}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func pageClamp(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// likeTerm builds a LIKE pattern, escaping % and _ so user input stays literal.
func likeTerm(q string) string {
	q = strings.ReplaceAll(q, `\`, `\\`)
	q = strings.ReplaceAll(q, "%", `\%`)
	q = strings.ReplaceAll(q, "_", `\_`)
	return "%" + q + "%"
}

func wrapErr(op string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return fmt.Errorf("%s: %w", op, err)
}

func isUnique(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "UNIQUE constraint failed") || strings.Contains(s, "constraint failed: UNIQUE")
}
