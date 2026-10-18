package security

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// TestValidateOutboundURLBlocksInternalTargets covers the exact inputs the
// hardening checklist requires, plus a few classic SSRF bypasses.
func TestValidateOutboundURLBlocksInternalTargets(t *testing.T) {
	p := NewOutboundPolicy(nil, false)
	blocked := []string{
		"http://169.254.169.254/",          // cloud metadata
		"http://127.0.0.1:22",              // loopback on any port
		"http://10.0.0.1/",                 // RFC1918
		"http://192.168.1.1/",              // RFC1918
		"http://[::1]/",                    // IPv6 loopback
		"http://localhost/",                // name that resolves to loopback
		"http://127.0.0.1.nip.io/",         // rebinding helper name (resolves locally)
		"http://0.0.0.0/",                  // unspecified
		"http://100.64.0.1/",               // CGNAT
		"http://[fd00::1]/",                // IPv6 ULA
		"http://2130706433/",               // 127.0.0.1 as a decimal integer
		"http://0x7f000001/",               // 127.0.0.1 as hex
		"http://metadata.google.internal/", // well known metadata name
		"ftp://8.8.8.8/",                   // scheme we never deliver to
		"file:///etc/passwd",               // local file read
		"http://",                          // no host at all
	}
	for _, raw := range blocked {
		err := p.Validate(testCtx(t), raw)
		if err == nil {
			t.Errorf("Validate(%q) = nil, want a rejection", raw)
			continue
		}
		if !errors.Is(err, ErrBlockedAddress) {
			t.Errorf("Validate(%q) error = %v, want ErrBlockedAddress", raw, err)
		}
	}
}

func TestValidateOutboundURLAllowsPublicHosts(t *testing.T) {
	p := NewOutboundPolicy(nil, false)
	// IP literals keep the test offline and deterministic.
	for _, raw := range []string{"http://93.184.216.34/", "https://8.8.8.8/path?q=1"} {
		if err := p.Validate(testCtx(t), raw); err != nil {
			t.Errorf("Validate(%q) = %v, want nil", raw, err)
		}
	}
}

// TestCheckRedirectRejectsLoopback proves a redirect from a public domain to a
// loopback address is refused before the second request is issued.
func TestCheckRedirectRejectsLoopback(t *testing.T) {
	p := NewOutboundPolicy(nil, false)
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:22/", nil)
	via := []*http.Request{httptest.NewRequest(http.MethodGet, "https://example.com/hook", nil)}
	err := p.CheckRedirect(req, via)
	if err == nil {
		t.Fatal("CheckRedirect(public -> 127.0.0.1) = nil, want a rejection")
	}
	if !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("CheckRedirect error = %v, want ErrBlockedAddress", err)
	}
}

// TestClientBlocksRedirectToMetadata drives the whole client: the first hop is
// allowed through the explicit allowlist, the redirect target is not.
func TestClientBlocksRedirectToMetadata(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer srv.Close()

	host := httptest.NewRequest(http.MethodGet, srv.URL, nil).URL.Hostname()
	p := NewOutboundPolicy([]string{host}, false)
	resp, err := p.Client(3 * time.Second).Get(srv.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("client followed a redirect to the metadata endpoint")
	}
	if !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("client error = %v, want ErrBlockedAddress", err)
	}
}

func TestValidateOutboundURLHonoursAllowlist(t *testing.T) {
	p := NewOutboundPolicy([]string{"127.0.0.1"}, false)
	if err := p.Validate(testCtx(t), "http://127.0.0.1:8080/hook"); err != nil {
		t.Fatalf("allowlisted host rejected: %v", err)
	}
	if err := p.Validate(testCtx(t), "http://127.0.0.2:8080/hook"); err == nil {
		t.Fatal("only the listed host should be allowed")
	}
}

func TestBlockedIPClassifiesSpecialRanges(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1":       true,
		"10.1.2.3":        true,
		"192.168.0.9":     true,
		"169.254.169.254": true,
		"100.64.0.1":      true,
		"::1":             true,
		"fd12::1":         true,
		"fe80::1":         true,
		"93.184.216.34":   false,
		"8.8.8.8":         false,
		"2606:4700::1111": false,
	}
	for raw, want := range cases {
		ip := net.ParseIP(raw)
		if ip == nil {
			t.Fatalf("cannot parse %q", raw)
		}
		if got := blockedIP(ip); got != want {
			t.Errorf("blockedIP(%s) = %v, want %v", raw, got, want)
		}
	}
}
