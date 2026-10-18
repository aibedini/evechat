package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/models"
)

// CannedRepo persists saved agent replies.
type CannedRepo struct{ db *db.DB }

const cannedCols = `id, project_id, shortcut, title, body, created_at`

func scanCanned(s scanner) (*models.CannedResponse, error) {
	var (
		c         models.CannedResponse
		createdAt models.NullTime
	)
	if err := s.Scan(&c.ID, &c.ProjectID, &c.Shortcut, &c.Title, &c.Body, &createdAt); err != nil {
		return nil, err
	}
	c.CreatedAt = createdAt.Time
	return &c, nil
}

// Create inserts a canned response.
func (r *CannedRepo) Create(ctx context.Context, c *models.CannedResponse) error {
	if c.ID == "" {
		c.ID = id.NewPrefixed(id.PrefixKB)
	}
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO canned_responses (id, project_id, shortcut, title, body, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, c.ID, c.ProjectID, c.Shortcut, c.Title, c.Body, models.NowStr())
	if err != nil {
		if isUnique(err) {
			return ErrDuplicate
		}
		return fmt.Errorf("creating canned response: %w", err)
	}
	return nil
}

// List returns all canned responses of a project.
func (r *CannedRepo) List(ctx context.Context, projectID string) ([]*models.CannedResponse, error) {
	rows, err := r.db.R.QueryContext(ctx, `SELECT `+cannedCols+` FROM canned_responses WHERE project_id = ? ORDER BY shortcut ASC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("listing canned responses: %w", err)
	}
	defer rows.Close()
	out := make([]*models.CannedResponse, 0, 16)
	for rows.Next() {
		c, err := scanCanned(rows)
		if err != nil {
			return nil, wrapErr("scanning canned response", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetByID loads one canned response.
func (r *CannedRepo) GetByID(ctx context.Context, id string) (*models.CannedResponse, error) {
	c, err := scanCanned(r.db.R.QueryRowContext(ctx, `SELECT `+cannedCols+` FROM canned_responses WHERE id = ?`, id))
	if err != nil {
		return nil, wrapErr("getting canned response", err)
	}
	return c, nil
}

// Update persists a canned response.
func (r *CannedRepo) Update(ctx context.Context, c *models.CannedResponse) error {
	res, err := r.db.W.ExecContext(ctx, `UPDATE canned_responses SET shortcut = ?, title = ?, body = ? WHERE id = ?`,
		c.Shortcut, c.Title, c.Body, c.ID)
	if err != nil {
		if isUnique(err) {
			return ErrDuplicate
		}
		return fmt.Errorf("updating canned response: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a canned response.
func (r *CannedRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.W.ExecContext(ctx, `DELETE FROM canned_responses WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting canned response: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// TicketFilter narrows a ticket listing.
type TicketFilter struct {
	Status   string
	Priority string
	AgentID  string
	Query    string
	Limit    int
	Offset   int
}

// TicketRepo persists tickets.
type TicketRepo struct{ db *db.DB }

const ticketCols = `id, project_id, conversation_id, title, description, status, priority,
	assigned_agent_id, visitor_id, tags, internal_notes, due_at, resolved_at, created_at, updated_at`

func scanTicket(s scanner) (*models.Ticket, error) {
	var (
		t         models.Ticket
		conv      sql.NullString
		desc      sql.NullString
		assignee  sql.NullString
		visitor   sql.NullString
		dueAt     models.NullTime
		resolved  models.NullTime
		createdAt models.NullTime
		updatedAt models.NullTime
	)
	if err := s.Scan(&t.ID, &t.ProjectID, &conv, &t.Title, &desc, &t.Status, &t.Priority, &assignee,
		&visitor, &t.Tags, &t.InternalNotes, &dueAt, &resolved, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	t.ConversationID = str(conv)
	t.Description = str(desc)
	t.AssignedAgentID = str(assignee)
	t.VisitorID = str(visitor)
	t.DueAt = dueAt.Ptr()
	t.ResolvedAt = resolved.Ptr()
	t.CreatedAt = createdAt.Time
	t.UpdatedAt = updatedAt.Time
	return &t, nil
}

// Create inserts a ticket.
func (r *TicketRepo) Create(ctx context.Context, t *models.Ticket) error {
	now := models.NowStr()
	if t.Status == "" {
		t.Status = models.TicketOpen
	}
	if t.Priority == "" {
		t.Priority = models.PriorityNormal
	}
	_, err := r.db.W.ExecContext(ctx, `INSERT INTO tickets
		(id, project_id, conversation_id, title, description, status, priority, assigned_agent_id,
		 visitor_id, tags, internal_notes, due_at, resolved_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?)`,
		t.ID, t.ProjectID, toNullStr(t.ConversationID), t.Title, toNullStr(t.Description), string(t.Status),
		string(t.Priority), toNullStr(t.AssignedAgentID), toNullStr(t.VisitorID), t.Tags, t.InternalNotes,
		nullTimeVal(t.DueAt), now, now)
	if err != nil {
		return fmt.Errorf("creating ticket: %w", err)
	}
	return nil
}

func nullTimeVal(t *time.Time) any {
	if t == nil {
		return nil
	}
	return models.FormatTime(*t)
}

// GetByID loads one ticket.
func (r *TicketRepo) GetByID(ctx context.Context, id string) (*models.Ticket, error) {
	t, err := scanTicket(r.db.R.QueryRowContext(ctx, `SELECT `+ticketCols+` FROM tickets WHERE id = ?`, id))
	if err != nil {
		return nil, wrapErr("getting ticket", err)
	}
	return t, nil
}

// GetByConversation returns the ticket created from a conversation.
func (r *TicketRepo) GetByConversation(ctx context.Context, convID string) (*models.Ticket, error) {
	t, err := scanTicket(r.db.R.QueryRowContext(ctx, `SELECT `+ticketCols+` FROM tickets WHERE conversation_id = ?
		ORDER BY created_at DESC LIMIT 1`, convID))
	if err != nil {
		return nil, wrapErr("getting ticket by conversation", err)
	}
	return t, nil
}

// List returns a filtered page plus the total count.
func (r *TicketRepo) List(ctx context.Context, projectID string, f TicketFilter) ([]*models.Ticket, int, error) {
	limit, offset := pageClamp(f.Limit, f.Offset)
	where := []string{"project_id = ?"}
	args := []any{projectID}
	if f.Status != "" && f.Status != "all" {
		where = append(where, "status = ?")
		args = append(args, f.Status)
	}
	if f.Priority != "" {
		where = append(where, "priority = ?")
		args = append(args, f.Priority)
	}
	if f.AgentID != "" {
		where = append(where, "assigned_agent_id = ?")
		args = append(args, f.AgentID)
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		where = append(where, `(title LIKE ? ESCAPE '\' OR description LIKE ? ESCAPE '\')`)
		pat := likeTerm(q)
		args = append(args, pat, pat)
	}
	cond := strings.Join(where, " AND ")
	var total int
	if err := r.db.R.QueryRowContext(ctx, `SELECT COUNT(*) FROM tickets WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting tickets: %w", err)
	}
	qargs := append(append([]any{}, args...), limit, offset)
	rows, err := r.db.R.QueryContext(ctx, `SELECT `+ticketCols+` FROM tickets WHERE `+cond+
		` ORDER BY CASE priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 WHEN 'normal' THEN 2 ELSE 3 END,
		created_at DESC LIMIT ? OFFSET ?`, qargs...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing tickets: %w", err)
	}
	defer rows.Close()
	out := make([]*models.Ticket, 0, limit)
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, 0, wrapErr("scanning ticket", err)
		}
		out = append(out, t)
	}
	return out, total, rows.Err()
}

// Update persists the editable ticket fields.
func (r *TicketRepo) Update(ctx context.Context, t *models.Ticket) error {
	var resolved any
	if t.Status == models.TicketResolved || t.Status == models.TicketClosed {
		resolved = models.NowStr()
	}
	res, err := r.db.W.ExecContext(ctx, `UPDATE tickets SET title = ?, description = ?, status = ?, priority = ?,
		assigned_agent_id = ?, tags = ?, internal_notes = ?, due_at = ?, resolved_at = COALESCE(?, resolved_at), updated_at = ?
		WHERE id = ?`,
		t.Title, toNullStr(t.Description), string(t.Status), string(t.Priority), toNullStr(t.AssignedAgentID),
		t.Tags, t.InternalNotes, nullTimeVal(t.DueAt), resolved, models.NowStr(), t.ID)
	if err != nil {
		return fmt.Errorf("updating ticket: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetStatus transitions a ticket, appending an audit note.
func (r *TicketRepo) SetStatus(ctx context.Context, id string, status models.TicketStatus, note models.Note) error {
	t, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	t.Status = status
	if note.ID != "" {
		t.InternalNotes = append(t.InternalNotes, note)
	}
	return r.Update(ctx, t)
}

// AddNote appends an internal note.
func (r *TicketRepo) AddNote(ctx context.Context, id string, note models.Note) error {
	t, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	t.InternalNotes = append(t.InternalNotes, note)
	return r.Update(ctx, t)
}

// Delete removes a ticket.
func (r *TicketRepo) Delete(ctx context.Context, id string) error {
	res, err := r.db.W.ExecContext(ctx, `DELETE FROM tickets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting ticket: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// CountOpen counts non resolved tickets for a project.
func (r *TicketRepo) CountOpen(ctx context.Context, projectID string) (int, error) {
	var n int
	if err := r.db.R.QueryRowContext(ctx, `SELECT COUNT(*) FROM tickets
		WHERE project_id = ? AND status IN ('open','in_progress','waiting')`, projectID).Scan(&n); err != nil {
		return 0, fmt.Errorf("counting tickets: %w", err)
	}
	return n, nil
}
