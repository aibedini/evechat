package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/crypto"
	"github.com/evechat/evechat/internal/hub"
	"github.com/evechat/evechat/internal/i18n"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

// MaxMessageRunes is the message length limit.
const MaxMessageRunes = 4000

// MessageService implements the send pipeline.
type MessageService struct {
	repo  *repo.Repo
	hub   *hub.Hub
	enc   *crypto.Service
	notif *NotificationService
	log   zerolog.Logger
}

// NewMessageService builds the service.
func NewMessageService(r *repo.Repo, h *hub.Hub, enc *crypto.Service, n *NotificationService, log zerolog.Logger) *MessageService {
	return &MessageService{repo: r, hub: h, enc: enc, notif: n, log: log}
}

// SendInput is one outbound message.
type SendInput struct {
	ConversationID  string
	SenderType      models.SenderType
	SenderID        string
	SenderName      string
	Content         string
	ContentType     string
	ClientMessageID string
	Attachments     []string
	IsInternal      bool
	// SenderConnID lets the hub avoid echoing the message back to the socket
	// that sent it (other tabs of the same identity still receive it).
	SenderConnID string
	// SkipAuthorization is used by trusted server-side flows (AI answers,
	// inbound Telegram/SMS relays) that already proved the sender.
	SkipAuthorization bool
}

