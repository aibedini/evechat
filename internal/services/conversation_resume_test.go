package services

import (
	"context"
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

// TestStartRecordsActiveConversationWithoutSession is the BUG-1 regression: the
// widget creates conversations over the WebSocket, where Start has no session
// token. The visitor row must still point at the new conversation, otherwise
// every REST fallback (SSE/poll/CSAT) starts from "no conversation".
func TestStartRecordsActiveConversationWithoutSession(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "test.db")
	d, err := db.Open(cfg)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()
	if err := db.Migrate(ctx, d, zerolog.Nop()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	r := repo.New(d)

	project := &models.Project{
		ID: "proj_resume", Name: "Resume", APIKey: "pk_resume", SecretKey: "sk",
		Domains: models.StringList{}, Settings: models.JSONMap{}, IsActive: true,
	}
	if err := r.Projects.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	visitor := &models.Visitor{
		ID: "vis_resume", ProjectID: project.ID,
		Metadata: models.JSONMap{}, SubscriptionInfo: models.JSONMap{},
	}
	if err := r.Visitors.Upsert(ctx, visitor); err != nil {
		t.Fatalf("create visitor: %v", err)
	}
	sess, err := r.Sessions.Create(ctx, visitor.ID, project.ID, "tok_resume", 24*time.Hour)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	enc, err := crypto.New(map[int][]byte{1: make([]byte, 32)}, 1)
	if err != nil {
		t.Fatalf("crypto: %v", err)
	}
	h := hub.New(hub.HubConfig{}, zerolog.Nop())
	msgs := NewMessageService(r, h, enc, nil, zerolog.Nop())
	routing := NewRoutingService(r, zerolog.Nop())
	convs := NewConversationService(r, h, routing, msgs, nil, nil, zerolog.Nop())

	// nil session is exactly what OnVisitorMessage passes.
	conv, _, err := convs.Start(ctx, nil, visitor, StartInput{Message: "سلام"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	got, err := r.Sessions.Get(ctx, sess.Token)
	if err != nil {
		t.Fatalf("reload session: %v", err)
	}
	if got.ActiveConvID == nil || *got.ActiveConvID != conv.ID {
		t.Fatalf("active_conversation_id = %v, want %s", got.ActiveConvID, conv.ID)
	}
}
