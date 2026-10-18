package services

import "testing"

func TestJSONPathGet(t *testing.T) {
	payload := map[string]any{
		"user":  map[string]any{"id": "u-1", "name": "Ali", "tags": []any{"a", "b"}},
		"items": []any{map[string]any{"sku": "X1"}, map[string]any{"sku": "X2"}},
		"count": float64(7), "ok": true,
		"a b": "spaced",
	}
	cases := []struct {
		path string
		want string
	}{
		{"$.user.id", "u-1"},
		{"user.name", ""},
		{"$.items[1].sku", "X2"},
		{"$.count", "7"},
		{"$.ok", "true"},
		{"$['a b']", "spaced"},
		{"$.user.tags[0]", "a"},
		{"$.missing.path", ""},
		{"$.items[9]", ""},
	}
	for _, c := range cases {
		if got := jsonPathString(payload, c.path); got != c.want {
			t.Errorf("jsonPathString(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func TestJSONPathRejectsUnsupportedPaths(t *testing.T) {
	payload := map[string]any{"a": 1}
	for _, path := range []string{"", "a.b", "$.a[*]", "$..a"} {
		if _, ok := jsonPathGet(payload, path); ok {
			t.Errorf("jsonPathGet(%q) unexpectedly resolved", path)
		}
	}
}

func TestScrubBodyRemovesSecrets(t *testing.T) {
	body := []byte(`{"user":{"id":"u-1","password":"hunter2"},"api_key":"pk_live","message":"hi"}`)
	scrubbed := ScrubBody(body)
	if scrubbed == "" {
		t.Fatal("ScrubBody returned nothing")
	}
	for _, leaked := range []string{"hunter2", "pk_live"} {
		if contains(scrubbed, leaked) {
			t.Errorf("scrubbed body still contains %q: %s", leaked, scrubbed)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
