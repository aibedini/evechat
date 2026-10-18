package handlers

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/doctor"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/integrations/smtp"
	"github.com/evechat/evechat/internal/middleware"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/runtimecfg"
	"github.com/evechat/evechat/internal/security"
)

const (
	settingSetupAdminID   = "setup.admin_id"
	settingSetupProjectID = "setup.project_id"
)

// setupAuthorized enforces the anti-hijack token: loopback requests are trusted,
// remote requests must present X-Setup-Token.
func (h *Handlers) setupAuthorized(r *http.Request) bool {
	ip := middleware.ClientIP(r, h.Cfg.Server.TrustProxy)
	if ip == "127.0.0.1" || ip == "::1" || ip == "localhost" {
		return true
	}
	token := strings.TrimSpace(r.Header.Get("X-Setup-Token"))
	if token == "" {
		return false
	}
	expected := h.State.Value(runtimecfg.KeySetupToken)
	if expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1
}

// SetupStatus reports whether the wizard is still required.
func (h *Handlers) SetupStatus(w http.ResponseWriter, r *http.Request) error {
	if !h.setupAuthorized(r) {
		return apierr.Unauthorized("مجوز ویزارد نصب را ندارید")
	}
	checks := h.setupChecks(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"setup_required": !h.State.SetupCompleted(),
		"version":        h.Version,
		"instance_id":    h.State.InstanceID(),
		"admin_exists":   h.State.Value(settingSetupAdminID) != "",
		"project_exists": h.State.Value(settingSetupProjectID) != "",
		"checks":         checks,
	})
	return nil
}

// SetupCheck re-runs the preflight doctor.
func (h *Handlers) SetupCheck(w http.ResponseWriter, r *http.Request) error {
	if !h.setupAuthorized(r) {
		return apierr.Unauthorized("مجوز ویزارد نصب را ندارید")
	}
	report := doctor.RunAll(r.Context(), doctor.Options{
		ConfigPath: h.ConfigPath,
		Cfg:        h.Cfg,
		Log:        h.Log,
		Version:    h.Version,
		Only:       []string{"config.file", "config.jwt", "config.keys", "fs.data", "fs.uploads", "db.open", "db.migrations", "net.port", "time.skew", "assets.dashboard", "assets.font", "widget.size"},
	})
	writeJSON(w, http.StatusOK, report)
	return nil
}

func (h *Handlers) setupChecks(ctx context.Context) []map[string]any {
	report := doctor.RunAll(ctx, doctor.Options{
		ConfigPath: h.ConfigPath,
		Cfg:        h.Cfg,
		Log:        h.Log,
		Version:    h.Version,
		Only:       []string{"fs.data", "db.open", "net.port", "config.keys", "time.skew", "assets.font"},
	})
	out := make([]map[string]any, 0, len(report.Checks))
	for _, c := range report.Checks {
		out = append(out, map[string]any{
			"id":      c.ID,
			"name":    c.Name,
			"ok":      c.Result.OK,
			"fatal":   c.Level == doctor.Fatal,
			"message": c.Result.Message,
			"fix":     c.Result.Fix,
		})
	}
	return out
}

type setupAdminInput struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// SetupAdmin creates the first superadmin (and a placeholder project because
// agents are project-scoped by design).
func (h *Handlers) SetupAdmin(w http.ResponseWriter, r *http.Request) error {
	if !h.setupAuthorized(r) {
		return apierr.Unauthorized("مجوز ویزارد نصب را ندارید")
	}
	if h.State.SetupCompleted() {
		return apierr.Forbidden("نصب قبلاً کامل شده است")
	}
	var in setupAdminInput
	if err := bind(r, &in); err != nil {
		return err
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if !strings.Contains(email, "@") {
		return apierr.BadRequest("ایمیل معتبر لازم است")
	}
	if len(in.Password) < 8 {
		return apierr.BadRequest("رمز عبور باید حداقل ۸ کاراکتر باشد")
	}
	projectID := h.State.Value(settingSetupProjectID)
	if projectID == "" {
		project := &models.Project{
			ID:        id.NewPrefixed(id.PrefixProject),
			Name:      "EVE",
			APIKey:    id.NewPrefixed("pk"),
			SecretKey: id.Token(32),
			Domains:   models.StringList{},
			Settings: models.JSONMap{
				"greeting":        "سلام 👋 چطور می‌توانیم کمک کنیم؟",
				"offline_message": "در حال حاضر آفلاین هستیم؛ پیام بگذارید تا پاسخ دهیم.",
				"position":        "bottom-left",
				"colors":          models.JSONMap{"primary": "#2563eb"},
			},
			IsActive: true,
		}
		if err := h.Repo.Projects.Create(r.Context(), project); err != nil {
			return apierr.Internal(err)
		}
		projectID = project.ID
		_ = h.State.Set(r.Context(), settingSetupProjectID, projectID)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), h.Cfg.Auth.BcryptCost)
	if err != nil {
		return apierr.Internal(err)
	}
	agent := &models.Agent{
		ID:               id.NewPrefixed(id.PrefixAgent),
		ProjectID:        projectID,
		Email:            email,
		PasswordHash:     string(hash),
		Name:             firstNonEmpty(in.Name, "مدیر سیستم"),
		Role:             models.RoleSuperAdmin,
		MaxConversations: 50,
		IsActive:         true,
	}
	if err := h.Repo.Agents.Create(r.Context(), agent); err != nil {
		return apierr.Internal(err)
	}
	_ = h.State.Set(r.Context(), settingSetupAdminID, agent.ID)
	h.recordAudit(r, agent.ProjectID, "setup.admin_created", "agent", agent.ID, map[string]any{"email": agent.Email})
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "agent_id": agent.ID, "project_id": projectID})
	return nil
}

