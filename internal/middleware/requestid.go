package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/logx"
)

// RequestID injects a ULID request id into the context and response headers.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := r.Header.Get("X-Request-ID")
		if rid == "" || len(rid) > 64 {
			rid = id.New()
		}
		w.Header().Set("X-Request-ID", rid)
		next.ServeHTTP(w, r.WithContext(logx.WithRequestID(r.Context(), rid)))
	})
}

// Recover is defined in recover.go.

// ClientIP resolves the caller address, honouring X-Forwarded-For only when
// the server is configured to trust a reverse proxy.
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		// Only believe forwarding headers when the socket peer is a trusted
		// proxy; otherwise a client can spoof its own address.
		if peer, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && isTrustedProxy(peer) {
			// X-Forwarded-For is append-only: walk from the RIGHT and skip
			// trusted proxies, so an attacker-supplied left-hand value is
			// never selected.
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				parts := strings.Split(xff, ",")
				for i := len(parts) - 1; i >= 0; i-- {
					ip := strings.TrimSpace(parts[i])
					if ip == "" {
						continue
					}
					if !isTrustedProxy(ip) {
						return ip
					}
				}
			}
			if rip := strings.TrimSpace(r.Header.Get("X-Real-IP")); rip != "" && !isTrustedProxy(rip) {
				return rip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// isTrustedProxy reports whether an address belongs to a loopback or private
// network, i.e. a reverse proxy we are willing to trust for client IPs.
func isTrustedProxy(raw string) bool {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate()
}

// WriteJSON writes a JSON response with the standard content type.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError writes an apierr and returns it.
func WriteError(w http.ResponseWriter, err error) error {
	apierr.Write(w, err)
	return err
}
