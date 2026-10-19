package middleware

import (
	"net/http"
	"strings"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/security"
)

// SecurityHeaders adds the standard hardening headers. X-Frame-Options is only
// applied to the dashboard: the widget must remain embeddable on customer sites.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if isDashboardPath(r.URL.Path) {
			h.Set("X-Frame-Options", "SAMEORIGIN")
			h.Set("Content-Security-Policy",
				"default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; "+
					"font-src 'self' data:; connect-src 'self' ws: wss:; script-src 'self' 'unsafe-inline'; "+
					"frame-ancestors 'self'; base-uri 'self'; form-action 'self'")
		}
		next.ServeHTTP(w, r)
	})
}

func isDashboardPath(path string) bool {
	if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/ws/") {
		return false
	}
	if path == "/widget.js" || strings.HasPrefix(path, "/assets/") || strings.HasPrefix(path, "/uploads/") {
		return false
	}
	return true
}

// Blocklist rejects callers matched by the runtime IP/UA/CIDR rules.
func Blocklist(bl *security.Blocklist, trustProxy bool, trustedProxies []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if bl != nil && bl.Blocked(ClientIPWithProxies(r, trustProxy, trustedProxies), r.UserAgent()) {
				apierr.Write(w, apierr.Forbidden("دسترسی شما مسدود شده است"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// NoStore disables caching for authenticated API responses.
func NoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
