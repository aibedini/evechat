package services

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/hub"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
	"github.com/evechat/evechat/internal/security"
)

// Realtime implements hub.MessageHandler: it is the bridge between websocket
// frames and the service layer.
type Realtime struct {
	convThrottle *security.Throttle
	msgThrottle  *security.Throttle
	convLimit    int
	msgLimit     int
	repo         *repo.Repo
	hub          *hub.Hub
	convs        *ConversationService
	msgs         *MessageService
	ai           *AIService
	log          zerolog.Logger
}

// NewRealtime builds the handler.
func NewRealtime(r *repo.Repo, h *hub.Hub, convs *ConversationService, msgs *MessageService, log zerolog.Logger) *Realtime {
	return &Realtime{repo: r, hub: h, convs: convs, msgs: msgs, log: log}
}

// SetThrottles installs the per-visitor / per-conversation abuse limits.
func (r *Realtime) SetThrottles(conv, msg *security.Throttle, convLimit, msgLimit int) {
	r.convThrottle = conv
	r.msgThrottle = msg
	r.convLimit = convLimit
	r.msgLimit = msgLimit
}

// SetAI enables the Phase 2 assistant hook.
func (r *Realtime) SetAI(ai *AIService) { r.ai = ai }

// SyncResult is the payload of a sync.response.
type SyncResult struct {
	Conversation *models.Conversation `json:"conversation,omitempty"`
	Messages     []*models.Message    `json:"messages,omitempty"`
	Events       []hub.Envelope       `json:"events,omitempty"`
	Now          int64                `json:"now"`
}

func (r *Realtime) sendError(c *hub.Client, code, msg string) {
	c.Send(hub.ErrorEnvelope(code, msg))
}

func decode(e hub.Envelope, dst any) bool {
	if len(e.Data) == 0 {
		return true
	}
	return json.Unmarshal(e.Data, dst) == nil
}

// OnVisitorConnect prepares a visitor socket: presence and room membership.
func (r *Realtime) OnVisitorConnect(ctx context.Context, c *hub.Client) error {
	v, err := r.repo.Visitors.GetByID(ctx, c.ID)
	if err != nil {
		return err
	}
	_ = r.repo.Visitors.UpdateLastSeen(ctx, v.ID)
	if conv, err := r.repo.Conversations.OpenForVisitor(ctx, v.ID); err == nil {
		r.hub.JoinRoom(conv.ID, c)
	}
	r.hub.ToProjectAgents(v.ProjectID, hub.NewEnvelope(hub.TypeVisitorOnline, map[string]any{
		"visitor_id": v.ID,
		"name":       v.DisplayName(),
	}))
	return nil
}

// OnVisitorDisconnect marks the visitor away from the project agents.
func (r *Realtime) OnVisitorDisconnect(ctx context.Context, c *hub.Client) {
	_ = r.repo.Visitors.UpdateLastSeen(ctx, c.ID)
	r.hub.ToProjectAgents(c.ProjectID, hub.NewEnvelope(hub.TypeVisitorOffline, map[string]any{
		"visitor_id": c.ID,
	}))
}

// OnAgentConnect marks the agent online and joins their open rooms.
func (r *Realtime) OnAgentConnect(ctx context.Context, c *hub.Client) error {
	agent, err := r.repo.Agents.GetByID(ctx, c.ID)
	if err != nil {
		return err
	}
	if !agent.IsActive {
		return errors.New("agent inactive")
	}
	_ = r.repo.Agents.SetOnline(ctx, agent.ID, true)
	convs, err := r.repo.Conversations.OpenForAgent(ctx, agent.ID)
	if err == nil {
		for _, conv := range convs {
			r.hub.JoinRoom(conv.ID, c)
		}
	}
	r.hub.ToProjectAgents(agent.ProjectID, hub.NewEnvelope(hub.TypePresenceUpdate, map[string]any{
		"agent_id": agent.ID,
		"name":     agent.Name,
		"online":   true,
	}))
	return nil
}

