package services

import (
	"context"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/config"
	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/integrations/fcm"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

type sentPush struct {
	Message fcm.Message
}

type fakeFCMSender struct {
	mu   sync.Mutex
	sent []sentPush
	err  error
}

func (f *fakeFCMSender) Send(_ context.Context, msg fcm.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sentPush{Message: msg})
	return f.err
}

func (f *fakeFCMSender) tokens() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.sent))
	for _, s := range f.sent {
		out = append(out, s.Message.Token)
	}
	sort.Strings(out)
	return out
}

func (f *fakeFCMSender) byToken(token string) *sentPush {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.sent {
		if f.sent[i].Message.Token == token {
			return &f.sent[i]
		}
	}
	return nil
}

func pushTestService(t *testing.T) (*repo.Repo, *models.Conversation, *models.Visitor) {
	t.Helper()
	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "push-svc.db")
	cfg.Database.MaxReadConns = 2
	d, err := db.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, d, zerolog.Nop()); err != nil {
		t.Fatal(err)
	}
	r := repo.New(d)
	if err := r.Projects.Create(ctx, &models.Project{
		ID: "p", Name: "P", APIKey: "pk", SecretKey: "sk",
		Domains: models.StringList{}, Settings: models.JSONMap{}, IsActive: true,
	}); err != nil {
		t.Fatal(err)
	}
	for _, a := range []*models.Agent{
		{ID: "a1", ProjectID: "p", Email: "a1@example.com", Name: "A1", Role: models.RoleAgent, MaxConversations: 1, IsActive: true},
		{ID: "a2", ProjectID: "p", Email: "a2@example.com", Name: "A2", Role: models.RoleAgent, MaxConversations: 1, IsActive: true},
	} {
		if err := r.Agents.Create(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	name := "Sara"
	v := &models.Visitor{ID: "v", ProjectID: "p", Name: &name, Metadata: models.JSONMap{}, SubscriptionInfo: models.JSONMap{}}
	if err := r.Visitors.Upsert(ctx, v); err != nil {
		t.Fatal(err)
	}
	for _, d := range []*models.PushDevice{
		{DeviceID: "d1", AgentID: "a1", ProjectID: "p", Token: "t1", Platform: "android", Provider: "fcm"},
		{DeviceID: "d2", AgentID: "a2", ProjectID: "p", Token: "t2", Platform: "android", Provider: "fcm"},
	} {
		if _, err := r.Push.Upsert(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	conv := &models.Conversation{ID: "c", ProjectID: "p", VisitorID: "v", Status: models.StatusActive, Channel: "web"}
	if err := r.Conversations.Create(ctx, conv); err != nil {
		t.Fatal(err)
	}
	return r, conv, v
}

func TestPushAssignedVisitorMessageGoesOnlyToOwner(t *testing.T) {
	r, conv, _ := pushTestService(t)
	a1 := "a1"
	conv.AssignedAgentID = &a1
	if err := r.Conversations.Assign(context.Background(), conv.ID, a1); err != nil {
		t.Fatal(err)
	}
	sender := &fakeFCMSender{}
	c := NewPushChannel(sender, r, zerolog.Nop())
	ev := Event{
		Name: EventMessageReceived, ProjectID: "p", ConversationID: "c", At: time.Unix(100, 0),
		Data: map[string]any{
			"sender_type": "visitor", "message_id": "m1", "content": "hello there",
		},
	}
	if err := c.Notify(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if got := sender.tokens(); len(got) != 1 || got[0] != "t1" {
		t.Fatalf("recipients = %v", got)
	}
	msg := sender.byToken("t1").Message
	if msg.Options.Priority != "HIGH" || msg.Options.TTL != "3600s" {
		t.Fatalf("options = %#v", msg.Options)
	}
	if msg.Data["type"] != pushTypeMessageReceived || msg.Data["agent_id"] != "a1" ||
		msg.Data["source"] != "evechat" || msg.Data["v"] != "1" || msg.Data["message_id"] != "m1" {
		t.Fatalf("data = %#v", msg.Data)
	}
	if msg.Data["title"] != "Sara" || msg.Data["body"] != "hello there" {
		t.Fatalf("fallback text = %#v", msg.Data)
	}

	// Agent messages are not pushed.
	sender = &fakeFCMSender{}
	c = NewPushChannel(sender, r, zerolog.Nop())
	ev.Data["sender_type"] = "agent"
	if err := c.Notify(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(sender.sent) != 0 {
		t.Fatalf("agent message produced %d pushes", len(sender.sent))
	}
}

func TestPushNewConversationUsesMessageID(t *testing.T) {
	r, _, _ := pushTestService(t)
	sender := &fakeFCMSender{}
	c := NewPushChannel(sender, r, zerolog.Nop())
	if err := c.Notify(context.Background(), Event{
		Name: EventChatStarted, ProjectID: "p", ConversationID: "c", At: time.Now(),
		Data: map[string]any{"visitor_name": "Sara", "message": "hello", "message_id": "m1"},
	}); err != nil {
		t.Fatal(err)
	}
	if got := sender.tokens(); len(got) != 2 {
		t.Fatalf("recipients = %v", got)
	}
	msg := sender.byToken("t1").Message
	if msg.Data["type"] != pushTypeConversationNew || msg.Data["message_id"] != "m1" {
		t.Fatalf("data = %#v", msg.Data)
	}
	if msg.Options.Priority != "HIGH" || msg.Options.TTL != "3600s" {
		t.Fatalf("options = %#v", msg.Options)
	}
}

func TestPushUnassignedMessageGoesToAllActiveAgents(t *testing.T) {
	r, _, _ := pushTestService(t)
	sender := &fakeFCMSender{}
	c := NewPushChannel(sender, r, zerolog.Nop())
	ev := Event{
		Name: EventMessageReceived, ProjectID: "p", ConversationID: "c", At: time.Now(),
		Data: map[string]any{"sender_type": "visitor", "message_id": "m1", "content": "hi"},
	}
	if err := c.Notify(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	got := sender.tokens()
	if len(got) != 2 || got[0] != "t1" || got[1] != "t2" {
		t.Fatalf("recipients = %v", got)
	}
}

func TestPushAssignmentNotifiesAssigneeAndWithdrawsOthers(t *testing.T) {
	r, conv, _ := pushTestService(t)
	a2 := "a2"
	if err := r.Conversations.Assign(context.Background(), conv.ID, a2); err != nil {
		t.Fatal(err)
	}
	sender := &fakeFCMSender{}
	c := NewPushChannel(sender, r, zerolog.Nop())
	if err := c.Notify(context.Background(), Event{
		Name: EventChatAssigned, ProjectID: "p", ConversationID: conv.ID, At: time.Now(),
		Data: map[string]any{"agent_id": a2},
	}); err != nil {
		t.Fatal(err)
	}
	if got := sender.tokens(); len(got) != 2 {
		t.Fatalf("recipients = %v", got)
	}
	newOwner := sender.byToken("t2").Message
	if newOwner.Data["type"] != pushTypeAgentAssigned || newOwner.Data["agent_id"] != "a2" {
		t.Fatalf("assignee data = %#v", newOwner.Data)
	}
	if newOwner.Options.CollapseKey != conv.ID || newOwner.Options.Priority != "NORMAL" {
		t.Fatalf("assignee options = %#v", newOwner.Options)
	}
	other := sender.byToken("t1").Message
	if other.Data["type"] != pushTypeConversationUpdate || other.Data["agent_id"] != "a1" {
		t.Fatalf("other data = %#v", other.Data)
	}
}

func TestPushResolveAndReopenUseAssignedAgent(t *testing.T) {
	r, conv, _ := pushTestService(t)
	a1 := "a1"
	if err := r.Conversations.Assign(context.Background(), conv.ID, a1); err != nil {
		t.Fatal(err)
	}
	sender := &fakeFCMSender{}
	c := NewPushChannel(sender, r, zerolog.Nop())
	if err := c.Notify(context.Background(), Event{Name: EventChatResolved, ProjectID: "p", ConversationID: conv.ID, At: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if got := sender.tokens(); len(got) != 1 || got[0] != "t1" {
		t.Fatalf("resolve recipients = %v", got)
	}
	if sender.sent[0].Message.Data["type"] != pushTypeConversationDone {
		t.Fatalf("resolve data = %#v", sender.sent[0].Message.Data)
	}

	sender = &fakeFCMSender{}
	c = NewPushChannel(sender, r, zerolog.Nop())
	if err := c.Notify(context.Background(), Event{Name: EventConversationReopened, ProjectID: "p", ConversationID: conv.ID, At: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if len(sender.sent) != 1 || sender.sent[0].Message.Data["type"] != pushTypeConversationUpdate {
		t.Fatalf("reopen = %#v", sender.sent)
	}
}

func TestPushInvalidTokenIsDeleted(t *testing.T) {
	r, conv, _ := pushTestService(t)
	a1 := "a1"
	if err := r.Conversations.Assign(context.Background(), conv.ID, a1); err != nil {
		t.Fatal(err)
	}
	sender := &fakeFCMSender{err: &fcm.Error{StatusCode: 404, Code: "UNREGISTERED"}}
	c := NewPushChannel(sender, r, zerolog.Nop())
	if err := c.Notify(context.Background(), Event{
		Name: EventMessageReceived, ProjectID: "p", ConversationID: conv.ID, At: time.Now(),
		Data: map[string]any{"sender_type": "visitor", "message_id": "m1", "content": "x"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Push.Get(context.Background(), "d1"); err != repo.ErrNotFound {
		t.Fatalf("invalid token row still present: %v", err)
	}
}
