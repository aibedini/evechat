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

// MessageRepo persists encrypted messages.
type MessageRepo struct{ db *db.DB }

const msgCols = `id, conversation_id, client_message_id, sender_type, sender_id, sender_name, content_type,
	content_encrypted, content_iv, key_version, attachments, telegram_message_id, is_internal, is_read, created_at`

func scanMessage(s scanner) (*models.Message, error) {
	var (
		m           models.Message
		clientMsgID sql.NullString
		senderID    sql.NullString
		senderName  sql.NullString
		tgID        sql.NullInt64
		createdAt   models.NullTime
	)
	if err := s.Scan(&m.ID, &m.ConversationID, &clientMsgID, &m.SenderType, &senderID, &senderName,
		&m.ContentType, &m.ContentEncrypted, &m.ContentIV, &m.KeyVersion, &m.Attachments, &tgID,
		&m.IsInternal, &m.IsRead, &createdAt); err != nil {
		return nil, err
	}
	m.ClientMessageID = str(clientMsgID)
	m.SenderID = str(senderID)
	m.SenderName = str(senderName)
	if tgID.Valid {
		v := tgID.Int64
		m.TelegramMessageID = &v
	}
	m.CreatedAt = createdAt.Time
	return &m, nil
}

// Create inserts a message. It returns ErrDuplicate when the
// (conversation_id, client_message_id) unique index already holds that id.
func (r *MessageRepo) Create(ctx context.Context, m *models.Message) error {
	if m.KeyVersion == 0 {
		m.KeyVersion = 1
	}
	if m.Attachments == nil {
		m.Attachments = models.StringList{}
	}
	now := models.NowStr()
	if m.CreatedAt.IsZero() {
		m.CreatedAt, _ = models.ParseTime(now)
	}
	var tg any
	if m.TelegramMessageID != nil {
		tg = *m.TelegramMessageID
	}
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO messages
		(id, conversation_id, client_message_id, sender_type, sender_id, sender_name, content_type,
		 content_encrypted, content_iv, key_version, attachments, telegram_message_id, is_internal, is_read, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.ConversationID, toNullStr(m.ClientMessageID), string(m.SenderType), toNullStr(m.SenderID),
		toNullStr(m.SenderName), m.ContentType, m.ContentEncrypted, m.ContentIV, m.KeyVersion,
		m.Attachments, tg, boolInt(m.IsInternal), boolInt(m.IsRead), now)
	if err != nil {
		if isUnique(err) {
			return ErrDuplicate
		}
		return fmt.Errorf("creating message: %w", err)
	}
	return nil
}