// OnAgentDisconnect marks the agent offline.
func (r *Realtime) OnAgentDisconnect(ctx context.Context, c *hub.Client) {
	_ = r.repo.Agents.SetOnline(ctx, c.ID, false)
	r.hub.ToProjectAgents(c.ProjectID, hub.NewEnvelope(hub.TypePresenceUpdate, map[string]any{
		"agent_id": c.ID,
		"online":   false,
	}))
}

// OnVisitorMessage handles visitor-originated frames.
func (r *Realtime) OnVisitorMessage(ctx context.Context, c *hub.Client, e hub.Envelope) error {
	switch e.Type {
	case hub.TypeConversationStart:
		var d struct {
			Message   string   `json:"message"`
			Subject   string   `json:"subject"`
			Channel   string   `json:"channel"`
			Page      PageInfo `json:"page"`
			ClientMsg string   `json:"client_message_id"`
		}
		if !decode(e, &d) {
			r.sendError(c, "invalid_request", "داده‌های گفتگو نامعتبر است")
			return nil
		}
		if r.convThrottle != nil && r.convLimit > 0 && r.convThrottle.Hit(c.ID) > r.convLimit {
			r.sendError(c, "rate_limited", "تعداد گفتگوهای شما بیش از حد مجاز است")
			return nil
		}
		clientMsgID := d.ClientMsg
		if clientMsgID == "" {
			// The envelope id is the widget's client_message_id for ack correlation.
			clientMsgID = e.ID
		}
		v, err := r.repo.Visitors.GetByID(ctx, c.ID)
		if err != nil {
			r.sendError(c, "not_found", "کاربر یافت نشد")
			return nil
		}
		conv, msg, err := r.convs.Start(ctx, nil, v, StartInput{
			Message:         d.Message,
			Subject:         d.Subject,
			Channel:         d.Channel,
			Page:            d.Page,
			ClientMessageID: clientMsgID,
			SenderName:      v.DisplayName(),
		})
		if err != nil {
			r.fail(c, err)
			return nil
		}
		if clientMsgID != "" && msg != nil {
			ack := hub.NewEnvelope(hub.TypeMessageSent, map[string]any{
				"id":              msg.ID,
				"client_id":       clientMsgID,
				"created_at":      msg.CreatedAt.UnixMilli(),
				"conversation_id": conv.ID,
			})
			ack.ID = clientMsgID
			c.Send(ack)
		}
		return nil

	case hub.TypeMessageSend:
		var d struct {
			ConversationID  string `json:"conversation_id"`
			Content         string `json:"content"`
			ContentType     string `json:"content_type"`
			ClientMessageID string `json:"client_message_id"`
		}
		if !decode(e, &d) {
			r.sendError(c, "invalid_request", "داده پیام نامعتبر است")
			return nil
		}
		convID := d.ConversationID
		if convID == "" {
			conv, err := r.repo.Conversations.OpenForVisitor(ctx, c.ID)
			if err != nil {
				// No open conversation yet: start one with this message.
				v, verr := r.repo.Visitors.GetByID(ctx, c.ID)
				if verr != nil {
					r.sendError(c, "not_found", "کاربر یافت نشد")
					return nil
				}
				conv, msg, serr := r.convs.Start(ctx, nil, v, StartInput{
					Message:         d.Content,
					ClientMessageID: d.ClientMessageID,
					SenderName:      v.DisplayName(),
				})
				if serr != nil {
					r.fail(c, serr)
					return nil
				}
				if d.ClientMessageID != "" && msg != nil {
					ack := hub.NewEnvelope(hub.TypeMessageSent, map[string]any{
						"id": msg.ID, "client_id": d.ClientMessageID,
						"created_at": msg.CreatedAt.UnixMilli(), "conversation_id": conv.ID,
					})
					ack.ID = d.ClientMessageID
					c.Send(ack)
				}
				return nil
			}
			convID = conv.ID
		}
		conv, err := r.repo.Conversations.GetByID(ctx, convID)
		if err != nil || conv.VisitorID != c.ID {
			r.sendError(c, "forbidden", "شما به این گفتگو دسترسی ندارید")
			return nil
		}
		msg, duplicate, err := r.msgs.Send(ctx, SendInput{
			ConversationID:  convID,
			SenderType:      models.SenderVisitor,
			SenderID:        c.ID,
			Content:         d.Content,
			ContentType:     d.ContentType,
			ClientMessageID: d.ClientMessageID,
			SenderConnID:    c.ConnID,
		})
		if err != nil {
			r.fail(c, err)
			return nil
		}
		if duplicate {
			c.Send(hub.NewEnvelope(hub.TypeMessageSent, map[string]any{
				"id": msg.ID, "client_id": d.ClientMessageID,
				"created_at": msg.CreatedAt.UnixMilli(), "duplicate": true,
			}))
		}
		r.maybeAI(ctx, conv, msg)
		return nil

	case hub.TypeTypingStart, hub.TypeTypingStop:
		var d struct {
			ConversationID string `json:"conversation_id"`
		}
		_ = decode(e, &d)
		if d.ConversationID == "" {
			return nil
		}
		typ := hub.TypeVisitorTyping
		if e.Type == hub.TypeTypingStop {
			typ = hub.TypeVisitorTypingStop
		}
		r.hub.ToConversation(d.ConversationID, hub.NewEnvelope(typ, map[string]any{
			"conversation_id": d.ConversationID,
			"visitor_id":      c.ID,
		}), c.ConnID)
		return nil

	case hub.TypeReadAck:
		var d struct {
			ConversationID string `json:"conversation_id"`
		}
		_ = decode(e, &d)
		if d.ConversationID == "" {
			return nil
		}
		_ = r.repo.Messages.MarkVisitorMessagesRead(ctx, d.ConversationID)
		_ = r.repo.Conversations.ClearUnread(ctx, d.ConversationID, "visitor")
		conv, err := r.repo.Conversations.GetByID(ctx, d.ConversationID)
		if err == nil {
			r.hub.ToProjectAgents(conv.ProjectID, hub.NewEnvelope(hub.TypeConversationUpdated, map[string]any{
				"conversation_id": d.ConversationID,
				"read_by_visitor": true,
			}))
		}
		return nil

	case hub.TypeSyncRequest:
		var d struct {
			ConversationID string `json:"conversation_id"`
			Since          int64  `json:"since"`
		}
		_ = decode(e, &d)
		if d.ConversationID == "" {
			return nil
		}
		res, err := r.Sync(ctx, d.ConversationID, d.Since)
		if err != nil {
			r.fail(c, err)
			return nil
		}
		c.Send(hub.NewEnvelope(hub.TypeSyncResponse, res))
		return nil

	case hub.TypeCSATSubmit:
		var d struct {
			ConversationID string `json:"conversation_id"`
			Score          int    `json:"score"`
			Comment        string `json:"comment"`
		}
		if !decode(e, &d) {
			r.sendError(c, "invalid_request", "داده ارزیابی نامعتبر است")
			return nil
		}
		if d.ConversationID == "" {
			if conv, err := r.repo.Conversations.OpenForVisitor(ctx, c.ID); err == nil {
				d.ConversationID = conv.ID
			}
		}
		if d.ConversationID == "" {
			r.sendError(c, "not_found", "گفتگو یافت نشد")
			return nil
		}
		if err := r.convs.SubmitCSAT(ctx, d.ConversationID, d.Score, d.Comment); err != nil {
			r.fail(c, err)
		}
		return nil
	}

	r.sendError(c, "invalid_request", "نوع پیام پشتیبانی نمی‌شود")
	return nil
}

