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

// ConversationRepo persists chat threads.
type ConversationRepo struct{ db *db.DB }

const convCols = `id, project_id, visitor_id, assigned_agent_id, routing_rule_id, status, channel,
	subject, page_url, page_title, referrer, routed_telegram_id, routed_phone, ticket_id,
	unread_agent, unread_visitor, last_message_at, first_response_at, resolved_at, csat_score, csat_comment, created_at`

// ConvFilter narrows conversation listings.
type ConvFilter struct {
	Status    string
	Statuses  []string
	AgentID   string
	VisitorID string
	Channel   string
	Query     string
	From      time.Time
	To        time.Time
	// RestrictToAgentID implements the `agent` role visibility rule: a plain
	// agent only sees conversations assigned to them plus unassigned ones.
	RestrictToAgentID string
	// HideClosed excludes resolved+closed conversations (default inbox).
	HideClosed bool
	Limit      int
	Offset     int
}

func scanConv(s scanner) (*models.Conversation, error) {
	var (
		c           models.Conversation
		assigned    sql.NullString
		rule        sql.NullString
		subject     sql.NullString
		pageURL     sql.NullString
		pageTitle   sql.NullString
		referrer    sql.NullString
		tg          sql.NullString
		phone       sql.NullString
		ticket      sql.NullString
		lastMsg     models.NullTime
		firstResp   models.NullTime
		resolved    models.NullTime
		csat        sql.NullInt64
		csatComment sql.NullString
		createdAt   models.NullTime
	)
	if err := s.Scan(&c.ID, &c.ProjectID, &c.VisitorID, &assigned, &rule, &c.Status, &c.Channel,
		&subject, &pageURL, &pageTitle, &referrer, &tg, &phone, &ticket,
		&c.UnreadAgent, &c.UnreadVisitor, &lastMsg, &firstResp, &resolved, &csat, &csatComment, &createdAt); err != nil {
		return nil, err
	}
	c.AssignedAgentID = str(assigned)
	c.RoutingRuleID = str(rule)
	c.Subject = str(subject)
	c.PageURL = str(pageURL)
	c.PageTitle = str(pageTitle)
	c.Referrer = str(referrer)
	c.RoutedTelegramID = str(tg)
	c.RoutedPhone = str(phone)
	c.TicketID = str(ticket)
	c.LastMessageAt = lastMsg.Ptr()
	c.FirstResponseAt = firstResp.Ptr()
	c.ResolvedAt = resolved.Ptr()
	if csat.Valid {
		v := int(csat.Int64)
		c.CSATScore = &v
	}
	c.CSATComment = str(csatComment)
	c.CreatedAt = createdAt.Time
	return &c, nil
}

// Create inserts a conversation.
func (r *ConversationRepo) Create(ctx context.Context, c *models.Conversation) error {
	now := models.NowStr()
	if c.CreatedAt.IsZero() {
		c.CreatedAt, _ = models.ParseTime(now)
	}
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO conversations
		(id, project_id, visitor_id, assigned_agent_id, routing_rule_id, status, channel, subject,
		 page_url, page_title, referrer, routed_telegram_id, routed_phone, ticket_id, unread_agent,
		 unread_visitor, last_message_at, first_response_at, resolved_at, csat_score, csat_comment, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.ProjectID, c.VisitorID, toNullStr(c.AssignedAgentID), toNullStr(c.RoutingRuleID),
		string(c.Status), c.Channel, toNullStr(c.Subject), toNullStr(c.PageURL), toNullStr(c.PageTitle),
		toNullStr(c.Referrer), toNullStr(c.RoutedTelegramID), toNullStr(c.RoutedPhone), toNullStr(c.TicketID),
		c.UnreadAgent, c.UnreadVisitor, nil, nil, nil, nil, nil, now)
	if err != nil {
		return fmt.Errorf("creating conversation: %w", err)
	}
	return nil
}

// GetByID loads one conversation.
func (r *ConversationRepo) GetByID(ctx context.Context, id string) (*models.Conversation, error) {
	c, err := scanConv(r.db.R.QueryRowContext(ctx, `SELECT `+convCols+` FROM conversations WHERE id = ?`, id))
	if err != nil {
		return nil, wrapErr("getting conversation", err)
	}
	return c, nil
}

