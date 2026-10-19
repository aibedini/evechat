package middleware

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPIgnoresProxyHeadersWhenUntrusted(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "198.51.100.7:54321"
	r.Header.Set("X-Forwarded-For", "203.0.113.9")
	r.Header.Set("X-Real-IP", "203.0.113.10")

	if got := ClientIP(r, false); got != "198.51.100.7" {
		t.Fatalf("ClientIP(trustProxy=false) = %q, want the socket address 198.51.100.7", got)
	}
}

func TestClientIPUsesFirstHopWhenTrusted(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.2:443"
	r.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.2, 10.0.0.3")

	if got := ClientIP(r, true); got != "203.0.113.9" {
		t.Fatalf("ClientIP(trustProxy=true) = %q, want the first hop 203.0.113.9", got)
	}
}

func TestClientIPFallsBackToRealIPThenSocket(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.2:443"
	r.Header.Set("X-Real-IP", "203.0.113.11")
	if got := ClientIP(r, true); got != "203.0.113.11" {
		t.Fatalf("ClientIP with only X-Real-IP = %q, want 203.0.113.11", got)
	}

	plain := httptest.NewRequest("GET", "/", nil)
	plain.RemoteAddr = "10.0.0.2:443"
	if got := ClientIP(plain, true); got != "10.0.0.2" {
		t.Fatalf("ClientIP without proxy headers = %q, want 10.0.0.2", got)
	}
}
func TestClientIPRejectsSpoofedLeftmostHop(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.2:443"
	// An attacker sent "X-Forwarded-For: 1.2.3.4"; nginx appended the real
	// address, producing "1.2.3.4, 203.0.113.9".
	r.Header.Set("X-Forwarded-For", "1.2.3.4, 203.0.113.9")
	if got := ClientIP(r, true); got != "203.0.113.9" {
		t.Fatalf("spoofed ClientIP = %q, want the nearest untrusted hop 203.0.113.9", got)
	}
}

func TestClientIPIgnoresHeadersFromUntrustedPeer(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "198.51.100.7:443"
	r.Header.Set("X-Forwarded-For", "203.0.113.9")
	if got := ClientIP(r, true); got != "198.51.100.7" {
		t.Fatalf("untrusted peer ClientIP = %q, want 198.51.100.7", got)
	}
}
