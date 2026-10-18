package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/hub"
	"github.com/evechat/evechat/internal/i18n"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

// ConversationService owns the conversation lifecycle.
type ConversationService struct {
	repo    *repo.Repo
	hub     *hub.Hub
	routing *RoutingService
	msgs    *MessageService
	notif   *NotificationService
	csat    *CSATScheduler
	log     zerolog.Logger
}

// NewConversationService builds the service.
func NewConversationService(r *repo.Repo, h *hub.Hub, routing *RoutingService, msgs *MessageService,
	n *NotificationService, csat *CSATScheduler, log zerolog.Logger) *ConversationService {
	return &ConversationService{repo: r, hub: h, routing: routing, msgs: msgs, notif: n, csat: csat, log: log}
}

// StartInput is the payload of a conversation.start frame.
type StartInput struct {
	Message         string
	Subject         string
	Channel         string
	Page            PageInfo
	ClientMessageID string
	SenderName      string
}

// ConversationPayload is the REST/WS representation of a thread.
type ConversationPayload struct {
	*models.Conversation
	Visitor *models.Visitor `json:"visitor,omitempty"`
	Message *models.Message `json:"message,omitempty"`
}

// Start opens a conversation, routes it and stores the first message.
func (s *ConversationService) Start(ctx context.Context, sess *models.VisitorSession, v *models.Visitor, in StartInput) (*models.Conversation, *models.Message, error) {
	channel := in.Channel
	if channel == "" {
		channel = "web"
	}
	conv := &models.Conversation{
		ID:        id.NewPrefixed(id.PrefixConversation),
		ProjectID: v.ProjectID,
		VisitorID: v.ID,
		Status:    models.StatusPending,
		Channel:   channel,
		PageURL:   nilIfEmpty(in.Page.URL),
		PageTitle: nilIfEmpty(in.Page.Title),
		Referrer:  nilIfEmpty(in.Page.Referrer),
		Subject:   nilIfEmpty(in.Subject),
	}

	decision, err := s.routing.Decide(ctx, conv, v)
	if err != nil {
		s.log.Warn().Err(err).Msg("routing decision failed, conversation stays pending")
		decision = Decision{Fallback: true}
	}
	if decision.AgentID != "" {
		// Routing (including the capacity fallback) may pre-assign, but only a
		// human claim or the first human reply activates the conversation.
		agentID := decision.AgentID
		conv.AssignedAgentID = &agentID
	}
	if decision.RuleID != "" {
		ruleID := decision.RuleID
		conv.RoutingRuleID = &ruleID
	}
	if decision.TelegramID != "" {
		tg := decision.TelegramID
		conv.RoutedTelegramID = &tg
	}
	if decision.Phone != "" {
		ph := decision.Phone
		conv.RoutedPhone = &ph
	}

	if err := s.repo.Conversations.Create(ctx, conv); err != nil {
		return nil, nil, fmt.Errorf("creating conversation: %w", err)
	}
	_ = s.repo.Conversations.SetRouting(ctx, conv.ID, decision.RuleID, decision.TelegramID, decision.Phone)

	var first *models.Message
	if strings.TrimSpace(in.Message) != "" {
		first, err = s.msgs.CreateQuiet(ctx, conv, models.SenderVisitor, v.ID, in.SenderName,
			strings.TrimSpace(in.Message), in.ClientMessageID, false)
		if err != nil {
			// Compensate: never leave an empty conversation behind.
			_ = s.repo.Conversations.SetStatus(ctx, conv.ID, models.StatusClosed)
			return nil, nil, fmt.Errorf("storing first message: %w", err)
		}
		_ = s.repo.Conversations.IncrementUnread(ctx, conv.ID, "agent")
	}

	// Record the active conversation on the visitor's session. The widget's
	// normal path opens conversations over the WebSocket, where no session token
	// is available, so fall back to updating every session of the visitor.
	if sess != nil {
		_ = s.repo.Sessions.SetActiveConv(ctx, sess.Token, conv.ID)
	} else {
		_ = s.repo.Sessions.SetActiveConvForVisitor(ctx, v.ID, conv.ID)
	}
	s.hub.JoinRoomForVisitor(conv.ID, conv.VisitorID)
	if conv.AssignedAgentID != nil {
		s.hub.JoinRoomForAgents(conv.ID, *conv.AssignedAgentID)
	}
	// The unclaimed queue belongs to every agent: subscribe them all so live
	// messages and typing indicators arrive before anybody assigns the thread.
	s.joinProjectAgents(ctx, conv.ID, conv.ProjectID)

	payload := ConversationPayload{Conversation: conv, Visitor: v, Message: first}
	s.hub.ToVisitor(v.ID, hub.NewEnvelope(hub.TypeConversationCreated, payload))
	s.hub.ToProjectAgents(conv.ProjectID, hub.NewEnvelope(hub.TypeConversationNew, payload))
	if first != nil {
		s.hub.Record(conv.ID, hub.NewEnvelope(hub.TypeMessageReceived, first))
	}
	if conv.AssignedAgentID != nil {
		s.hub.ToVisitor(v.ID, hub.NewEnvelope(hub.TypeAgentAssigned, map[string]any{
			"conversation_id": conv.ID,
			"agent_id":        *conv.AssignedAgentID,
		}))
	}

	if s.notif != nil {
		name := v.DisplayName()
		s.notif.Enqueue(Event{
			Name:           EventChatStarted,
			ProjectID:      conv.ProjectID,
			ConversationID: conv.ID,
			VisitorID:      conv.VisitorID,
			Data: map[string]any{
				"visitor_name": name,
				"device":       strDeref(v.DeviceType),
				"os":           strDeref(v.OS),
				"browser":      strDeref(v.Browser),
				"page_url":     in.Page.URL,
				"page_title":   in.Page.Title,
				"message":      in.Message,
				"telegram_id":  decision.TelegramID,
				"phone":        decision.Phone,
				"agent_id":     decision.AgentID,
				"subscription": v.SubscriptionInfo,
			},
		})
	}

	// Auto reply (rule-defined or the no-agent default) is sent after the
	// conversation is visible to both sides.
	autoReply := decision.AutoReply
	if autoReply == "" && decision.AgentID == "" && decision.TelegramID == "" {
		autoReply = i18n.AutoReplyNoAgent
	}
	if autoReply != "" {
		if _, err := s.msgs.BotMessage(ctx, conv.ID, autoReply); err != nil {
			s.log.Warn().Err(err).Str("conv_id", conv.ID).Msg("auto reply failed")
		}
	}

	return conv, first, nil
}

