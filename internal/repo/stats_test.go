package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/evechat/evechat/internal/models"
)

func TestStatsOverviewCountsAgentMessages(t *testing.T) {
	r := testRepo(t)
	ctx := context.Background()
	now := time.Now()
	msgs := []*models.Message{
		{ID: "msg_v", ConversationID: "conv_t", SenderType: models.SenderVisitor},
		{ID: "msg_a1", ConversationID: "conv_t", SenderType: models.SenderAgent},
		{ID: "msg_a2", ConversationID: "conv_t", SenderType: models.SenderAgent},
	}
	for _, m := range msgs {
		m.ContentType = "text"
		m.ContentEncrypted = "cipher"
		m.ContentIV = "iv"
		m.KeyVersion = 1
		if err := r.Messages.Create(ctx, m); err != nil {
			t.Fatalf("create %s: %v", m.ID, err)
		}
	}
	stats, err := r.Stats.Overview(ctx, "proj_t", now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if stats.Messages != 3 {
		t.Fatalf("messages = %d, want 3", stats.Messages)
	}
	if stats.AgentMessages != 2 {
		t.Fatalf("agent_messages = %d, want 2", stats.AgentMessages)
	}
}
