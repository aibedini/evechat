package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Config is the whole application configuration. Precedence:
// defaults -> yaml file -> environment (EVECHAT_ prefix, "__" = nesting).
type Config struct {
	Server struct {
		Host            string   `koanf:"host"`
		Port            int      `koanf:"port"`
		PublicURL       string   `koanf:"public_url"`
		TrustProxy      bool     `koanf:"trust_proxy"`
		TrustedProxies  []string `koanf:"trusted_proxies"`
		ReadTimeout     string   `koanf:"read_timeout"`
		WriteTimeout    string   `koanf:"write_timeout"`
		ShutdownTimeout string   `koanf:"shutdown_timeout"`
		TLS             struct {
			Enabled    bool   `koanf:"enabled"`
			CertFile   string `koanf:"cert_file"`
			KeyFile    string `koanf:"key_file"`
			ACMEDomain string `koanf:"acme_domain"`
			ACMECache  string `koanf:"acme_cache"`
		} `koanf:"tls"`
	} `koanf:"server"`

	Database struct {
		Path          string `koanf:"path"`
		MaxOpenConns  int    `koanf:"max_open_conns"`
		MaxReadConns  int    `koanf:"max_read_conns"`
		BusyTimeoutMS int    `koanf:"busy_timeout_ms"`
	} `koanf:"database"`

	Storage struct {
		UploadsDir    string `koanf:"uploads_dir"`
		MaxFileSizeMB int    `koanf:"max_file_size_mb"`
	} `koanf:"storage"`

	Auth struct {
		JWTSecret     string `koanf:"jwt_secret"`
		AccessExpiry  string `koanf:"access_expiry"`
		RefreshExpiry string `koanf:"refresh_expiry"`
		BcryptCost    int    `koanf:"bcrypt_cost"`
	} `koanf:"auth"`

	Encryption struct {
		CurrentVersion int `koanf:"current_version"`
	} `koanf:"encryption"`

	Hub struct {
		PingIntervalSeconds int `koanf:"ping_interval_seconds"`
		PongWaitSeconds     int `koanf:"pong_wait_seconds"`
		SendBufferSize      int `koanf:"send_buffer_size"`
		RingBufferSize      int `koanf:"ring_buffer_size"`
		MaxMessageBytes     int `koanf:"max_message_bytes"`
	} `koanf:"hub"`

	RateLimit struct {
		WidgetSessionPerMin int `koanf:"widget_session_per_min"`
		MessagePerMin       int `koanf:"message_per_min"`
		LoginPerMin         int `koanf:"login_per_min"`
	} `koanf:"ratelimit"`

	// Security tunes abuse protection. Blocklists are also editable at runtime
	// through app_settings; these values are the boot defaults.
	Security struct {
		POWDifficulty           int      `koanf:"pow_difficulty"`
		MessagesPerConversation int      `koanf:"messages_per_conversation_per_min"`
		ConversationsPerHour    int      `koanf:"conversations_per_visitor_per_hour"`
		BlockedIPs              []string `koanf:"blocked_ips"`
		BlockedCIDRs            []string `koanf:"blocked_cidrs"`
		BlockedUserAgents       []string `koanf:"blocked_user_agents"`
		// OutboundAllowHosts lists internal hosts (webhook receivers, EVE and
		// GMweb gateways) that outbound HTTP is allowed to reach even though
		// they resolve to private or loopback addresses.
		OutboundAllowHosts []string `koanf:"outbound_allow_hosts"`
		// AllowPrivateOutbound disables the outbound address filter entirely.
		// Only for isolated networks that only talk to internal services.
		AllowPrivateOutbound bool `koanf:"allow_private_outbound"`
	} `koanf:"security"`

	Metrics struct {
		Enabled bool   `koanf:"enabled"`
		Token   string `koanf:"token"`
	} `koanf:"metrics"`

	Audit struct {
		RetentionDays int `koanf:"retention_days"`
	} `koanf:"audit"`

	Telegram struct {
		BotToken   string `koanf:"bot_token"`
		WebhookURL string `koanf:"webhook_url"`
		Secret     string `koanf:"secret"`
		Enabled    bool   `koanf:"enabled"`
	} `koanf:"telegram"`

	Integrations struct {
		EVE   EVEConfig   `koanf:"eve"`
		GMweb GMwebConfig `koanf:"gmweb"`
	} `koanf:"integrations"`

	SMTP     SMTPConfig     `koanf:"smtp"`
	Firebase FirebaseConfig `koanf:"firebase"`

	AI struct {
		Enabled      bool   `koanf:"enabled"`
		ClaudeAPIKey string `koanf:"claude_api_key"`
		Model        string `koanf:"model"`
		MaxTokens    int    `koanf:"max_tokens"`
		HandoffAfter int    `koanf:"handoff_after"`
	} `koanf:"ai"`

	Log struct {
		Level      string `koanf:"level"`
		Pretty     bool   `koanf:"pretty"`
		File       string `koanf:"file"`
		MaxSizeMB  int    `koanf:"max_size_mb"`
		MaxBackups int    `koanf:"max_backups"`
	} `koanf:"log"`
}