// List returns a filtered page of conversations and the total count.
func (r *ConversationRepo) List(ctx context.Context, projectID string, f ConvFilter) ([]*models.Conversation, int, error) {
	limit, offset := pageClamp(f.Limit, f.Offset)
	where := []string{"c.project_id = ?"}
	args := []any{projectID}

	if f.Status != "" && f.Status != "all" {
		where = append(where, "c.status = ?")
		args = append(args, f.Status)
	}
	if len(f.Statuses) > 0 {
		ph := make([]string, len(f.Statuses))
		for i, st := range f.Statuses {
			ph[i] = "?"
			args = append(args, st)
		}
		where = append(where, "c.status IN ("+strings.Join(ph, ",")+")")
	}
	if f.AgentID != "" {
		where = append(where, "c.assigned_agent_id = ?")
		args = append(args, f.AgentID)
	}
	if f.VisitorID != "" {
		where = append(where, "c.visitor_id = ?")
		args = append(args, f.VisitorID)
	}
	if f.Channel != "" {
		where = append(where, "c.channel = ?")
		args = append(args, f.Channel)
	}
	if f.RestrictToAgentID != "" {
		// An agent sees their own conversations plus the unclaimed queue.
		// Assignment alone takes a conversation out of the shared queue without
		// activating it (see Assign).
		where = append(where, "(c.assigned_agent_id = ? OR (c.status = 'pending' AND c.assigned_agent_id IS NULL))")
		args = append(args, f.RestrictToAgentID)
	}
	if f.HideClosed {
		where = append(where, "c.status IN ('pending','active')")
	}
	if !f.From.IsZero() {
		where = append(where, "c.created_at >= ?")
		args = append(args, models.FormatTime(f.From))
	}
	if !f.To.IsZero() {
		where = append(where, "c.created_at <= ?")
		args = append(args, models.FormatTime(f.To))
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		pat := likeTerm(q)
		where = append(where, `(c.subject LIKE ? ESCAPE '\' OR c.id = ? OR c.visitor_id IN
			(SELECT id FROM visitors WHERE name LIKE ? ESCAPE '\' OR email LIKE ? ESCAPE '\' OR phone LIKE ? ESCAPE '\'))`)
		args = append(args, pat, q, pat, pat, pat)
	}
	cond := strings.Join(where, " AND ")

	var total int
	if err := r.db.R.QueryRowContext(ctx, `SELECT COUNT(*) FROM conversations c WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting conversations: %w", err)
	}

	qargs := append(append([]any{}, args...), limit, offset)
	rows, err := r.db.R.QueryContext(ctx, `SELECT `+prefixed(convCols, "c")+` FROM conversations c WHERE `+cond+
		` ORDER BY COALESCE(c.last_message_at, c.created_at) DESC LIMIT ? OFFSET ?`, qargs...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing conversations: %w", err)
	}
	defer rows.Close()
	out := make([]*models.Conversation, 0, limit)
	for rows.Next() {
		c, err := scanConv(rows)
		if err != nil {
			return nil, 0, wrapErr("scanning conversation", err)
		}
		out = append(out, c)
	}
	return out, total, rows.Err()
}

// Assign sets the assignee. Assignment is not activation: a routed or manually
// assigned conversation stays pending until somebody claims it or replies, which
// keeps the "pending" queue honest (routing rules assign automatically).
func (r *ConversationRepo) Assign(ctx context.Context, convID, agentID string) error {
	res, err := r.db.W.ExecContext(ctx, `UPDATE conversations SET assigned_agent_id = ? WHERE id = ?`, agentID, convID)
	if err != nil {
		return fmt.Errorf("assigning conversation: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Claim atomically takes a pending conversation for agentID and activates it.
// It reports false when the conversation is no longer pending (somebody else
// claimed it first, or a reply already activated it), which the caller maps to
// a conflict instead of silently reassigning.
func (r *ConversationRepo) Claim(ctx context.Context, convID, agentID string) (bool, error) {
	res, err := r.db.W.ExecContext(ctx, `UPDATE conversations
		SET assigned_agent_id = ?, status = 'active'
		WHERE id = ? AND status = 'pending' AND (assigned_agent_id IS NULL OR assigned_agent_id = ?)`,
		agentID, convID, agentID)
	if err != nil {
		return false, fmt.Errorf("claiming conversation: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("claiming conversation: %w", err)
	}
	return n > 0, nil
}

// ActivateIfPending moves a pending conversation to active. It is the implicit
// activation that runs on the first human reply and is a no-op afterwards.
func (r *ConversationRepo) ActivateIfPending(ctx context.Context, convID string) (bool, error) {
	res, err := r.db.W.ExecContext(ctx, `UPDATE conversations SET status = 'active'
		WHERE id = ? AND status = 'pending'`, convID)
	if err != nil {
		return false, fmt.Errorf("activating conversation: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("activating conversation: %w", err)
	}
	return n > 0, nil
}

// Unassign clears the assignee.
func (r *ConversationRepo) Unassign(ctx context.Context, convID string) error {
	_, err := r.db.W.ExecContext(ctx, `UPDATE conversations SET assigned_agent_id = NULL WHERE id = ?`, convID)
	if err != nil {
		return fmt.Errorf("unassigning conversation: %w", err)
	}
	return nil
}

// SetStatus transitions the conversation state.
func (r *ConversationRepo) SetStatus(ctx context.Context, convID string, status models.ConvStatus) error {
	var resolved any
	if status == models.StatusResolved || status == models.StatusClosed {
		resolved = models.NowStr()
	}
	res, err := r.db.W.ExecContext(ctx, `UPDATE conversations SET status = ?, resolved_at = ? WHERE id = ?`,
		string(status), resolved, convID)
	if err != nil {
		return fmt.Errorf("setting conversation status: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetRouting stores the routing decision for a conversation.
func (r *ConversationRepo) SetRouting(ctx context.Context, convID, ruleID, telegramID, phone string) error {
	var rule, tg, ph any
	if ruleID != "" {
		rule = ruleID
	}
	if telegramID != "" {
		tg = telegramID
	}
	if phone != "" {
		ph = phone
	}
	_, err := r.db.W.ExecContext(ctx, `UPDATE conversations SET routing_rule_id = ?, routed_telegram_id = ?, routed_phone = ? WHERE id = ?`,
		rule, tg, ph, convID)
	if err != nil {
		return fmt.Errorf("setting conversation routing: %w", err)
	}
	return nil
}

// TouchLastMessage updates ordering timestamps.
func (r *ConversationRepo) TouchLastMessage(ctx context.Context, convID string, at time.Time) error {
	_, err := r.db.W.ExecContext(ctx, `UPDATE conversations SET last_message_at = ?,
		status = CASE WHEN status = 'pending' THEN 'pending' ELSE status END WHERE id = ?`,
		models.FormatTime(at), convID)
	if err != nil {
		return fmt.Errorf("touching conversation: %w", err)
	}
	return nil
}

// IncrementUnread bumps the unread counter of one side ("agent" or "visitor").
func (r *ConversationRepo) IncrementUnread(ctx context.Context, convID, side string) error {
	col := "unread_agent"
	if side == "visitor" {
		col = "unread_visitor"
	}
	_, err := r.db.W.ExecContext(ctx, `UPDATE conversations SET `+col+` = `+col+` + 1 WHERE id = ?`, convID)
	if err != nil {
		return fmt.Errorf("incrementing unread: %w", err)
	}
	return nil
}

// SetUnread sets an absolute unread counter.
func (r *ConversationRepo) SetUnread(ctx context.Context, convID, side string, n int) error {
	col := "unread_agent"
	if side == "visitor" {
		col = "unread_visitor"
	}
	_, err := r.db.W.ExecContext(ctx, `UPDATE conversations SET `+col+` = ? WHERE id = ?`, n, convID)
	if err != nil {
		return fmt.Errorf("setting unread: %w", err)
	}
	return nil
}

// ClearUnread resets the unread counter of one side.
func (r *ConversationRepo) ClearUnread(ctx context.Context, convID, side string) error {
	col := "unread_agent"
	if side == "visitor" {
		col = "unread_visitor"
	}
	_, err := r.db.W.ExecContext(ctx, `UPDATE conversations SET `+col+` = 0 WHERE id = ?`, convID)
	if err != nil {
		return fmt.Errorf("clearing unread: %w", err)
	}
	return nil
}

// SetFirstResponse records the first agent reply once.
func (r *ConversationRepo) SetFirstResponse(ctx context.Context, convID string, at time.Time) error {
	_, err := r.db.W.ExecContext(ctx, `UPDATE conversations SET first_response_at = ?
		WHERE id = ? AND first_response_at IS NULL`, models.FormatTime(at), convID)
	if err != nil {
		return fmt.Errorf("setting first response: %w", err)
	}
	return nil
}

// GetByTelegramID returns the latest open conversation relayed to a Telegram chat.
func (r *ConversationRepo) GetByTelegramID(ctx context.Context, tgID string) (*models.Conversation, error) {
	c, err := scanConv(r.db.R.QueryRowContext(ctx, `SELECT `+convCols+` FROM conversations
		WHERE routed_telegram_id = ? AND status IN ('pending','active')
		ORDER BY created_at DESC LIMIT 1`, tgID))
	if err != nil {
		return nil, wrapErr("getting conversation by telegram id", err)
	}
	return c, nil
}

// GetByPhone returns the latest open conversation relayed to a phone number.
func (r *ConversationRepo) GetByPhone(ctx context.Context, phone string) (*models.Conversation, error) {
	c, err := scanConv(r.db.R.QueryRowContext(ctx, `SELECT `+convCols+` FROM conversations
		WHERE routed_phone = ? AND status IN ('pending','active')
		ORDER BY created_at DESC LIMIT 1`, phone))
	if err != nil {
		return nil, wrapErr("getting conversation by phone", err)
	}
	return c, nil
}

// SetCSAT stores a satisfaction score.
func (r *ConversationRepo) SetCSAT(ctx context.Context, convID string, score int, comment string) error {
	var cm any
	if comment != "" {
		cm = comment
	}
	res, err := r.db.W.ExecContext(ctx, `UPDATE conversations SET csat_score = ?, csat_comment = ? WHERE id = ?`,
		score, cm, convID)
	if err != nil {
		return fmt.Errorf("setting csat: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetTicket links a ticket to the conversation.
func (r *ConversationRepo) SetTicket(ctx context.Context, convID, ticketID string) error {
	_, err := r.db.W.ExecContext(ctx, `UPDATE conversations SET ticket_id = ? WHERE id = ?`, ticketID, convID)
	if err != nil {
		return fmt.Errorf("linking ticket: %w", err)
	}
	return nil
}

// OpenForVisitor returns the open conversation of a visitor, if any.
func (r *ConversationRepo) OpenForVisitor(ctx context.Context, visitorID string) (*models.Conversation, error) {
	c, err := scanConv(r.db.R.QueryRowContext(ctx, `SELECT `+convCols+` FROM conversations
		WHERE visitor_id = ? AND status IN ('pending','active') ORDER BY created_at DESC LIMIT 1`, visitorID))
	if err != nil {
		return nil, wrapErr("getting open conversation", err)
	}
	return c, nil
}

// OpenForAgent lists the open conversations assigned to an agent.
func (r *ConversationRepo) OpenForAgent(ctx context.Context, agentID string) ([]*models.Conversation, error) {
	rows, err := r.db.R.QueryContext(ctx, `SELECT `+convCols+` FROM conversations
		WHERE assigned_agent_id = ? AND status IN ('pending','active') ORDER BY created_at DESC`, agentID)
	if err != nil {
		return nil, fmt.Errorf("listing agent conversations: %w", err)
	}
	defer rows.Close()
	out := make([]*models.Conversation, 0, 8)
	for rows.Next() {
		c, err := scanConv(rows)
		if err != nil {
			return nil, wrapErr("scanning conversation", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListByVisitor returns the conversation history of a visitor.
func (r *ConversationRepo) ListByVisitor(ctx context.Context, visitorID string, limit int) ([]*models.Conversation, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.R.QueryContext(ctx, `SELECT `+convCols+` FROM conversations WHERE visitor_id = ?
		ORDER BY created_at DESC LIMIT ?`, visitorID, limit)
	if err != nil {
		return nil, fmt.Errorf("listing visitor conversations: %w", err)
	}
	defer rows.Close()
	out := make([]*models.Conversation, 0, limit)
	for rows.Next() {
		c, err := scanConv(rows)
		if err != nil {
			return nil, wrapErr("scanning conversation", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DueForCSAT lists resolved conversations without a score, for the CSAT timer.
func (r *ConversationRepo) DueForCSAT(ctx context.Context, resolvedBefore time.Time, limit int) ([]*models.Conversation, error) {
	rows, err := r.db.R.QueryContext(ctx, `SELECT `+convCols+` FROM conversations
		WHERE status IN ('resolved','closed') AND csat_score IS NULL AND resolved_at IS NOT NULL AND resolved_at <= ?
		ORDER BY resolved_at ASC LIMIT ?`, models.FormatTime(resolvedBefore), limit)
	if err != nil {
		return nil, fmt.Errorf("listing conversations due for csat: %w", err)
	}
	defer rows.Close()
	out := make([]*models.Conversation, 0, limit)
	for rows.Next() {
		c, err := scanConv(rows)
		if err != nil {
			return nil, wrapErr("scanning conversation", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func prefixed(cols, alias string) string {
	parts := strings.Split(cols, ",")
	for i, p := range parts {
		parts[i] = alias + "." + strings.TrimSpace(p)
	}
	return strings.Join(parts, ", ")
}
