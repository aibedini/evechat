package repo

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/models"
)

// RuleRepo persists routing rules.
type RuleRepo struct{ db *db.DB }

const ruleCols = `id, project_id, name, priority, conditions, route_to, auto_reply, is_active, created_at`

func scanRule(s scanner) (*models.RoutingRule, error) {
	var (
		r         models.RoutingRule
		autoReply sql.NullString
		createdAt models.NullTime
	)
	if err := s.Scan(&r.ID, &r.ProjectID, &r.Name, &r.Priority, &r.Conditions, &r.RouteTo, &autoReply, &r.IsActive, &createdAt); err != nil {
		return nil, err
	}
	r.AutoReply = str(autoReply)
	r.CreatedAt = createdAt.Time
	return &r, nil
}

// Create inserts a rule.
func (r *RuleRepo) Create(ctx context.Context, rule *models.RoutingRule) error {
	now := models.NowStr()
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO routing_rules
		(id, project_id, name, priority, conditions, route_to, auto_reply, is_active, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rule.ID, rule.ProjectID, rule.Name, rule.Priority, rule.Conditions, rule.RouteTo,
		toNullStr(rule.AutoReply), boolInt(rule.IsActive), now)
	if err != nil {
		return fmt.Errorf("creating routing rule: %w", err)
	}
	return nil
}

// GetByID loads one rule.
func (r *RuleRepo) GetByID(ctx context.Context, id string) (*models.RoutingRule, error) {
	rule, err := scanRule(r.db.R.QueryRowContext(ctx, `SELECT `+ruleCols+` FROM routing_rules WHERE id = ?`, id))
	if err != nil {
		return nil, wrapErr("getting routing rule", err)
	}
	return rule, nil
}

// List returns rules ordered by priority (highest first) then creation time.
func (r *RuleRepo) List(ctx context.Context, projectID string, activeOnly bool) ([]*models.RoutingRule, error) {
	q := `SELECT ` + ruleCols + ` FROM routing_rules WHERE project_id = ?`
	if activeOnly {
		q += ` AND is_active = 1`
	}
	q += ` ORDER BY priority DESC, created_at ASC`
	rows, err := r.db.R.QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("listing routing rules: %w", err)
	}
	defer rows.Close()
	out := make([]*models.RoutingRule, 0, 8)
	for rows.Next() {
		rule, err := scanRule(rows)
		if err != nil {
			return nil, wrapErr("scanning routing rule", err)
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

// Update persists a rule.
func (r *RuleRepo) Update(ctx context.Context, rule *models.RoutingRule) error {
	res, err := r.db.W.ExecContext(ctx, `UPDATE routing_rules SET name = ?, priority = ?, conditions = ?,
		route_to = ?, auto_reply = ?, is_active = ? WHERE id = ?`,
		rule.Name, rule.Priority, rule.Conditions, rule.RouteTo, toNullStr(rule.AutoReply), boolInt(rule.IsActive), rule.ID)
	if err != nil {
		return fmt.Errorf("updating routing rule: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// IDPriority is one entry of a reorder request.
type IDPriority struct {
	ID       string `json:"id"`
	Priority int    `json:"priority"`
}

// Reorder applies a batch of priority changes atomically.
func (r *RuleRepo) Reorder(ctx context.Context, order []IDPriority) error {
	return r.db.Tx(ctx, func(tx *sql.Tx) error {
		for _, o := range order {
			if _, err := tx.ExecContext(ctx, `UPDATE routing_rules SET priority = ? WHERE id = ?`, o.Priority, o.ID); err != nil {
				return fmt.Errorf("reordering rule %s: %w", o.ID, err)
			}
		}
		return nil
	})
}

// Delete removes a rule.
func (r *RuleRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.W.ExecContext(ctx, `DELETE FROM routing_rules WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting routing rule: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
