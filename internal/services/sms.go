package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/i18n"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/integrations/gmweb"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

// SMSChannel delivers notifications through GMweb.
type SMSChannel struct {
	client *gmweb.Client
	repo   *repo.Repo
	msgs   *MessageService
	log    zerolog.Logger
}

// NewSMSChannel builds the channel.
func NewSMSChannel(c *gmweb.Client, r *repo.Repo, msgs *MessageService, log zerolog.Logger) *SMSChannel {
	return &SMSChannel{client: c, repo: r, msgs: msgs, log: log}
}

// Name implements Channel.
func (c *SMSChannel) Name() string { return "sms" }

// Notify sends the SMS for supported events.
func (c *SMSChannel) Notify(ctx context.Context, ev Event) error {
	if c.client == nil || !c.client.Enabled() {
		return nil
	}
	phone := ev.Str("phone")
	if phone == "" {
		return nil
	}
	var text string
	switch ev.Name {
	case EventChatStarted:
		text = fmt.Sprintf(i18n.SMSChatStarted, ev.Str("visitor_name"))
	case EventTicketCreated:
		text = fmt.Sprintf(i18n.SMSTicketCreated, ev.Str("title"))
	default:
		return nil
	}
	return c.client.SendSMS(ctx, phone, text)
}

// HandleInbound posts an incoming SMS into its conversation.
func (c *SMSChannel) HandleInbound(ctx context.Context, from, text string) error {
	phone := NormalizePhone(from)
	conv, err := c.repo.Conversations.GetByPhone(ctx, phone)
	if err != nil {
		// Try the raw value as stored by the routing rule.
		conv, err = c.repo.Conversations.GetByPhone(ctx, from)
		if err != nil {
			return fmt.Errorf("no conversation routed to %s", MaskPhone(phone))
		}
	}
	_, _, err = c.msgs.Send(ctx, SendInput{
		ConversationID:    conv.ID,
		SenderType:        models.SenderAgent,
		SenderName:        "پشتیبان (پیامک)",
		Content:           strings.TrimSpace(text),
		SkipAuthorization: true,
	})
	return err
}

// TicketService coordinates ticket lifecycle notifications.
type TicketService struct {
	repo  *repo.Repo
	notif *NotificationService
	log   zerolog.Logger
}

// NewTicketService builds the service.
func NewTicketService(r *repo.Repo, n *NotificationService, log zerolog.Logger) *TicketService {
	return &TicketService{repo: r, notif: n, log: log}
}

// Create stores a ticket and notifies the configured channels.
func (s *TicketService) Create(ctx context.Context, t *models.Ticket) error {
	if strings.TrimSpace(t.Title) == "" {
		return errors.New("عنوان تیکت لازم است")
	}
	if err := s.repo.Tickets.Create(ctx, t); err != nil {
		return err
	}
	if s.notif != nil {
		data := map[string]any{
			"title":       t.Title,
			"priority":    string(t.Priority),
			"description": strDeref(t.Description),
		}
		if t.VisitorID != nil {
			if v, err := s.repo.Visitors.GetByID(ctx, *t.VisitorID); err == nil {
				data["visitor_name"] = v.DisplayName()
				data["phone"] = strDeref(v.Phone)
			}
		}
		s.notif.Enqueue(Event{
			Name:           EventTicketCreated,
			ProjectID:      t.ProjectID,
			ConversationID: strDeref(t.ConversationID),
			VisitorID:      strDeref(t.VisitorID),
			TicketID:       t.ID,
			Data:           data,
		})
	}
	return nil
}

// AddNote appends an internal note.
func (s *TicketService) AddNote(ctx context.Context, ticketID, authorID, text string) error {
	note := models.Note{ID: newNoteID(), AuthorID: authorID, Text: text, At: timeNow()}
	return s.repo.Tickets.AddNote(ctx, ticketID, note)
}

func newNoteID() string { return id.NewPrefixed("note") }
