package services

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/config"
	"github.com/evechat/evechat/internal/crypto"
	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/hub"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

func newChannelFixture(t *testing.T) (*repo.Repo, *ChannelService) {
	t.Helper()
	ctx := context.Background()
	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "test.db")
	d, err := db.Open(cfg)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := db.Migrate(ctx, d, zerolog.Nop()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	r := repo.New(d)
	if err := r.Projects.Create(ctx, &models.Project{
		ID: "proj_ch", Name: "Channels", APIKey: "pk_ch", SecretKey: "sk",
		Domains: models.StringList{}, Settings: models.JSONMap{}, IsActive: true,
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	enc, err := crypto.New(map[int][]byte{1: make([]byte, 32)}, 1)
	if err != nil {
		t.Fatalf("crypto: %v", err)
	}
	h := hub.New(hub.HubConfig{}, zerolog.Nop())
	msgs := NewMessageService(r, h, enc, nil, zerolog.Nop())
	routing := NewRoutingService(r, zerolog.Nop())
	convs := NewConversationService(r, h, routing, msgs, nil, nil, zerolog.Nop())
	svc := NewChannelService(r, convs, msgs, zerolog.Nop())
	return r, svc
}

func TestChannelIngestReusesIdentityAndConversation(t *testing.T) {
	r, svc := newChannelFixture(t)
	ctx := context.Background()
	ch := &repo.Channel{
		ID: "ch_t", ProjectID: "proj_ch", Name: "CRM", Kind: "webhook", IsActive: true,
		InboundSecret: "secret",
		Mapping:       json.RawMessage(`{"external_id":"$.user.id","name":"$.user.name","message":"$.text"}`),
	}
	if err := r.Channels.Create(ctx, ch); err != nil {
		t.Fatalf("create channel: %v", err)
	}
	body := []byte(`{"user":{"id":"u-1","name":"Ali"},"text":"سلام از CRM"}`)
	first, err := svc.Ingest(ctx, IngestInput{Channel: ch, Body: body, RemoteIP: "127.0.0.1"})
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	second, err := svc.Ingest(ctx, IngestInput{Channel: ch, Body: body, RemoteIP: "127.0.0.1"})
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if first.ConversationID != second.ConversationID {
		t.Fatalf("identity did not reuse the conversation: %s vs %s", first.ConversationID, second.ConversationID)
	}
	if first.VisitorID != second.VisitorID {
		t.Fatal("identity created two visitors")
	}
	// The conversation also carries the no-agent auto reply, so count only the
	// visitor messages the channel produced.
	msgs, err := r.Messages.ListByConversation(ctx, first.ConversationID, time.Time{}, 50)
	if err != nil {
		t.Fatalf("listing messages: %v", err)
	}
	visitorMsgs := 0
	for _, m := range msgs {
		if m.SenderType == models.SenderVisitor {
			visitorMsgs++
		}
	}
	if visitorMsgs != 2 {
		t.Fatalf("visitor messages = %d, want 2", visitorMsgs)
	}
	conv, err := r.Conversations.GetByID(ctx, first.ConversationID)
	if err != nil {
		t.Fatalf("get conversation: %v", err)
	}
	if conv.Channel != "webhook" {
		t.Fatalf("conversation channel = %q, want webhook", conv.Channel)
	}
}

func TestChannelIngestCreatesADistinctVisitorPerExternalID(t *testing.T) {
	r, svc := newChannelFixture(t)
	ctx := context.Background()
	ch := &repo.Channel{
		ID: "ch_t2", ProjectID: "proj_ch", Name: "CRM", Kind: "webhook", IsActive: true,
		InboundSecret: "secret",
	}
	if err := r.Channels.Create(ctx, ch); err != nil {
		t.Fatalf("create channel: %v", err)
	}
	a, err := svc.Ingest(ctx, IngestInput{Channel: ch, Body: []byte(`{"external_id":"u-1","message":"one"}`)})
	if err != nil {
		t.Fatalf("ingest a: %v", err)
	}
	b, err := svc.Ingest(ctx, IngestInput{Channel: ch, Body: []byte(`{"external_id":"u-2","message":"two"}`)})
	if err != nil {
		t.Fatalf("ingest b: %v", err)
	}
	if a.VisitorID == b.VisitorID || a.ConversationID == b.ConversationID {
		t.Fatal("different external ids shared a visitor or conversation")
	}
}

func TestChannelIngestRejectsUnmappableBody(t *testing.T) {
	r, svc := newChannelFixture(t)
	ctx := context.Background()
	ch := &repo.Channel{ID: "ch_t3", ProjectID: "proj_ch", Name: "CRM", Kind: "webhook", IsActive: true, InboundSecret: "secret"}
	if err := r.Channels.Create(ctx, ch); err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if _, err := svc.Ingest(ctx, IngestInput{Channel: ch, Body: []byte(`{"message":"no id"}`)}); err == nil {
		t.Fatal("ingest accepted a body without an external id")
	}
	if _, err := svc.Ingest(ctx, IngestInput{Channel: ch, Body: []byte(`not json`)}); err == nil {
		t.Fatal("ingest accepted a non-JSON body")
	}
}