// Send validates, deduplicates, encrypts, persists and broadcasts a message.
// The boolean result reports whether the message was a duplicate.
func (s *MessageService) Send(ctx context.Context, in SendInput) (*models.Message, bool, error) {
	content := strings.TrimSpace(in.Content)
	if content == "" {
		return nil, false, apierr.BadRequest("متن پیام خالی است")
	}
	if utf8.RuneCountInString(content) > MaxMessageRunes {
		return nil, false, apierr.BadRequest("متن پیام بیش از حد طولانی است")
	}
	contentType := in.ContentType
	if contentType == "" {
		contentType = "text"
	}

	conv, err := s.repo.Conversations.GetByID(ctx, in.ConversationID)
	if err != nil {
		return nil, false, apierr.NotFound("گفتگو یافت نشد")
	}
	if conv.Status == models.StatusClosed {
		return nil, false, apierr.BadRequest("این گفتگو بسته شده است")
	}

	if !in.SkipAuthorization {
		switch in.SenderType {
		case models.SenderVisitor:
			if conv.VisitorID != in.SenderID {
				return nil, false, apierr.Forbidden("شما به این گفتگو دسترسی ندارید")
			}
			banned, err := s.repo.Visitors.IsBanned(ctx, conv.VisitorID)
			if err != nil {
				return nil, false, fmt.Errorf("checking ban: %w", err)
			}
			if banned {
				return nil, false, apierr.Forbidden("ارسال پیام برای شما مسدود شده است")
			}
		case models.SenderAgent:
			agent, err := s.repo.Agents.GetByID(ctx, in.SenderID)
			if err != nil || agent.ProjectID != conv.ProjectID {
				return nil, false, apierr.Forbidden("شما به این گفتگو دسترسی ندارید")
			}
			if !agent.IsActive {
				return nil, false, apierr.Forbidden("حساب شما غیرفعال است")
			}
		}
	}

	if in.ClientMessageID != "" {
		existing, err := s.repo.Messages.GetByClientID(ctx, conv.ID, in.ClientMessageID)
		if err == nil {
			_ = s.decryptOne(existing)
			return existing, true, nil
		}
		if !errors.Is(err, repo.ErrNotFound) {
			return nil, false, fmt.Errorf("checking duplicate: %w", err)
		}
	}

	ct, iv, ver, err := s.enc.Encrypt(content)
	if err != nil {
		return nil, false, fmt.Errorf("encrypting message: %w", err)
	}
	attachments := models.StringList(in.Attachments)
	msg := &models.Message{
		ID:               id.NewPrefixed(id.PrefixMessage),
		ConversationID:   conv.ID,
		SenderType:       in.SenderType,
		ContentType:      contentType,
		ContentEncrypted: ct,
		ContentIV:        iv,
		KeyVersion:       ver,
		Attachments:      attachments,
		IsInternal:       in.IsInternal,
	}
	if in.ClientMessageID != "" {
		cid := in.ClientMessageID
		msg.ClientMessageID = &cid
	}
	if in.SenderID != "" {
		sid := in.SenderID
		msg.SenderID = &sid
	}
	if name := strings.TrimSpace(in.SenderName); name != "" {
		msg.SenderName = &name
	}

	if err := s.repo.Messages.Create(ctx, msg); err != nil {
		if errors.Is(err, repo.ErrDuplicate) && in.ClientMessageID != "" {
			existing, gerr := s.repo.Messages.GetByClientID(ctx, conv.ID, in.ClientMessageID)
			if gerr == nil {
				_ = s.decryptOne(existing)
				return existing, true, nil
			}
		}
		return nil, false, fmt.Errorf("persisting message: %w", err)
	}
	msg.Content = content

	now := msg.CreatedAt
	if now.IsZero() {
		now = timeNow()
	}
	_ = s.repo.Conversations.TouchLastMessage(ctx, conv.ID, now)
	if in.SenderType == models.SenderAgent {
		_ = s.repo.Conversations.SetFirstResponse(ctx, conv.ID, now)
		_ = s.repo.Conversations.ClearUnread(ctx, conv.ID, "agent")
		// A human reply is the implicit claim: the first one activates a pending
		// conversation even when nobody assigned it explicitly.
		if !in.IsInternal {
			if activated, aerr := s.repo.Conversations.ActivateIfPending(ctx, conv.ID); aerr == nil && activated {
				conv.Status = models.StatusActive
				s.hub.ToProjectAgents(conv.ProjectID, hub.NewEnvelope(hub.TypeConversationUpdated, map[string]any{
					"conversation_id": conv.ID,
					"status":          models.StatusActive,
					"activated_by":    in.SenderID,
				}))
			}
		}
	} else if in.SenderType == models.SenderVisitor {
		_ = s.repo.Conversations.ClearUnread(ctx, conv.ID, "visitor")
	}

	if in.IsInternal {
		// Internal notes never reach visitor sockets.
		s.hub.Record(conv.ID, hub.NewEnvelope(hub.TypeNoteAdded, msg))
		s.hub.ToProjectAgents(conv.ProjectID, hub.NewEnvelope(hub.TypeNoteAdded, map[string]any{
			"conversation_id": conv.ID,
			"message":         msg,
		}))
		return msg, false, nil
	}

	if in.SenderType == models.SenderVisitor {
		_ = s.repo.Conversations.IncrementUnread(ctx, conv.ID, "agent")
	} else if in.SenderType != models.SenderSystem {
		_ = s.repo.Conversations.IncrementUnread(ctx, conv.ID, "visitor")
	}

	received := hub.NewEnvelope(hub.TypeMessageReceived, msg)
	s.hub.ToConversationAndBuffer(conv.ID, received, in.SenderConnID)

	if in.SenderType == models.SenderVisitor {
		ack := hub.NewEnvelope(hub.TypeMessageSent, map[string]any{
			"id":         msg.ID,
			"client_id":  in.ClientMessageID,
			"created_at": msg.CreatedAt.UnixMilli(),
		})
		ack.ID = in.ClientMessageID
		s.sendToSender(conv.VisitorID, ack)
	}

	s.hub.ToProjectAgents(conv.ProjectID, hub.NewEnvelope(hub.TypeConversationUpdated, map[string]any{
		"conversation_id": conv.ID,
		"status":          conv.Status,
		"last_message_at": msg.CreatedAt,
		"unread_agent":    conv.UnreadAgent,
		"sender_type":     in.SenderType,
	}))

	if s.notif != nil {
		s.notif.Enqueue(Event{
			Name:           EventMessageReceived,
			ProjectID:      conv.ProjectID,
			ConversationID: conv.ID,
			VisitorID:      conv.VisitorID,
			Data: map[string]any{
				"message_id":   msg.ID,
				"content":      content,
				"sender_type":  string(in.SenderType),
				"sender_name":  in.SenderName,
				"is_internal":  false,
				"project_id":   conv.ProjectID,
				"visitor_id":   conv.VisitorID,
				"routed_phone": strDeref(conv.RoutedPhone),
				"telegram_id":  strDeref(conv.RoutedTelegramID),
			},
		})
	}

	return msg, false, nil
}