// OnAgentMessage handles dashboard-originated frames.
func (r *Realtime) OnAgentMessage(ctx context.Context, c *hub.Client, e hub.Envelope) error {
	agent, err := r.repo.Agents.GetByID(ctx, c.ID)
	if err != nil {
		r.sendError(c, "unauthorized", "کارشناس یافت نشد")
		return nil
	}

	switch e.Type {
	case hub.TypeMessageSend, hub.TypeNoteAdd:
		var d struct {
			ConversationID  string   `json:"conversation_id"`
			Content         string   `json:"content"`
			ContentType     string   `json:"content_type"`
			ClientMessageID string   `json:"client_message_id"`
			Attachments     []string `json:"attachments"`
			IsInternal      bool     `json:"is_internal"`
		}
		if !decode(e, &d) {
			r.sendError(c, "invalid_request", "داده پیام نامعتبر است")
			return nil
		}
		conv, err := r.repo.Conversations.GetByID(ctx, d.ConversationID)
		if err != nil || conv.ProjectID != agent.ProjectID {
			r.sendError(c, "forbidden", "شما به این گفتگو دسترسی ندارید")
			return nil
		}
		if agent.Role == models.RoleAgent && conv.AssignedAgentID != nil && *conv.AssignedAgentID != agent.ID {
			r.sendError(c, "forbidden", "این گفتگو به کارشناس دیگری واگذار شده است")
			return nil
		}
		isInternal := d.IsInternal || e.Type == hub.TypeNoteAdd
		msg, duplicate, err := r.msgs.Send(ctx, SendInput{
			ConversationID:  conv.ID,
			SenderType:      models.SenderAgent,
			SenderID:        agent.ID,
			SenderName:      agent.Name,
			Content:         d.Content,
			ContentType:     d.ContentType,
			ClientMessageID: d.ClientMessageID,
			Attachments:     d.Attachments,
			IsInternal:      isInternal,
			SenderConnID:    c.ConnID,
		})
		if err != nil {
			r.fail(c, err)
			return nil
		}
		if duplicate {
			c.Send(hub.NewEnvelope(hub.TypeMessageSent, map[string]any{
				"id": msg.ID, "client_id": d.ClientMessageID, "duplicate": true,
			}))
		} else {
			c.Send(hub.NewEnvelope(hub.TypeMessageSent, map[string]any{
				"id": msg.ID, "client_id": d.ClientMessageID,
				"created_at": msg.CreatedAt.UnixMilli(),
			}))
		}
		// First reply on a pending conversation claims it.
		if conv.AssignedAgentID == nil && !isInternal {
			if err := r.convs.Assign(ctx, conv.ID, agent.ID, agent.ID); err != nil {
				r.log.Warn().Err(err).Msg("auto-assign on first reply failed")
			}
		}
		return nil

	case hub.TypeTypingStart, hub.TypeTypingStop:
		var d struct {
			ConversationID string `json:"conversation_id"`
		}
		_ = decode(e, &d)
		if d.ConversationID == "" {
			return nil
		}
		typ := hub.TypeAgentTyping
		if e.Type == hub.TypeTypingStop {
			typ = hub.TypeAgentTypingStop
		}
		r.hub.ToConversation(d.ConversationID, hub.NewEnvelope(typ, map[string]any{
			"conversation_id": d.ConversationID,
			"agent_id":        agent.ID,
			"agent_name":      agent.Name,
		}), c.ConnID)
		return nil

	case hub.TypeConversationAssign:
		var d struct {
			ConversationID string `json:"conversation_id"`
			AgentID        string `json:"agent_id"`
		}
		if !decode(e, &d) || d.ConversationID == "" {
			r.sendError(c, "invalid_request", "داده ارجاع نامعتبر است")
			return nil
		}
		if d.AgentID == "" {
			d.AgentID = agent.ID
		}
		if err := r.convs.Assign(ctx, d.ConversationID, d.AgentID, agent.ID); err != nil {
			r.fail(c, err)
		}
		return nil

	case hub.TypeConversationResolve:
		var d struct {
			ConversationID string `json:"conversation_id"`
		}
		if !decode(e, &d) || d.ConversationID == "" {
			r.sendError(c, "invalid_request", "شناسه گفتگو لازم است")
			return nil
		}
		if err := r.convs.Resolve(ctx, d.ConversationID, agent.ID); err != nil {
			r.fail(c, err)
		}
		return nil

	case hub.TypeConversationTransfer:
		var d struct {
			ConversationID string `json:"conversation_id"`
			AgentID        string `json:"agent_id"`
			Note           string `json:"note"`
		}
		if !decode(e, &d) || d.ConversationID == "" || d.AgentID == "" {
			r.sendError(c, "invalid_request", "مقصد ارجاع لازم است")
			return nil
		}
		if err := r.convs.Transfer(ctx, d.ConversationID, d.AgentID, d.Note); err != nil {
			r.fail(c, err)
		}
		return nil

	case hub.TypeConversationReopen:
		var d struct {
			ConversationID string `json:"conversation_id"`
		}
		if !decode(e, &d) || d.ConversationID == "" {
			r.sendError(c, "invalid_request", "شناسه گفتگو لازم است")
			return nil
		}
		if err := r.convs.Reopen(ctx, d.ConversationID); err != nil {
			r.fail(c, err)
		}
		return nil

	case hub.TypeReadAck:
		var d struct {
			ConversationID string `json:"conversation_id"`
			UpToMessageID  string `json:"up_to_message_id"`
		}
		_ = decode(e, &d)
		if d.ConversationID == "" {
			return nil
		}
		_ = r.repo.Messages.MarkRead(ctx, d.ConversationID, d.UpToMessageID, "agent")
		_ = r.repo.Conversations.ClearUnread(ctx, d.ConversationID, "agent")
		r.hub.ToVisitor(c.ID, hub.NewEnvelope(hub.TypeMessageAck, map[string]any{
			"conversation_id": d.ConversationID,
			"read":            true,
		}))
		return nil

	case hub.TypePresenceSet:
		var d struct {
			Online *bool `json:"online"`
		}
		_ = decode(e, &d)
		online := true
		if d.Online != nil {
			online = *d.Online
		}
		_ = r.repo.Agents.SetOnline(ctx, agent.ID, online)
		r.hub.ToProjectAgents(agent.ProjectID, hub.NewEnvelope(hub.TypePresenceUpdate, map[string]any{
			"agent_id": agent.ID,
			"name":     agent.Name,
			"online":   online,
		}))
		return nil
	}

	r.sendError(c, "invalid_request", "نوع پیام پشتیبانی نمی‌شود")
	return nil
}

