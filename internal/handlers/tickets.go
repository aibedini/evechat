package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

// ListTickets returns the Kanban dataset.
func (h *Handlers) ListTickets(w http.ResponseWriter, r *http.Request) error {
	limit, offset := parsePage(r)
	q := r.URL.Query()
	tickets, total, err := h.Repo.Tickets.List(r.Context(), h.scopeProject(r), repo.TicketFilter{
		Status:   q.Get("status"),
		Priority: q.Get("priority"),
		AgentID:  q.Get("agent"),
		Query:    q.Get("q"),
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusOK, newList(tickets, total, limit, offset))
	return nil
}

type ticketInput struct {
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Priority        string   `json:"priority"`
	Status          string   `json:"status"`
	Tags            []string `json:"tags"`
	VisitorID       *string  `json:"visitor_id"`
	ConversationID  *string  `json:"conversation_id"`
	AssignedAgentID *string  `json:"assigned_agent_id"`
	DueAt           string   `json:"due_at"`
}

// CreateTicket creates a standalone ticket.
func (h *Handlers) CreateTicket(w http.ResponseWriter, r *http.Request) error {
	var in ticketInput
	if err := bind(r, &in); err != nil {
		return err
	}
	if strings.TrimSpace(in.Title) == "" {
		return apierr.BadRequest("عنوان تیکت لازم است")
	}
	priority := models.TicketPriority(in.Priority)
	switch priority {
	case models.PriorityLow, models.PriorityNormal, models.PriorityHigh, models.PriorityUrgent:
	default:
		priority = models.PriorityNormal
	}
	t := &models.Ticket{
		ID:             id.NewPrefixed(id.PrefixTicket),
		ProjectID:      h.scopeProject(r),
		Title:          strings.TrimSpace(in.Title),
		Description:    nilIfEmptyH(in.Description),
		Status:         models.TicketOpen,
		Priority:       priority,
		Tags:           models.StringList(in.Tags),
		InternalNotes:  models.NoteList{},
		VisitorID:      in.VisitorID,
		ConversationID: in.ConversationID,
	}
	if in.AssignedAgentID != nil {
		t.AssignedAgentID = in.AssignedAgentID
	}
	if in.DueAt != "" {
		due, derr := time.Parse(time.RFC3339, in.DueAt)
		if derr != nil {
			return apierr.BadRequest("تاریخ سررسید نامعتبر است")
		}
		t.DueAt = &due
	}
	if err := h.Tickets.Create(r.Context(), t); err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusCreated, t)
	return nil
}

// GetTicket returns one ticket.
func (h *Handlers) GetTicket(w http.ResponseWriter, r *http.Request) error {
	t, err := h.Repo.Tickets.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("تیکت یافت نشد")
	}
	if err := h.checkProjectScope(r, t.ProjectID); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, t)
	return nil
}

// UpdateTicket edits a ticket.
func (h *Handlers) UpdateTicket(w http.ResponseWriter, r *http.Request) error {
	t, err := h.Repo.Tickets.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("تیکت یافت نشد")
	}
	if err := h.checkProjectScope(r, t.ProjectID); err != nil {
		return err
	}
	var in ticketInput
	if err := bind(r, &in); err != nil {
		return err
	}
	if in.Title != "" {
		t.Title = strings.TrimSpace(in.Title)
	}
	if in.Description != "" {
		t.Description = nilIfEmptyH(in.Description)
	}
	if in.Priority != "" {
		t.Priority = models.TicketPriority(in.Priority)
	}
	if in.Status != "" {
		t.Status = models.TicketStatus(in.Status)
	}
	if in.Tags != nil {
		t.Tags = models.StringList(in.Tags)
	}
	if in.AssignedAgentID != nil {
		t.AssignedAgentID = in.AssignedAgentID
	}
	if in.DueAt != "" {
		due, derr := time.Parse(time.RFC3339, in.DueAt)
		if derr != nil {
			return apierr.BadRequest("تاریخ سررسید نامعتبر است")
		}
		t.DueAt = &due
	}
	if err := h.Repo.Tickets.Update(r.Context(), t); err != nil {
		return apierr.Internal(err)
	}
	return ok(w, t)
}

// SetTicketStatus transitions a ticket and logs the change.
func (h *Handlers) SetTicketStatus(w http.ResponseWriter, r *http.Request) error {
	t, err := h.Repo.Tickets.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("تیکت یافت نشد")
	}
	if err := h.checkProjectScope(r, t.ProjectID); err != nil {
		return err
	}
	var in struct {
		Status string `json:"status"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	switch models.TicketStatus(in.Status) {
	case models.TicketOpen, models.TicketInProgress, models.TicketWaiting, models.TicketResolved, models.TicketClosed:
	default:
		return apierr.BadRequest("وضعیت تیکت نامعتبر است")
	}
	note := models.Note{
		ID:       id.NewPrefixed("note"),
		AuthorID: claimsOf(r).Subject,
		Text:     "وضعیت به " + in.Status + " تغییر کرد",
		At:       time.Now().UTC(),
	}
	if err := h.Repo.Tickets.SetStatus(r.Context(), t.ID, models.TicketStatus(in.Status), note); err != nil {
		return apierr.Internal(err)
	}
	return okMsg(w)
}

// AddTicketNote appends an internal note.
func (h *Handlers) AddTicketNote(w http.ResponseWriter, r *http.Request) error {
	t, err := h.Repo.Tickets.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("تیکت یافت نشد")
	}
	if err := h.checkProjectScope(r, t.ProjectID); err != nil {
		return err
	}
	var in struct {
		Text string `json:"text"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	if strings.TrimSpace(in.Text) == "" {
		return apierr.BadRequest("متن یادداشت لازم است")
	}
	if err := h.Tickets.AddNote(r.Context(), t.ID, claimsOf(r).Subject, in.Text); err != nil {
		return apierr.Internal(err)
	}
	updated, _ := h.Repo.Tickets.GetByID(r.Context(), t.ID)
	return ok(w, updated)
}

// DeleteTicket removes a ticket.
func (h *Handlers) DeleteTicket(w http.ResponseWriter, r *http.Request) error {
	t, err := h.Repo.Tickets.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("تیکت یافت نشد")
	}
	if err := h.checkProjectScope(r, t.ProjectID); err != nil {
		return err
	}
	if err := h.Repo.Tickets.Delete(r.Context(), t.ID); err != nil {
		return apierr.Internal(err)
	}
	return okMsg(w)
}