// joinProjectAgents subscribes every agent socket of a project to a room.
func (s *ConversationService) joinProjectAgents(ctx context.Context, convID, projectID string) {
	agents, err := s.repo.Agents.List(ctx, projectID)
	if err != nil || len(agents) == 0 {
		return
	}
	ids := make([]string, 0, len(agents))
	for _, a := range agents {
		ids = append(ids, a.ID)
	}
	s.hub.JoinRoomForAgents(convID, ids...)
}

// leaveOtherAgents unsubscribes every agent of the project except keepID once a
// conversation has an owner.
func (s *ConversationService) leaveOtherAgents(ctx context.Context, convID, projectID, keepID string) {
	agents, err := s.repo.Agents.List(ctx, projectID)
	if err != nil || len(agents) == 0 {
		return
	}
	ids := make([]string, 0, len(agents))
	for _, a := range agents {
		if a.ID != keepID {
			ids = append(ids, a.ID)
		}
	}
	if len(ids) > 0 {
		s.hub.LeaveRoomForAgents(convID, ids...)
	}
}

// Claim hands a pending conversation to the calling agent and activates it.
// Unlike Assign it is atomic: when two agents claim at the same time only one
// wins, and the loser gets a conflict instead of a silent reassignment.
func (s *ConversationService) Claim(ctx context.Context, convID, agentID string) error {
	conv, err := s.repo.Conversations.GetByID(ctx, convID)
	if err != nil {
		return apierr.NotFound("گفتگو یافت نشد")
	}
	agent, err := s.repo.Agents.GetByID(ctx, agentID)
	if err != nil || agent.ProjectID != conv.ProjectID || !agent.IsActive {
		return apierr.Forbidden("شما به این گفتگو دسترسی ندارید")
	}
	claimed, err := s.repo.Conversations.Claim(ctx, convID, agentID)
	if err != nil {
		return fmt.Errorf("claiming conversation: %w", err)
	}
	if !claimed {
		return apierr.Conflict("این گفتگو قبلاً برداشته شده یا در حال بررسی است")
	}
	s.hub.JoinRoomForAgents(convID, agentID)
	s.leaveOtherAgents(ctx, convID, conv.ProjectID, agentID)
	s.hub.ToConversation(convID, hub.NewEnvelope(hub.TypeAgentAssigned, map[string]any{
		"conversation_id": convID,
		"agent_id":        agentID,
		"agent_name":      agent.Name,
	}), "")
	s.hub.ToProjectAgents(conv.ProjectID, hub.NewEnvelope(hub.TypeConversationUpdated, map[string]any{
		"conversation_id":   convID,
		"assigned_agent_id": agentID,
		"status":            models.StatusActive,
	}))
	if s.notif != nil {
		s.notif.Enqueue(Event{
			Name:           EventChatAssigned,
			ProjectID:      conv.ProjectID,
			ConversationID: convID,
			VisitorID:      conv.VisitorID,
			Data: map[string]any{
				"agent_id":     agentID,
				"agent_name":   agent.Name,
				"telegram_id":  strDeref(agent.TelegramID),
				"by_agent_id":  agentID,
				"conversation": convID,
				"claimed":      true,
			},
		})
	}
	return nil
}