// SetupProject finalises the first project (name, allowed domains, snippet).
func (h *Handlers) SetupProject(w http.ResponseWriter, r *http.Request) error {
	if !h.setupAuthorized(r) {
		return apierr.Unauthorized("مجوز ویزارد نصب را ندارید")
	}
	if h.State.SetupCompleted() {
		return apierr.Forbidden("نصب قبلاً کامل شده است")
	}
	var in struct {
		Name    string   `json:"name"`
		Domains []string `json:"domains"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	projectID := h.State.Value(settingSetupProjectID)
	if projectID == "" {
		return apierr.BadRequest("ابتدا حساب مدیر را بسازید")
	}
	project, err := h.Repo.Projects.GetByID(r.Context(), projectID)
	if err != nil {
		return apierr.NotFound("پروژه یافت نشد")
	}
	if strings.TrimSpace(in.Name) != "" {
		project.Name = strings.TrimSpace(in.Name)
	}
	domains := make(models.StringList, 0, len(in.Domains))
	for _, d := range in.Domains {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" {
			continue
		}
		if !strings.Contains(d, ".") && d != "localhost" {
			return apierr.BadRequest("دامنه نامعتبر است: " + d)
		}
		domains = append(domains, d)
	}
	project.Domains = domains
	if err := h.Repo.Projects.Update(r.Context(), project); err != nil {
		return apierr.Internal(err)
	}
	h.recordAudit(r, project.ID, "setup.project_updated", "project", project.ID, map[string]any{
		"name":    project.Name,
		"domains": []string(project.Domains),
	})
	snippet := fmt.Sprintf(`<script>
  window.EveChat = { projectKey: %q, apiUrl: %q };
</script>
<script async src="%s/widget.js"></script>`, project.APIKey, h.Cfg.BaseURL(), h.Cfg.BaseURL())
	writeJSON(w, http.StatusOK, map[string]any{
		"project":       project,
		"api_key":       project.APIKey,
		"embed_snippet": snippet,
	})
	return nil
}

// SetupIntegrations stores (encrypted) integration settings and optionally tests.
func (h *Handlers) SetupIntegrations(w http.ResponseWriter, r *http.Request) error {
	if !h.setupAuthorized(r) {
		return apierr.Unauthorized("مجوز ویزارد نصب را ندارید")
	}
	var in struct {
		Telegram *struct {
			Token  string `json:"token"`
			Secret string `json:"secret"`
		} `json:"telegram"`
		EVE *struct {
			BaseURL string `json:"base_url"`
			APIKey  string `json:"api_key"`
			Secret  string `json:"secret"`
		} `json:"eve"`
		GMweb *struct {
			BaseURL string `json:"base_url"`
			APIKey  string `json:"api_key"`
		} `json:"gmweb"`
		AI *struct {
			APIKey string `json:"api_key"`
			Model  string `json:"model"`
		} `json:"ai"`
		SMTP *struct {
			Host     string `json:"host"`
			Port     int    `json:"port"`
			Username string `json:"username"`
			Password string `json:"password"`
			FromName string `json:"from_name"`
			From     string `json:"from_email"`
			TLSMode  string `json:"tls_mode"`
		} `json:"smtp"`
	}
	if err := bind(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	results := map[string]any{}
	if in.Telegram != nil && in.Telegram.Token != "" {
		_ = h.State.SetSecret(ctx, "integration.telegram.token", in.Telegram.Token)
		_ = h.State.SetSecret(ctx, "integration.telegram.secret", firstNonEmpty(in.Telegram.Secret, id.Token(18)))
		results["telegram"] = testTelegram(ctx, in.Telegram.Token)
	}
	if in.EVE != nil && in.EVE.BaseURL != "" {
		if err := security.Outbound.Validate(ctx, in.EVE.BaseURL); err != nil {
			results["eve"] = map[string]any{"ok": false, "message": "آدرس EVE به مقصد مجاز اشاره نمی‌کند"}
		} else {
			_ = h.State.Set(ctx, "integration.eve.base_url", in.EVE.BaseURL)
			if in.EVE.APIKey != "" {
				_ = h.State.SetSecret(ctx, "integration.eve.api_key", in.EVE.APIKey)
			}
			if in.EVE.Secret != "" {
				_ = h.State.SetSecret(ctx, "integration.eve.hmac_secret", in.EVE.Secret)
			}
			results["eve"] = testEVE(ctx, in.EVE.BaseURL, in.EVE.APIKey)
		}
	}
	if in.GMweb != nil && in.GMweb.BaseURL != "" {
		if err := security.Outbound.Validate(ctx, in.GMweb.BaseURL); err != nil {
			results["gmweb"] = map[string]any{"ok": false, "message": "آدرس درگاه پیامک به مقصد مجاز اشاره نمی‌کند"}
		} else {
			_ = h.State.Set(ctx, "integration.gmweb.base_url", in.GMweb.BaseURL)
			if in.GMweb.APIKey != "" {
				_ = h.State.SetSecret(ctx, "integration.gmweb.api_key", in.GMweb.APIKey)
			}
			results["gmweb"] = testURL(ctx, in.GMweb.BaseURL)
		}
	}
	if in.AI != nil && in.AI.APIKey != "" {
		_ = h.State.SetSecret(ctx, "integration.ai.api_key", in.AI.APIKey)
		if in.AI.Model != "" {
			_ = h.State.Set(ctx, "integration.ai.model", in.AI.Model)
		}
		results["ai"] = testAnthropic(ctx, in.AI.APIKey, firstNonEmpty(in.AI.Model, h.Cfg.AI.Model))
	}
	if in.SMTP != nil && in.SMTP.Host != "" {
		_ = h.State.Set(ctx, "integration.smtp.host", in.SMTP.Host)
		_ = h.State.Set(ctx, "integration.smtp.port", fmt.Sprint(in.SMTP.Port))
		_ = h.State.Set(ctx, "integration.smtp.username", in.SMTP.Username)
		_ = h.State.SetSecret(ctx, "integration.smtp.password", in.SMTP.Password)
		_ = h.State.Set(ctx, "integration.smtp.from_name", in.SMTP.FromName)
		_ = h.State.Set(ctx, "integration.smtp.from_email", in.SMTP.From)
		_ = h.State.Set(ctx, "integration.smtp.tls_mode", firstNonEmpty(in.SMTP.TLSMode, "starttls"))
		client := smtp.New(smtp.Config{Host: in.SMTP.Host, Port: in.SMTP.Port, Username: in.SMTP.Username,
			Password: in.SMTP.Password, FromName: in.SMTP.FromName, FromEmail: in.SMTP.From,
			TLSMode: firstNonEmpty(in.SMTP.TLSMode, "starttls"), Timeout: h.Cfg.SMTPTimeout()})
		if err := client.Check(ctx); err != nil {
			results["smtp"] = map[string]any{"ok": false, "message": err.Error()}
		} else {
			results["smtp"] = map[string]any{"ok": true, "message": "اتصال برقرار شد"}
		}
	}
	if len(results) > 0 {
		kinds := make([]string, 0, len(results))
		for kind := range results {
			kinds = append(kinds, kind)
		}
		h.recordAudit(r, h.State.Value(settingSetupProjectID), "setup.integrations_saved", "settings", "integrations",
			map[string]any{"configured": kinds})
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "results": results})
	return nil
}

// SetupTest tests one integration kind without saving it.
func (h *Handlers) SetupTest(w http.ResponseWriter, r *http.Request) error {
	if !h.setupAuthorized(r) {
		return apierr.Unauthorized("مجوز ویزارد نصب را ندارید")
	}
	kind := chiURLParam(r, "kind")
	var in map[string]any
	_ = json.NewDecoder(r.Body).Decode(&in)
	str := func(k string) string {
		if v, ok := in[k].(string); ok {
			return v
		}
		return ""
	}
	ctx := r.Context()
	var result map[string]any
	switch kind {
	case "telegram":
		result = testTelegram(ctx, str("token"))
	case "eve":
		result = testEVE(ctx, str("base_url"), str("api_key"))
	case "gmweb":
		result = testURL(ctx, str("base_url"))
	case "ai":
		result = testAnthropic(ctx, str("api_key"), firstNonEmpty(str("model"), h.Cfg.AI.Model))
	case "smtp":
		port := 587
		if v, ok := in["port"].(float64); ok {
			port = int(v)
		}
		client := smtp.New(smtp.Config{Host: str("host"), Port: port, Username: str("username"),
			Password: str("password"), FromName: str("from_name"), FromEmail: str("from_email"),
			TLSMode: firstNonEmpty(str("tls_mode"), "starttls"), Timeout: h.Cfg.SMTPTimeout()})
		if err := client.Check(ctx); err != nil {
			result = map[string]any{"ok": false, "message": err.Error()}
		} else {
			result = map[string]any{"ok": true, "message": "اتصال برقرار شد"}
		}
	default:
		return apierr.BadRequest("نوع یکپارچه‌سازی نامعتبر است")
	}
	writeJSON(w, http.StatusOK, result)
	return nil
}

// SetupComplete marks the wizard done and returns a dashboard token.
func (h *Handlers) SetupComplete(w http.ResponseWriter, r *http.Request) error {
	if !h.setupAuthorized(r) {
		return apierr.Unauthorized("مجوز ویزارد نصب را ندارید")
	}
	adminID := h.State.Value(settingSetupAdminID)
	if adminID == "" {
		return apierr.BadRequest("حساب مدیر ساخته نشده است")
	}
	agent, err := h.Repo.Agents.GetByID(r.Context(), adminID)
	if err != nil {
		return apierr.Internal(err)
	}
	if err := h.State.CompleteSetup(r.Context()); err != nil {
		return apierr.Internal(err)
	}
	pair, err := h.Tokens.Issue(agent)
	if err != nil {
		return apierr.Internal(err)
	}
	h.recordAudit(r, agent.ProjectID, "setup.completed", "project", agent.ProjectID, nil)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"access_token":  pair.AccessToken,
		"refresh_token": pair.RefreshToken,
		"redirect":      "/inbox",
	})
	return nil
}

// ---- live integration probes ----

func testTelegram(ctx context.Context, token string) map[string]any {
	if strings.TrimSpace(token) == "" {
		return map[string]any{"ok": false, "message": "توکن ربات لازم است"}
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.telegram.org/bot"+token+"/getMe", nil)
	resp, err := (&http.Client{Timeout: 6 * time.Second}).Do(req)
	if err != nil {
		return map[string]any{"ok": false, "message": "اتصال به تلگرام برقرار نشد"}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return map[string]any{"ok": false, "message": "توکن ربات پذیرفته نشد"}
	}
	var payload struct {
		Result struct {
			Username string `json:"username"`
		} `json:"result"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&payload)
	return map[string]any{"ok": true, "message": "ربات @" + payload.Result.Username + " متصل شد", "username": payload.Result.Username}
}

