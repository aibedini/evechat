package security

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrBlockedAddress is returned when an outbound target is rejected: a bad
// scheme, a name that cannot be resolved, or an address the server must never
// connect to (loopback, private, link-local, multicast, CGNAT, metadata...).
var ErrBlockedAddress = errors.New("outbound target is not allowed")

// blockedBlocks are the address ranges that are never legitimate outbound
// destinations. The stdlib predicates cover loopback/private/link-local; these
// are the extra special-use blocks that SSRF filters commonly miss.
var blockedBlocks = mustCIDRs(
	"0.0.0.0/8",       // "this network"
	"100.64.0.0/10",   // CGNAT
	"192.0.0.0/24",    // IETF protocol assignments
	"192.0.2.0/24",    // TEST-NET-1
	"198.18.0.0/15",   // benchmarking
	"198.51.100.0/24", // TEST-NET-2
	"203.0.113.0/24",  // TEST-NET-3
	"240.0.0.0/4",     // reserved
	"255.255.255.255/32",
	"64:ff9b::/96",   // NAT64
	"64:ff9b:1::/48", // local-use NAT64
	"100::/64",       // discard-only
	"2001:db8::/32",  // documentation
	"2001:10::/28",   // ORCHID
	"3fff::/20",      // documentation
)

func mustCIDRs(raw ...string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(raw))
	for _, s := range raw {
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			panic("security: bad CIDR " + s)
		}
		out = append(out, n)
	}
	return out
}

// OutboundPolicy decides which outbound HTTP targets are allowed. The zero value
// is not usable; build one with NewOutboundPolicy.
type OutboundPolicy struct {
	allow          map[string]struct{}
	allowPrivate   bool
	maxRedirects   int
	dialTimeout    time.Duration
	requestTimeout time.Duration
}

// NewOutboundPolicy builds a policy. allowHosts are exact hostnames that are
// always permitted (for internal gateways); allowPrivate disables the address
// filter entirely and should only be used on isolated networks.
func NewOutboundPolicy(allowHosts []string, allowPrivate bool) *OutboundPolicy {
	allow := make(map[string]struct{}, len(allowHosts))
	for _, h := range allowHosts {
		h = strings.ToLower(strings.TrimSpace(h))
		if h != "" {
			allow[h] = struct{}{}
		}
	}
	return &OutboundPolicy{
		allow:          allow,
		allowPrivate:   allowPrivate,
		maxRedirects:   5,
		dialTimeout:    10 * time.Second,
		requestTimeout: 10 * time.Second,
	}
}

// Outbound is the policy used by server-side HTTP clients. It is configured once
// at startup from the configuration and must not be mutated afterwards.
var Outbound = NewOutboundPolicy(nil, false)

// ValidateOutboundURL checks a user supplied URL against the default policy.
func ValidateOutboundURL(raw string) error {
	return Outbound.Validate(context.Background(), raw)
}

// Validate checks that raw is an http(s) URL whose host does not resolve to a
// blocked address. Every resolved address is checked, so a DNS name with one
// public and one private record is rejected.
func (p *OutboundPolicy) Validate(ctx context.Context, raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("%w: invalid URL", ErrBlockedAddress)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: unsupported scheme %q", ErrBlockedAddress, u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("%w: missing host", ErrBlockedAddress)
	}
	if p.allowPrivate || p.hostAllowed(host) {
		return nil
	}
	if isLocalName(host) || isNumericHost(host) {
		return fmt.Errorf("%w: %s", ErrBlockedAddress, host)
	}
	ips, err := p.resolvedIPs(ctx, host)
	if err != nil {
		return err
	}
	for _, ip := range ips {
		if blockedIP(ip) {
			return fmt.Errorf("%w: %s resolves to %s", ErrBlockedAddress, host, ip)
		}
	}
	return nil
}

// Client returns an http.Client that enforces the policy on the first request,
// on every redirect hop and at dial time (which also defeats DNS rebinding).
func (p *OutboundPolicy) Client(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = p.requestTimeout
	}
	// No proxy: a proxy would let a caller bypass the address filter.
	tr := &http.Transport{
		DialContext:         p.dialContext,
		MaxIdleConns:        16,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		ForceAttemptHTTP2:   true,
	}
	return &http.Client{Timeout: timeout, Transport: tr, CheckRedirect: p.CheckRedirect}
}

// CheckRedirect re-validates every redirect target. It is exported so it can be
// tested directly and so callers can attach it to their own clients.
func (p *OutboundPolicy) CheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= p.maxRedirects {
		return fmt.Errorf("%w: too many redirects", ErrBlockedAddress)
	}
	return p.Validate(req.Context(), req.URL.String())
}

func (p *OutboundPolicy) hostAllowed(host string) bool {
	_, ok := p.allow[strings.ToLower(host)]
	return ok
}

func (p *OutboundPolicy) resolvedIPs(ctx context.Context, host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addrs) == 0 {
		return nil, fmt.Errorf("%w: cannot resolve %s", ErrBlockedAddress, host)
	}
	ips := make([]net.IP, 0, len(addrs))
	seen := make(map[string]struct{}, len(addrs))
	for _, a := range addrs {
		key := a.IP.String()
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		ips = append(ips, a.IP)
	}
	return ips, nil
}

func (p *OutboundPolicy) dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := p.resolvedIPs(ctx, host)
	if err != nil {
		return nil, err
	}
	if !p.allowPrivate && !p.hostAllowed(host) {
		for _, ip := range ips {
			if blockedIP(ip) {
				return nil, fmt.Errorf("%w: %s", ErrBlockedAddress, ip)
			}
		}
	}
	dialer := &net.Dialer{Timeout: p.dialTimeout, KeepAlive: 30 * time.Second}
	var lastErr error
	for _, ip := range ips {
		conn, derr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if derr == nil {
			return conn, nil
		}
		lastErr = derr
		if ctx.Err() != nil {
			break
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("%w: no usable address for %s", ErrBlockedAddress, host)
	}
	return nil, lastErr
}

// blockedIP reports whether ip belongs to a range that outbound requests must
// never reach.
func blockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	for _, block := range blockedBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// isLocalName rejects names that resolve locally or are used by cloud metadata
// services before any DNS traffic is generated.
func isLocalName(host string) bool {
	h := strings.ToLower(strings.TrimSuffix(host, "."))
	if h == "localhost" || h == "metadata" || h == "metadata.google.internal" {
		return true
	}
	for _, suffix := range []string{".localhost", ".local", ".internal", ".home.arpa"} {
		if strings.HasSuffix(h, suffix) {
			return true
		}
	}
	return false
}

// isNumericHost catches the legacy integer forms of IPv4 addresses
// (2130706433, 0x7f000001) that resolvers happily pass to inet_aton style
// parsers but that url.Parse does not treat as IPs.
func isNumericHost(host string) bool {
	if host == "" {
		return false
	}
	if strings.HasPrefix(host, "0x") || strings.HasPrefix(host, "0X") {
		rest := host[2:]
		if rest == "" {
			return false
		}
		for _, r := range rest {
			if !isHex(r) {
				return false
			}
		}
		return true
	}
	digits := true
	for _, r := range host {
		if r < '0' || r > '9' {
			digits = false
			break
		}
	}
	return digits
}

func isHex(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}