// Assign assigns an agent and notifies both sides.
func (s *ConversationService) Assign(ctx context.Context, convID, agentID, byAgentID string) error {
	conv, err := s.repo.Conversations.GetByID(ctx, convID)
	if err != nil {
		return apierr.NotFound("گفتگو یافت نشد")
	}
	agent, err := s.repo.Agents.GetByID(ctx, agentID)
	if err != nil || agent.ProjectID != conv.ProjectID {
		return apierr.BadRequest("کارشناس انتخاب‌شده معتبر نیست")
	}
	previous := ""
	if conv.AssignedAgentID != nil {
		previous = *conv.AssignedAgentID
	}
	if err := s.repo.Conversations.Assign(ctx, convID, agentID); err != nil {
		return fmt.Errorf("assigning conversation: %w", err)
	}
	s.hub.JoinRoomForAgents(convID, agentID)
	if previous != "" && previous != agentID {
		s.hub.LeaveRoomForAgents(convID, previous)
	}
	s.leaveOtherAgents(ctx, convID, conv.ProjectID, agentID)
	conv.AssignedAgentID = &agentID
	// Assignment is not activation: conv.Status is deliberately left alone, so
	// the conversation stays pending until the assignee claims or replies.

	s.hub.ToConversation(convID, hub.NewEnvelope(hub.TypeAgentAssigned, map[string]any{
		"conversation_id": convID,
		"agent_id":        agentID,
		"agent_name":      agent.Name,
	}), "")
	s.hub.ToProjectAgents(conv.ProjectID, hub.NewEnvelope(hub.TypeConversationUpdated, map[string]any{
		"conversation_id":   convID,
		"assigned_agent_id": agentID,
		"status":            conv.Status,
	}))
	if s.notif != nil {
		s.notif.Enqueue(Event{
			Name:           EventChatAssigned,
			ProjectID:      conv.ProjectID,
			ConversationID: convID,
			VisitorID:      conv.VisitorID,
			Data: map[string]any{
				"agent_id":     agentID,
				"agent_name":   agent.Name,
				"telegram_id":  strDeref(agent.TelegramID),
				"by_agent_id":  byAgentID,
				"conversation": convID,
			},
		})
	}
	return nil
}

// Transfer reassigns a conversation and leaves an audit trail.
func (s *ConversationService) Transfer(ctx context.Context, convID, toAgentID, note string) error {
	if err := s.Assign(ctx, convID, toAgentID, ""); err != nil {
		return err
	}
	text := i18n.TransferNote
	if strings.TrimSpace(note) != "" {
		text = text + "\n" + strings.TrimSpace(note)
	}
	if _, err := s.msgs.SystemMessage(ctx, convID, text); err != nil {
		s.log.Warn().Err(err).Msg("transfer note failed")
	}
	return nil
}

// Resolve closes a conversation as resolved.
func (s *ConversationService) Resolve(ctx context.Context, convID, byAgentID string) error {
	conv, err := s.repo.Conversations.GetByID(ctx, convID)
	if err != nil {
		return apierr.NotFound("گفتگو یافت نشد")
	}
	if conv.Status == models.StatusResolved || conv.Status == models.StatusClosed {
		return nil
	}
	if err := s.repo.Conversations.SetStatus(ctx, convID, models.StatusResolved); err != nil {
		return fmt.Errorf("resolving conversation: %w", err)
	}
	s.hub.ToConversationAndBuffer(convID, hub.NewEnvelope(hub.TypeConversationResolved, map[string]any{
		"conversation_id": convID,
		"status":          models.StatusResolved,
		"by_agent_id":     byAgentID,
	}), "")
	s.hub.ToProjectAgents(conv.ProjectID, hub.NewEnvelope(hub.TypeConversationUpdated, map[string]any{
		"conversation_id": convID,
		"status":          models.StatusResolved,
	}))
	if s.notif != nil {
		s.notif.Enqueue(Event{
			Name:           EventChatResolved,
			ProjectID:      conv.ProjectID,
			ConversationID: convID,
			VisitorID:      conv.VisitorID,
			Data:           map[string]any{"by_agent_id": byAgentID},
		})
	}
	if s.csat != nil {
		s.csat.Schedule(convID, 2*time.Minute)
	}
	return nil
}

