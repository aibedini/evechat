package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/models"
)

// Channel is one inbound/outbound integration endpoint.
type Channel struct {
	ID              string            `json:"id"`
	ProjectID       string            `json:"project_id"`
	Name            string            `json:"name"`
	Kind            string            `json:"kind"`
	InboundSecret   string            `json:"-"`
	OutboundURL     *string           `json:"outbound_url,omitempty"`
	OutboundSecret  *string           `json:"-"`
	OutboundHeaders map[string]string `json:"-"`
	SessionTTL      int               `json:"session_ttl_seconds"`
	Mapping         json.RawMessage   `json:"mapping"`
	IsActive        bool              `json:"is_active"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// MaskedHeader replaces a sensitive header value in every API response.
const MaskedHeader = "••••••"

// IsSensitiveHeader reports whether a header value must never be returned or
// logged. X-API-Key, Authorization, cookies and signatures all match.
func IsSensitiveHeader(name string) bool {
	l := strings.ToLower(name)
	for _, part := range []string{"key", "secret", "token", "auth", "password", "cookie", "signature"} {
		if strings.Contains(l, part) {
			return true
		}
	}
	return false
}

// MaskHeaders returns a copy safe for API responses; sensitive values become
// MaskedHeader. The UI sends the mask back on update, and the handler merges
// it with the stored value.
func MaskHeaders(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		if IsSensitiveHeader(k) {
			out[k] = MaskedHeader
		} else {
			out[k] = v
		}
	}
	return out
}

// Public renders the channel without its secrets for API responses.
func (c *Channel) Public() map[string]any {
	return map[string]any{
		"id": c.ID, "project_id": c.ProjectID, "name": c.Name, "kind": c.Kind,
		"outbound_url": c.OutboundURL, "mapping": c.Mapping,
		"outbound_headers":    MaskHeaders(c.OutboundHeaders),
		"session_ttl_seconds": c.SessionTTL,
		"is_active":           c.IsActive, "created_at": c.CreatedAt, "updated_at": c.UpdatedAt,
	}
}

// ChannelRepo persists channels, their identities and the inbound inbox.
type ChannelRepo struct{ db *db.DB }

const channelCols = `id, project_id, name, kind, inbound_secret, outbound_url, outbound_secret, outbound_headers, session_ttl_seconds, mapping, is_active, created_at, updated_at`

func scanChannel(s scanner) (*Channel, error) {
	var (
		c       Channel
		outURL  sql.NullString
		outSec  sql.NullString
		headers sql.NullString
		mapping sql.NullString
		created models.NullTime
		updated models.NullTime
	)
	if err := s.Scan(&c.ID, &c.ProjectID, &c.Name, &c.Kind, &c.InboundSecret, &outURL, &outSec,
		&headers, &c.SessionTTL, &mapping, &c.IsActive, &created, &updated); err != nil {
		return nil, err
	}
	c.OutboundURL = str(outURL)
	c.OutboundSecret = str(outSec)
	c.OutboundHeaders = map[string]string{}
	if headers.Valid && headers.String != "" {
		_ = json.Unmarshal([]byte(headers.String), &c.OutboundHeaders)
	}
	if mapping.Valid && mapping.String != "" {
		c.Mapping = json.RawMessage(mapping.String)
	} else {
		c.Mapping = json.RawMessage("{}")
	}
	c.CreatedAt = created.Time
	c.UpdatedAt = updated.Time
	return &c, nil
}

// Create stores a channel.
func (r *ChannelRepo) Create(ctx context.Context, c *Channel) error {
	now := models.NowStr()
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO channels
		(id, project_id, name, kind, inbound_secret, outbound_url, outbound_secret, outbound_headers,
		 session_ttl_seconds, mapping, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.ProjectID, c.Name, c.Kind, c.InboundSecret, toNullStr(c.OutboundURL),
		toNullStr(c.OutboundSecret), headersJSON(c.OutboundHeaders), c.SessionTTL,
		string(c.Mapping), boolInt(c.IsActive), now, now)
	if err != nil {
		return fmt.Errorf("creating channel: %w", err)
	}
	return nil
}

// Update rewrites the editable fields of a channel.
func (r *ChannelRepo) Update(ctx context.Context, c *Channel) error {
	res, err := r.db.W.ExecContext(ctx, `UPDATE channels SET name = ?, kind = ?, outbound_url = ?,
		outbound_secret = ?, outbound_headers = ?, session_ttl_seconds = ?, mapping = ?,
		is_active = ?, updated_at = ? WHERE id = ?`,
		c.Name, c.Kind, toNullStr(c.OutboundURL), toNullStr(c.OutboundSecret),
		headersJSON(c.OutboundHeaders), c.SessionTTL, string(c.Mapping),
		boolInt(c.IsActive), models.NowStr(), c.ID)
	if err != nil {
		return fmt.Errorf("updating channel: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func headersJSON(h map[string]string) string {
	if len(h) == 0 {
		return "{}"
	}
	b, err := json.Marshal(h)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// RotateSecret replaces the inbound secret.
func (r *ChannelRepo) RotateSecret(ctx context.Context, id, secret string) error {
	res, err := r.db.W.ExecContext(ctx, `UPDATE channels SET inbound_secret = ?, updated_at = ? WHERE id = ?`,
		secret, models.NowStr(), id)
	if err != nil {
		return fmt.Errorf("rotating channel secret: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetByID loads one channel.
func (r *ChannelRepo) GetByID(ctx context.Context, id string) (*Channel, error) {
	c, err := scanChannel(r.db.R.QueryRowContext(ctx, `SELECT `+channelCols+` FROM channels WHERE id = ?`, id))
	if err != nil {
		return nil, wrapErr("getting channel", err)
	}
	return c, nil
}

// List returns the project channels.
func (r *ChannelRepo) List(ctx context.Context, projectID string) ([]*Channel, error) {
	rows, err := r.db.R.QueryContext(ctx, `SELECT `+channelCols+` FROM channels WHERE project_id = ? ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("listing channels: %w", err)
	}
	defer rows.Close()
	out := make([]*Channel, 0, 8)
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Delete removes a channel and its identities (inbox rows are kept for audits).
func (r *ChannelRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.W.ExecContext(ctx, `DELETE FROM channels WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting channel: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ChannelIdentity links an external user to a visitor.
type ChannelIdentity struct {
	ID         string
	ChannelID  string
	ExternalID string
	VisitorID  string
	Metadata   json.RawMessage
	CreatedAt  time.Time
}

// UpsertIdentity stores the mapping and returns the visitor id.
func (r *ChannelRepo) UpsertIdentity(ctx context.Context, channelID, externalID, visitorID string, metadata json.RawMessage) error {
	if len(metadata) == 0 {
		metadata = json.RawMessage("{}")
	}
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO channel_identities
		(id, channel_id, external_id, visitor_id, metadata, created_at) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(channel_id, external_id) DO UPDATE SET visitor_id = excluded.visitor_id, metadata = excluded.metadata`,
		id.NewPrefixed(id.PrefixChannelIdent), channelID, externalID, visitorID, string(metadata), models.NowStr())
	if err != nil {
		return fmt.Errorf("upserting channel identity: %w", err)
	}
	return nil
}

