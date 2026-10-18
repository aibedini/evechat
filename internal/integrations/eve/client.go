// Package eve is the client for the EVE mobile-platform API.
package eve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/evechat/evechat/internal/crypto"
)

// User is the EVE profile of a mobile number.
type User struct {
	UserID       string         `json:"user_id"`
	Mobile       string         `json:"mobile"`
	ResellerID   string         `json:"reseller_id"`
	Subscription map[string]any `json:"subscription"`
}

// Client talks to the EVE API.
type Client struct {
	baseURL          string
	apiKey           string
	hmacSecret       string
	requireSignature bool
	http             *http.Client
}

// New builds the client.
func New(baseURL, apiKey, hmacSecret string, requireSignature bool) *Client {
	return &Client{
		baseURL:          strings.TrimRight(baseURL, "/"),
		apiKey:           apiKey,
		hmacSecret:       hmacSecret,
		requireSignature: requireSignature,
		http:             &http.Client{Timeout: 5 * time.Second},
	}
}

// Enabled reports whether the integration is configured.
func (c *Client) Enabled() bool { return c != nil && c.baseURL != "" && c.apiKey != "" }

// RequireSignature reports whether signed contexts are mandatory.
func (c *Client) RequireSignature() bool { return c != nil && c.requireSignature }

// GetUserByMobile fetches the EVE profile of a mobile number.
func (c *Client) GetUserByMobile(ctx context.Context, mobile string) (*User, error) {
	if !c.Enabled() {
		return nil, errors.New("eve integration is not configured")
	}
	endpoint := fmt.Sprintf("%s/api/user/%s", c.baseURL, url.PathEscape(strings.TrimSpace(mobile)))
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		u, err := c.get(ctx, endpoint)
		if err == nil {
			return u, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	return nil, fmt.Errorf("eve: get user %s: %w", mobile, lastErr)
}

func (c *Client) get(ctx context.Context, endpoint string) (*User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	var wrapper struct {
		Data *User `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapper); err == nil && wrapper.Data != nil {
		return wrapper.Data, nil
	}
	var u User
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &u, nil
}

// VerifySignedContext validates the HMAC of a visitor context payload.
func (c *Client) VerifySignedContext(payload []byte, ts int64, sig string) bool {
	if c == nil || c.hmacSecret == "" {
		return false
	}
	return crypto.VerifyEVEContext(c.hmacSecret, payload, ts, sig, 5*time.Minute)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
