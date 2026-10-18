package demo

import (
	"strings"
	"testing"
)

func TestDemoBundleStaysUnder300KB(t *testing.T) {
	const budget = 300 << 10
	if size := Size(); size == 0 {
		t.Fatal("demo assets are not embedded")
	} else if size > budget {
		t.Fatalf("demo bundle is %d bytes, budget is %d", size, budget)
	}
}

func TestPagesRenderTheInstanceValues(t *testing.T) {
	// devices.html only embeds iframes of /demo/test, so it carries no key.
	withKey := []string{"shop.html", "test.html", "split.html", "events.html"}
	for _, name := range append(withKey, "devices.html") {
		page, err := Page(name, "http://127.0.0.1:3000/", "pk_demo")
		if err != nil {
			t.Fatalf("rendering %s: %v", name, err)
		}
		if strings.Contains(page, "__EVECHAT_PROJECT_KEY__") {
			t.Errorf("%s still has an unreplaced placeholder", name)
		}
		if strings.Contains(strings.Join(withKey, ","), name) && !strings.Contains(page, "pk_demo") {
			t.Errorf("%s does not carry the project key", name)
		}
	}
}

func TestPageEscapesHostileValues(t *testing.T) {
	page, err := Page("shop.html", "http://x", `</script><script>alert(1)</script>`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(page, "</script><script>alert(1)") {
		t.Fatal("the project key was not escaped for the JS string context")
	}
}

func TestUnknownPageFails(t *testing.T) {
	if _, err := Page("nope.html", "http://x", "pk"); err == nil {
		t.Fatal("unknown page rendered")
	}
}
