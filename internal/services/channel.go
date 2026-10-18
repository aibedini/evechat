package services

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/crypto"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
	"github.com/evechat/evechat/internal/security"
)

// ChannelService ingests inbound channel traffic and delivers outbound acks.
type ChannelService struct {
	repo     *repo.Repo
	convs    *ConversationService
	msgs     *MessageService
	log      zerolog.Logger
	client   *http.Client
	throttle *security.Throttle
	perMin   int
	mu       sync.Mutex
}

// NewChannelService builds the service.
func NewChannelService(r *repo.Repo, convs *ConversationService, msgs *MessageService, log zerolog.Logger) *ChannelService {
	return &ChannelService{
		repo: r, convs: convs, msgs: msgs, log: log,
		client: security.Outbound.Client(10 * time.Second),
	}
}

// SetThrottle installs the per-channel/IP inbound limit (requests per minute).
func (s *ChannelService) SetThrottle(t *security.Throttle, perMin int) {
	s.throttle = t
	s.perMin = perMin
}

// IngestInput is one raw inbound request.
type IngestInput struct {
	Channel  *repo.Channel
	Body     []byte
	RemoteIP string
	Method   string
}

// IngestResult reports what the inbound request produced.
type IngestResult struct {
	ConversationID string `json:"conversation_id"`
	MessageID      string `json:"message_id"`
	VisitorID      string `json:"visitor_id"`
	Duplicate      bool   `json:"duplicate,omitempty"`
}

// VerifySecret compares the presented secret in constant time.
func (s *ChannelService) VerifySecret(ch *repo.Channel, presented string) bool {
	if ch == nil || ch.InboundSecret == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(presented), []byte(ch.InboundSecret)) == 1
}

// mapping is the JSONPath configuration stored on a channel.
type mapping struct {
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	Message    string `json:"message"`
	Metadata   string `json:"metadata"`
}

func (m mapping) withDefaults() mapping {
	if m.ExternalID == "" {
		m.ExternalID = "$.external_id"
	}
	if m.Message == "" {
		m.Message = "$.message"
	}
	if m.Name == "" {
		m.Name = "$.name"
	}
	return m
}

// Ingest runs the ten ingestion steps in order. Every failure is returned to
// the caller, which records it in the webhook inspector.
//
//  1. the channel exists and is active
//  2. the shared secret matched (checked by the handler)
//  3. the per-channel/IP rate limit allows the request
//  4. the body parses as a JSON object
//  5. the JSONPath mapping resolves the external id, message, name and metadata
//  6. the identity maps to a visitor (created on first sight)
//  7. the visitor's open conversation is used, or a new one is opened
//  8. the message is stored encrypted and broadcast to the conversation room
//  9. the outbound acknowledgement is scheduled with the retry ladder
//
// 10. the result is returned for the inspector record
func (s *ChannelService) Ingest(ctx context.Context, in IngestInput) (*IngestResult, error) {
	ch := in.Channel
	// 1. the channel must exist and be active
	if ch == nil || !ch.IsActive {
		return nil, apierr.NotFound("کانال فعال نیست")
	}
	// 2. the secret was verified by the handler
	// 3. rate limit per channel+ip
	if s.throttle != nil && s.perMin > 0 {
		if s.throttle.Hit(ch.ID+"|"+in.RemoteIP) > s.perMin {
			return nil, apierr.New(http.StatusTooManyRequests, "rate_limited", "تعداد درخواست‌های این کانال بیش از حد مجاز است")
		}
	}
	// 4. parse the body (size limit enforced by the handler)
	var payload map[string]any
	if err := json.Unmarshal(in.Body, &payload); err != nil {
		return nil, apierr.BadRequest("بدنه باید یک شیء JSON باشد")
	}
	var m mapping
	_ = json.Unmarshal(ch.Mapping, &m)
	m = m.withDefaults()
	// 5. map the external fields
	externalID := jsonPathString(payload, m.ExternalID)
	text := jsonPathString(payload, m.Message)
	name := jsonPathString(payload, m.Name)
	email := jsonPathString(payload, m.Email)
	if externalID == "" {
		return nil, apierr.BadRequest("شناسه خارجی در بدنه یافت نشد")
	}
	if text == "" {
		return nil, apierr.BadRequest("متن پیام در بدنه یافت نشد")
	}
	var metadata json.RawMessage
	if m.Metadata != "" {
		if v, ok := jsonPathGet(payload, m.Metadata); ok {
			if b, err := json.Marshal(v); err == nil {
				metadata = b
			}
		}
	}
	// 6. resolve the identity to a visitor, creating both on first sight
	visitorID, err := s.repo.Channels.IdentityVisitor(ctx, ch.ID, externalID)
	if err != nil {
		if !errors.Is(err, repo.ErrNotFound) {
			return nil, fmt.Errorf("resolving identity: %w", err)
		}
		v := &models.Visitor{
			ID: id.NewPrefixed(id.PrefixVisitor), ProjectID: ch.ProjectID,
			Metadata:         models.JSONMap{"channel_id": ch.ID, "external_id": externalID},
			SubscriptionInfo: models.JSONMap{},
		}
		if name != "" {
			v.Name = &name
		}
		if email != "" {
			v.Email = &email
		}
		if err := s.repo.Visitors.Upsert(ctx, v); err != nil {
			return nil, fmt.Errorf("creating channel visitor: %w", err)
		}
		if err := s.repo.Channels.UpsertIdentity(ctx, ch.ID, externalID, v.ID, metadata); err != nil {
			return nil, err
		}
		visitorID = v.ID
	} else {
		_ = s.repo.Channels.UpsertIdentity(ctx, ch.ID, externalID, visitorID, metadata)
	}
	visitor, err := s.repo.Visitors.GetByID(ctx, visitorID)
	if err != nil {
		return nil, fmt.Errorf("loading channel visitor: %w", err)
	}
	// 7. continue the open conversation of the identity, or open a new one
	if conv, cerr := s.repo.Conversations.OpenForVisitor(ctx, visitorID); cerr == nil {
		msg, duplicate, serr := s.msgs.Send(ctx, SendInput{
			ConversationID: conv.ID, SenderType: models.SenderVisitor, SenderID: visitorID,
			SenderName: visitor.DisplayName(), Content: text, SkipAuthorization: true,
		})
		if serr != nil {
			return nil, serr
		}
		// 8. the message is stored and broadcast to the room by Send
		if !duplicate {
			s.deliverOutbound(ch, conv.ID, msg.ID, text)
		}
		return &IngestResult{ConversationID: conv.ID, MessageID: msg.ID, VisitorID: visitorID, Duplicate: duplicate}, nil
	}
	conv, msg, err := s.convs.Start(ctx, nil, visitor, StartInput{
		Message: text, Channel: ch.Kind, SenderName: visitor.DisplayName(),
	})
	if err != nil {
		return nil, err
	}
	if msg != nil {
		s.deliverOutbound(ch, conv.ID, msg.ID, text)
	}
	return &IngestResult{ConversationID: conv.ID, MessageID: msg.ID, VisitorID: visitorID}, nil
}