// Reopen reactivates a resolved conversation.
func (s *ConversationService) Reopen(ctx context.Context, convID string) error {
	conv, err := s.repo.Conversations.GetByID(ctx, convID)
	if err != nil {
		return apierr.NotFound("گفتگو یافت نشد")
	}
	if err := s.repo.Conversations.SetStatus(ctx, convID, models.StatusActive); err != nil {
		return fmt.Errorf("reopening conversation: %w", err)
	}
	s.hub.ToConversationAndBuffer(convID, hub.NewEnvelope(hub.TypeConversationUpdated, map[string]any{
		"conversation_id": convID,
		"status":          models.StatusActive,
	}), "")
	s.hub.ToProjectAgents(conv.ProjectID, hub.NewEnvelope(hub.TypeConversationUpdated, map[string]any{
		"conversation_id": convID,
		"status":          models.StatusActive,
	}))
	_, _ = s.msgs.SystemMessage(ctx, convID, i18n.ConversationReopened)
	return nil
}

// TicketInput is the payload of convert-ticket.
type TicketInput struct {
	Title       string
	Description string
	Priority    models.TicketPriority
	Tags        []string
	DueAt       *time.Time
	AgentID     string
}

// ConvertToTicket creates a ticket linked to the conversation.
func (s *ConversationService) ConvertToTicket(ctx context.Context, convID string, in TicketInput) (*models.Ticket, error) {
	conv, err := s.repo.Conversations.GetByID(ctx, convID)
	if err != nil {
		return nil, apierr.NotFound("گفتگو یافت نشد")
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		if conv.Subject != nil && *conv.Subject != "" {
			title = *conv.Subject
		} else {
			title = "تیکت از گفتگو " + convID
		}
	}
	priority := in.Priority
	switch priority {
	case models.PriorityLow, models.PriorityNormal, models.PriorityHigh, models.PriorityUrgent:
	default:
		priority = models.PriorityNormal
	}
	t := &models.Ticket{
		ID:             id.NewPrefixed(id.PrefixTicket),
		ProjectID:      conv.ProjectID,
		ConversationID: &convID,
		Title:          title,
		Description:    nilIfEmpty(in.Description),
		Status:         models.TicketOpen,
		Priority:       priority,
		VisitorID:      &conv.VisitorID,
		Tags:           models.StringList(in.Tags),
		InternalNotes:  models.NoteList{},
		DueAt:          in.DueAt,
	}
	if conv.AssignedAgentID != nil {
		t.AssignedAgentID = conv.AssignedAgentID
	}
	if in.AgentID != "" {
		t.AssignedAgentID = &in.AgentID
	}
	if err := s.repo.Tickets.Create(ctx, t); err != nil {
		return nil, fmt.Errorf("creating ticket: %w", err)
	}
	_ = s.repo.Conversations.SetTicket(ctx, convID, t.ID)
	if s.notif != nil {
		s.notif.Enqueue(Event{
			Name:           EventTicketCreated,
			ProjectID:      t.ProjectID,
			ConversationID: convID,
			VisitorID:      conv.VisitorID,
			TicketID:       t.ID,
			Data: map[string]any{
				"title":       t.Title,
				"priority":    string(t.Priority),
				"description": strDeref(t.Description),
			},
		})
	}
	return t, nil
}

// SubmitCSAT stores a satisfaction score.
func (s *ConversationService) SubmitCSAT(ctx context.Context, convID string, score int, comment string) error {
	if score < 1 || score > 5 {
		return apierr.BadRequest("امتیاز باید بین ۱ تا ۵ باشد")
	}
	conv, err := s.repo.Conversations.GetByID(ctx, convID)
	if err != nil {
		return apierr.NotFound("گفتگو یافت نشد")
	}
	if err := s.repo.Conversations.SetCSAT(ctx, convID, score, strings.TrimSpace(comment)); err != nil {
		return fmt.Errorf("storing csat: %w", err)
	}
	if s.notif != nil {
		s.notif.Enqueue(Event{
			Name:           EventCSATReceived,
			ProjectID:      conv.ProjectID,
			ConversationID: convID,
			VisitorID:      conv.VisitorID,
			Data:           map[string]any{"score": score, "comment": comment},
		})
	}
	if _, err := s.msgs.SystemMessage(ctx, convID, i18n.CSATThanks); err != nil && !errors.Is(err, repo.ErrNotFound) {
		s.log.Debug().Err(err).Msg("csat thanks message failed")
	}
	return nil
}
