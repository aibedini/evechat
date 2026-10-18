package repo_test

import (
	"context"
	"testing"

	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

func createAgent(t *testing.T, r *repo.Repo, id, projectID string) {
	t.Helper()
	if err := r.Agents.Create(context.Background(), &models.Agent{
		ID: id, ProjectID: projectID, Email: id + "@example.com", PasswordHash: "x",
		Name: id, Role: models.RoleAgent, MaxConversations: 5, IsActive: true,
	}); err != nil {
		t.Fatalf("creating agent %s: %v", id, err)
	}
}

// TestAssignDoesNotActivate pins the BUG-2 rule: handing work to somebody is not
// the same as somebody engaging with it.
func TestAssignDoesNotActivate(t *testing.T) {
	r := testRepo(t)
	ctx := context.Background()
	createAgent(t, r, "agt_assign", "proj_t")

	if err := r.Conversations.Assign(ctx, "conv_t", "agt_assign"); err != nil {
		t.Fatalf("assign: %v", err)
	}
	conv, err := r.Conversations.GetByID(ctx, "conv_t")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if conv.AssignedAgentID == nil || *conv.AssignedAgentID != "agt_assign" {
		t.Fatalf("assignee = %v, want agt_assign", conv.AssignedAgentID)
	}
	if conv.Status != models.StatusPending {
		t.Fatalf("status after assign = %q, want pending (assign != activate)", conv.Status)
	}
}

func TestClaimActivatesAndIsExclusive(t *testing.T) {
	r := testRepo(t)
	ctx := context.Background()
	createAgent(t, r, "agt_first", "proj_t")
	createAgent(t, r, "agt_second", "proj_t")

	claimed, err := r.Conversations.Claim(ctx, "conv_t", "agt_first")
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if !claimed {
		t.Fatal("first claim was rejected")
	}
	conv, err := r.Conversations.GetByID(ctx, "conv_t")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if conv.Status != models.StatusActive {
		t.Fatalf("status after claim = %q, want active", conv.Status)
	}
	if conv.AssignedAgentID == nil || *conv.AssignedAgentID != "agt_first" {
		t.Fatalf("assignee after claim = %v, want agt_first", conv.AssignedAgentID)
	}

	// A second agent must lose the race, not steal the conversation.
	claimed, err = r.Conversations.Claim(ctx, "conv_t", "agt_second")
	if err != nil {
		t.Fatalf("second claim returned an error instead of a conflict: %v", err)
	}
	if claimed {
		t.Fatal("a second agent claimed an already active conversation")
	}
	conv, _ = r.Conversations.GetByID(ctx, "conv_t")
	if conv.AssignedAgentID == nil || *conv.AssignedAgentID != "agt_first" {
		t.Fatalf("second claim reassigned the conversation to %v", conv.AssignedAgentID)
	}
}

func TestActivateIfPendingIsIdempotent(t *testing.T) {
	r := testRepo(t)
	ctx := context.Background()

	activated, err := r.Conversations.ActivateIfPending(ctx, "conv_t")
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	if !activated {
		t.Fatal("first activation was a no-op")
	}
	activated, err = r.Conversations.ActivateIfPending(ctx, "conv_t")
	if err != nil {
		t.Fatalf("second activate: %v", err)
	}
	if activated {
		t.Fatal("activation ran twice on an active conversation")
	}
}

// TestAgentQueueHidesAssignedConversations covers the inbox rule that goes with
// BUG-2: the shared queue is "pending and unassigned", while an assignee keeps
// seeing their own conversation (still pending) in their list.
func TestAgentQueueHidesAssignedConversations(t *testing.T) {
	r := testRepo(t)
	ctx := context.Background()
	createAgent(t, r, "agt_queue", "proj_t")

	// Unassigned and pending: everybody sees it.
	_, total, err := r.Conversations.List(ctx, "proj_t", repo.ConvFilter{RestrictToAgentID: "agt_queue"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 {
		t.Fatalf("unassigned pending conversation not visible: total=%d", total)
	}

	if err := r.Conversations.Assign(ctx, "conv_t", "agt_queue"); err != nil {
		t.Fatalf("assign: %v", err)
	}
	// The assignee still sees it even though it is pending.
	_, total, err = r.Conversations.List(ctx, "proj_t", repo.ConvFilter{RestrictToAgentID: "agt_queue"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 {
		t.Fatalf("assignee lost sight of their own conversation: total=%d", total)
	}
}
