package middleware

import (
	"hash/fnv"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/evechat/evechat/internal/apierr"
)

const rlShards = 16

type bucket struct {
	lim  *rate.Limiter
	seen time.Time
}

type rlShard struct {
	mu sync.Mutex
	m  map[string]*bucket
}

// RateLimiter is a sharded token-bucket limiter with idle eviction.
// No Redis is involved: limits are per node.
type RateLimiter struct {
	shards    [rlShards]*rlShard
	limit     rate.Limit
	burst     int
	ttl       time.Duration
	calls     int64
	sweepMu   sync.Mutex
	lastSweep time.Time
	mu        sync.Mutex
}

// NewRateLimiter builds a limiter allowing perMin requests per key with the
// given burst.
func NewRateLimiter(perMin, burst int) *RateLimiter {
	if perMin <= 0 {
		perMin = 60
	}
	if burst <= 0 {
		burst = perMin
	}
	l := &RateLimiter{
		limit:     rate.Limit(float64(perMin) / 60.0),
		burst:     burst,
		ttl:       10 * time.Minute,
		lastSweep: time.Now(),
	}
	for i := 0; i < rlShards; i++ {
		l.shards[i] = &rlShard{m: make(map[string]*bucket, 64)}
	}
	return l
}

func shardIndex(key string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return h.Sum32() % rlShards
}

// Allow consumes one token for key.
func (l *RateLimiter) Allow(key string) bool {
	idx := shardIndex(key)
	sh := l.shards[idx]
	now := time.Now()

	sh.mu.Lock()
	b, ok := sh.m[key]
	if !ok {
		b = &bucket{lim: rate.NewLimiter(l.limit, l.burst)}
		sh.m[key] = b
	}
	b.seen = now
	allowed := b.lim.AllowN(now, 1)
	sh.mu.Unlock()

	l.mu.Lock()
	l.calls++
	needSweep := l.calls%2048 == 0 && now.Sub(l.lastSweep) > time.Minute
	if needSweep {
		l.lastSweep = now
	}
	l.mu.Unlock()
	if needSweep {
		l.sweep(now)
	}
	return allowed
}

func (l *RateLimiter) sweep(now time.Time) {
	l.sweepMu.Lock()
	defer l.sweepMu.Unlock()
	cutoff := now.Add(-l.ttl)
	for _, sh := range l.shards {
		sh.mu.Lock()
		for k, b := range sh.m {
			if b.seen.Before(cutoff) {
				delete(sh.m, k)
			}
		}
		sh.mu.Unlock()
	}
}

// KeyFunc derives the bucket key of a request.
type KeyFunc func(*http.Request) string

// Middleware rejects requests over the limit with 429 and Retry-After.
func (l *RateLimiter) Middleware(keyFn KeyFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFn(r)
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}
			if !l.Allow(key) {
				secs := int(1 / float64(l.limit))
				if secs < 1 {
					secs = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(secs))
				apierr.Write(w, apierr.RateLimited("تعداد درخواست‌ها بیش از حد مجاز است"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