func (r *Realtime) maybeAI(ctx context.Context, conv *models.Conversation, msg *models.Message) {
	if r.ai == nil || msg == nil || conv.AssignedAgentID != nil {
		return
	}
	go func() {
		actx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := r.ai.MaybeAnswer(actx, conv, msg); err != nil {
			r.log.Debug().Err(err).Str("conv_id", conv.ID).Msg("ai answer skipped")
		}
	}()
}

// Sync builds the reconciliation payload for a conversation.
func (r *Realtime) Sync(ctx context.Context, convID string, sinceMS int64) (*SyncResult, error) {
	conv, err := r.repo.Conversations.GetByID(ctx, convID)
	if err != nil {
		return nil, err
	}
	res := &SyncResult{Conversation: conv, Now: time.Now().UnixMilli()}
	if events, ok := r.hub.SyncBuffer(convID, sinceMS); ok {
		res.Events = events
		return res, nil
	}
	msgs, err := r.repo.Messages.ListByConversation(ctx, convID, timeFromMS(sinceMS), 200)
	if err != nil {
		return nil, err
	}
	r.msgs.DecryptAll(msgs)
	res.Messages = msgs
	return res, nil
}

func (r *Realtime) fail(c *hub.Client, err error) {
	var ae *apierr.Error
	if errors.As(err, &ae) {
		r.sendError(c, ae.Code, ae.Message)
		return
	}
	r.sendError(c, "internal", i18nUnexpected)
}

const i18nUnexpected = "خطای غیرمنتظره رخ داد"
