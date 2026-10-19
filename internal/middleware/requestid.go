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

// DefaultTrustedProxies is the loopback-only default for server.trusted_proxies.
// A local reverse proxy (nginx, Caddy, Traefik) connects from one of these.
var DefaultTrustedProxies = []string{"127.0.0.1/32", "::1/128"}

// ClientIP resolves the caller address using the default trusted proxies.
func ClientIP(r *http.Request, trustProxy bool) string {
	return ClientIPWithProxies(r, trustProxy, DefaultTrustedProxies)
}

// ClientIPWithProxies resolves the caller address from the socket peer and,
// only when that peer is trusted, from X-Forwarded-For / X-Real-IP.
//
// X-Forwarded-For is append-only: a proxy appends the address it saw, so the
// right-most entry is the closest hop. We walk from the right, skip every hop
// that belongs to trusted_proxies, and return the first untrusted address. A
// value injected by the client on the left can therefore never win.
func ClientIPWithProxies(r *http.Request, trustProxy bool, trustedProxies []string) string {
	peer := remoteHost(r.RemoteAddr)
	if !trustProxy || peer == "" {
		return peer
	}
	nets := parseProxyNetworks(trustedProxies)
	if !addressInNetworks(peer, nets) {
		// The connection did not come from a proxy we trust. Forwarding
		// headers are attacker-controlled and must be ignored.
		return peer
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for i := len(parts) - 1; i >= 0; i-- {
			raw := strings.TrimSpace(parts[i])
			if raw == "" {
				continue
			}
			if net.ParseIP(strings.Trim(raw, "[]")) == nil {
				// A malformed right-most hop means the header is unusable.
				// Stop instead of falling back to attacker-supplied values.
				break
			}
			if !addressInNetworks(raw, nets) {
				return strings.Trim(raw, "[]")
			}
		}
	}

	if rip := strings.TrimSpace(r.Header.Get("X-Real-IP")); rip != "" {
		rip = strings.Trim(rip, "[]")
		if net.ParseIP(rip) != nil && !addressInNetworks(rip, nets) {
			return rip
		}
	}
	return peer
}

// remoteHost extracts the address from RemoteAddr, tolerating a bare address.
func remoteHost(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return strings.Trim(strings.TrimSpace(remoteAddr), "[]")
	}
	return host
}

// parseProxyNetworks compiles trusted_proxies into networks. Bare addresses
// become single-host blocks; invalid entries are ignored (config.Validate
// rejects them at startup).
func parseProxyNetworks(trustedProxies []string) []*net.IPNet {
	nets := make([]*net.IPNet, 0, len(trustedProxies))
	for _, raw := range trustedProxies {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if _, n, err := net.ParseCIDR(raw); err == nil {
			nets = append(nets, n)
			continue
		}
		ip := net.ParseIP(strings.Trim(raw, "[]"))
		if ip == nil {
			continue
		}
		bits := 32
		if ip.To4() == nil {
			bits = 128
		}
		nets = append(nets, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
	}
	return nets
}

func addressInNetworks(raw string, nets []*net.IPNet) bool {
	ip := net.ParseIP(strings.Trim(strings.TrimSpace(raw), "[]"))
	if ip == nil {
		return false
	}
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
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
