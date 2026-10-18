package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/evechat/evechat/internal/metrics"
)

// Instrument records request counts and latency per chi route pattern.
func Instrument(reg *metrics.Registry) func(http.Handler) http.Handler {
	if reg == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			route := "unmatched"
			if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
				route = rc.RoutePattern()
			}
			labels := map[string]string{
				"route":  route,
				"method": r.Method,
				"status": strconv.Itoa(ww.Status()),
			}
			reg.Inc("evechat_http_requests_total", labels)
			reg.Observe("evechat_http_request_duration_seconds", map[string]string{"route": route}, time.Since(start).Seconds())
		})
	}
}

// RateLimitCounter records rejected requests when a limiter returns 429.
// It is installed by the limiter middleware through the registry accessor.
func RateLimitCounter(reg *metrics.Registry, scope string) {
	if reg == nil {
		return
	}
	reg.Inc("evechat_rate_limited_total", map[string]string{"scope": scope})
}