// EVEConfig configures the EVE mobile-platform integration.
type EVEConfig struct {
	BaseURL          string `koanf:"base_url"`
	APIKey           string `koanf:"api_key"`
	HMACSecret       string `koanf:"hmac_secret"`
	RequireSignature bool   `koanf:"require_signature"`
}

// GMwebConfig configures the GMweb SMS gateway.
type GMwebConfig struct {
	BaseURL       string `koanf:"base_url"`
	APIKey        string `koanf:"api_key"`
	WebhookSecret string `koanf:"webhook_secret"`
}

// FirebaseConfig enables FCM push for the Android agent app. The service
// account JSON is secret; keep it in a file or in FIREBASE_SERVICE_ACCOUNT_JSON
// outside the repository. Push stays optional: without credentials the
// registration endpoints still work and notifications are skipped.
type FirebaseConfig struct {
	Enabled            bool   `koanf:"enabled"`
	ProjectID          string `koanf:"project_id"`
	ServiceAccountJSON string `koanf:"service_account_json"`
	ServiceAccountFile string `koanf:"service_account_file"`
	BaseURL            string `koanf:"base_url"`
	TokenURL           string `koanf:"token_url"`
}

// SMTPConfig configures the email notification channel.
type SMTPConfig struct {
	Enabled   bool   `koanf:"enabled"`
	Host      string `koanf:"host"`
	Port      int    `koanf:"port"`
	Username  string `koanf:"username"`
	Password  string `koanf:"password"`
	FromName  string `koanf:"from_name"`
	FromEmail string `koanf:"from_email"`
	TLSMode   string `koanf:"tls_mode"` // plain | starttls | implicit
	Timeout   string `koanf:"timeout"`
}

func defaults() *Config {
	c := &Config{}
	c.Server.Host = "0.0.0.0"
	c.Server.Port = 3000
	c.Server.PublicURL = "http://localhost:3000"
	c.Server.TrustProxy = false
	c.Server.TrustedProxies = []string{"127.0.0.1/32", "::1/128"}
	c.Server.ReadTimeout = "15s"
	c.Server.WriteTimeout = "15s"
	c.Server.ShutdownTimeout = "20s"
	c.Server.TLS.ACMECache = "/data/acme"

	c.Database.Path = "/data/evechat.db"
	c.Database.MaxOpenConns = 1
	c.Database.MaxReadConns = 4
	c.Database.BusyTimeoutMS = 5000

	c.Storage.UploadsDir = "/data/uploads"
	c.Storage.MaxFileSizeMB = 10

	c.Auth.AccessExpiry = "24h"
	c.Auth.RefreshExpiry = "720h"
	c.Auth.BcryptCost = 12

	c.Encryption.CurrentVersion = 1

	c.Hub.PingIntervalSeconds = 30
	c.Hub.PongWaitSeconds = 90
	c.Hub.SendBufferSize = 256
	c.Hub.RingBufferSize = 50
	c.Hub.MaxMessageBytes = 32768

	c.RateLimit.WidgetSessionPerMin = 10
	c.RateLimit.MessagePerMin = 60
	c.RateLimit.LoginPerMin = 5

	c.Security.POWDifficulty = 16
	c.Security.MessagesPerConversation = 30
	c.Security.ConversationsPerHour = 5

	c.Metrics.Enabled = false
	c.Audit.RetentionDays = 180

	c.SMTP.Port = 587
	c.SMTP.TLSMode = "starttls"
	c.SMTP.Timeout = "10s"
	c.SMTP.FromName = "EveChat"

	// Push is opt-in; credentials come from FIREBASE_* or firebase.* config.

	c.AI.Model = "claude-haiku-4-5"
	c.AI.MaxTokens = 512
	c.AI.HandoffAfter = 3

	c.Log.Level = "info"
	c.Log.MaxSizeMB = 50
	c.Log.MaxBackups = 5
	return c
}

// Default returns a fully populated default configuration.
func Default() *Config { return defaults() }

// Load reads defaults, then the optional yaml file, then EVECHAT_ env vars.
func Load(path string) (*Config, error) {
	cfg := defaults()
	k := koanf.New(".")

	if path != "" {
		if _, err := os.Stat(path); err == nil {
			if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
				return nil, fmt.Errorf("loading config file %s: %w", path, err)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("stat config file %s: %w", path, err)
		}
	}

	if err := k.Load(env.Provider("EVECHAT_", ".", func(s string) string {
		s = strings.TrimPrefix(s, "EVECHAT_")
		return strings.ToLower(strings.ReplaceAll(s, "__", "."))
	}), nil); err != nil {
		return nil, fmt.Errorf("loading env config: %w", err)
	}

	if err := k.Unmarshal("", cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}
	// List values arrive comma separated from the environment.
	cfg.Security.BlockedIPs = splitCSV(cfg.Security.BlockedIPs)
	cfg.Security.BlockedCIDRs = splitCSV(cfg.Security.BlockedCIDRs)
	cfg.Security.BlockedUserAgents = splitCSV(cfg.Security.BlockedUserAgents)
	cfg.Security.OutboundAllowHosts = splitCSV(cfg.Security.OutboundAllowHosts)
	cfg.Server.TrustedProxies = splitCSV(cfg.Server.TrustedProxies)
	if len(cfg.Server.TrustedProxies) == 0 {
		// An explicitly emptied list must not silently trust every private or
		// loopback peer; keep the loopback-only default instead.
		cfg.Server.TrustedProxies = []string{"127.0.0.1/32", "::1/128"}
	}
	return cfg, nil
}

