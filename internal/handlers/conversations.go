package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
	"github.com/evechat/evechat/internal/services"
)

type conversationView struct {
	*models.Conversation
	Visitor *models.Visitor `json:"visitor,omitempty"`
	Agent   *models.Agent   `json:"agent,omitempty"`
}

// conversationAccess enforces the authorization matrix.
func (h *Handlers) conversationAccess(r *http.Request, conv *models.Conversation) error {
	c := claimsOf(r)
	if c == nil {
		return apierr.Unauthorized("احراز هویت لازم است")
	}
	if c.Role != models.RoleSuperAdmin && c.ProjectID != conv.ProjectID {
		return apierr.Forbidden("شما به این گفتگو دسترسی ندارید")
	}
	if c.Role == models.RoleAgent && conv.AssignedAgentID != nil &&
		*conv.AssignedAgentID != c.Subject && conv.Status != models.StatusPending {
		return apierr.Forbidden("این گفتگو به کارشناس دیگری واگذار شده است")
	}
	return nil
}

// enrichConversations batch-loads visitors and agents for a page.
func (h *Handlers) enrichConversations(r *http.Request, convs []*models.Conversation) ([]conversationView, error) {
	visitorIDs := make([]string, 0, len(convs))
	agentIDs := make([]string, 0, len(convs))
	seenV := map[string]struct{}{}
	seenA := map[string]struct{}{}
	for _, c := range convs {
		if _, ok := seenV[c.VisitorID]; !ok {
			seenV[c.VisitorID] = struct{}{}
			visitorIDs = append(visitorIDs, c.VisitorID)
		}
		if c.AssignedAgentID != nil {
			if _, ok := seenA[*c.AssignedAgentID]; !ok {
				seenA[*c.AssignedAgentID] = struct{}{}
				agentIDs = append(agentIDs, *c.AssignedAgentID)
			}
		}
	}
	visitors, err := h.Repo.Visitors.GetByIDs(r.Context(), visitorIDs)
	if err != nil {
		return nil, err
	}
	agents, err := h.Repo.Agents.GetByIDs(r.Context(), agentIDs)
	if err != nil {
		return nil, err
	}
	out := make([]conversationView, 0, len(convs))
	for _, c := range convs {
		v := visitors[c.VisitorID]
		var a *models.Agent
		if c.AssignedAgentID != nil {
			a = agents[*c.AssignedAgentID]
		}
		out = append(out, conversationView{Conversation: c, Visitor: v, Agent: a})
	}
	return out, nil
}

// ListConversations returns the inbox page.
func (h *Handlers) ListConversations(w http.ResponseWriter, r *http.Request) error {
	claims := claimsOf(r)
	projectID := h.scopeProject(r)
	limit, offset := parsePage(r)
	q := r.URL.Query()
	from, err := parseTimeQuery(r, "from")
	if err != nil {
		return err
	}
	to, err := parseTimeQuery(r, "to")
	if err != nil {
		return err
	}
	f := repo.ConvFilter{
		Status:     q.Get("status"),
		AgentID:    q.Get("agent"),
		Channel:    q.Get("channel"),
		Query:      q.Get("q"),
		From:       from,
		To:         to,
		Limit:      limit,
		Offset:     offset,
		HideClosed: q.Get("status") == "" || q.Get("status") == "open",
	}
	if f.AgentID == "me" {
		f.AgentID = claims.Subject
	}
	if claims.Role == models.RoleAgent {
		f.RestrictToAgentID = claims.Subject
	}
	convs, total, err := h.Repo.Conversations.List(r.Context(), projectID, f)
	if err != nil {
		return apierr.Internal(err)
	}
	items, err := h.enrichConversations(r, convs)
	if err != nil {
		return apierr.Internal(err)
	}
	writeJSON(w, http.StatusOK, newList(items, total, limit, offset))
	return nil
}

// GetConversation returns one conversation with its visitor, agent and messages.
func (h *Handlers) GetConversation(w http.ResponseWriter, r *http.Request) error {
	conv, err := h.Repo.Conversations.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("گفتگو یافت نشد")
	}
	if err := h.conversationAccess(r, conv); err != nil {
		return err
	}
	items, err := h.enrichConversations(r, []*models.Conversation{conv})
	if err != nil {
		return apierr.Internal(err)
	}
	msgs, err := h.Msgs.Latest(r.Context(), conv.ID, 100)
	if err != nil {
		return apierr.Internal(err)
	}
	var ticket *models.Ticket
	if t, terr := h.Repo.Tickets.GetByConversation(r.Context(), conv.ID); terr == nil {
		ticket = t
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"conversation": items[0],
		"messages":     msgs,
		"ticket":       ticket,
	})
	return nil
}

