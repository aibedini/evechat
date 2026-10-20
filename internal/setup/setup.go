// Package setup implements the first-run installation logic shared by the CLI
// wizard (`evechat setup`) and the browser wizard (`/api/setup/*`).
package setup

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"

	"github.com/evechat/evechat/internal/config"
	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

// Plan is everything the setup flow needs to know.
type Plan struct {
	ConfigPath     string
	DataDir        string
	BindHost       string
	Port           int
	PublicURL      string
	TrustProxy     bool
	TrustedProxies []string
	JWTSecret      string
	EncryptionKey  string
	AdminName      string
	AdminEmail     string
	AdminPassword  string
	ProjectName    string
	Domains        []string

	TelegramToken   string
	TelegramSecret  string
	TelegramEnabled bool
	EVEBaseURL      string
	EVEAPIKey       string
	EVESecret       string
	GMwebBaseURL    string
	GMwebAPIKey     string
	AIEnabled       bool
	AIKey           string
	AIModel         string
	SMTPEnabled     bool
	SMTPHost        string
	SMTPPort        int
	SMTPUser        string
	SMTPPassword    string
	SMTPFrom        string
	SMTPFromName    string
	SMTPTLSMode     string
}

// Result is the outcome printed by the CLI and returned by the web wizard.
type Result struct {
	AlreadyConfigured bool   `json:"already_configured"`
	AdminName         string `json:"admin_name"`
	AdminEmail        string `json:"admin_email"`
	AdminPassword     string `json:"admin_password,omitempty"`
	ProjectID         string `json:"project_id"`
	ProjectName       string `json:"project_name"`
	ProjectKey        string `json:"project_key"`
	DashboardURL      string `json:"dashboard_url"`
	EmbedSnippet      string `json:"embed_snippet"`
	ConfigPath        string `json:"config_path"`
	DataDir           string `json:"data_dir"`
	WroteEnv          bool   `json:"wrote_env"`
	Created           bool   `json:"created"`
}

// DefaultPlan derives a plan from the current configuration.
func DefaultPlan(cfg *config.Config, configPath string) Plan {
	p := Plan{
		ConfigPath:     configPath,
		DataDir:        cfg.DataDir(),
		BindHost:       cfg.Server.Host,
		Port:           cfg.Server.Port,
		PublicURL:      cfg.BaseURL(),
		TrustProxy:     cfg.Server.TrustProxy,
		TrustedProxies: append([]string(nil), cfg.Server.TrustedProxies...),
		AdminName:      "مدیر سیستم",
		AdminEmail:     "admin@evechat.local",
		ProjectName:    "EVE",
		AIModel:        cfg.AI.Model,
		SMTPPort:       587,
		SMTPTLSMode:    "starttls",
	}
	if p.PublicURL == "" {
		p.PublicURL = fmt.Sprintf("http://localhost:%d", p.Port)
	}
	if strings.TrimSpace(p.BindHost) == "" {
		p.BindHost = "0.0.0.0"
	}
	return p
}

// RandomSecret returns n random bytes as base64.
func RandomSecret(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d-%d", time.Now().UnixNano(), n)))
	}
	return base64.StdEncoding.EncodeToString(b)
}

// RandomPassword returns a readable 16 character password.
func RandomPassword() string {
	const alphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#$"
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return id.Token(12)
	}
	out := make([]byte, len(b))
	for i, v := range b {
		out[i] = alphabet[int(v)%len(alphabet)]
	}
	return string(out)
}

// GenerateSecrets fills the secret fields when missing.
func (p *Plan) GenerateSecrets() {
	if p.JWTSecret == "" {
		p.JWTSecret = RandomSecret(48)
	}
	if p.EncryptionKey == "" {
		p.EncryptionKey = RandomSecret(32)
	}
	if p.AdminPassword == "" {
		p.AdminPassword = RandomPassword()
	}
}

