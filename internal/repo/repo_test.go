package repo_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/config"
	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

func testRepo(t *testing.T) *repo.Repo {
	t.Helper()
	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "test.db")
	cfg.Database.MaxReadConns = 2
	d, err := db.Open(cfg)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, d, zerolog.Nop()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	r := repo.New(d)

	if err := r.Projects.Create(ctx, &models.Project{
		ID: "proj_t", Name: "Test", APIKey: "pk_test", SecretKey: "secret",
		Domains: models.StringList{}, Settings: models.JSONMap{}, IsActive: true,
	}); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := r.Visitors.Upsert(ctx, &models.Visitor{
		ID: "vis_t", ProjectID: "proj_t", Metadata: models.JSONMap{}, SubscriptionInfo: models.JSONMap{},
	}); err != nil {
		t.Fatalf("create visitor: %v", err)
	}
	if err := r.Conversations.Create(ctx, &models.Conversation{
		ID: "conv_t", ProjectID: "proj_t", VisitorID: "vis_t", Status: models.StatusPending, Channel: "web",
	}); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	return r
}

func TestMessageDedupUniqueIndex(t *testing.T) {
	r := testRepo(t)
	ctx := context.Background()
	cid := "client-1"
	m1 := &models.Message{
		ID: "msg_1", ConversationID: "conv_t", ClientMessageID: &cid,
		SenderType: models.SenderVisitor, ContentType: "text",
		ContentEncrypted: "cipher", ContentIV: "iv", KeyVersion: 1,
	}
	if err := r.Messages.Create(ctx, m1); err != nil {
		t.Fatalf("first create: %v", err)
	}
	m2 := &models.Message{
		ID: "msg_2", ConversationID: "conv_t", ClientMessageID: &cid,
		SenderType: models.SenderVisitor, ContentType: "text",
		ContentEncrypted: "cipher", ContentIV: "iv", KeyVersion: 1,
	}
	if err := r.Messages.Create(ctx, m2); err != repo.ErrDuplicate {
		t.Fatalf("expected ErrDuplicate, got %v", err)
	}
	found, err := r.Messages.GetByClientID(ctx, "conv_t", cid)
	if err != nil {
		t.Fatalf("get by client id: %v", err)
	}
	if found.ID != "msg_1" {
		t.Fatalf("expected msg_1, got %s", found.ID)
	}
	n, err := r.Messages.CountByConversation(ctx, "conv_t")
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected exactly one row, got %d", n)
	}
}