// CreateQuiet encrypts and inserts a message without broadcasting it. Used by
// conversation.Start so conversation.created can be emitted first.
func (s *MessageService) CreateQuiet(ctx context.Context, conv *models.Conversation, senderType models.SenderType,
	senderID, senderName, content, clientMsgID string, internal bool) (*models.Message, error) {
	ct, iv, ver, err := s.enc.Encrypt(content)
	if err != nil {
		return nil, fmt.Errorf("encrypting message: %w", err)
	}
	msg := &models.Message{
		ID:               id.NewPrefixed(id.PrefixMessage),
		ConversationID:   conv.ID,
		SenderType:       senderType,
		ContentType:      "text",
		ContentEncrypted: ct,
		ContentIV:        iv,
		KeyVersion:       ver,
		IsInternal:       internal,
		Content:          content,
	}
	if clientMsgID != "" {
		cid := clientMsgID
		msg.ClientMessageID = &cid
	}
	if senderID != "" {
		sid := senderID
		msg.SenderID = &sid
	}
	if senderName != "" {
		n := senderName
		msg.SenderName = &n
	}
	if err := s.repo.Messages.Create(ctx, msg); err != nil {
		return nil, err
	}
	_ = s.repo.Conversations.TouchLastMessage(ctx, conv.ID, msg.CreatedAt)
	return msg, nil
}

// sendToSender is kept for explicit acks targeted at one identity.
func (s *MessageService) sendToSender(visitorID string, e hub.Envelope) {
	s.hub.ToVisitor(visitorID, e)
}

func timeNow() time.Time { return time.Now().UTC() }

func timeFromMS(ms int64) time.Time {
	if ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
}

// SystemMessage inserts a system notice without authorization checks.
func (s *MessageService) SystemMessage(ctx context.Context, convID, content string) (*models.Message, error) {
	msg, _, err := s.Send(ctx, SendInput{
		ConversationID:    convID,
		SenderType:        models.SenderSystem,
		SenderName:        "سیستم",
		Content:           content,
		SkipAuthorization: true,
	})
	return msg, err
}

// BotMessage inserts an automated agent-side answer.
func (s *MessageService) BotMessage(ctx context.Context, convID, content string) (*models.Message, error) {
	msg, _, err := s.Send(ctx, SendInput{
		ConversationID:    convID,
		SenderType:        models.SenderBot,
		SenderName:        i18n.AITookOver,
		Content:           content,
		SkipAuthorization: true,
	})
	return msg, err
}

// Decrypt fills Content for a single message.
func (s *MessageService) Decrypt(m *models.Message) error {
	if m == nil {
		return nil
	}
	plain, err := s.enc.Decrypt(m.ContentEncrypted, m.ContentIV, m.KeyVersion)
	if err != nil {
		return err
	}
	m.Content = plain
	return nil
}

func (s *MessageService) decryptOne(m *models.Message) error { return s.Decrypt(m) }

// DecryptAll decrypts a slice in place.
func (s *MessageService) DecryptAll(msgs []*models.Message) {
	for _, m := range msgs {
		if err := s.Decrypt(m); err != nil {
			s.log.Error().Err(err).Str("message_id", m.ID).Msg("decrypt failed")
			m.Content = ""
		}
	}
}

// Latest decrypts the last n messages of a conversation.
func (s *MessageService) Latest(ctx context.Context, convID string, n int) ([]*models.Message, error) {
	msgs, err := s.repo.Messages.LatestN(ctx, convID, n)
	if err != nil {
		return nil, err
	}
	s.DecryptAll(msgs)
	return msgs, nil
}

// List decrypts a page of messages.
func (s *MessageService) List(ctx context.Context, convID string, sinceTS int64, limit int) ([]*models.Message, error) {
	var since = timeFromMS(sinceTS)
	msgs, err := s.repo.Messages.ListByConversation(ctx, convID, since, limit)
	if err != nil {
		return nil, err
	}
	s.DecryptAll(msgs)
	return msgs, nil
}

func strDeref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
