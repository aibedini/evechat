package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/config"
	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/middleware"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

func pushTestRepo(t *testing.T) (*repo.Repo, []*models.Agent) {
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
	for _, p := range []*models.Project{
		{ID: "proj_push", Name: "Push", APIKey: "pk_push", SecretKey: "sk", Domains: models.StringList{}, Settings: models.JSONMap{}, IsActive: true},
		{ID: "proj_other", Name: "Other", APIKey: "pk_other", SecretKey: "sk", Domains: models.StringList{}, Settings: models.JSONMap{}, IsActive: true},
	} {
		if err := r.Projects.Create(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	agents := []*models.Agent{
		{ID: "agent_one", ProjectID: "proj_push", Email: "one@example.com", Name: "One", Role: models.RoleAgent, MaxConversations: 5, IsActive: true},
		{ID: "agent_two", ProjectID: "proj_push", Email: "two@example.com", Name: "Two", Role: models.RoleAgent, MaxConversations: 5, IsActive: true},
		{ID: "agent_admin", ProjectID: "proj_push", Email: "admin@example.com", Name: "Admin", Role: models.RoleAdmin, MaxConversations: 5, IsActive: true},
		{ID: "agent_other", ProjectID: "proj_other", Email: "other@example.com", Name: "Other", Role: models.RoleAdmin, MaxConversations: 5, IsActive: true},
	}
	for _, a := range agents {
		if err := r.Agents.Create(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	return r, agents
}

func pushCall(h *Handlers, method, target, body string, c *middleware.Claims) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if c != nil {
		req = req.WithContext(middleware.WithClaims(req.Context(), c))
	}
	rec := httptest.NewRecorder()
	if method == http.MethodDelete {
		deviceID := req.URL.Path[strings.LastIndex(req.URL.Path, "/")+1:]
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("device_id", deviceID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		h.H(h.DeletePushDevice)(rec, req)
	} else {
		h.H(h.RegisterPushDevice)(rec, req)
	}
	return rec
}

func claims(subject, project string, role models.Role) *middleware.Claims {
	c := &middleware.Claims{ProjectID: project, Role: role}
	c.Subject = subject
	return c
}

func TestRegisterPushDeviceUpsertAndMove(t *testing.T) {
	r, _ := pushTestRepo(t)
	h := &Handlers{Repo: r}
	body := `{
		"device_id":"device-1","platform":"android","provider":"fcm","token":"token-1",
		"app_version":"3.9.0","app_build":155,"os_version":"Android 14",
		"device_model":"SM-A546E","locale":"fa-IR","timezone":"Asia/Tehran"
	}`
	rec := pushCall(h, http.MethodPost, "/api/v1/push/devices", body, claims("agent_one", "proj_push", models.RoleAgent))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body.String())
	}
	d, err := r.Push.Get(context.Background(), "device-1")
	if err != nil {
		t.Fatal(err)
	}
	if d.AgentID != "agent_one" || d.Token != "token-1" || d.AppBuild != 155 {
		t.Fatalf("device = %#v", d)
	}

	// Re-register the same device as another agent: 200 and ownership moves.
	body2 := strings.Replace(body, `"token-1"`, `"token-2"`, 1)
	body2 = strings.Replace(body2, `"3.9.0"`, `"3.9.1"`, 1)
	rec = pushCall(h, http.MethodPost, "/api/v1/push/devices", body2, claims("agent_two", "proj_push", models.RoleAgent))
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", rec.Code, rec.Body.String())
	}
	d, err = r.Push.Get(context.Background(), "device-1")
	if err != nil {
		t.Fatal(err)
	}
	if d.AgentID != "agent_two" || d.Token != "token-2" || d.AppVersion != "3.9.1" {
		t.Fatalf("updated device = %#v", d)
	}
}

func TestRegisterPushDeviceValidation(t *testing.T) {
	r, _ := pushTestRepo(t)
	h := &Handlers{Repo: r}
	cases := []string{
		`{"device_id":"","token":"t"}`,
		`{"device_id":"d","token":""}`,
		`{"device_id":"d","token":"t","provider":"apns"}`,
	}
	for _, body := range cases {
		rec := pushCall(h, http.MethodPost, "/api/v1/push/devices", body, claims("agent_one", "proj_push", models.RoleAgent))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %s status = %d", body, rec.Code)
		}
		var e apierr.Error
		if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil || e.Code != "invalid_request" {
			t.Fatalf("error body = %s", rec.Body.String())
		}
	}
}

func TestDeletePushDeviceOwnershipAndAdmin(t *testing.T) {
	r, _ := pushTestRepo(t)
	h := &Handlers{Repo: r}
	body := `{"device_id":"device-owner","token":"token-owner"}`
	if rec := pushCall(h, http.MethodPost, "/api/v1/push/devices", body, claims("agent_one", "proj_push", models.RoleAgent)); rec.Code != http.StatusCreated {
		t.Fatalf("register = %d", rec.Code)
	}

	// Another agent in the same project cannot delete it.
	if rec := pushCall(h, http.MethodDelete, "/api/v1/push/devices/device-owner", "", claims("agent_two", "proj_push", models.RoleAgent)); rec.Code != http.StatusForbidden {
		t.Fatalf("non-owner delete = %d", rec.Code)
	}
	// An admin in the same project can.
	if rec := pushCall(h, http.MethodDelete, "/api/v1/push/devices/device-owner", "", claims("agent_admin", "proj_push", models.RoleAdmin)); rec.Code != http.StatusNoContent {
		t.Fatalf("admin delete = %d body=%s", rec.Code, rec.Body.String())
	}
	// Already gone.
	if rec := pushCall(h, http.MethodDelete, "/api/v1/push/devices/device-owner", "", claims("agent_one", "proj_push", models.RoleAgent)); rec.Code != http.StatusNotFound {
		t.Fatalf("missing delete = %d", rec.Code)
	}

	// A device in another project is invisible to this project's admin.
	if rec := pushCall(h, http.MethodPost, "/api/v1/push/devices", body, claims("agent_other", "proj_other", models.RoleAdmin)); rec.Code != http.StatusCreated {
		t.Fatalf("other project register = %d", rec.Code)
	}
	if rec := pushCall(h, http.MethodDelete, "/api/v1/push/devices/device-owner", "", claims("agent_admin", "proj_push", models.RoleAdmin)); rec.Code != http.StatusNotFound {
		t.Fatalf("cross-project delete = %d", rec.Code)
	}

	// Owner deletes normally.
	if rec := pushCall(h, http.MethodDelete, "/api/v1/push/devices/device-owner", "", claims("agent_other", "proj_other", models.RoleAdmin)); rec.Code != http.StatusNoContent {
		t.Fatalf("owner delete = %d", rec.Code)
	}
}

func TestPushDeviceRequiresClaims(t *testing.T) {
	r, _ := pushTestRepo(t)
	h := &Handlers{Repo: r}
	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		rec := pushCall(h, method, "/api/v1/push/devices/device-x", `{"device_id":"device-x","token":"t"}`, nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s without claims = %d", method, rec.Code)
		}
	}
}
