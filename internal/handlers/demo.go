package handlers

import (
	"net"
	"net/http"
	"strings"

	"github.com/evechat/evechat/internal/middleware"
	"github.com/evechat/evechat/web/demo"
)

// DemoTest serves the manual widget test page. The page carries the project API
// key, so it is a local-only debugging surface: any request that does not come
// from the machine itself gets a 404 rather than a 403, which would confirm that
// the route exists.
func (h *Handlers) DemoTest(w http.ResponseWriter, r *http.Request) error {
	if !isLoopbackIP(middleware.ClientIP(r, h.Cfg.Server.TrustProxy)) {
		http.NotFound(w, r)
		return nil
	}
	key := ""
	if projects, err := h.Repo.Projects.List(r.Context()); err == nil && len(projects) > 0 {
		key = projects[0].APIKey
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(demo.Render(h.Cfg.BaseURL(), key)))
	return nil
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
