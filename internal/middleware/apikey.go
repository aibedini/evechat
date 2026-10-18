package middleware

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

type projectKey struct{}

// WithProject stores the resolved project in a context.
func WithProject(ctx context.Context, p *models.Project) context.Context {
	return context.WithValue(ctx, projectKey{}, p)
}

// ProjectFrom returns the project resolved by the API key middleware.
func ProjectFrom(ctx context.Context) (*models.Project, bool) {
	p, ok := ctx.Value(projectKey{}).(*models.Project)
	return p, ok
}

type projectCacheEntry struct {
	p   *models.Project
	exp time.Time
}

// APIKey resolves X-Project-Key (or ?project_key=) to a project, with a 60s
// in-memory cache and a domain allow-list check.
type APIKey struct {
	repo       *repo.Repo
	log        zerolog.Logger
	ttl        time.Duration
	mu         sync.RWMutex
	cache      map[string]projectCacheEntry
	allowHosts []string
}

// NewAPIKey creates the middleware. allowHosts are always accepted origins
// (the dashboard / widget host itself).
func NewAPIKey(r *repo.Repo, log zerolog.Logger, allowHosts ...string) *APIKey {
	return &APIKey{
		repo:       r,
		log:        log,
		ttl:        60 * time.Second,
		cache:      make(map[string]projectCacheEntry, 64),
		allowHosts: allowHosts,
	}
}

func (m *APIKey) lookup(ctx context.Context, key string) (*models.Project, error) {
	now := time.Now()
	m.mu.RLock()
	e, ok := m.cache[key]
	m.mu.RUnlock()
	if ok && e.exp.After(now) {
		return e.p, nil
	}
	p, err := m.repo.Projects.GetByAPIKey(ctx, key)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	if len(m.cache) > 5000 {
		m.cache = make(map[string]projectCacheEntry, 64)
	}
	m.cache[key] = projectCacheEntry{p: p, exp: now.Add(m.ttl)}
	m.mu.Unlock()
	return p, nil
}

// Handler validates the project key and origin.
func (m *APIKey) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimSpace(r.Header.Get("X-Project-Key"))
		if key == "" {
			key = strings.TrimSpace(r.URL.Query().Get("project_key"))
		}
		if key == "" {
			apierr.Write(w, apierr.Unauthorized("کلید پروژه ارسال نشده است"))
			return
		}
		p, err := m.lookup(r.Context(), key)
		if err != nil {
			apierr.Write(w, apierr.Unauthorized("کلید پروژه معتبر نیست"))
			return
		}
		if !p.IsActive {
			apierr.Write(w, apierr.Forbidden("این پروژه غیرفعال است"))
			return
		}
		if host := requestHost(r); host != "" && !m.originAllowed(p, host) {
			m.log.Warn().Str("project_id", p.ID).Str("host", host).Msg("widget origin rejected")
			apierr.Write(w, apierr.Forbidden("این دامنه برای ویجت مجاز نیست"))
			return
		}
		next.ServeHTTP(w, r.WithContext(WithProject(r.Context(), p)))
	})
}

func requestHost(r *http.Request) string {
	if o := r.Header.Get("Origin"); o != "" && o != "null" {
		if u, err := url.Parse(o); err == nil && u.Host != "" {
			return strings.ToLower(u.Hostname())
		}
	}
	if ref := r.Header.Get("Referer"); ref != "" {
		if u, err := url.Parse(ref); err == nil && u.Host != "" {
			return strings.ToLower(u.Hostname())
		}
	}
	return ""
}

func (m *APIKey) originAllowed(p *models.Project, host string) bool {
	if len(p.Domains) == 0 {
		return true // dev mode: no allow-list configured
	}
	for _, d := range p.Domains {
		d = strings.ToLower(strings.TrimSpace(d))
		d = strings.TrimPrefix(d, "www.")
		if d == "" {
			continue
		}
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	for _, h := range m.allowHosts {
		h = strings.ToLower(strings.TrimSpace(h))
		if h != "" && (host == h || strings.HasSuffix(host, "."+h)) {
			return true
		}
	}
	return false
}

// Invalidate drops a cached project (used after settings changes).
func (m *APIKey) Invalidate(apiKey string) {
	m.mu.Lock()
	delete(m.cache, apiKey)
	m.mu.Unlock()
}
