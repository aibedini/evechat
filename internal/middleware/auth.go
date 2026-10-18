package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/models"
)

// Claims is the JWT payload for dashboard sessions.
type Claims struct {
	jwt.RegisteredClaims
	ProjectID string      `json:"pid"`
	Role      models.Role `json:"role"`
	Typ       string      `json:"typ"`
	Email     string      `json:"email,omitempty"`
	Name      string      `json:"name,omitempty"`
}

// TokenPair is what login/refresh return.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

// TokenManager issues and validates HS256 tokens.
type TokenManager struct {
	secret  []byte
	access  time.Duration
	refresh time.Duration
}

// NewTokenManager builds a manager.
func NewTokenManager(secret string, access, refresh time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), access: access, refresh: refresh}
}

// Issue creates an access/refresh pair for an agent.
func (m *TokenManager) Issue(a *models.Agent) (TokenPair, error) {
	now := time.Now()
	accessExp := now.Add(m.access)
	base := jwt.RegisteredClaims{
		Subject:   a.ID,
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now.Add(-30 * time.Second)),
	}
	accessClaims := Claims{
		RegisteredClaims: base,
		ProjectID:        a.ProjectID,
		Role:             a.Role,
		Typ:              "access",
		Email:            a.Email,
		Name:             a.Name,
	}
	accessClaims.ID = ""
	accessClaims.ExpiresAt = jwt.NewNumericDate(accessExp)

	refreshClaims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   a.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-30 * time.Second)),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refresh)),
		},
		ProjectID: a.ProjectID,
		Role:      a.Role,
		Typ:       "refresh",
	}
	at, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString(m.secret)
	if err != nil {
		return TokenPair{}, err
	}
	rt, err := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString(m.secret)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:  at,
		RefreshToken: rt,
		TokenType:    "Bearer",
		ExpiresIn:    int64(m.access.Seconds()),
	}, nil
}

// Parse validates a token and returns its claims.
func (m *TokenManager) Parse(token string) (*Claims, error) {
	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// ParseTyp validates a token and its type ("access" or "refresh").
func (m *TokenManager) ParseTyp(token, typ string) (*Claims, error) {
	c, err := m.Parse(token)
	if err != nil {
		return nil, err
	}
	if c.Typ != typ {
		return nil, errors.New("wrong token type")
	}
	return c, nil
}

type claimsKey struct{}

// WithClaims stores claims in a context.
func WithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, c)
}

// CurrentAgent returns the authenticated dashboard claims.
func CurrentAgent(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(claimsKey{}).(*Claims)
	return c, ok
}

// BearerToken extracts a token from the Authorization header or ?token=.
func BearerToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		if len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
			return strings.TrimSpace(h[7:])
		}
	}
	return strings.TrimSpace(r.URL.Query().Get("token"))
}

// Auth authenticates dashboard requests.
func Auth(m *TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok := BearerToken(r)
			if tok == "" {
				apierr.Write(w, apierr.Unauthorized("برای دسترسی باید وارد شوید"))
				return
			}
			claims, err := m.ParseTyp(tok, "access")
			if err != nil {
				apierr.Write(w, apierr.Unauthorized("نشست شما منقضی شده است"))
				return
			}
			next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), claims)))
		})
	}
}

// RequireRole restricts a route to the given roles.
func RequireRole(roles ...models.Role) func(http.Handler) http.Handler {
	allowed := make(map[models.Role]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := CurrentAgent(r.Context())
			if !ok {
				apierr.Write(w, apierr.Unauthorized("احراز هویت لازم است"))
				return
			}
			if _, ok := allowed[claims.Role]; !ok {
				apierr.Write(w, apierr.Forbidden("شما به این بخش دسترسی ندارید"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
