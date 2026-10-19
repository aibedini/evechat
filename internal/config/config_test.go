package config

import "testing"

func TestDefaultTrustedProxiesAreLoopbackOnly(t *testing.T) {
	cfg := Default()
	want := map[string]bool{"127.0.0.1/32": true, "::1/128": true}
	if len(cfg.Server.TrustedProxies) != len(want) {
		t.Fatalf("default trusted_proxies = %v, want loopback only", cfg.Server.TrustedProxies)
	}
	for _, raw := range cfg.Server.TrustedProxies {
		if !want[raw] {
			t.Fatalf("unexpected default trusted proxy %q", raw)
		}
	}
}

func TestValidProxyAddress(t *testing.T) {
	tests := []struct {
		raw  string
		want bool
	}{
		{"127.0.0.1/32", true},
		{"::1/128", true},
		{"10.0.0.0/8", true},
		{"2001:db8::/32", true},
		{"192.0.2.10", true},
		{"2001:db8::1", true},
		{"[::1]", true},
		{"", false},
		{"not-a-cidr", false},
		{"10.0.0.0/33", false},
	}
	for _, tt := range tests {
		if got := validProxyAddress(tt.raw); got != tt.want {
			t.Errorf("validProxyAddress(%q) = %v, want %v", tt.raw, got, tt.want)
		}
	}
}