// Apply writes configuration, secrets and the first project/admin.
func Apply(ctx context.Context, p Plan) (*Result, error) {
	if p.ConfigPath == "" {
		p.ConfigPath = "./evechat.yaml"
	}
	if p.AdminEmail == "" || !strings.Contains(p.AdminEmail, "@") {
		return nil, errors.New("ایمیل مدیر معتبر نیست")
	}
	if p.AdminPassword == "" {
		p.AdminPassword = RandomPassword()
	}
	if p.JWTSecret == "" || p.EncryptionKey == "" {
		p.GenerateSecrets()
	}
	if p.DataDir == "" {
		p.DataDir = filepath.Dir(p.ConfigPath)
	}
	if err := os.MkdirAll(p.DataDir, 0o750); err != nil {
		return nil, fmt.Errorf("creating data dir: %w", err)
	}

	wroteEnv, err := writeEnv(p)
	if err != nil {
		return nil, err
	}
	// Make the new secrets visible to this process before loading the config.
	_ = os.Setenv("EVECHAT_AUTH__JWT_SECRET", p.JWTSecret)
	_ = os.Setenv("EVECHAT_ENCRYPTION_KEY_V1", p.EncryptionKey)
	if err := writeYAML(p); err != nil {
		return nil, err
	}

	cfg, err := config.Load(p.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("reloading generated config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	d, err := db.Open(cfg)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	if err := db.Migrate(ctx, d, zerolog.Nop()); err != nil {
		return nil, err
	}
	r := repo.New(d)

	existing, err := r.Agents.CountSuperadmins(ctx)
	if err != nil {
		return nil, err
	}
	baseURL := strings.TrimRight(p.PublicURL, "/")
	res := &Result{
		AlreadyConfigured: existing > 0,
		AdminName:         p.AdminName,
		AdminEmail:        p.AdminEmail,
		DashboardURL:      baseURL,
		ConfigPath:        p.ConfigPath,
		DataDir:           p.DataDir,
		WroteEnv:          wroteEnv,
	}
	if existing > 0 {
		// Never change anything when the instance is already configured.
		if project, perr := r.Projects.List(ctx); perr == nil && len(project) > 0 {
			res.ProjectID = project[0].ID
			res.ProjectName = project[0].Name
			res.ProjectKey = project[0].APIKey
			res.EmbedSnippet = EmbedSnippet(baseURL, project[0].APIKey)
		}
		return res, nil
	}

	project := &models.Project{
		ID:        id.NewPrefixed(id.PrefixProject),
		Name:      firstNonEmpty(p.ProjectName, "EVE"),
		APIKey:    id.NewPrefixed("pk"),
		SecretKey: id.Token(32),
		Domains:   models.StringList(p.Domains),
		Settings: models.JSONMap{
			"greeting":        "سلام 👋 چطور می‌توانیم کمک کنیم؟",
			"offline_message": "در حال حاضر آفلاین هستیم؛ پیام بگذارید تا در اولین فرصت پاسخ دهیم.",
			"position":        "bottom-left",
			"colors":          models.JSONMap{"primary": "#2563eb"},
		},
		IsActive: true,
	}
	if err := r.Projects.Create(ctx, project); err != nil {
		return nil, fmt.Errorf("creating project: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(p.AdminPassword), cfg.Auth.BcryptCost)
	if err != nil {
		return nil, err
	}
	admin := &models.Agent{
		ID:               id.NewPrefixed(id.PrefixAgent),
		ProjectID:        project.ID,
		Email:            strings.ToLower(strings.TrimSpace(p.AdminEmail)),
		PasswordHash:     string(hash),
		Name:             firstNonEmpty(p.AdminName, "مدیر سیستم"),
		Role:             models.RoleSuperAdmin,
		MaxConversations: 50,
		IsActive:         true,
	}
	if err := r.Agents.Create(ctx, admin); err != nil {
		return nil, fmt.Errorf("creating admin: %w", err)
	}
	_ = r.SaaS.SetTenantPlan(ctx, project.ID, "free")
	_ = r.Settings.Set(ctx, "setup_completed", "1")
	_ = r.Settings.Set(ctx, "setup_completed_at", time.Now().UTC().Format(time.RFC3339))

	res.Created = true
	res.AdminPassword = p.AdminPassword
	res.ProjectID = project.ID
	res.ProjectName = project.Name
	res.ProjectKey = project.APIKey
	res.EmbedSnippet = EmbedSnippet(baseURL, project.APIKey)

	_ = os.WriteFile(filepath.Join(p.DataDir, "embed-snippet.html"), []byte(res.EmbedSnippet), 0o640)
	summary := fmt.Sprintf(`EveChat setup summary
====================
Dashboard : %s
Email     : %s
Project   : %s
API key   : %s
Config    : %s
Data dir  : %s
`, baseURL, res.AdminEmail, project.ID, project.APIKey, p.ConfigPath, p.DataDir)
	_ = os.WriteFile(filepath.Join(p.DataDir, "setup-summary.txt"), []byte(summary), 0o600)
	return res, nil
}

// EmbedSnippet is the ready-to-paste widget snippet.
func EmbedSnippet(baseURL, projectKey string) string {
	return fmt.Sprintf(`<script>
  window.EveChat = { projectKey: %q, apiUrl: %q };
</script>
<script async src="%s/widget.js"></script>`, projectKey, baseURL, baseURL)
}

func writeEnv(p Plan) (bool, error) {
	dir := p.DataDir
	if dir == "" {
		dir = filepath.Dir(p.ConfigPath)
	}
	path := filepath.Join(dir, ".env")
	if _, err := os.Stat(path); err == nil {
		return false, nil
	}
	var b strings.Builder
	b.WriteString("# Generated by evechat setup — keep this file private (chmod 600).\n")
	fmt.Fprintf(&b, "EVECHAT_AUTH__JWT_SECRET=%s\n", p.JWTSecret)
	fmt.Fprintf(&b, "EVECHAT_ENCRYPTION_KEY_V1=%s\n", p.EncryptionKey)
	if p.TelegramToken != "" {
		fmt.Fprintf(&b, "EVECHAT_TELEGRAM__BOT_TOKEN=%s\n", p.TelegramToken)
		fmt.Fprintf(&b, "EVECHAT_TELEGRAM__SECRET=%s\n", firstNonEmpty(p.TelegramSecret, RandomSecret(24)))
		fmt.Fprintf(&b, "EVECHAT_TELEGRAM__WEBHOOK_URL=%s/api/v1/integrations/telegram/webhook\n", strings.TrimRight(p.PublicURL, "/"))
		fmt.Fprintf(&b, "EVECHAT_TELEGRAM__ENABLED=true\n")
	}
	if p.EVEAPIKey != "" {
		fmt.Fprintf(&b, "EVECHAT_INTEGRATIONS__EVE__API_KEY=%s\n", p.EVEAPIKey)
		fmt.Fprintf(&b, "EVECHAT_INTEGRATIONS__EVE__HMAC_SECRET=%s\n", p.EVESecret)
	}
	if p.GMwebAPIKey != "" {
		fmt.Fprintf(&b, "EVECHAT_INTEGRATIONS__GMWEB__API_KEY=%s\n", p.GMwebAPIKey)
	}
	if p.AIKey != "" {
		fmt.Fprintf(&b, "EVECHAT_AI__CLAUDE_API_KEY=%s\n", p.AIKey)
		fmt.Fprintf(&b, "EVECHAT_AI__ENABLED=true\n")
	}
	if p.SMTPPassword != "" {
		fmt.Fprintf(&b, "EVECHAT_SMTP__PASSWORD=%s\n", p.SMTPPassword)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return false, fmt.Errorf("writing .env: %w", err)
	}
	_ = os.Chmod(path, 0o600)
	return true, nil
}

func writeYAML(p Plan) error {
	port := p.Port
	if port == 0 {
		port = 3000
	}
	publicURL := p.PublicURL
	if publicURL == "" {
		publicURL = fmt.Sprintf("http://localhost:%d", port)
	}
	dataDir := p.DataDir
	if dataDir == "" {
		dataDir = "."
	}
	domains := "[]"
	if len(p.Domains) > 0 {
		quoted := make([]string, 0, len(p.Domains))
		for _, d := range p.Domains {
			quoted = append(quoted, fmt.Sprintf("%q", d))
		}
		domains = "[" + strings.Join(quoted, ", ") + "]"
	}
	tgEnabled := "false"
	if p.TelegramToken != "" {
		tgEnabled = "true"
	}
	aiEnabled := "false"
	if p.AIKey != "" {
		aiEnabled = "true"
	}
	smtpEnabled := "false"
	if p.SMTPEnabled && p.SMTPHost != "" {
		smtpEnabled = "true"
	}
	smtpPort := p.SMTPPort
	if smtpPort == 0 {
		smtpPort = 587
	}
	smtpTLS := firstNonEmpty(p.SMTPTLSMode, "starttls")
	aiModel := firstNonEmpty(p.AIModel, "claude-haiku-4-5")
	bindHost := firstNonEmpty(p.BindHost, "0.0.0.0")
	trustedProxies := ""
	if p.TrustProxy {
		proxies := p.TrustedProxies
		if len(proxies) == 0 {
			proxies = []string{"127.0.0.1/32", "::1/128"}
		}
		var b strings.Builder
		b.WriteString("  trusted_proxies:\n")
		for _, cidr := range proxies {
			fmt.Fprintf(&b, "    - %q\n", cidr)
		}
		trustedProxies = b.String()
	}

	yaml := fmt.Sprintf(`# Generated by evechat setup on %s
server:
  host: %q
  port: %d
  public_url: %q
  trust_proxy: %t
%s  read_timeout: "15s"
  shutdown_timeout: "20s"

database:
  path: %q
  max_open_conns: 1
  max_read_conns: 4
  busy_timeout_ms: 5000

storage:
  uploads_dir: %q
  max_file_size_mb: 10

auth:
  access_expiry: "24h"
  refresh_expiry: "720h"
  bcrypt_cost: 12

encryption:
  current_version: 1

hub:
  ping_interval_seconds: 30
  pong_wait_seconds: 90
  send_buffer_size: 256
  ring_buffer_size: 50
  max_message_bytes: 32768

ratelimit:
  widget_session_per_min: 10
  message_per_min: 60
  login_per_min: 5

security:
  pow_difficulty: 16
  messages_per_conversation_per_min: 30
  conversations_per_visitor_per_hour: 5

audit:
  retention_days: 180

telegram:
  enabled: %s
  webhook_url: %q

integrations:
  eve:
    base_url: %q
    require_signature: false
  gmweb:
    base_url: %q

smtp:
  enabled: %s
  host: %q
  port: %d
  username: %q
  from_name: %q
  from_email: %q
  tls_mode: %q

ai:
  enabled: %s
  model: %q
  max_tokens: 512
  handoff_after: 3

log:
  level: "info"
  pretty: false
  max_size_mb: 50
  max_backups: 5
`, time.Now().UTC().Format(time.RFC3339), bindHost, port, publicURL, p.TrustProxy, trustedProxies,
		filepath.Join(dataDir, "evechat.db"), filepath.Join(dataDir, "uploads"),
		tgEnabled, strings.TrimRight(publicURL, "/")+"/api/v1/integrations/telegram/webhook",
		p.EVEBaseURL, p.GMwebBaseURL,
		smtpEnabled, p.SMTPHost, smtpPort, p.SMTPUser, firstNonEmpty(p.SMTPFromName, "EveChat"), p.SMTPFrom, smtpTLS,
		aiEnabled, aiModel)

	if err := os.WriteFile(p.ConfigPath, []byte(yaml), 0o640); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	_ = os.WriteFile(filepath.Join(filepath.Dir(p.ConfigPath), "domains.txt"), []byte(strings.Join(p.Domains, "\n")), 0o640)
	_ = domains
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// LoadDotEnv exports KEY=VALUE pairs from a .env file without overriding the
// existing environment.
func LoadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}