// deliverOutbound posts an acknowledgement to the channel callback and retries
// with the webhook backoff ladder. After the final failure a system message is
// written into the conversation so the team sees the broken integration.
func (s *ChannelService) deliverOutbound(ch *repo.Channel, convID, msgID, text string) {
	if ch == nil || ch.OutboundURL == nil || *ch.OutboundURL == "" {
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"event": "message.received", "channel_id": ch.ID, "conversation_id": convID,
		"message_id": msgID, "text": text, "timestamp": time.Now().Unix(),
	})
	go func() {
		for attempt, delay := range retrySchedule {
			if delay > 0 {
				time.Sleep(delay)
			}
			if err := s.postOutbound(ch, payload); err == nil {
				return
			} else if attempt == len(retrySchedule)-1 {
				s.log.Warn().Err(err).Str("channel_id", ch.ID).Msg("channel outbound gave up")
				if _, serr := s.msgs.SystemMessage(context.Background(), convID,
					"ارسال پاسخ به کانال ناموفق بود؛ اتصال کانال را بررسی کنید."); serr != nil {
					s.log.Warn().Err(serr).Msg("channel failure notice failed")
				}
				return
			}
		}
	}()
}

func (s *ChannelService) postOutbound(ch *repo.Channel, body []byte) error {
	now := time.Now().Unix()
	req, err := http.NewRequest(http.MethodPost, *ch.OutboundURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "EveChat-Channel/1.0")
	req.Header.Set("X-EveChat-Channel", ch.ID)
	req.Header.Set("X-EveChat-Timestamp", fmt.Sprint(now))
	if ch.OutboundSecret != nil && *ch.OutboundSecret != "" {
		req.Header.Set("X-EveChat-Signature", crypto.WebhookSignature(*ch.OutboundSecret, now, body))
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("channel callback status %d", resp.StatusCode)
	}
	return nil
}

// PurgeInbox deletes inspector rows older than 48 hours (PII retention).
func (s *ChannelService) PurgeInbox(ctx context.Context) {
	n, err := s.repo.Channels.PurgeInbox(ctx, time.Now().UTC().Add(-48*time.Hour))
	if err != nil {
		s.log.Warn().Err(err).Msg("purging channel inbox failed")
		return
	}
	if n > 0 {
		s.log.Info().Int64("rows", n).Msg("channel inbox purged")
	}
}

// StartRetention runs the inbox purge hourly until the context ends.
func (s *ChannelService) StartRetention(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.PurgeInbox(ctx)
			}
		}
	}()
}

// ScrubBody redacts secret-looking keys before a body is stored for the
// inspector, so the retention window never keeps a credential.
func ScrubBody(body []byte) string {
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	out, err := json.Marshal(scrubValue(payload))
	if err != nil {
		return ""
	}
	return string(out)
}

func scrubValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return redact(val)
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = scrubValue(item)
		}
		return out
	default:
		return v
	}
}

// PreviewMapping resolves a mapping against a sample payload for the live
// preview on the channels page.
func PreviewMapping(externalIDPath, namePath, messagePath, metadataPath string, raw json.RawMessage) map[string]any {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return map[string]any{"error": "payload must be a JSON object"}
	}
	m := mapping{ExternalID: externalIDPath, Name: namePath, Message: messagePath, Metadata: metadataPath}.withDefaults()
	out := map[string]any{
		"external_id": jsonPathString(payload, m.ExternalID),
		"name":        jsonPathString(payload, m.Name),
		"message":     jsonPathString(payload, m.Message),
	}
	if m.Metadata != "" {
		if v, ok := jsonPathGet(payload, m.Metadata); ok {
			out["metadata"] = v
		}
	}
	return out
}

// ChannelCurl renders the copy-paste command an integrator needs for a channel.
func ChannelCurl(baseURL string, ch *repo.Channel) string {
	if ch == nil {
		return ""
	}
	body := `{"external_id":"user-42","name":"Test User","message":"hello from curl"}`
	return fmt.Sprintf("curl -X POST %s/api/channels/%s/inbound \n  -H 'Content-Type: application/json' \n  -H 'X-Channel-Secret: <INBOUND_SECRET>' \n  -d '%s'",
		strings.TrimRight(baseURL, "/"), ch.ID, body)
}
