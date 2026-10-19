package doctor

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/config"
)

func checkByID(t *testing.T, cfg *config.Config, id string) Check {
	t.Helper()
	for _, c := range checks(cfg, "", zerolog.Nop()) {
		if c.ID == id {
			return c
		}
	}
	t.Fatalf("check %q not found", id)
	return Check{}
}

func TestNetBindWarnsOnWideBindWithTrustProxy(t *testing.T) {
	cfg := config.Default()
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.TrustProxy = true
	if res := checkByID(t, cfg, "net.bind").Run(context.Background()); res.OK {
		t.Fatalf("wide bind + trust_proxy should warn, got %+v", res)
	}

	cfg.Server.Host = "127.0.0.1"
	if res := checkByID(t, cfg, "net.bind").Run(context.Background()); !res.OK {
		t.Fatalf("loopback bind should pass, got %+v", res)
	}

	cfg.Server.Host = "0.0.0.0"
	cfg.Server.TrustProxy = false
	if res := checkByID(t, cfg, "net.bind").Run(context.Background()); !res.OK {
		t.Fatalf("wide bind without trust_proxy should pass, got %+v", res)
	}
}

func TestNetHTTPSWarnsOnPlainPublicURL(t *testing.T) {
	cfg := config.Default()
	cfg.Server.PublicURL = "http://chat.example.com"
	if res := checkByID(t, cfg, "net.https").Run(context.Background()); res.OK {
		t.Fatalf("plain public URL should warn, got %+v", res)
	}

	cfg.Server.PublicURL = "https://chat.example.com"
	if res := checkByID(t, cfg, "net.https").Run(context.Background()); !res.OK {
		t.Fatalf("https public URL should pass, got %+v", res)
	}

	cfg.Server.PublicURL = "http://127.0.0.1:3000"
	if res := checkByID(t, cfg, "net.https").Run(context.Background()); !res.OK {
		t.Fatalf("loopback http should pass, got %+v", res)
	}
}

func TestNetProxySkipsLocalhost(t *testing.T) {
	cfg := config.Default()
	cfg.Server.PublicURL = "http://127.0.0.1:3000"
	res := checkByID(t, cfg, "net.proxy").Run(context.Background())
	if !res.OK || !strings.Contains(res.Message, "localhost") {
		t.Fatalf("localhost proxy check = %+v, want a skip", res)
	}
}

func TestWidgetDomainsWarnsWithoutDomains(t *testing.T) {
	cfg := config.Default()
	cfg.Database.Path = filepath.Join(t.TempDir(), "evechat.db")
	res := checkByID(t, cfg, "widget.domains").Run(context.Background())
	if res.OK {
		t.Fatalf("no domains should warn, got %+v", res)
	}
	if !strings.Contains(res.Message, "آزمایشی") && !strings.Contains(res.Message, "دامنه") {
		t.Fatalf("unexpected message: %+v", res)
	}
}

func TestNetPortsIsInformational(t *testing.T) {
	cfg := config.Default()
	res := checkByID(t, cfg, "net.ports").Run(context.Background())
	if !res.OK {
		t.Fatalf("net.ports must never fail, got %+v", res)
	}
	if !strings.Contains(res.Message, "آزاد") && !strings.Contains(res.Message, "استفاده") {
		t.Fatalf("unexpected message: %+v", res)
	}
}