func TestMessageDedupAllowsNullClientID(t *testing.T) {
	r := testRepo(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		m := &models.Message{
			ID: "msg_null_" + string(rune('a'+i)), ConversationID: "conv_t",
			SenderType: models.SenderAgent, ContentType: "text",
			ContentEncrypted: "c", ContentIV: "i", KeyVersion: 1,
		}
		if err := r.Messages.Create(ctx, m); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	n, _ := r.Messages.CountByConversation(ctx, "conv_t")
	if n != 3 {
		t.Fatalf("expected 3 rows, got %d", n)
	}
}

func TestMessageOrderingAndReads(t *testing.T) {
	r := testRepo(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		m := &models.Message{
			ID: "m" + string(rune('1'+i)), ConversationID: "conv_t",
			SenderType: models.SenderVisitor, ContentType: "text",
			ContentEncrypted: "c", ContentIV: "i", KeyVersion: 1,
		}
		if err := r.Messages.Create(ctx, m); err != nil {
			t.Fatal(err)
		}
		time.Sleep(1100 * time.Millisecond) // created_at has second resolution
	}
	msgs, err := r.Messages.LatestN(ctx, "conv_t", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2, got %d", len(msgs))
	}
	if msgs[0].ID != "m2" || msgs[1].ID != "m3" {
		t.Fatalf("unexpected order: %s, %s", msgs[0].ID, msgs[1].ID)
	}
	if err := r.Messages.MarkRead(ctx, "conv_t", "m2", "agent"); err != nil {
		t.Fatal(err)
	}
	first, _ := r.Messages.GetByID(ctx, "m1")
	second, _ := r.Messages.GetByID(ctx, "m2")
	if !first.IsRead || !second.IsRead {
		t.Fatal("expected both messages marked read")
	}
}

func TestConversationFiltersAndUnread(t *testing.T) {
	r := testRepo(t)
	ctx := context.Background()
	if err := r.Conversations.IncrementUnread(ctx, "conv_t", "agent"); err != nil {
		t.Fatal(err)
	}
	conv, err := r.Conversations.GetByID(ctx, "conv_t")
	if err != nil {
		t.Fatal(err)
	}
	if conv.UnreadAgent != 1 {
		t.Fatalf("expected unread_agent 1, got %d", conv.UnreadAgent)
	}
	if err := r.Conversations.ClearUnread(ctx, "conv_t", "agent"); err != nil {
		t.Fatal(err)
	}
	conv, _ = r.Conversations.GetByID(ctx, "conv_t")
	if conv.UnreadAgent != 0 {
		t.Fatalf("expected unread_agent 0, got %d", conv.UnreadAgent)
	}

	conv.Status = models.StatusActive
	if err := r.Conversations.SetStatus(ctx, "conv_t", models.StatusResolved); err != nil {
		t.Fatal(err)
	}
	list, total, err := r.Conversations.List(ctx, "proj_t", repo.ConvFilter{Status: string(models.StatusResolved)})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("expected the resolved conversation, got %d/%d", total, len(list))
	}
	hidden, _, err := r.Conversations.List(ctx, "proj_t", repo.ConvFilter{HideClosed: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(hidden) != 0 {
		t.Fatalf("expected resolved conversation to be hidden, got %d", len(hidden))
	}
}

func TestAgentVisibilityRule(t *testing.T) {
	r := testRepo(t)
	ctx := context.Background()
	if err := r.Agents.Create(ctx, &models.Agent{
		ID: "agt_other", ProjectID: "proj_t", Email: "other@example.com", PasswordHash: "x",
		Name: "Other", Role: models.RoleAgent, MaxConversations: 5, IsActive: true,
	}); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	list, total, err := r.Conversations.List(ctx, "proj_t", repo.ConvFilter{RestrictToAgentID: "agt_x"})
	if err != nil {
		t.Fatal(err)
	}
	// Pending conversations are visible to every agent.
	if total != 1 || len(list) != 1 {
		t.Fatalf("expected pending conversation visible, got %d", total)
	}
	if err := r.Conversations.Assign(ctx, "conv_t", "agt_other"); err != nil {
		t.Fatalf("assign: %v", err)
	}
	_, total, err = r.Conversations.List(ctx, "proj_t", repo.ConvFilter{RestrictToAgentID: "agt_x"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Fatalf("expected assigned conversation hidden from other agents, got %d", total)
	}
}

func TestKBFtsSearch(t *testing.T) {
	r := testRepo(t)
	ctx := context.Background()
	article := &models.KBArticle{
		ID: "kb_1", ProjectID: "proj_t", Title: "تمدید اشتراک",
		Slug: "tamdid", Content: "برای تمدید اشتراک به صفحه پرداخت بروید",
		Tags: models.StringList{"billing"}, IsPublished: true,
	}
	if err := r.KB.Create(ctx, article); err != nil {
		t.Fatal(err)
	}
	hits, err := r.KB.Search(ctx, "proj_t", "تمدید", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 || hits[0].ID != "kb_1" {
		t.Fatalf("expected the article, got %d hits", len(hits))
	}
	if _, err := r.KB.Search(ctx, "other_project", "تمدید", 5); err != nil {
		t.Fatalf("cross-project search must not error: %v", err)
	}
	if hits, _ := r.KB.Search(ctx, "other_project", "تمدید", 5); len(hits) != 0 {
		t.Fatal("search must be project-scoped")
	}
}
