package handlers

import (
	"net/http/httptest"
	"testing"

	"github.com/evechat/evechat/internal/config"
)

// TestSetupAuthorizedRejectsSpoofedLoopback covers the pre-auth bypass that
// chi's RealIP middleware created: a remote caller could send
// "X-Forwarded-For: 127.0.0.1" and be treated as local, which unlocks the
// first-run wizard (and would allow creating the first superadmin).
func TestSetupAuthorizedRejectsSpoofedLoopback(t *testing.T) {
	h := &Handlers{Cfg: config.Default()} // trust_proxy defaults to false

	remote := httptest.NewRequest("GET", "/api/setup/status", nil)
	remote.RemoteAddr = "203.0.113.9:41234"
	remote.Header.Set("X-Forwarded-For", "127.0.0.1")
	remote.Header.Set("X-Real-IP", "127.0.0.1")
	if h.setupAuthorized(remote) {
		t.Fatal("a remote request with spoofed proxy headers was treated as loopback")
	}

	local := httptest.NewRequest("GET", "/api/setup/status", nil)
	local.RemoteAddr = "127.0.0.1:41234"
	if !h.setupAuthorized(local) {
		t.Fatal("a real loopback request was rejected")
	}
}
