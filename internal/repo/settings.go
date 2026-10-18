package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/models"
)

// SettingsRepo persists instance-wide key/value settings (app_settings).
type SettingsRepo struct{ db *db.DB }

// Get reads a setting. The boolean is false when the key does not exist.
func (r *SettingsRepo) Get(ctx context.Context, key string) (string, bool, error) {
	var v string
	err := r.db.R.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, key).Scan(&v)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, fmt.Errorf("reading setting %s: %w", key, err)
	}
	return v, true, nil
}

// Set writes a setting (insert or update).
func (r *SettingsRepo) Set(ctx context.Context, key, value string) error {
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO app_settings (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`, key, value)
	if err != nil {
		return fmt.Errorf("writing setting %s: %w", key, err)
	}
	return nil
}

// Delete removes a setting.
func (r *SettingsRepo) Delete(ctx context.Context, key string) error {
	_, err := r.db.W.ExecContext(ctx, `DELETE FROM app_settings WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("deleting setting %s: %w", key, err)
	}
	return nil
}

// All returns every setting.
func (r *SettingsRepo) All(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.R.QueryContext(ctx, `SELECT key, value FROM app_settings`)
	if err != nil {
		return nil, fmt.Errorf("listing settings: %w", err)
	}
	defer rows.Close()
	out := make(map[string]string, 16)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("scanning setting: %w", err)
		}
		out[k] = v
	}
	return out, rows.Err()
}

// ---- audit log ----

// AuditEntry is one recorded admin action.
type AuditEntry struct {
	ID         string `json:"id"`
	ProjectID  string `json:"project_id,omitempty"`
	ActorType  string `json:"actor_type"`
	ActorID    string `json:"actor_id,omitempty"`
	ActorName  string `json:"actor_name,omitempty"`
	Action     string `json:"action"`
	TargetType string `json:"target_type,omitempty"`
	TargetID   string `json:"target_id,omitempty"`
	IP         string `json:"ip,omitempty"`
	UserAgent  string `json:"user_agent,omitempty"`
	Detail     string `json:"detail"`
	CreatedAt  string `json:"created_at"`
}

// AuditRepo persists the audit log.
type AuditRepo struct{ db *db.DB }

// Create appends an entry.
func (r *AuditRepo) Create(ctx context.Context, e *AuditEntry) error {
	if e.Detail == "" {
		e.Detail = "{}"
	}
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO audit_log
		(id, project_id, actor_type, actor_id, actor_name, action, target_type, target_id, ip, user_agent, detail, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		e.ID, nullStrValue(e.ProjectID), e.ActorType, nullStrValue(e.ActorID), nullStrValue(e.ActorName), e.Action,
		nullStrValue(e.TargetType), nullStrValue(e.TargetID), nullStrValue(e.IP), nullStrValue(e.UserAgent), e.Detail)
	if err != nil {
		return fmt.Errorf("writing audit entry: %w", err)
	}
	return nil
}

// AuditFilter narrows the listing.
type AuditFilter struct {
	ProjectID string
	Action    string
	ActorID   string
	From      string
	To        string
	Limit     int
	Offset    int
}

// List returns a page of audit entries plus the total count.
func (r *AuditRepo) List(ctx context.Context, f AuditFilter) ([]*AuditEntry, int, error) {
	limit, offset := pageClamp(f.Limit, f.Offset)
	where := []string{"1=1"}
	args := []any{}
	if f.ProjectID != "" {
		where = append(where, "project_id = ?")
		args = append(args, f.ProjectID)
	}
	if f.Action != "" {
		where = append(where, "action LIKE ?")
		args = append(args, f.Action+"%")
	}
	if f.ActorID != "" {
		where = append(where, "actor_id = ?")
		args = append(args, f.ActorID)
	}
	if f.From != "" {
		where = append(where, "created_at >= ?")
		args = append(args, f.From)
	}
	if f.To != "" {
		where = append(where, "created_at <= ?")
		args = append(args, f.To)
	}
	cond := ""
	for i, w := range where {
		if i > 0 {
			cond += " AND "
		}
		cond += w
	}
	var total int
	if err := r.db.R.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_log WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting audit entries: %w", err)
	}
	qargs := append(append([]any{}, args...), limit, offset)
	rows, err := r.db.R.QueryContext(ctx, `SELECT id, project_id, actor_type, actor_id, actor_name, action,
		target_type, target_id, ip, user_agent, detail, created_at
		FROM audit_log WHERE `+cond+` ORDER BY created_at DESC LIMIT ? OFFSET ?`, qargs...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing audit entries: %w", err)
	}
	defer rows.Close()
	out := make([]*AuditEntry, 0, limit)
	for rows.Next() {
		var (
			e          AuditEntry
			projectID  sql.NullString
			actorID    sql.NullString
			actorName  sql.NullString
			targetType sql.NullString
			targetID   sql.NullString
			ip         sql.NullString
			ua         sql.NullString
			createdAt  models.NullTime
		)
		if err := rows.Scan(&e.ID, &projectID, &e.ActorType, &actorID, &actorName, &e.Action,
			&targetType, &targetID, &ip, &ua, &e.Detail, &createdAt); err != nil {
			return nil, 0, wrapErr("scanning audit entry", err)
		}
		e.ProjectID = strVal(projectID)
		e.ActorID = strVal(actorID)
		e.ActorName = strVal(actorName)
		e.TargetType = strVal(targetType)
		e.TargetID = strVal(targetID)
		e.IP = strVal(ip)
		e.UserAgent = strVal(ua)
		e.CreatedAt = createdAt.Time.Format("2006-01-02T15:04:05Z07:00")
		out = append(out, &e)
	}
	return out, total, rows.Err()
}

// Prune deletes entries older than the cutoff.
func (r *AuditRepo) Prune(ctx context.Context, cutoff string) (int64, error) {
	res, err := r.db.W.ExecContext(ctx, `DELETE FROM audit_log WHERE created_at < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("pruning audit log: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
