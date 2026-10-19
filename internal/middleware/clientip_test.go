package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	loopbackOnly := []string{"127.0.0.1/32", "::1/128"}
	privateProxy := []string{"127.0.0.1/32", "::1/128", "10.0.0.0/8"}

	tests := []struct {
		name     string
		remote   string
		trust    bool
		trusted  []string
		xff      string
		xreal    string
		expected string
	}{
		{
			name:     "trust disabled ignores a spoofed header",
			remote:   "198.51.100.7:54321",
			trust:    false,
			trusted:  loopbackOnly,
			xff:      "203.0.113.9",
			xreal:    "203.0.113.10",
			expected: "198.51.100.7",
		},
		{
			name:     "trust disabled ignores a spoofed header even from loopback",
			remote:   "127.0.0.1:54321",
			trust:    false,
			trusted:  loopbackOnly,
			xff:      "203.0.113.9",
			expected: "127.0.0.1",
		},
		{
			name:     "spoofed header from an untrusted peer is ignored",
			remote:   "198.51.100.7:443",
			trust:    true,
			trusted:  loopbackOnly,
			xff:      "127.0.0.1",
			expected: "198.51.100.7",
		},
		{
			name:    "left-most spoofed hop from a trusted peer is ignored",
			remote:  "127.0.0.1:443",
			trust:   true,
			trusted: loopbackOnly,
			// The attacker sent 1.2.3.4 and the proxy appended the real
			// caller 203.0.113.9.
			xff:      "1.2.3.4, 203.0.113.9",
			expected: "203.0.113.9",
		},
		{
			name:     "multi-hop chain skips every trusted proxy",
			remote:   "127.0.0.1:443",
			trust:    true,
			trusted:  privateProxy,
			xff:      "1.2.3.4, 203.0.113.9, 10.1.2.3",
			expected: "203.0.113.9",
		},
		{
			name:     "all hops trusted falls back to the socket peer",
			remote:   "127.0.0.1:443",
			trust:    true,
			trusted:  privateProxy,
			xff:      "10.0.0.1, 10.1.2.3",
			expected: "127.0.0.1",
		},
		{
			name:     "no proxy headers uses the socket peer",
			remote:   "10.0.0.2:443",
			trust:    true,
			trusted:  privateProxy,
			expected: "10.0.0.2",
		},
		{
			name:     "X-Real-IP is accepted from a trusted peer",
			remote:   "10.0.0.2:443",
			trust:    true,
			trusted:  privateProxy,
			xreal:    "203.0.113.11",
			expected: "203.0.113.11",
		},
		{
			name:     "X-Real-IP from an untrusted peer is ignored",
			remote:   "198.51.100.7:443",
			trust:    true,
			trusted:  loopbackOnly,
			xreal:    "203.0.113.11",
			expected: "198.51.100.7",
		},
		{
			name:     "IPv6 client from a trusted IPv6 peer",
			remote:   "[::1]:443",
			trust:    true,
			trusted:  loopbackOnly,
			xff:      "2001:db8::1",
			expected: "2001:db8::1",
		},
		{
			name:     "IPv6 spoofed header from an untrusted peer is ignored",
			remote:   "[2001:db8::2]:443",
			trust:    true,
			trusted:  loopbackOnly,
			xff:      "::1",
			expected: "2001:db8::2",
		},
		{
			name:     "IPv6 trusted hop is skipped",
			remote:   "[::1]:443",
			trust:    true,
			trusted:  []string{"::1/128", "2001:db8::/32"},
			xff:      "2001:db9::9, 2001:db8::6",
			expected: "2001:db9::9",
		},
		{
			name:     "custom IPv4 trusted network",
			remote:   "192.0.2.10:443",
			trust:    true,
			trusted:  []string{"192.0.2.0/24"},
			xff:      "203.0.113.9",
			expected: "203.0.113.9",
		},
		{
			name:     "malformed right-most hop stops the walk",
			remote:   "127.0.0.1:443",
			trust:    true,
			trusted:  loopbackOnly,
			xff:      "1.2.3.4, not-an-ip",
			expected: "127.0.0.1",
		},
		{
			name:     "empty XFF segments are skipped",
			remote:   "127.0.0.1:443",
			trust:    true,
			trusted:  loopbackOnly,
			xff:      "1.2.3.4, , 203.0.113.9",
			expected: "203.0.113.9",
		},
		{
			name:     "bare address in RemoteAddr still works",
			remote:   "198.51.100.7",
			trust:    true,
			trusted:  loopbackOnly,
			xff:      "203.0.113.9",
			expected: "198.51.100.7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remote
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xreal != "" {
				r.Header.Set("X-Real-IP", tt.xreal)
			}

			if got := ClientIPWithProxies(r, tt.trust, tt.trusted); got != tt.expected {
				t.Fatalf("ClientIPWithProxies() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestClientIPDefaultTrustsLoopbackOnly(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "127.0.0.1:443"
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 203.0.113.9")
	if got := ClientIP(r, true); got != "203.0.113.9" {
		t.Fatalf("ClientIP(default) = %q, want 203.0.113.9", got)
	}

	untrusted := httptest.NewRequest(http.MethodGet, "/", nil)
	untrusted.RemoteAddr = "10.0.0.2:443"
	untrusted.Header.Set("X-Forwarded-For", "203.0.113.9")
	if got := ClientIP(untrusted, true); got != "10.0.0.2" {
		t.Fatalf("ClientIP(default) from a private peer = %q, want 10.0.0.2", got)
	}
}
