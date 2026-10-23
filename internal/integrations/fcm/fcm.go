// Package fcm sends data-only FCM HTTP v1 messages with a Google service
// account. It intentionally has no Firebase Admin SDK dependency: the server
// only needs an OAuth assertion, one POST per token, and the documented error
// mapping for stale registrations.
package fcm

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultBaseURL is the FCM HTTP v1 endpoint root.
	DefaultBaseURL = "https://fcm.googleapis.com/v1"
	// DefaultTokenURL is Google's OAuth 2.0 token endpoint.
	DefaultTokenURL = "https://oauth2.googleapis.com/token"
	// Scope is the only OAuth scope required by FCM HTTP v1.
	Scope = "https://www.googleapis.com/auth/firebase.messaging"
)

// Config is the subset of the Firebase service-account JSON needed by FCM.
// ServiceAccountJSON wins over ServiceAccountFile.
type Config struct {
	ProjectID          string
	ServiceAccountJSON string
	ServiceAccountFile string
	// BaseURL and TokenURL are overridable for tests and private endpoints.
	BaseURL    string
	TokenURL   string
	HTTPClient *http.Client
}

// Options control Android delivery. Priority is HIGH only for events a person
// should see; TTL and CollapseKey follow the app contract.
type Options struct {
	Priority    string
	TTL         string
	CollapseKey string
}

// Message is one data-only FCM message. Data values must be strings.
type Message struct {
	Token   string
	Data    map[string]string
	Options Options
}

// Sender is the interface the notification channel depends on.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// Error is a non-2xx FCM response. InvalidToken reports registrations that
// should be removed (UNREGISTERED or INVALID_ARGUMENT / 400 / 404).
type Error struct {
	StatusCode int
	Code       string
	Status     string
	Message    string
}

func (e *Error) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("fcm: HTTP %d %s: %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("fcm: HTTP %d: %s", e.StatusCode, e.Message)
}

// InvalidToken reports whether the device token must be deleted.
func (e *Error) InvalidToken() bool {
	if e == nil {
		return false
	}
	switch e.Code {
	case "UNREGISTERED", "INVALID_ARGUMENT":
		return true
	}
	if e.Status == "NOT_FOUND" || e.Status == "INVALID_ARGUMENT" {
		return true
	}
	return e.StatusCode == http.StatusBadRequest || e.StatusCode == http.StatusNotFound
}

