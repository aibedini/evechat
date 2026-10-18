package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/evechat/evechat/internal/config"
)

func TestIsLoopbackIP(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1":        true,
		"::1":              true,
		"localhost":        true,
		" 127.0.0.2  ":     true,
		"::ffff:127.0.0.1": true,
		"203.0.113.9":      false,
		"10.0.0.1":         false,
		"":                 false,
		"not-an-ip":        false,
	}
	for raw, want := range cases {
		if got := isLoopbackIP(raw); got != want {
			t.Errorf("isLoopbackIP(%q) = %v, want %v", raw, got, want)
		}
	}
}

// TestDemoTestRejectsSpoofedLoopback makes sure the /demo/test gate cannot be
// unlocked with a proxy header when trust_proxy is off.
func TestDemoTestRejectsSpoofedLoopback(t *testing.T) {
	h := &Handlers{Cfg: config.Default()}
	r := httptest.NewRequest("GET", "/demo/test", nil)
	r.RemoteAddr = "203.0.113.9:41234"
	r.Header.Set("X-Forwarded-For", "127.0.0.1")
	w := httptest.NewRecorder()
	if err := h.DemoTest(w, r); err != nil {
		t.Fatalf("DemoTest returned an error: %v", err)
	}
	if w.Code != 404 {
		t.Fatalf("status = %d, want 404 (and not 403, which would confirm the route)", w.Code)
	}
}