// AssignConversation assigns an agent to the conversation.
func (h *Handlers) AssignConversation(w http.ResponseWriter, r *http.Request) error {
	conv, err := h.Repo.Conversations.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("گفتگو یافت نشد")
	}
	if err := h.conversationAccess(r, conv); err != nil {
		return err
	}
	var in struct {
		AgentID string `json:"agent_id"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	if strings.TrimSpace(in.AgentID) == "" {
		return apierr.BadRequest("شناسه کارشناس لازم است")
	}
	claims := claimsOf(r)
	if err := h.Convs.Assign(r.Context(), conv.ID, in.AgentID, claims.Subject); err != nil {
		return err
	}
	return okMsg(w)
}

// ClaimConversation takes a pending conversation for the calling agent and
// activates it. This is the self-service path; AssignConversation remains for
// managers handing work to somebody else.
func (h *Handlers) ClaimConversation(w http.ResponseWriter, r *http.Request) error {
	conv, err := h.Repo.Conversations.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("گفتگو یافت نشد")
	}
	if err := h.conversationAccess(r, conv); err != nil {
		return err
	}
	claims := claimsOf(r)
	if err := h.Convs.Claim(r.Context(), conv.ID, claims.Subject); err != nil {
		return err
	}
	return okMsg(w)
}

// TransferConversation transfers the conversation with an optional note.
func (h *Handlers) TransferConversation(w http.ResponseWriter, r *http.Request) error {
	conv, err := h.Repo.Conversations.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("گفتگو یافت نشد")
	}
	if err := h.conversationAccess(r, conv); err != nil {
		return err
	}
	var in struct {
		AgentID string `json:"agent_id"`
		Note    string `json:"note"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	if in.AgentID == "" {
		return apierr.BadRequest("شناسه کارشناس مقصد لازم است")
	}
	if err := h.Convs.Transfer(r.Context(), conv.ID, in.AgentID, in.Note); err != nil {
		return err
	}
	return okMsg(w)
}

// ResolveConversation marks the conversation resolved.
func (h *Handlers) ResolveConversation(w http.ResponseWriter, r *http.Request) error {
	conv, err := h.Repo.Conversations.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("گفتگو یافت نشد")
	}
	if err := h.conversationAccess(r, conv); err != nil {
		return err
	}
	if err := h.Convs.Resolve(r.Context(), conv.ID, claimsOf(r).Subject); err != nil {
		return err
	}
	return okMsg(w)
}

// ReopenConversation reopens a resolved conversation.
func (h *Handlers) ReopenConversation(w http.ResponseWriter, r *http.Request) error {
	conv, err := h.Repo.Conversations.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("گفتگو یافت نشد")
	}
	if err := h.conversationAccess(r, conv); err != nil {
		return err
	}
	if err := h.Convs.Reopen(r.Context(), conv.ID); err != nil {
		return err
	}
	return okMsg(w)
}

// ConvertToTicket creates a ticket from the conversation.
func (h *Handlers) ConvertToTicket(w http.ResponseWriter, r *http.Request) error {
	conv, err := h.Repo.Conversations.GetByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		return apierr.NotFound("گفتگو یافت نشد")
	}
	if err := h.conversationAccess(r, conv); err != nil {
		return err
	}
	var in struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Priority    string   `json:"priority"`
		Tags        []string `json:"tags"`
		DueAt       string   `json:"due_at"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	var due *time.Time
	if in.DueAt != "" {
		t, err := time.Parse(time.RFC3339, in.DueAt)
		if err != nil {
			return apierr.BadRequest("تاریخ سررسید نامعتبر است")
		}
		due = &t
	}
	ticket, err := h.Convs.ConvertToTicket(r.Context(), conv.ID, services.TicketInput{
		Title:       in.Title,
		Description: in.Description,
		Priority:    models.TicketPriority(in.Priority),
		Tags:        in.Tags,
		DueAt:       due,
		AgentID:     claimsOf(r).Subject,
	})
	if err != nil {
		return err
	}
	writeJSON(w, http.StatusCreated, ticket)
	return nil
}

// ExportConversations exports conversations as JSON or CSV.
func (h *Handlers) ExportConversations(w http.ResponseWriter, r *http.Request) error {
	projectID := h.scopeProject(r)
	from, err := parseTimeQuery(r, "from")
	if err != nil {
		return err
	}
	to, err := parseTimeQuery(r, "to")
	if err != nil {
		return err
	}
	convs, _, err := h.Repo.Conversations.List(r.Context(), projectID, repo.ConvFilter{
		From: from, To: to, Limit: 100, Offset: 0, Status: "all",
	})
	if err != nil {
		return apierr.Internal(err)
	}
	items, err := h.enrichConversations(r, convs)
	if err != nil {
		return apierr.Internal(err)
	}
	if r.URL.Query().Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="conversations.csv"`)
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"id", "status", "channel", "visitor", "agent", "created_at", "last_message_at", "subject"})
		for _, c := range items {
			visitorName, agentName := "", ""
			if c.Visitor != nil {
				visitorName = c.Visitor.DisplayName()
			}
			if c.Agent != nil {
				agentName = c.Agent.Name
			}
			last := ""
			if c.LastMessageAt != nil {
				last = c.LastMessageAt.Format(time.RFC3339)
			}
			_ = cw.Write([]string{c.ID, string(c.Status), c.Channel, visitorName, agentName,
				c.CreatedAt.Format(time.RFC3339), last, strDeref(c.Subject)})
		}
		cw.Flush()
		return cw.Error()
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="conversations-%s.json"`, time.Now().Format("20060102")))
	writeJSON(w, http.StatusOK, map[string]any{"data": items, "exported_at": time.Now().UTC()})
	return nil
}

func strDeref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