func splitCSV(in []string) []string {
	out := make([]string, 0, len(in))
	for _, raw := range in {
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

// Validate hard-fails on insecure or unusable configuration.
func (c *Config) Validate() error {
	if c.Auth.JWTSecret == "" || c.Auth.JWTSecret == "CHANGE_ME" {
		return errors.New("auth.jwt_secret must be set to a strong random value")
	}
	if len(c.Auth.JWTSecret) < 24 {
		return errors.New("auth.jwt_secret must be at least 24 characters")
	}
	keys, err := c.EncryptionKeys()
	if err != nil {
		return err
	}
	if _, ok := keys[c.Encryption.CurrentVersion]; !ok {
		return fmt.Errorf("no encryption key for current version %d (set EVECHAT_ENCRYPTION_KEY_V%d)", c.Encryption.CurrentVersion, c.Encryption.CurrentVersion)
	}
	if c.Database.Path != ":memory:" && !strings.HasPrefix(c.Database.Path, "file::memory:") {
		dir := filepath.Dir(c.Database.Path)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("database directory %s is not writable: %w", dir, err)
		}
		probe := filepath.Join(dir, ".write-probe")
		if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
			return fmt.Errorf("database directory %s is not writable: %w", dir, err)
		}
		_ = os.Remove(probe)
	}
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port %d is invalid", c.Server.Port)
	}
	for _, raw := range c.Server.TrustedProxies {
		if !validProxyAddress(raw) {
			return fmt.Errorf("server.trusted_proxies: %q is not a valid IP or CIDR", raw)
		}
	}
	return nil
}

// validProxyAddress accepts a CIDR block or a bare IPv4/IPv6 address. A bare
// address is treated as a single-host block by the middleware.
func validProxyAddress(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	if _, _, err := net.ParseCIDR(raw); err == nil {
		return true
	}
	return net.ParseIP(strings.Trim(raw, "[]")) != nil
}

// EncryptionKeys reads EVECHAT_ENCRYPTION_KEY_V1..V9 (base64, 32 bytes each).
func (c *Config) EncryptionKeys() (map[int][]byte, error) {
	keys := make(map[int][]byte, 3)
	for i := 1; i <= 9; i++ {
		raw := os.Getenv("EVECHAT_ENCRYPTION_KEY_V" + strconv.Itoa(i))
		if raw == "" {
			continue
		}
		b, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			b, err = base64.RawStdEncoding.DecodeString(raw)
		}
		if err != nil {
			return nil, fmt.Errorf("EVECHAT_ENCRYPTION_KEY_V%d is not valid base64: %w", i, err)
		}
		if len(b) != 32 {
			return nil, fmt.Errorf("EVECHAT_ENCRYPTION_KEY_V%d must decode to 32 bytes, got %d", i, len(b))
		}
		keys[i] = b
	}
	return keys, nil
}

func (c *Config) dur(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}

func (c *Config) ReadTimeout() time.Duration  { return c.dur(c.Server.ReadTimeout, 15*time.Second) }
func (c *Config) WriteTimeout() time.Duration { return c.dur(c.Server.WriteTimeout, 15*time.Second) }
func (c *Config) ShutdownTimeout() time.Duration {
	return c.dur(c.Server.ShutdownTimeout, 20*time.Second)
}
func (c *Config) AccessExpiry() time.Duration  { return c.dur(c.Auth.AccessExpiry, 24*time.Hour) }
func (c *Config) RefreshExpiry() time.Duration { return c.dur(c.Auth.RefreshExpiry, 720*time.Hour) }

// SMTPTimeout is the outbound mail timeout.
func (c *Config) SMTPTimeout() time.Duration { return c.dur(c.SMTP.Timeout, 10*time.Second) }

// BaseURL is server.public_url without a trailing slash.
func (c *Config) BaseURL() string { return strings.TrimRight(c.Server.PublicURL, "/") }

// Addr is the listen address.
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// DataDir is the directory that holds the database.
func (c *Config) DataDir() string {
	if c.Database.Path == "" || strings.HasPrefix(c.Database.Path, "file:") || c.Database.Path == ":memory:" {
		return "."
	}
	return filepath.Dir(c.Database.Path)
}