// IdentityVisitor resolves an external id to its visitor.
func (r *ChannelRepo) IdentityVisitor(ctx context.Context, channelID, externalID string) (string, error) {
	var visitorID string
	err := r.db.R.QueryRowContext(ctx, `SELECT visitor_id FROM channel_identities WHERE channel_id = ? AND external_id = ?`,
		channelID, externalID).Scan(&visitorID)
	if err != nil {
		return "", wrapErr("getting channel identity", err)
	}
	return visitorID, nil
}

// InboxEntry is one raw inbound request for the inspector.
type InboxEntry struct {
	ID             string    `json:"id"`
	ChannelID      *string   `json:"channel_id,omitempty"`
	ProjectID      *string   `json:"project_id,omitempty"`
	Method         string    `json:"method"`
	RemoteIP       string    `json:"remote_ip"`
	Status         string    `json:"status"`
	Error          *string   `json:"error,omitempty"`
	Body           string    `json:"body,omitempty"`
	Mapped         string    `json:"mapped,omitempty"`
	ConversationID *string   `json:"conversation_id,omitempty"`
	ReceivedAt     time.Time `json:"received_at"`
}

// CreateInbox stores one inbound request.
func (r *ChannelRepo) CreateInbox(ctx context.Context, e *InboxEntry) error {
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO webhook_inbox
		(id, channel_id, project_id, method, remote_ip, status, error, body, mapped, conversation_id, received_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, toNullStr(e.ChannelID), toNullStr(e.ProjectID), e.Method, e.RemoteIP, e.Status,
		toNullStr(e.Error), e.Body, e.Mapped, toNullStr(e.ConversationID), models.NowStr())
	if err != nil {
		return fmt.Errorf("storing inbox entry: %w", err)
	}
	return nil
}

// UpdateInbox rewrites the outcome of an inbox entry.
func (r *ChannelRepo) UpdateInbox(ctx context.Context, id, status string, resultErr *string, mapped, conversationID string) error {
	_, err := r.db.W.ExecContext(ctx, `UPDATE webhook_inbox SET status = ?, error = ?, mapped = ?, conversation_id = ? WHERE id = ?`,
		status, toNullStr(resultErr), mapped, conversationID, id)
	if err != nil {
		return fmt.Errorf("updating inbox entry: %w", err)
	}
	return nil
}

// ListInbox returns the newest entries of a channel (or the whole project).
func (r *ChannelRepo) ListInbox(ctx context.Context, projectID, channelID string, limit int) ([]*InboxEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := `SELECT id, channel_id, project_id, method, remote_ip, status, error, body, mapped, conversation_id, received_at
		FROM webhook_inbox WHERE project_id = ?`
	args := []any{projectID}
	if channelID != "" {
		query += ` AND channel_id = ?`
		args = append(args, channelID)
	}
	query += ` ORDER BY received_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := r.db.R.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing inbox: %w", err)
	}
	defer rows.Close()
	out := make([]*InboxEntry, 0, 16)
	for rows.Next() {
		var (
			e        InboxEntry
			channel  sql.NullString
			project  sql.NullString
			errText  sql.NullString
			body     sql.NullString
			mapped   sql.NullString
			convID   sql.NullString
			received models.NullTime
		)
		if err := rows.Scan(&e.ID, &channel, &project, &e.Method, &e.RemoteIP, &e.Status, &errText,
			&body, &mapped, &convID, &received); err != nil {
			return nil, err
		}
		e.ChannelID = str(channel)
		e.ProjectID = str(project)
		e.Error = str(errText)
		e.Body = body.String
		e.Mapped = mapped.String
		e.ConversationID = str(convID)
		e.ReceivedAt = received.Time
		out = append(out, &e)
	}
	return out, rows.Err()
}

// PurgeInbox deletes entries older than the retention window (48h by default).
func (r *ChannelRepo) PurgeInbox(ctx context.Context, olderThan time.Time) (int64, error) {
	res, err := r.db.W.ExecContext(ctx, `DELETE FROM webhook_inbox WHERE received_at < ?`, models.FormatTime(olderThan))
	if err != nil {
		return 0, fmt.Errorf("purging inbox: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
