package services

import (
	"net/url"
	"testing"
	"time"

	"github.com/evechat/evechat/internal/models"
)

func TestMatchConditionsTable(t *testing.T) {
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	pageURL := "https://eve.ir/subscription/plan?x=1"
	base := MatchContext{
		PageURL:         pageURL,
		PageTitle:       "خرید اشتراک",
		DeviceType:      "mobile",
		Language:        "fa-IR",
		ResellerID:      "reseller_abc",
		Metadata:        models.JSONMap{"utm_source": "google", "plan": "gold"},
		Subscription:    models.JSONMap{"plan": "pro", "expiry": "2027-01-01"},
		InBusinessHours: true,
		Now:             now,
	}

	cases := []struct {
		name string
		cond models.JSONMap
		want bool
	}{
		{"empty matches everything", models.JSONMap{}, true},
		{"domain match", models.JSONMap{"domains": []any{"eve.ir"}}, true},
		{"domain mismatch", models.JSONMap{"domains": []any{"example.com"}}, false},
		{"subdomain match", models.JSONMap{"domains": []any{"ir"}}, true},
		{"page glob suffix", models.JSONMap{"pages": []any{"*/subscription*"}}, true},
		{"page glob exact", models.JSONMap{"pages": []any{"/subscription/plan"}}, true},
		{"page glob mismatch", models.JSONMap{"pages": []any{"/billing"}}, false},
		{"reseller match", models.JSONMap{"eve_reseller_id": []any{"reseller_abc"}}, true},
		{"reseller mismatch", models.JSONMap{"eve_reseller_id": []any{"reseller_xyz"}}, false},
		{"device match", models.JSONMap{"device_types": []any{"mobile", "tablet"}}, true},
		{"device mismatch", models.JSONMap{"device_types": []any{"desktop"}}, false},
		{"language prefix match", models.JSONMap{"languages": []any{"fa"}}, true},
		{"language mismatch", models.JSONMap{"languages": []any{"en"}}, false},
		{"has subscription true", models.JSONMap{"has_subscription": true}, true},
		{"has subscription false", models.JSONMap{"has_subscription": false}, false},
		{"business hours true", models.JSONMap{"business_hours": true}, true},
		{"business hours false", models.JSONMap{"business_hours": false}, false},
		{"variable match", models.JSONMap{"variables": map[string]any{"utm_source": []any{"google"}}}, true},
		{"variable mismatch", models.JSONMap{"variables": map[string]any{"utm_source": []any{"bing"}}}, false},
		{"all conditions must match", models.JSONMap{
			"domains":      []any{"eve.ir"},
			"device_types": []any{"mobile"},
			"pages":        []any{"/subscription/*"},
		}, true},
		{"one failing condition fails the rule", models.JSONMap{
			"domains":      []any{"eve.ir"},
			"device_types": []any{"desktop"},
		}, false},
		{"empty condition list is ignored", models.JSONMap{"domains": []any{}}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchConditions(tc.cond, base); got != tc.want {
				t.Fatalf("matchConditions(%v) = %v, want %v", tc.cond, got, tc.want)
			}
		})
	}
}

func TestGlobMatch(t *testing.T) {
	cases := []struct {
		pattern, s string
		want       bool
	}{
		{"/subscription*", "/subscription/plan", true},
		{"*/billing", "/account/billing", true},
		{"*/billing", "/account/billing/x", false},
		{"*", "/anything", true},
		{"exact", "exact", true},
		{"exact", "exactly", false},
		{"", "", false},
	}
	for _, tc := range cases {
		if got := globMatch(tc.pattern, tc.s); got != tc.want {
			t.Fatalf("globMatch(%q,%q)=%v want %v", tc.pattern, tc.s, got, tc.want)
		}
	}
}

func TestInBusinessHours(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Tehran")
	if err != nil {
		t.Skip("tzdata unavailable")
	}
	// Monday 2026-10-19 10:00 Tehran.
	inside := time.Date(2026, 10, 19, 10, 0, 0, 0, loc).UTC()
	outside := time.Date(2026, 10, 19, 22, 0, 0, 0, loc).UTC()

	settings := models.JSONMap{"business_hours": map[string]any{
		"enabled":  true,
		"timezone": "Asia/Tehran",
		"days":     []any{"1", "2", "3", "4", "5"},
		"start":    "09:00",
		"end":      "18:00",
	}}
	if !InBusinessHours(settings, inside) {
		t.Fatal("expected inside business hours")
	}
	if InBusinessHours(settings, outside) {
		t.Fatal("expected outside business hours")
	}
	if !InBusinessHours(models.JSONMap{}, outside) {
		t.Fatal("unconfigured business hours must be treated as open")
	}
}

func TestSubscriptionActive(t *testing.T) {
	now := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		sub  models.JSONMap
		want bool
	}{
		{"empty", models.JSONMap{}, false},
		{"future expiry rfc3339", models.JSONMap{"plan": "pro", "expiry": "2027-01-01T00:00:00Z"}, true},
		{"past expiry", models.JSONMap{"plan": "pro", "expiry": "2025-01-01T00:00:00Z"}, false},
		{"no expiry with plan", models.JSONMap{"plan": "lifetime"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := subscriptionActive(tc.sub, now); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestParseUA(t *testing.T) {
	cases := []struct {
		ua                       string
		device, osName, browser string
	}{
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36", "desktop", "Windows", "Chrome"},
		{"Mozilla/5.0 (Windows NT 10.0) AppleWebKit/537.36 Edg/120.0", "desktop", "Windows", "Edge"},
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Version/17.0 Mobile/15E148 Safari/604.1", "mobile", "iOS", "Safari"},
		{"Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Version/17.0 Mobile/15E148 Safari/604.1", "tablet", "iOS", "Safari"},
		{"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 Chrome/120.0 Mobile Safari/537.36", "mobile", "Android", "Chrome"},
		{"Mozilla/5.0 (Linux; Android 14; SM-X700) AppleWebKit/537.36 SamsungBrowser/23.0 Chrome/115.0 Safari/537.36", "tablet", "Android", "Samsung"},
		{"Mozilla/5.0 (X11; Linux x86_64) Gecko/20100101 Firefox/121.0", "desktop", "Linux", "Firefox"},
		{"Opera/9.80 (X11; Linux x86_64) Presto/2.12 Version/12.16", "desktop", "Linux", "Opera"},
	}
	for _, tc := range cases {
		d, o, b := ParseUA(tc.ua)
		if d != tc.device || o != tc.osName || b != tc.browser {
			t.Fatalf("ParseUA(%q) = %s/%s/%s, want %s/%s/%s", tc.ua, d, o, b, tc.device, tc.osName, tc.browser)
		}
	}
}

func TestHostAndPath(t *testing.T) {
	u, _ := url.Parse("https://sub.eve.ir/a/b?c=1")
	if hostOf(u.String()) != "sub.eve.ir" {
		t.Fatal("hostOf failed")
	}
	if pathOf(u.String()) != "/a/b" {
		t.Fatal("pathOf failed")
	}
}