// CreateTx inserts a message inside an existing transaction.
func (r *MessageRepo) CreateTx(ctx context.Context, tx *sql.Tx, m *models.Message) error {
	if m.KeyVersion == 0 {
		m.KeyVersion = 1
	}
	if m.Attachments == nil {
		m.Attachments = models.StringList{}
	}
	now := models.NowStr()
	var tg any
	if m.TelegramMessageID != nil {
		tg = *m.TelegramMessageID
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO messages
		(id, conversation_id, client_message_id, sender_type, sender_id, sender_name, content_type,
		 content_encrypted, content_iv, key_version, attachments, telegram_message_id, is_internal, is_read, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.ConversationID, toNullStr(m.ClientMessageID), string(m.SenderType), toNullStr(m.SenderID),
		toNullStr(m.SenderName), m.ContentType, m.ContentEncrypted, m.ContentIV, m.KeyVersion,
		m.Attachments, tg, boolInt(m.IsInternal), boolInt(m.IsRead), now)
	if err != nil {
		if isUnique(err) {
			return ErrDuplicate
		}
		return fmt.Errorf("creating message in tx: %w", err)
	}
	return nil
}

// GetByID loads one message.
func (r *MessageRepo) GetByID(ctx context.Context, id string) (*models.Message, error) {
	m, err := scanMessage(r.db.R.QueryRowContext(ctx, `SELECT `+msgCols+` FROM messages WHERE id = ?`, id))
	if err != nil {
		return nil, wrapErr("getting message", err)
	}
	return m, nil
}

// GetByClientID finds a message by its idempotency key.
func (r *MessageRepo) GetByClientID(ctx context.Context, convID, clientMessageID string) (*models.Message, error) {
	m, err := scanMessage(r.db.R.QueryRowContext(ctx, `SELECT `+msgCols+` FROM messages
		WHERE conversation_id = ? AND client_message_id = ? LIMIT 1`, convID, clientMessageID))
	if err != nil {
		return nil, wrapErr("getting message by client id", err)
	}
	return m, nil
}

// ListByConversation returns messages ordered oldest first.
func (r *MessageRepo) ListByConversation(ctx context.Context, convID string, since time.Time, limit int) ([]*models.Message, error) {
	limit, _ = pageClamp(limit, 0)
	where := "conversation_id = ?"
	args := []any{convID}
	if !since.IsZero() {
		where += " AND created_at > ?"
		args = append(args, models.FormatTime(since))
	}
	args = append(args, limit)
	rows, err := r.db.R.QueryContext(ctx, `SELECT `+msgCols+` FROM messages WHERE `+where+` ORDER BY created_at ASC LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("listing messages: %w", err)
	}
	defer rows.Close()
	out := make([]*models.Message, 0, limit)
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, wrapErr("scanning message", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// LatestN returns the newest n messages in chronological order.
func (r *MessageRepo) LatestN(ctx context.Context, convID string, n int) ([]*models.Message, error) {
	if n <= 0 {
		n = 50
	}
	rows, err := r.db.R.QueryContext(ctx, `SELECT `+msgCols+` FROM messages WHERE conversation_id = ?
		ORDER BY created_at DESC, id DESC LIMIT ?`, convID, n)
	if err != nil {
		return nil, fmt.Errorf("listing latest messages: %w", err)
	}
	defer rows.Close()
	out := make([]*models.Message, 0, n)
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, wrapErr("scanning message", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// reverse
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

// MarkRead marks messages as read up to (and including) upToID.
func (r *MessageRepo) MarkRead(ctx context.Context, convID, upToID, side string) error {
	if upToID == "" {
		_, err := r.db.W.ExecContext(ctx, `UPDATE messages SET is_read = 1
			WHERE conversation_id = ? AND sender_type != ? AND is_read = 0`, convID, oppositeSender(side))
		return err
	}
	var createdAt models.NullTime
	if err := r.db.R.QueryRowContext(ctx, `SELECT created_at FROM messages WHERE id = ? AND conversation_id = ?`,
		upToID, convID).Scan(&createdAt); err != nil {
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		return wrapErr("reading message timestamp", err)
	}
	_, err := r.db.W.ExecContext(ctx, `UPDATE messages SET is_read = 1
		WHERE conversation_id = ? AND created_at <= ? AND is_read = 0`, convID, models.FormatTime(createdAt.Time))
	if err != nil {
		return fmt.Errorf("marking messages read: %w", err)
	}
	return nil
}

func oppositeSender(side string) string {
	if side == "visitor" {
		return string(models.SenderVisitor)
	}
	return string(models.SenderAgent)
}

// GetByTelegramMsgID resolves a Telegram reply target inside one chat.
func (r *MessageRepo) GetByTelegramMsgID(ctx context.Context, tgID int64) (*models.Message, error) {
	m, err := scanMessage(r.db.R.QueryRowContext(ctx, `SELECT `+msgCols+` FROM messages
		WHERE telegram_message_id = ? ORDER BY created_at DESC LIMIT 1`, tgID))
	if err != nil {
		return nil, wrapErr("getting message by telegram id", err)
	}
	return m, nil
}

// SetTelegramMsgID links an outbound message to its Telegram counterpart.
func (r *MessageRepo) SetTelegramMsgID(ctx context.Context, msgID string, tgID int64) error {
	_, err := r.db.W.ExecContext(ctx, `UPDATE messages SET telegram_message_id = ? WHERE id = ?`, tgID, msgID)
	if err != nil {
		return fmt.Errorf("setting telegram message id: %w", err)
	}
	return nil
}

// CountByConversation counts messages in a conversation.
func (r *MessageRepo) CountByConversation(ctx context.Context, convID string) (int, error) {
	var n int
	if err := r.db.R.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages WHERE conversation_id = ?`, convID).Scan(&n); err != nil {
		return 0, fmt.Errorf("counting messages: %w", err)
	}
	return n, nil
}

// SearchWindow loads a bounded window of recent messages for decrypted search.
func (r *MessageRepo) SearchWindow(ctx context.Context, convIDs []string, limit int) ([]*models.Message, error) {
	if len(convIDs) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 1000
	}
	ph := make([]string, len(convIDs))
	args := make([]any, len(convIDs), len(convIDs)+1)
	for i, id := range convIDs {
		ph[i] = "?"
		args[i] = id
	}
	args = append(args, limit)
	rows, err := r.db.R.QueryContext(ctx, `SELECT `+msgCols+` FROM messages
		WHERE conversation_id IN (`+strings.Join(ph, ",")+`) ORDER BY created_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("searching messages: %w", err)
	}
	defer rows.Close()
	out := make([]*models.Message, 0, 64)
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, wrapErr("scanning message", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ConversationIDsOfProject returns recent conversation ids for search scoping.
func (r *ConversationRepo) ConversationIDsOfProject(ctx context.Context, projectID string, since time.Time, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 500
	}
	rows, err := r.db.R.QueryContext(ctx, `SELECT id FROM conversations
		WHERE project_id = ? AND created_at >= ? ORDER BY created_at DESC LIMIT ?`,
		projectID, models.FormatTime(since), limit)
	if err != nil {
		return nil, fmt.Errorf("listing conversation ids: %w", err)
	}
	defer rows.Close()
	out := make([]string, 0, limit)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, wrapErr("scanning conversation id", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// MarkVisitorMessagesRead flags every visitor message of a conversation.
func (r *MessageRepo) MarkVisitorMessagesRead(ctx context.Context, convID string) error {
	_, err := r.db.W.ExecContext(ctx, `UPDATE messages SET is_read = 1
		WHERE conversation_id = ? AND sender_type = 'visitor' AND is_read = 0`, convID)
	if err != nil {
		return fmt.Errorf("marking visitor messages read: %w", err)
	}
	return nil
}
