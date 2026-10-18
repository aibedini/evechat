package handlers

import "testing"

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
