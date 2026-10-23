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

func testPushRepo(t *testing.T) *repo.Repo {
	t.Helper()
	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "push.db")
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
		{ID: "off", ProjectID: "p", Email: "off@example.com", Name: "Off", Role: models.RoleAgent, MaxConversations: 1, IsActive: false},
	} {
		if err := r.Agents.Create(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

func TestPushUpsertMovesDeviceAndToken(t *testing.T) {
	r := testPushRepo(t)
	ctx := context.Background()
	d := &models.PushDevice{DeviceID: "d1", AgentID: "a1", ProjectID: "p", Token: "t1", Platform: "android", Provider: "fcm"}
	created, err := r.Push.Upsert(ctx, d)
	if err != nil || !created {
		t.Fatalf("create: created=%v err=%v", created, err)
	}
	d.AgentID = "a2"
	d.Token = "t2"
	d.AppVersion = "3.9.1"
	created, err = r.Push.Upsert(ctx, d)
	if err != nil || created {
		t.Fatalf("update: created=%v err=%v", created, err)
	}
	got, err := r.Push.Get(ctx, "d1")
	if err != nil {
		t.Fatal(err)
	}
	if got.AgentID != "a2" || got.Token != "t2" || got.AppVersion != "3.9.1" {
		t.Fatalf("got %#v", got)
	}

	// Reusing t2 under a new DeviceID replaces the stale registration.
	if _, err := r.Push.Upsert(ctx, &models.PushDevice{
		DeviceID: "d2", AgentID: "a1", ProjectID: "p", Token: "t2", Platform: "android", Provider: "fcm",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Push.Get(ctx, "d1"); err != repo.ErrNotFound {
		t.Fatalf("stale device still exists: %v", err)
	}
}

func TestPushListsActiveAgentsAndPrunes(t *testing.T) {
	r := testPushRepo(t)
	ctx := context.Background()
	for _, d := range []*models.PushDevice{
		{DeviceID: "d1", AgentID: "a1", ProjectID: "p", Token: "t1"},
		{DeviceID: "d2", AgentID: "a2", ProjectID: "p", Token: "t2"},
		{DeviceID: "doff", AgentID: "off", ProjectID: "p", Token: "t3"},
	} {
		if _, err := r.Push.Upsert(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	all, err := r.Push.ListByProject(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("active project devices = %d, want 2", len(all))
	}
	without, err := r.Push.ListByProjectExceptAgent(ctx, "p", "a1")
	if err != nil {
		t.Fatal(err)
	}
	if len(without) != 1 || without[0].AgentID != "a2" {
		t.Fatalf("without a1 = %#v", without)
	}

	if _, err := r.DB().W.ExecContext(ctx, `UPDATE push_devices SET updated_at = ? WHERE device_id = 'd1'`,
		models.FormatTime(time.Now().UTC().AddDate(0, 0, -61))); err != nil {
		t.Fatal(err)
	}
	n, err := r.Push.PruneStale(ctx, time.Now().UTC().AddDate(0, 0, -60))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("pruned = %d, want 1", n)
	}
}
