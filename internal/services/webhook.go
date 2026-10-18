package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/crypto"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

// WebhookService delivers signed webhooks with a retry schedule.
type WebhookService struct {
	repo   *repo.Repo
	log    zerolog.Logger
	client *http.Client
	queue  chan *models.WebhookDelivery
	wg     sync.WaitGroup
}

// retrySchedule is the delay before each retry attempt.
var retrySchedule = []time.Duration{0, 30 * time.Second, 2 * time.Minute, 10 * time.Minute, time.Hour, 6 * time.Hour}

// NewWebhookService builds the fan-out worker.
func NewWebhookService(r *repo.Repo, log zerolog.Logger) *WebhookService {
	return &WebhookService{
		repo: r,
		log:  log,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		queue: make(chan *models.WebhookDelivery, 512),
	}
}

// Start launches delivery workers and the retry scanner.
func (s *WebhookService) Start(ctx context.Context) {
	for i := 0; i < 4; i++ {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case d := <-s.queue:
					s.deliver(context.Background(), d)
				}
			}
		}()
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.retryPending(ctx)
			}
		}
	}()
}

// Stop waits for in-flight deliveries.
func (s *WebhookService) Stop() { s.wg.Wait() }

// Enqueue creates deliveries for every matching active webhook.
func (s *WebhookService) Enqueue(ctx context.Context, projectID, event string, payload map[string]any) {
	hooks, err := s.repo.Webhooks.List(ctx, projectID, true)
	if err != nil || len(hooks) == 0 {
		return
	}
	body, err := json.Marshal(map[string]any{
		"event":     event,
		"timestamp": time.Now().Unix(),
		"data":      payload,
	})
	if err != nil {
		return
	}
	for _, hook := range hooks {
		if !hook.Events.Contains(event) && !hook.Events.Contains("*") {
			continue
		}
		d := &models.WebhookDelivery{
			ID:        id.NewPrefixed(id.PrefixDelivery),
			WebhookID: hook.ID,
			Event:     event,
			Payload:   string(body),
		}
		if err := s.repo.Webhooks.CreateDelivery(ctx, d); err != nil {
			s.log.Warn().Err(err).Str("webhook_id", hook.ID).Msg("creating webhook delivery failed")
			continue
		}
		s.push(d)
	}
}

func (s *WebhookService) push(d *models.WebhookDelivery) {
	select {
	case s.queue <- d:
	default:
		s.log.Warn().Str("delivery_id", d.ID).Msg("webhook queue full")
	}
}

// Test sends a sample delivery synchronously.
func (s *WebhookService) Test(ctx context.Context, hook *models.Webhook) error {
	body, _ := json.Marshal(map[string]any{
		"event":     "webhook.test",
		"timestamp": time.Now().Unix(),
		"data":      map[string]any{"message": "این یک ارسال آزمایشی است"},
	})
	status, resp, err := s.post(ctx, hook, body)
	out := &models.WebhookDelivery{
		ID:        id.NewPrefixed(id.PrefixDelivery),
		WebhookID: hook.ID,
		Event:     "webhook.test",
		Payload:   string(body),
		Attempt:   1,
	}
	if status > 0 {
		out.StatusCode = &status
	}
	if resp != "" {
		out.ResponseBody = &resp
	}
	if err != nil {
		msg := err.Error()
		out.Error = &msg
	}
	if err := s.repo.Webhooks.CreateDelivery(ctx, out); err != nil {
		return err
	}
	return err
}

func (s *WebhookService) retryPending(ctx context.Context) {
	due, err := s.repo.Webhooks.PendingDeliveries(ctx, time.Now().UTC(), 50)
	if err != nil {
		return
	}
	for _, d := range due {
		s.push(d)
	}
}

func (s *WebhookService) deliver(ctx context.Context, d *models.WebhookDelivery) {
	hook, err := s.repo.Webhooks.GetByID(ctx, d.WebhookID)
	if err != nil {
		return
	}
	if !hook.IsActive {
		return
	}
	status, resp, err := s.post(ctx, hook, []byte(d.Payload))
	attempt := d.Attempt + 1

	var (
		statusPtr *int
		respPtr   *string
		errPtr    *string
		next      *time.Time
	)
	if status > 0 {
		statusPtr = &status
	}
	if resp != "" {
		respPtr = &resp
	}
	if err != nil {
		msg := err.Error()
		if len(msg) > 500 {
			msg = msg[:500]
		}
		errPtr = &msg
		if attempt < len(retrySchedule) {
			t := time.Now().UTC().Add(retrySchedule[attempt])
			next = &t
		}
	}
	success := err == nil && status >= 200 && status < 300
	if storeErr := s.repo.Webhooks.UpdateDelivery(ctx, d.ID, statusPtr, respPtr, errPtr, attempt, next, success); storeErr != nil {
		s.log.Warn().Err(storeErr).Str("delivery_id", d.ID).Msg("updating delivery failed")
	}
	if !success {
		s.log.Warn().Str("webhook", hook.URL).Int("attempt", attempt).Int("status", status).Msg("webhook delivery failed")
	}
}

func (s *WebhookService) post(ctx context.Context, hook *models.Webhook, body []byte) (int, string, error) {
	ts := time.Now().Unix()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hook.URL, strings.NewReader(string(body)))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "EveChat-Webhook/1.0")
	req.Header.Set("X-EveChat-Timestamp", fmt.Sprint(ts))
	req.Header.Set("X-EveChat-Signature", crypto.WebhookSignature(hook.Secret, ts, body))
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	buf := make([]byte, 1024)
	n, _ := resp.Body.Read(buf)
	return resp.StatusCode, string(buf[:n]), nil
}

// WebhookChannel adapts the webhook service to the notification interface.
type WebhookChannel struct {
	svc *WebhookService
}

// NewWebhookChannel wraps the webhook service.
func NewWebhookChannel(svc *WebhookService) *WebhookChannel { return &WebhookChannel{svc: svc} }

// Name implements Channel.
func (c *WebhookChannel) Name() string { return "webhook" }

// Notify fans a notification event out to project webhooks.
func (c *WebhookChannel) Notify(ctx context.Context, ev Event) error {
	c.svc.Enqueue(ctx, ev.ProjectID, ev.Name, ev.Data)
	return nil
}
