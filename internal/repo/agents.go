package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/models"
)

// AgentRepo persists dashboard users.
type AgentRepo struct{ db *db.DB }

const agentCols = `id, project_id, email, password_hash, name, avatar_url, role, telegram_id,
	max_conversations, is_online, is_active, last_seen_at, created_at, updated_at`

func scanAgent(s scanner) (*models.Agent, error) {
	var (
		a         models.Agent
		avatar    sql.NullString
		telegram  sql.NullString
		lastSeen  models.NullTime
		createdAt models.NullTime
		updatedAt models.NullTime
	)
	if err := s.Scan(&a.ID, &a.ProjectID, &a.Email, &a.PasswordHash, &a.Name, &avatar, &a.Role, &telegram,
		&a.MaxConversations, &a.IsOnline, &a.IsActive, &lastSeen, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	a.AvatarURL = str(avatar)
	a.TelegramID = str(telegram)
	a.LastSeenAt = lastSeen.Ptr()
	a.CreatedAt = createdAt.Time
	a.UpdatedAt = updatedAt.Time
	return &a, nil
}

// Create inserts an agent.
func (r *AgentRepo) Create(ctx context.Context, a *models.Agent) error {
	now := models.NowStr()
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO agents
		(id, project_id, email, password_hash, name, avatar_url, role, telegram_id, max_conversations,
		 is_online, is_active, last_seen_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.ProjectID, a.Email, a.PasswordHash, a.Name, toNullStr(a.AvatarURL), string(a.Role),
		toNullStr(a.TelegramID), a.MaxConversations, boolInt(a.IsOnline), boolInt(a.IsActive), nil, now, now)
	if err != nil {
		if isUnique(err) {
			return ErrDuplicate
		}
		return fmt.Errorf("creating agent: %w", err)
	}
	return nil
}

// GetByID loads one agent.
func (r *AgentRepo) GetByID(ctx context.Context, id string) (*models.Agent, error) {
	a, err := scanAgent(r.db.R.QueryRowContext(ctx, `SELECT `+agentCols+` FROM agents WHERE id = ?`, id))
	if err != nil {
		return nil, wrapErr("getting agent", err)
	}
	return a, nil
}

// GetByEmail loads an agent scoped to a project.
func (r *AgentRepo) GetByEmail(ctx context.Context, projectID, email string) (*models.Agent, error) {
	a, err := scanAgent(r.db.R.QueryRowContext(ctx, `SELECT `+agentCols+` FROM agents WHERE project_id = ? AND email = ?`,
		projectID, email))
	if err != nil {
		return nil, wrapErr("getting agent by email", err)
	}
	return a, nil
}

// GetByEmailAny loads agents by email across projects (single-project login).
func (r *AgentRepo) GetByEmailAny(ctx context.Context, email string) ([]*models.Agent, error) {
	rows, err := r.db.R.QueryContext(ctx, `SELECT `+agentCols+` FROM agents WHERE lower(email) = lower(?) ORDER BY created_at ASC`, email)
	if err != nil {
		return nil, fmt.Errorf("getting agents by email: %w", err)
	}
	defer rows.Close()
	out := make([]*models.Agent, 0, 2)
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, wrapErr("scanning agent", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// CountSuperadmins reports how many active superadmins exist instance-wide.
func (r *AgentRepo) CountSuperadmins(ctx context.Context) (int, error) {
	var n int
	err := r.db.R.QueryRowContext(ctx, `SELECT COUNT(*) FROM agents WHERE role = 'superadmin' AND is_active = 1`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("counting superadmins: %w", err)
	}
	return n, nil
}

// List returns all agents of a project.
func (r *AgentRepo) List(ctx context.Context, projectID string) ([]*models.Agent, error) {
	rows, err := r.db.R.QueryContext(ctx, `SELECT `+agentCols+` FROM agents WHERE project_id = ? ORDER BY created_at ASC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("listing agents: %w", err)
	}
	defer rows.Close()
	out := make([]*models.Agent, 0, 8)
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, wrapErr("scanning agent", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// Update persists the mutable agent fields.
func (r *AgentRepo) Update(ctx context.Context, a *models.Agent) error {
	res, err := r.db.W.ExecContext(ctx, `UPDATE agents SET
		name = ?, email = ?, avatar_url = ?, role = ?, telegram_id = ?, max_conversations = ?, is_active = ?, updated_at = ?
		WHERE id = ?`,
		a.Name, a.Email, toNullStr(a.AvatarURL), string(a.Role), toNullStr(a.TelegramID),
		a.MaxConversations, boolInt(a.IsActive), models.NowStr(), a.ID)
	if err != nil {
		if isUnique(err) {
			return ErrDuplicate
		}
		return fmt.Errorf("updating agent: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdatePassword replaces the bcrypt hash.
func (r *AgentRepo) UpdatePassword(ctx context.Context, id, hash string) error {
	res, err := r.db.W.ExecContext(ctx, `UPDATE agents SET password_hash = ?, updated_at = ? WHERE id = ?`,
		hash, models.NowStr(), id)
	if err != nil {
		return fmt.Errorf("updating password: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetOnline flips the presence flag and last_seen_at.
func (r *AgentRepo) SetOnline(ctx context.Context, id string, online bool) error {
	var lastSeen any
	if !online {
		lastSeen = models.NowStr()
	} else {
		lastSeen = models.NowStr()
	}
	_, err := r.db.W.ExecContext(ctx, `UPDATE agents SET is_online = ?, last_seen_at = ?, updated_at = ? WHERE id = ?`,
		boolInt(online), lastSeen, models.NowStr(), id)
	if err != nil {
		return fmt.Errorf("setting agent online state: %w", err)
	}
	return nil
}

// ActiveLoad maps agentID -> number of open conversations in the project.
func (r *AgentRepo) ActiveLoad(ctx context.Context, projectID string) (map[string]int, error) {
	rows, err := r.db.R.QueryContext(ctx, `SELECT assigned_agent_id, COUNT(*)
		FROM conversations
		WHERE project_id = ? AND status IN ('pending','active') AND assigned_agent_id IS NOT NULL
		GROUP BY assigned_agent_id`, projectID)
	if err != nil {
		return nil, fmt.Errorf("reading agent load: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, wrapErr("scanning agent load", err)
		}
		out[id] = n
	}
	return out, rows.Err()
}

// Delete removes an agent.
func (r *AgentRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.W.ExecContext(ctx, `DELETE FROM agents WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting agent: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// CountByRole counts active agents with the given role in a project.
func (r *AgentRepo) CountByRole(ctx context.Context, projectID string, role models.Role) (int, error) {
	var n int
	err := r.db.R.QueryRowContext(ctx, `SELECT COUNT(*) FROM agents WHERE project_id = ? AND role = ? AND is_active = 1`,
		projectID, string(role)).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("counting agents: %w", err)
	}
	return n, nil
}

// TouchSeen refreshes last_seen_at without changing presence.
func (r *AgentRepo) TouchSeen(ctx context.Context, id string) error {
	_, err := r.db.W.ExecContext(ctx, `UPDATE agents SET last_seen_at = ? WHERE id = ?`, models.NowStr(), id)
	if err != nil {
		return fmt.Errorf("touching agent: %w", err)
	}
	return nil
}

// GetByIDs batch-loads agents for list enrichment.
func (r *AgentRepo) GetByIDs(ctx context.Context, ids []string) (map[string]*models.Agent, error) {
	out := make(map[string]*models.Agent, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	ph := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		ph[i] = "?"
		args[i] = id
	}
	rows, err := r.db.R.QueryContext(ctx, `SELECT `+agentCols+` FROM agents WHERE id IN (`+strings.Join(ph, ",")+`)`, args...)
	if err != nil {
		return nil, fmt.Errorf("batch loading agents: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, wrapErr("scanning agent", err)
		}
		out[a.ID] = a
	}
	return out, rows.Err()
}

// ListWithTelegram returns agents that have a telegram chat id.
func (r *AgentRepo) ListWithTelegram(ctx context.Context, projectID string) ([]*models.Agent, error) {
	rows, err := r.db.R.QueryContext(ctx, `SELECT `+agentCols+` FROM agents
		WHERE project_id = ? AND telegram_id IS NOT NULL AND is_active = 1`, projectID)
	if err != nil {
		return nil, fmt.Errorf("listing agents with telegram: %w", err)
	}
	defer rows.Close()
	out := make([]*models.Agent, 0, 8)
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, wrapErr("scanning agent", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// CreateResetToken stores a hashed single-use password reset token.
func (r *AgentRepo) CreateResetToken(ctx context.Context, agentID, tokenHash string, expires time.Time) error {
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO password_reset_tokens (token_hash, agent_id, expires_at, created_at)
		VALUES (?, ?, ?, ?)`, tokenHash, agentID, models.FormatTime(expires), models.NowStr())
	if err != nil {
		return fmt.Errorf("creating reset token: %w", err)
	}
	return nil
}

// ConsumeResetToken validates and burns a token, returning the agent id.
func (r *AgentRepo) ConsumeResetToken(ctx context.Context, tokenHash string) (string, error) {
	tx, err := r.db.W.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin reset transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		agentID string
		expires models.NullTime
		used    models.NullTime
	)
	err = tx.QueryRowContext(ctx, `SELECT agent_id, expires_at, used_at FROM password_reset_tokens WHERE token_hash = ?`, tokenHash).
		Scan(&agentID, &expires, &used)
	if err != nil {
		return "", repoErrNotFound(err)
	}
	if used.Valid || expires.Time.Before(time.Now().UTC()) {
		return "", repoErrNotFound(nil)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE password_reset_tokens SET used_at = ? WHERE token_hash = ?`, models.NowStr(), tokenHash); err != nil {
		return "", fmt.Errorf("burning reset token: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("committing reset token: %w", err)
	}
	return agentID, nil
}

// PurgeResetTokens removes expired tokens (hourly janitor).
func (r *AgentRepo) PurgeResetTokens(ctx context.Context) (int64, error) {
	res, err := r.db.W.ExecContext(ctx, `DELETE FROM password_reset_tokens WHERE expires_at < ? OR used_at IS NOT NULL`, models.NowStr())
	if err != nil {
		return 0, fmt.Errorf("purging reset tokens: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

func repoErrNotFound(err error) error {
	if err == nil || err == sql.ErrNoRows {
		return ErrNotFound
	}
	return err
}
