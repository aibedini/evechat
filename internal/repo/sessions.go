package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/models"
)

// SessionRepo persists widget session tokens.
type SessionRepo struct{ db *db.DB }

const sessionCols = `token, visitor_id, project_id, active_conv_id, created_at, expires_at`

func scanSession(s scanner) (*models.VisitorSession, error) {
	var (
		sess      models.VisitorSession
		active    sql.NullString
		createdAt models.NullTime
		expiresAt models.NullTime
	)
	if err := s.Scan(&sess.Token, &sess.VisitorID, &sess.ProjectID, &active, &createdAt, &expiresAt); err != nil {
		return nil, err
	}
	sess.ActiveConvID = str(active)
	sess.CreatedAt = createdAt.Time
	sess.ExpiresAt = expiresAt.Time
	return &sess, nil
}

// Create issues a new session token.
func (r *SessionRepo) Create(ctx context.Context, visitorID, projectID, token string, ttl time.Duration) (*models.VisitorSession, error) {
	now := time.Now().UTC()
	exp := now.Add(ttl)
	sess := &models.VisitorSession{
		Token:     token,
		VisitorID: visitorID,
		ProjectID: projectID,
		CreatedAt: now,
		ExpiresAt: exp,
	}
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO visitor_sessions (token, visitor_id, project_id, active_conv_id, created_at, expires_at)
		VALUES (?, ?, ?, NULL, ?, ?)`, token, visitorID, projectID, models.FormatTime(now), models.FormatTime(exp))
	if err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}
	return sess, nil
}

// Get loads an unexpired session.
func (r *SessionRepo) Get(ctx context.Context, token string) (*models.VisitorSession, error) {
	s, err := scanSession(r.db.R.QueryRowContext(ctx, `SELECT `+sessionCols+` FROM visitor_sessions WHERE token = ?`, token))
	if err != nil {
		return nil, wrapErr("getting session", err)
	}
	if s.ExpiresAt.Before(time.Now().UTC()) {
		return nil, ErrNotFound
	}
	return s, nil
}

// Touch extends the session and refreshes activity.
func (r *SessionRepo) Touch(ctx context.Context, token string, ttl time.Duration) error {
	exp := models.FormatTime(time.Now().UTC().Add(ttl))
	_, err := r.db.W.ExecContext(ctx, `UPDATE visitor_sessions SET expires_at = ? WHERE token = ?`, exp, token)
	if err != nil {
		return fmt.Errorf("touching session: %w", err)
	}
	return nil
}

// SetActiveConv points the session at its open conversation.
func (r *SessionRepo) SetActiveConv(ctx context.Context, token, convID string) error {
	var v any
	if convID != "" {
		v = convID
	}
	_, err := r.db.W.ExecContext(ctx, `UPDATE visitor_sessions SET active_conv_id = ? WHERE token = ?`, v, token)
	if err != nil {
		return fmt.Errorf("setting active conversation: %w", err)
	}
	return nil
}

// SetActiveConvForVisitor points every live session of a visitor at convID.
// Conversations created over the WebSocket have no session token at hand, so
// without this the session row keeps a NULL (or stale) active conversation.
func (r *SessionRepo) SetActiveConvForVisitor(ctx context.Context, visitorID, convID string) error {
	var v any
	if convID != "" {
		v = convID
	}
	_, err := r.db.W.ExecContext(ctx, `UPDATE visitor_sessions SET active_conv_id = ? WHERE visitor_id = ?`, v, visitorID)
	if err != nil {
		return fmt.Errorf("setting active conversation for visitor: %w", err)
	}
	return nil
}

// DeleteExpired removes stale sessions.
func (r *SessionRepo) DeleteExpired(ctx context.Context) (int64, error) {
	res, err := r.db.W.ExecContext(ctx, `DELETE FROM visitor_sessions WHERE expires_at < ?`, models.NowStr())
	if err != nil {
		return 0, fmt.Errorf("deleting expired sessions: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// Delete removes a single session (logout / token revocation).
func (r *SessionRepo) Delete(ctx context.Context, token string) error {
	_, err := r.db.W.ExecContext(ctx, `DELETE FROM visitor_sessions WHERE token = ?`, token)
	if err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}

// DeleteByVisitor removes every session of a visitor.
func (r *SessionRepo) DeleteByVisitor(ctx context.Context, visitorID string) error {
	_, err := r.db.W.ExecContext(ctx, `DELETE FROM visitor_sessions WHERE visitor_id = ?`, visitorID)
	if err != nil {
		return fmt.Errorf("deleting visitor sessions: %w", err)
	}
	return nil
}
