package services

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// Notification event names.
const (
	EventChatStarted       = "chat.started"
	EventMessageReceived   = "message.received"
	EventChatAssigned      = "chat.assigned"
	EventChatResolved      = "chat.resolved"
	EventTicketCreated     = "ticket.created"
	EventCSATReceived      = "csat.received"
	EventMessageSent       = "message.sent"
	EventVisitorIdentified = "visitor.identified"
	EventChannelFailed     = "channel.failed"
)

// WebhookEvent describes one subscribable webhook event.
type WebhookEvent struct {
	Name    string `json:"name"`
	LabelFA string `json:"label_fa"`
}

// WebhookEvents is the single source of truth for the event catalogue.
func WebhookEvents() []WebhookEvent {
	return []WebhookEvent{
		{EventChatStarted, "گفتگوی جدید باز شد"},
		{EventMessageReceived, "پیام مشتری رسید"},
		{EventMessageSent, "پاسخ پشتیبان ارسال شد"},
		{EventChatAssigned, "گفتگو به پشتیبان ارجاع شد"},
		{EventChatResolved, "گفتگو بسته شد"},
		{EventTicketCreated, "تیکت ساخته شد"},
		{EventCSATReceived, "نظرسنجی رضایت ثبت شد"},
		{EventVisitorIdentified, "هویت بازدیدکننده مشخص شد"},
		{EventChannelFailed, "ارسال از یک کانال شکست خورد"},
	}
}

// Event is one notification. Data is channel-specific but always contains the
// plain-text content the channel needs.
type Event struct {
	Name           string         `json:"name"`
	ProjectID      string         `json:"project_id"`
	ConversationID string         `json:"conversation_id,omitempty"`
	VisitorID      string         `json:"visitor_id,omitempty"`
	TicketID       string         `json:"ticket_id,omitempty"`
	At             time.Time      `json:"at"`
	Data           map[string]any `json:"data,omitempty"`
}

// Str reads a string field of the event data.
func (e Event) Str(key string) string {
	if e.Data == nil {
		return ""
	}
	if v, ok := e.Data[key].(string); ok {
		return v
	}
	return ""
}

// Channel is one notification transport (Telegram, SMS, webhook, SMTP, ...).
type Channel interface {
	Name() string
	Notify(ctx context.Context, ev Event) error
}

// NotificationService fans events out to channels asynchronously. Enqueue never
// blocks the request path.
type NotificationService struct {
	mu       sync.RWMutex
	channels []Channel
	queue    chan Event
	workers  int
	log      zerolog.Logger
	wg       sync.WaitGroup
	dropped  atomic.Uint64
	started  atomic.Bool
}

// NewNotificationService builds a service with a 1024 event queue.
func NewNotificationService(log zerolog.Logger) *NotificationService {
	return &NotificationService{
		queue:   make(chan Event, 1024),
		workers: 4,
		log:     log,
	}
}

// Register adds a channel. Call before Start.
func (s *NotificationService) Register(ch Channel) {
	if ch == nil {
		return
	}
	s.mu.Lock()
	s.channels = append(s.channels, ch)
	s.mu.Unlock()
}

// Start launches the worker pool.
func (s *NotificationService) Start(ctx context.Context) {
	if !s.started.CompareAndSwap(false, true) {
		return
	}
	for i := 0; i < s.workers; i++ {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			for {
				select {
				case <-ctx.Done():
					// Drain whatever is already queued before exiting.
					for {
						select {
						case ev := <-s.queue:
							s.deliver(ev)
						default:
							return
						}
					}
				case ev := <-s.queue:
					s.deliver(ev)
				}
			}
		}()
	}
}

// Stop waits for the workers to drain.
func (s *NotificationService) Stop() {
	if s.started.Load() {
		s.wg.Wait()
	}
}

func (s *NotificationService) deliver(ev Event) {
	s.mu.RLock()
	channels := append([]Channel(nil), s.channels...)
	s.mu.RUnlock()

	for _, ch := range channels {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := ch.Notify(ctx, ev)
		cancel()
		if err != nil {
			s.log.Warn().Err(err).Str("channel", ch.Name()).Str("event", ev.Name).Msg("notification failed")
		}
	}
}

// Enqueue schedules an event; a full queue drops it with a warning.
func (s *NotificationService) Enqueue(ev Event) {
	if ev.At.IsZero() {
		ev.At = time.Now().UTC()
	}
	select {
	case s.queue <- ev:
	default:
		s.dropped.Add(1)
		s.log.Warn().Str("event", ev.Name).Msg("notification queue full, event dropped")
	}
}

// Dropped returns the number of dropped events.
func (s *NotificationService) Dropped() uint64 { return s.dropped.Load() }
