package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/evechat/evechat/internal/config"
)

func TestWriteYAMLKeepsBindHostAndTrustedProxies(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "evechat.yaml")

	plan := Plan{
		ConfigPath:     cfgPath,
		DataDir:        dir,
		BindHost:       "127.0.0.1",
		Port:           3110,
		PublicURL:      "https://chat.example.com",
		TrustProxy:     true,
		TrustedProxies: []string{"127.0.0.1/32", "10.0.0.0/8"},
		AdminEmail:     "admin@example.com",
	}
	if err := writeYAML(plan); err != nil {
		t.Fatalf("writeYAML: %v", err)
	}

	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(raw)
	for _, want := range []string{
		`host: "127.0.0.1"`,
		`port: 3110`,
		`public_url: "https://chat.example.com"`,
		`trust_proxy: true`,
		`trusted_proxies:`,
		`- "127.0.0.1/32"`,
		`- "10.0.0.0/8"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("generated config is missing %q:\n%s", want, text)
		}
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Fatalf("server.host = %q, want 127.0.0.1", cfg.Server.Host)
	}
	if !cfg.Server.TrustProxy {
		t.Fatal("server.trust_proxy = false, want true")
	}
	if len(cfg.Server.TrustedProxies) != 2 ||
		cfg.Server.TrustedProxies[0] != "127.0.0.1/32" ||
		cfg.Server.TrustedProxies[1] != "10.0.0.0/8" {
		t.Fatalf("trusted_proxies = %v, want the plan's list", cfg.Server.TrustedProxies)
	}
}

func TestDefaultPlanCopiesBindHost(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 3100
	cfg.Server.TrustedProxies = []string{"127.0.0.1/32", "10.0.0.0/8"}

	plan := DefaultPlan(cfg, "/tmp/evechat.yaml")
	if plan.BindHost != "127.0.0.1" {
		t.Fatalf("BindHost = %q, want 127.0.0.1", plan.BindHost)
	}
	if plan.Port != 3100 {
		t.Fatalf("Port = %d, want 3100", plan.Port)
	}
	if len(plan.TrustedProxies) != 2 || plan.TrustedProxies[1] != "10.0.0.0/8" {
		t.Fatalf("TrustedProxies = %v, want the config list", plan.TrustedProxies)
	}
}
