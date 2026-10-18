package security

import (
	"fmt"
	"net"
	"strings"
	"sync"
)

// Blocklist matches offending callers by exact IP, CIDR or user-agent substring.
// It can be rebuilt at runtime from app_settings without a restart.
type Blocklist struct {
	mu    sync.RWMutex
	ips   map[string]struct{}
	cidrs []*net.IPNet
	uas   []string
}

// NewBlocklist compiles the initial rules.
func NewBlocklist(ips, cidrs, uas []string) (*Blocklist, error) {
	b := &Blocklist{}
	if err := b.Update(ips, cidrs, uas); err != nil {
		return nil, err
	}
	return b, nil
}

// Update replaces every rule atomically.
func (b *Blocklist) Update(ips, cidrs, uas []string) error {
	parsedCIDRs := make([]*net.IPNet, 0, len(cidrs))
	for _, raw := range cidrs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		_, network, err := net.ParseCIDR(raw)
		if err != nil {
			return fmt.Errorf("invalid CIDR %q: %w", raw, err)
		}
		parsedCIDRs = append(parsedCIDRs, network)
	}
	ipSet := make(map[string]struct{}, len(ips))
	for _, raw := range ips {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		ipSet[raw] = struct{}{}
	}
	uaList := make([]string, 0, len(uas))
	for _, raw := range uas {
		raw = strings.ToLower(strings.TrimSpace(raw))
		if raw != "" {
			uaList = append(uaList, raw)
		}
	}
	b.mu.Lock()
	b.ips = ipSet
	b.cidrs = parsedCIDRs
	b.uas = uaList
	b.mu.Unlock()
	return nil
}

// Blocked reports whether the caller is blocked.
func (b *Blocklist) Blocked(ip, userAgent string) bool {
	if b == nil {
		return false
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if ip != "" {
		if _, ok := b.ips[ip]; ok {
			return true
		}
		parsed := net.ParseIP(ip)
		if parsed != nil {
			for _, network := range b.cidrs {
				if network.Contains(parsed) {
					return true
				}
			}
		}
	}
	if userAgent != "" && len(b.uas) > 0 {
		lower := strings.ToLower(userAgent)
		for _, needle := range b.uas {
			if strings.Contains(lower, needle) {
				return true
			}
		}
	}
	return false
}

// Rules returns the active rule counts (used by the settings UI).
func (b *Blocklist) Rules() (ips, cidrs, uas int) {
	if b == nil {
		return 0, 0, 0
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.ips), len(b.cidrs), len(b.uas)
}
