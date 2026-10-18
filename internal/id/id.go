package id

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	mu      sync.Mutex
	entropy = ulid.Monotonic(rand.Reader, 0)
)

// New returns a lowercase ULID string, monotonic within a process.
func New() string {
	mu.Lock()
	defer mu.Unlock()
	return strings.ToLower(ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String())
}

// NewPrefixed returns prefix + "_" + New(), e.g. "conv_01J...".
func NewPrefixed(prefix string) string {
	return prefix + "_" + New()
}

// Token returns n crypto/rand bytes as base64url (no padding).
func Token(n int) string {
	if n <= 0 {
		n = 32
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand never fails on supported platforms; fall back to ULID bytes
		// so the caller always gets a token instead of a panic.
		return base64.RawURLEncoding.EncodeToString([]byte(New() + New()))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// Public ID prefixes used across the codebase.
const (
	PrefixProject      = "proj"
	PrefixAgent        = "agt"
	PrefixVisitor      = "vis"
	PrefixConversation = "conv"
	PrefixMessage      = "msg"
	PrefixRule         = "rule"
	PrefixTicket       = "tkt"
	PrefixKB           = "kb"
	PrefixWebhook      = "wh"
	PrefixDelivery     = "whd"
	PrefixChannel      = "ch"
	PrefixChannelIdent = "chi"
	PrefixInbox        = "wbi"
)
