// Package security implements abuse protection: proof-of-work challenges,
// IP/UA blocklists and the adaptive difficulty heuristic.
package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// ChallengeTTL is how long a proof-of-work challenge stays valid.
const ChallengeTTL = 2 * time.Minute

type challengeEntry struct {
	difficulty int
	expires    time.Time
}

// Pow issues and verifies single-use proof-of-work challenges.
type Pow struct {
	mu    sync.Mutex
	items map[string]challengeEntry
	ttl   time.Duration
}

// NewPow creates a challenge store.
func NewPow(ttl time.Duration) *Pow {
	if ttl <= 0 {
		ttl = ChallengeTTL
	}
	return &Pow{items: make(map[string]challengeEntry, 256), ttl: ttl}
}

// Issue creates a challenge string with the requested difficulty (leading zero
// bits). Difficulty 0 means proof-of-work is disabled and the challenge is
// returned with difficulty 0; Verify then accepts any nonce.
func (p *Pow) Issue(difficulty int) (string, int64) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		copy(buf, []byte(fmt.Sprintf("%d-%d", time.Now().UnixNano(), difficulty)))
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	exp := time.Now().Add(p.ttl)
	p.mu.Lock()
	if len(p.items) > 8192 {
		now := time.Now()
		for k, v := range p.items {
			if now.After(v.expires) {
				delete(p.items, k)
			}
		}
	}
	p.items[token] = challengeEntry{difficulty: difficulty, expires: exp}
	p.mu.Unlock()
	return token, exp.Unix()
}

// Verify consumes a challenge and checks the nonce. Challenges are single-use.
func (p *Pow) Verify(token, nonce string) bool {
	if token == "" {
		return false
	}
	p.mu.Lock()
	entry, ok := p.items[token]
	if ok {
		delete(p.items, token)
	}
	p.mu.Unlock()
	if !ok || time.Now().After(entry.expires) {
		return false
	}
	if entry.difficulty <= 0 {
		return true
	}
	return Solve(token, nonce, entry.difficulty)
}

// Solve reports whether sha256(token+nonce) has at least difficulty leading
// zero bits.
func Solve(token, nonce string, difficulty int) bool {
	sum := sha256.Sum256([]byte(token + nonce))
	return leadingZeroBits(sum[:]) >= difficulty
}

// leadingZeroBits counts the zero bits at the start of a byte slice.
func leadingZeroBits(b []byte) int {
	bits := 0
	for _, by := range b {
		if by == 0 {
			bits += 8
			continue
		}
		for mask := byte(0x80); mask > 0 && by&mask == 0; mask >>= 1 {
			bits++
		}
		break
	}
	return bits
}

// FindSolution brute-forces a nonce (used by tests and the CLI).
func FindSolution(token string, difficulty int) string {
	for i := 0; ; i++ {
		nonce := strconv.Itoa(i)
		if Solve(token, nonce, difficulty) {
			return nonce
		}
	}
}

// Throttle counts events per key inside a sliding 10 minute window.
type Throttle struct {
	mu     sync.Mutex
	window time.Duration
	m      map[string]*bucket
}

type bucket struct {
	start time.Time
	count int
}

// NewThrottle builds a counter with the given window.
func NewThrottle(window time.Duration) *Throttle {
	if window <= 0 {
		window = 10 * time.Minute
	}
	return &Throttle{window: window, m: make(map[string]*bucket, 128)}
}

// Hit registers an event and returns the count in the current window.
func (t *Throttle) Hit(key string) int {
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()
	b, ok := t.m[key]
	if !ok || now.Sub(b.start) > t.window {
		b = &bucket{start: now}
		t.m[key] = b
	}
	b.count++
	if len(t.m) > 10000 {
		for k, v := range t.m {
			if now.Sub(v.start) > t.window {
				delete(t.m, k)
			}
		}
	}
	return b.count
}

// DifficultyFor returns the proof-of-work difficulty for an address given the
// base difficulty and the number of sessions it created recently.
func DifficultyFor(base, recentSessions int) int {
	if base <= 0 {
		return 0
	}
	switch {
	case recentSessions > 40:
		return base + 6
	case recentSessions > 20:
		return base + 4
	case recentSessions > 10:
		return base + 2
	}
	return base
}

// HashNonce is exposed for tests that want a stable digest.
func HashNonce(token, nonce string) string {
	sum := sha256.Sum256([]byte(token + nonce))
	return hex.EncodeToString(sum[:])
}

// Count returns the current window count without incrementing it.
func (t *Throttle) Count(key string) int {
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()
	b, ok := t.m[key]
	if !ok || now.Sub(b.start) > t.window {
		return 0
	}
	return b.count
}
