package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/models"
)

// WebhookRepo persists outbound webhooks and their delivery log.
type WebhookRepo struct{ db *db.DB }

const webhookCols = `id, project_id, url, events, secret, description, is_active, created_at, updated_at`

func scanWebhook(s scanner) (*models.Webhook, error) {
	var (
		w         models.Webhook
		desc      sql.NullString
		createdAt models.NullTime
		updatedAt models.NullTime
	)
	if err := s.Scan(&w.ID, &w.ProjectID, &w.URL, &w.Events, &w.Secret, &desc, &w.IsActive, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	w.Description = str(desc)
	w.CreatedAt = createdAt.Time
	w.UpdatedAt = updatedAt.Time
	return &w, nil
}

// Create inserts a webhook.
func (r *WebhookRepo) Create(ctx context.Context, w *models.Webhook) error {
	now := models.NowStr()
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO webhooks (id, project_id, url, events, secret, description, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, w.ID, w.ProjectID, w.URL, w.Events, w.Secret,
		toNullStr(w.Description), boolInt(w.IsActive), now, now)
	if err != nil {
		return fmt.Errorf("creating webhook: %w", err)
	}
	return nil
}

// GetByID loads one webhook.
func (r *WebhookRepo) GetByID(ctx context.Context, id string) (*models.Webhook, error) {
	w, err := scanWebhook(r.db.R.QueryRowContext(ctx, `SELECT `+webhookCols+` FROM webhooks WHERE id = ?`, id))
	if err != nil {
		return nil, wrapErr("getting webhook", err)
	}
	return w, nil
}

// List returns the webhooks of a project.
func (r *WebhookRepo) List(ctx context.Context, projectID string, activeOnly bool) ([]*models.Webhook, error) {
	q := `SELECT ` + webhookCols + ` FROM webhooks WHERE project_id = ?`
	if activeOnly {
		q += ` AND is_active = 1`
	}
	q += ` ORDER BY created_at ASC`
	rows, err := r.db.R.QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("listing webhooks: %w", err)
	}
	defer rows.Close()
	out := make([]*models.Webhook, 0, 8)
	for rows.Next() {
		w, err := scanWebhook(rows)
		if err != nil {
			return nil, wrapErr("scanning webhook", err)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// Update persists a webhook.
func (r *WebhookRepo) Update(ctx context.Context, w *models.Webhook) error {
	res, err := r.db.W.ExecContext(ctx, `UPDATE webhooks SET url = ?, events = ?, description = ?, is_active = ?, updated_at = ?
		WHERE id = ?`, w.URL, w.Events, toNullStr(w.Description), boolInt(w.IsActive), models.NowStr(), w.ID)
	if err != nil {
		return fmt.Errorf("updating webhook: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a webhook.
func (r *WebhookRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.W.ExecContext(ctx, `DELETE FROM webhooks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting webhook: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateDelivery records a delivery attempt.
func (r *WebhookRepo) CreateDelivery(ctx context.Context, d *models.WebhookDelivery) error {
	var next any
	if d.NextRetryAt != nil {
		next = models.FormatTime(*d.NextRetryAt)
	}
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO webhook_deliveries
		(id, webhook_id, event, payload, status_code, response_body, attempt, next_retry_at, error, delivered_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?)`,
		d.ID, d.WebhookID, d.Event, d.Payload, d.StatusCode, d.ResponseBody, d.Attempt, next,
		d.Error, models.NowStr())
	if err != nil {
		return fmt.Errorf("creating webhook delivery: %w", err)
	}
	return nil
}

// UpdateDelivery records the outcome of an attempt.
func (r *WebhookRepo) UpdateDelivery(ctx context.Context, id string, statusCode *int, respBody, errMsg *string, attempt int, nextRetryAt *time.Time, delivered bool) error {
	var next, deliveredAt any
	if nextRetryAt != nil {
		next = models.FormatTime(*nextRetryAt)
	}
	if delivered {
		deliveredAt = models.NowStr()
	}
	_, err := r.db.W.ExecContext(ctx, `UPDATE webhook_deliveries SET status_code = ?, response_body = ?,
		attempt = ?, next_retry_at = ?, error = ?, delivered_at = ? WHERE id = ?`,
		statusCode, respBody, attempt, next, errMsg, deliveredAt, id)
	if err != nil {
		return fmt.Errorf("updating webhook delivery: %w", err)
	}
	return nil
}

// ListDeliveries returns recent deliveries of a webhook.
func (r *WebhookRepo) ListDeliveries(ctx context.Context, webhookID string, limit int) ([]*models.WebhookDelivery, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.R.QueryContext(ctx, `SELECT id, webhook_id, event, payload, status_code, response_body,
		attempt, next_retry_at, error, delivered_at, created_at
		FROM webhook_deliveries WHERE webhook_id = ? ORDER BY created_at DESC LIMIT ?`, webhookID, limit)
	if err != nil {
		return nil, fmt.Errorf("listing webhook deliveries: %w", err)
	}
	defer rows.Close()
	out := make([]*models.WebhookDelivery, 0, limit)
	for rows.Next() {
		var (
			d          models.WebhookDelivery
			statusCode sql.NullInt64
			respBody   sql.NullString
			next       models.NullTime
			errMsg     sql.NullString
			delivered  models.NullTime
			createdAt  models.NullTime
		)
		if err := rows.Scan(&d.ID, &d.WebhookID, &d.Event, &d.Payload, &statusCode, &respBody,
			&d.Attempt, &next, &errMsg, &delivered, &createdAt); err != nil {
			return nil, wrapErr("scanning webhook delivery", err)
		}
		if statusCode.Valid {
			v := int(statusCode.Int64)
			d.StatusCode = &v
		}
		d.ResponseBody = str(respBody)
		d.NextRetryAt = next.Ptr()
		d.Error = str(errMsg)
		d.DeliveredAt = delivered.Ptr()
		d.CreatedAt = createdAt.Time
		out = append(out, &d)
	}
	return out, rows.Err()
}

// PendingDeliveries returns deliveries that are due for a retry.
func (r *WebhookRepo) PendingDeliveries(ctx context.Context, now time.Time, limit int) ([]*models.WebhookDelivery, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.R.QueryContext(ctx, `SELECT id, webhook_id, event, payload, status_code, response_body,
		attempt, next_retry_at, error, delivered_at, created_at
		FROM webhook_deliveries
		WHERE delivered_at IS NULL AND next_retry_at IS NOT NULL AND next_retry_at <= ?
		ORDER BY next_retry_at ASC LIMIT ?`, models.FormatTime(now), limit)
	if err != nil {
		return nil, fmt.Errorf("listing pending deliveries: %w", err)
	}
	defer rows.Close()
	out := make([]*models.WebhookDelivery, 0, limit)
	for rows.Next() {
		var (
			d          models.WebhookDelivery
			statusCode sql.NullInt64
			respBody   sql.NullString
			next       models.NullTime
			errMsg     sql.NullString
			delivered  models.NullTime
			createdAt  models.NullTime
		)
		if err := rows.Scan(&d.ID, &d.WebhookID, &d.Event, &d.Payload, &statusCode, &respBody,
			&d.Attempt, &next, &errMsg, &delivered, &createdAt); err != nil {
			return nil, wrapErr("scanning pending delivery", err)
		}
		if statusCode.Valid {
			v := int(statusCode.Int64)
			d.StatusCode = &v
		}
		d.ResponseBody = str(respBody)
		d.NextRetryAt = next.Ptr()
		d.Error = str(errMsg)
		d.DeliveredAt = delivered.Ptr()
		d.CreatedAt = createdAt.Time
		out = append(out, &d)
	}
	return out, rows.Err()
}
