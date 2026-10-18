package middleware

import (
	"net/http"
	"strings"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/runtimecfg"
)

// SetupGate forces every request through the first-run wizard until the
// instance is configured. This is what makes a bare `docker run` usable.
func SetupGate(state *runtimecfg.State) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			setupDone := state.SetupCompleted()

			if !setupDone {
				// Assets and the health/metrics endpoints must stay reachable
				// even before the wizard finishes (containers probe health).
				if isSetupPath(path) || isAssetPath(path) {
					next.ServeHTTP(w, r)
					return
				}
				if strings.HasPrefix(path, "/api/") {
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusConflict)
					_, _ = w.Write([]byte(`{"code":"setup_required","message":"نصب هنوز کامل نشده است. به /setup بروید."}`))
					return
				}
				// Browsers get redirected to the wizard.
				http.Redirect(w, r, "/setup", http.StatusFound)
				return
			}

			// Configured instances must not expose the wizard anymore.
			if strings.HasPrefix(path, "/api/setup") {
				apierr.Write(w, apierr.New(http.StatusGone, "setup_complete", "نصب قبلاً انجام شده است"))
				return
			}
			if path == "/setup" || strings.HasPrefix(path, "/setup/") {
				apierr.Write(w, apierr.NotFound("مسیر یافت نشد"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isSetupPath(path string) bool {
	if path == "/setup" || strings.HasPrefix(path, "/setup/") {
		return true
	}
	if path == "/api/setup/status" || path == "/api/setup/check" {
		return true
	}
	return strings.HasPrefix(path, "/api/setup/")
}

func isAssetPath(path string) bool {
	switch {
	case path == "/favicon.ico", path == "/widget.js":
		return true
	case strings.HasPrefix(path, "/assets/"):
		return true
	case path == "/api/v1/health", path == "/api/v1/config", path == "/metrics":
		return true
	}
	return false
}
