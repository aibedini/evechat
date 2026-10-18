package handlers

import (
	"net"
	"net/http"
	"strings"

	"github.com/evechat/evechat/internal/middleware"
	"github.com/evechat/evechat/web/demo"
)

// demoPages maps a clean URL to the embedded file that renders it.
var demoPages = map[string]string{
	"/demo":         "shop.html",
	"/demo/test":    "test.html",
	"/demo/split":   "split.html",
	"/demo/devices": "devices.html",
	"/demo/events":  "events.html",
}

// DemoPage serves one of the local-only demonstration pages. They carry the
// project API key, so every non-loopback caller gets a 404 (not a 403, which
// would confirm that the route exists).
func (h *Handlers) DemoPage(w http.ResponseWriter, r *http.Request) error {
	file, ok := demoPages[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return nil
	}
	return h.serveDemoFile(w, r, file)
}

// DemoAsset serves the shared demo stylesheet/script.
func (h *Handlers) DemoAsset(w http.ResponseWriter, r *http.Request) error {
	name := strings.TrimPrefix(r.URL.Path, "/demo/")
	if name != "demo.css" && name != "demo.js" || !h.demoAllowed(r) {
		http.NotFound(w, r)
		return nil
	}
	body, err := demo.Asset(name)
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	if strings.HasSuffix(name, ".css") {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	}
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(body)
	return nil
}

// DemoTest serves the manual widget test page.
func (h *Handlers) DemoTest(w http.ResponseWriter, r *http.Request) error {
	return h.serveDemoFile(w, r, "test.html")
}

func (h *Handlers) serveDemoFile(w http.ResponseWriter, r *http.Request, file string) error {
	if !h.demoAllowed(r) {
		http.NotFound(w, r)
		return nil
	}
	page, err := demo.Page(file, h.Cfg.BaseURL(), h.demoProjectKey(r))
	if err != nil {
		http.NotFound(w, r)
		return nil
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(page))
	return nil
}

func (h *Handlers) demoProjectKey(r *http.Request) string {
	if projects, err := h.Repo.Projects.List(r.Context()); err == nil && len(projects) > 0 {
		return projects[0].APIKey
	}
	return ""
}

// demoAllowed reports whether the request comes from the machine itself.
func (h *Handlers) demoAllowed(r *http.Request) bool {
	return isLoopbackIP(middleware.ClientIP(r, h.Cfg.Server.TrustProxy))
}

// isLoopbackIP reports whether raw is a loopback address (or the localhost name
// a proxy might forward).
func isLoopbackIP(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "localhost" {
		return true
	}
	ip := net.ParseIP(raw)
	return ip != nil && ip.IsLoopback()
}