type serviceAccount struct {
	ProjectID   string `json:"project_id"`
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

// Client caches the OAuth access token and sends messages.
type Client struct {
	projectID string
	email     string
	key       *rsa.PrivateKey
	baseURL   string
	tokenURL  string
	http      *http.Client

	mu          sync.Mutex
	accessToken string
	expiry      time.Time
}

// New parses the service account and returns a sender. It performs no network
// I/O; a missing or malformed credential is reported here so the caller can
// disable push without breaking the server.
func New(cfg Config) (*Client, error) {
	raw := strings.TrimSpace(cfg.ServiceAccountJSON)
	if raw == "" && strings.TrimSpace(cfg.ServiceAccountFile) != "" {
		b, err := os.ReadFile(strings.TrimSpace(cfg.ServiceAccountFile))
		if err != nil {
			return nil, fmt.Errorf("reading Firebase service account: %w", err)
		}
		raw = string(b)
	}
	if raw == "" {
		return nil, fmt.Errorf("Firebase service account JSON is empty")
	}

	var sa serviceAccount
	if err := json.Unmarshal([]byte(raw), &sa); err != nil {
		return nil, fmt.Errorf("parsing Firebase service account: %w", err)
	}
	projectID := strings.TrimSpace(cfg.ProjectID)
	if projectID == "" {
		projectID = strings.TrimSpace(sa.ProjectID)
	}
	if projectID == "" {
		return nil, fmt.Errorf("Firebase project id is missing")
	}
	if strings.TrimSpace(sa.ClientEmail) == "" {
		return nil, fmt.Errorf("Firebase client_email is missing")
	}
	key, err := parsePrivateKey(sa.PrivateKey)
	if err != nil {
		return nil, err
	}

	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	tokenURL := strings.TrimSpace(cfg.TokenURL)
	if tokenURL == "" {
		tokenURL = strings.TrimSpace(sa.TokenURI)
	}
	if tokenURL == "" {
		tokenURL = DefaultTokenURL
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	return &Client{
		projectID: projectID,
		email:     strings.TrimSpace(sa.ClientEmail),
		key:       key,
		baseURL:   baseURL,
		tokenURL:  tokenURL,
		http:      httpClient,
	}, nil
}

// ProjectID is safe to log (it is not a secret).
func (c *Client) ProjectID() string { return c.projectID }

// Send posts one data-only message to FCM HTTP v1.
func (c *Client) Send(ctx context.Context, msg Message) error {
	if strings.TrimSpace(msg.Token) == "" {
		return fmt.Errorf("fcm: empty device token")
	}
	access, err := c.token(ctx)
	if err != nil {
		return err
	}

	out := outgoing{
		Token: msg.Token,
		Data:  cloneData(msg.Data),
	}
	if msg.Options.Priority != "" || msg.Options.TTL != "" || msg.Options.CollapseKey != "" {
		out.Android = &androidOptions{
			Priority:    msg.Options.Priority,
			TTL:         msg.Options.TTL,
			CollapseKey: msg.Options.CollapseKey,
		}
	}
	body, err := json.Marshal(outgoingEnvelope{Message: out})
	if err != nil {
		return fmt.Errorf("encoding fcm message: %w", err)
	}

	endpoint := c.baseURL + "/projects/" + url.PathEscape(c.projectID) + "/messages:send"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("building fcm request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("sending fcm message: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return parseError(resp.StatusCode, respBody)
}

func (c *Client) token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.accessToken != "" && time.Now().Before(c.expiry.Add(-time.Minute)) {
		return c.accessToken, nil
	}

	assertion, err := c.assertion()
	if err != nil {
		return "", err
	}
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", assertion)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("building token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching Firebase access token: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Firebase token exchange failed: HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	var tr struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("decoding Firebase token response: %w", err)
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("Firebase token response had no access_token")
	}
	expires := tr.ExpiresIn
	if expires <= 0 {
		expires = 3600
	}
	c.accessToken = tr.AccessToken
	c.expiry = time.Now().Add(time.Duration(expires) * time.Second)
	return c.accessToken, nil
}

func (c *Client) assertion() (string, error) {
	now := time.Now()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims, err := json.Marshal(map[string]any{
		"iss":   c.email,
		"scope": Scope,
		"aud":   c.tokenURL,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("encoding JWT claims: %w", err)
	}
	unsigned := header + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, c.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("signing JWT: %w", err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func parsePrivateKey(raw string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, fmt.Errorf("Firebase private_key is not valid PEM")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing Firebase private_key: %w", err)
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("Firebase private_key is not an RSA key")
	}
	return key, nil
}

func parseError(status int, body []byte) error {
	var env struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Status  string `json:"status"`
			Details []struct {
				ErrorCode string `json:"errorCode"`
			} `json:"details"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &env)
	e := &Error{
		StatusCode: status,
		Status:     env.Error.Status,
		Message:    env.Error.Message,
	}
	for _, d := range env.Error.Details {
		if d.ErrorCode != "" {
			e.Code = d.ErrorCode
			break
		}
	}
	if e.Message == "" {
		e.Message = truncate(string(body), 200)
	}
	return e
}

type outgoingEnvelope struct {
	Message outgoing `json:"message"`
}

type outgoing struct {
	Token   string            `json:"token"`
	Data    map[string]string `json:"data,omitempty"`
	Android *androidOptions   `json:"android,omitempty"`
}

type androidOptions struct {
	Priority    string `json:"priority,omitempty"`
	TTL         string `json:"ttl,omitempty"`
	CollapseKey string `json:"collapse_key,omitempty"`
}

func cloneData(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
