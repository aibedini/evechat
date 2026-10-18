package router

import (
	"os"
	"strings"
	"testing"
)

// TestRealIPMiddlewareIsNotInstalled is a tripwire for a security regression.
//
// chi's RealIP middleware rewrites r.RemoteAddr from X-Forwarded-For (and
// X-Real-IP) on every request. Because middleware.ClientIP reads RemoteAddr
// unless server.trust_proxy is enabled, installing RealIP would let any
// unauthenticated caller choose its own address: the per-IP rate limiters, the
// IP blocklist, PoW throttling, the setup wizard's loopback check and the
// /demo/test gate would all be bypassable with one header.
//
// Proxy headers are honoured by middleware.ClientIP, which is trust_proxy
// aware. This test fails loudly if RealIP ever comes back.
func TestRealIPMiddlewareIsNotInstalled(t *testing.T) {
	src, err := os.ReadFile("router.go")
	if err != nil {
		t.Fatalf("reading router.go: %v", err)
	}
	// Comments are allowed to mention the middleware; only real calls matter.
	var code strings.Builder
	for _, line := range strings.Split(string(src), "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "//") {
			continue
		}
		code.WriteString(line)
		code.WriteString("\n")
	}
	for _, banned := range []string{"chimw.RealIP", "middleware.RealIP"} {
		if strings.Contains(code.String(), banned) {
			t.Fatalf("router.New installs %s; the client IP must come from middleware.ClientIP so "+
				"it can honour server.trust_proxy", banned)
		}
	}
}