func testEVE(ctx context.Context, baseURL, apiKey string) map[string]any {
	if baseURL == "" {
		return map[string]any{"ok": false, "message": "آدرس EVE لازم است"}
	}
	if err := security.Outbound.Validate(ctx, baseURL); err != nil {
		return map[string]any{"ok": false, "message": "آدرس EVE به مقصد مجاز اشاره نمی‌کند"}
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/user/0000000000", nil)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := security.Outbound.Client(6 * time.Second).Do(req)
	if err != nil {
		return map[string]any{"ok": false, "message": "EVE پاسخ نداد"}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return map[string]any{"ok": false, "message": "کلید EVE پذیرفته نشد"}
	}
	return map[string]any{"ok": true, "message": "EVE در دسترس است"}
}

func testURL(ctx context.Context, rawURL string) map[string]any {
	if rawURL == "" {
		return map[string]any{"ok": false, "message": "آدرس لازم است"}
	}
	if err := security.Outbound.Validate(ctx, rawURL); err != nil {
		return map[string]any{"ok": false, "message": "آدرس به مقصد مجاز اشاره نمی‌کند"}
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(rawURL, "/"), nil)
	resp, err := security.Outbound.Client(5 * time.Second).Do(req)
	if err != nil {
		return map[string]any{"ok": false, "message": "اتصال برقرار نشد"}
	}
	_ = resp.Body.Close()
	return map[string]any{"ok": true, "message": "در دسترس است"}
}

func testAnthropic(ctx context.Context, apiKey, model string) map[string]any {
	if apiKey == "" {
		return map[string]any{"ok": false, "message": "کلید API لازم است"}
	}
	body := fmt.Sprintf(`{"model":%q,"max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`, model)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", strings.NewReader(body))
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return map[string]any{"ok": false, "message": "اتصال به Anthropic برقرار نشد"}
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return map[string]any{"ok": false, "message": "کلید API پذیرفته نشد"}
	}
	if resp.StatusCode >= 500 {
		return map[string]any{"ok": false, "message": fmt.Sprintf("خطای سرویس (%d)", resp.StatusCode)}
	}
	return map[string]any{"ok": true, "message": "کلید معتبر است"}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
