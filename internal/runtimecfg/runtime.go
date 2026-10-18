// Package runtimecfg keeps the mutable, DB-backed part of the configuration:
// first-run setup state, instance id, integration secrets and blocklists.
// YAML/env values stay immutable at boot; these values can change at runtime.
package runtimecfg

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/crypto"
	"github.com/evechat/evechat/internal/repo"
	"github.com/evechat/evechat/internal/security"
)

// Known setting keys.
const (
	KeySetupCompleted   = "setup_completed"
	KeySetupCompletedAt = "setup_completed_at"
	KeyInstanceID       = "instance_id"
	KeyBlockedIPs       = "security.blocked_ips"
	KeyBlockedCIDRs     = "security.blocked_cidrs"
	KeyBlockedUAs       = "security.blocked_user_agents"
	KeySetupToken       = "setup.token"
	KeyPOWDifficulty    = "security.pow_difficulty"
)

// State is the live runtime configuration.
type State struct {
	repo *repo.Repo
	enc  *crypto.Service
	log  zerolog.Logger

	mu     sync.RWMutex
	values map[string]string

	blocklist *security.Blocklist
	pow       *security.Pow
	throttle  *security.Throttle
	baseURL   string
}

// New loads the persisted settings and fills in the instance id.
func New(ctx context.Context, r *repo.Repo, enc *crypto.Service, baseURL string, log zerolog.Logger) (*State, error) {
	s := &State{
		repo:      r,
		enc:       enc,
		log:       log,
		values:    map[string]string{},
		blocklist: &security.Blocklist{},
		pow:       security.NewPow(security.ChallengeTTL),
		throttle:  security.NewThrottle(10 * time.Minute),
		baseURL:   baseURL,
	}
	if err := s.Reload(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// Reload re-reads every persisted setting.
func (s *State) Reload(ctx context.Context) error {
	all, err := s.repo.Settings.All(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.values = all
	if s.values[KeyInstanceID] == "" {
		buf := make([]byte, 16)
		if _, err := rand.Read(buf); err != nil {
			buf = []byte(fmt.Sprintf("%d", time.Now().UnixNano()))
		}
		id := base64.RawURLEncoding.EncodeToString(buf)
		s.values[KeyInstanceID] = id
		s.mu.Unlock()
		if err := s.repo.Settings.Set(ctx, KeyInstanceID, id); err != nil {
			return err
		}
	} else {
		s.mu.Unlock()
	}
	return s.ReloadBlocklist(ctx)
}

// ReloadBlocklist rebuilds the matcher from stored rules.
func (s *State) ReloadBlocklist(ctx context.Context) error {
	ips := splitList(s.Value(KeyBlockedIPs))
	cidrs := splitList(s.Value(KeyBlockedCIDRs))
	uas := splitList(s.Value(KeyBlockedUAs))
	return s.blocklist.Update(ips, cidrs, uas)
}

func splitList(raw string) []string {
	out := []string{}
	for _, part := range splitAny(raw, ",\n") {
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func splitAny(raw, seps string) []string {
	var out []string
	cur := ""
	for _, r := range raw {
		isSep := false
		for _, sep := range seps {
			if r == sep {
				isSep = true
				break
			}
		}
		if isSep {
			out = append(out, trimSpace(cur))
			cur = ""
			continue
		}
		cur += string(r)
	}
	out = append(out, trimSpace(cur))
	return out
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// Value returns a setting or "".
func (s *State) Value(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.values[key]
}

// IntValue returns a setting parsed as int, or def.
func (s *State) IntValue(key string, def int) int {
	raw := s.Value(key)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

// Set persists a setting and refreshes the cache.
func (s *State) Set(ctx context.Context, key, value string) error {
	if err := s.repo.Settings.Set(ctx, key, value); err != nil {
		return err
	}
	s.mu.Lock()
	s.values[key] = value
	s.mu.Unlock()
	return nil
}

// SetSecret stores a secret encrypted at rest.
func (s *State) SetSecret(ctx context.Context, key, value string) error {
	ct, iv, ver, err := s.enc.Encrypt(value)
	if err != nil {
		return err
	}
	payload := fmt.Sprintf("%d:%s:%s", ver, ct, iv)
	return s.Set(ctx, key, payload)
}

// Secret reads a secret stored by SetSecret.
func (s *State) Secret(key string) (string, bool) {
	raw := s.Value(key)
	if raw == "" {
		return "", false
	}
	var ver int
	var ct, iv string
	if _, err := fmt.Sscanf(raw, "%d:", &ver); err != nil {
		return "", false
	}
	rest := raw[len(strconv.Itoa(ver))+1:]
	sep := -1
	for i := 0; i < len(rest); i++ {
		if rest[i] == ':' {
			sep = i
			break
		}
	}
	if sep < 0 {
		return "", false
	}
	ct, iv = rest[:sep], rest[sep+1:]
	plain, err := s.enc.Decrypt(ct, iv, ver)
	if err != nil {
		s.log.Warn().Err(err).Str("key", key).Msg("decrypting setting failed")
		return "", false
	}
	return plain, true
}

// SetupCompleted reports whether the first-run wizard finished.
func (s *State) SetupCompleted() bool { return s.Value(KeySetupCompleted) == "1" }

// CompleteSetup marks the instance as configured.
func (s *State) CompleteSetup(ctx context.Context) error {
	if err := s.Set(ctx, KeySetupCompleted, "1"); err != nil {
		return err
	}
	return s.Set(ctx, KeySetupCompletedAt, time.Now().UTC().Format(time.RFC3339))
}

// InstanceID identifies this installation (used by the doctor and diagnostics).
func (s *State) InstanceID() string { return s.Value(KeyInstanceID) }

// Blocklist exposes the compiled matcher.
func (s *State) Blocklist() *security.Blocklist { return s.blocklist }

// Pow exposes the proof-of-work store.
func (s *State) Pow() *security.Pow { return s.pow }

// Throttle exposes the per-IP session counter.
func (s *State) Throttle() *security.Throttle { return s.throttle }

// POWDifficulty returns the configured base difficulty (0 disables PoW).
func (s *State) POWDifficulty(def int) int { return s.IntValue(KeyPOWDifficulty, def) }

// SetupToken returns the anti-hijack token for the web wizard, generating it on
// first use and writing it to <dataDir>/setup-token.txt when a path is given.
func (s *State) SetupToken(ctx context.Context, dataDir string) (string, error) {
	existing := s.Value(KeySetupToken)
	if existing != "" {
		return existing, nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	if err := s.Set(ctx, KeySetupToken, token); err != nil {
		return "", err
	}
	if dataDir != "" {
		path := dataDir + "/setup-token.txt"
		_ = writeFile0600(path, token)
	}
	return token, nil
}

// BaseURL is the public dashboard URL captured at boot.
func (s *State) BaseURL() string { return s.baseURL }

func writeFile0600(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}
