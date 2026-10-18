package services

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/hub"
	"github.com/evechat/evechat/internal/i18n"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

// CSATScheduler asks for a satisfaction score shortly after resolution.
// Pending requests survive restarts because the recovery scan re-reads
// conversations that are resolved, old enough and still unscored.
type CSATScheduler struct {
	repo *repo.Repo
	hub  *hub.Hub
	log  zerolog.Logger

	mu     sync.Mutex
	timers map[string]*time.Timer
	stop   chan struct{}
	once   sync.Once
}

// NewCSATScheduler builds the scheduler.
func NewCSATScheduler(r *repo.Repo, h *hub.Hub, log zerolog.Logger) *CSATScheduler {
	return &CSATScheduler{repo: r, hub: h, log: log, timers: make(map[string]*time.Timer), stop: make(chan struct{})}
}

// Start launches the recovery loop.
func (s *CSATScheduler) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		s.recoverPending(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stop:
				return
			case <-ticker.C:
				s.recoverPending(ctx)
			}
		}
	}()
}

// Stop cancels every pending timer.
func (s *CSATScheduler) Stop() {
	s.once.Do(func() {
		close(s.stop)
		s.mu.Lock()
		for _, t := range s.timers {
			t.Stop()
		}
		s.timers = map[string]*time.Timer{}
		s.mu.Unlock()
	})
}

// Schedule emits the CSAT request after d.
func (s *CSATScheduler) Schedule(convID string, d time.Duration) {
	if convID == "" {
		return
	}
	s.mu.Lock()
	if t, ok := s.timers[convID]; ok {
		t.Stop()
	}
	s.timers[convID] = time.AfterFunc(d, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s.fire(ctx, convID)
		s.mu.Lock()
		delete(s.timers, convID)
		s.mu.Unlock()
	})
	s.mu.Unlock()
}

func (s *CSATScheduler) fire(ctx context.Context, convID string) {
	conv, err := s.repo.Conversations.GetByID(ctx, convID)
	if err != nil {
		return
	}
	if conv.CSATScore != nil {
		return
	}
	if conv.Status != models.StatusResolved && conv.Status != models.StatusClosed {
		return
	}
	s.hub.ToVisitor(conv.VisitorID, hub.NewEnvelope(hub.TypeCSATRequest, map[string]any{
		"conversation_id": convID,
		"question":        i18n.CSATRequest,
	}))
}

func (s *CSATScheduler) recoverPending(ctx context.Context) {
	convs, err := s.repo.Conversations.DueForCSAT(ctx, time.Now().UTC().Add(-2*time.Minute), 100)
	if err != nil {
		s.log.Warn().Err(err).Msg("csat recovery scan failed")
		return
	}
	for _, c := range convs {
		s.fire(ctx, c.ID)
	}
}
