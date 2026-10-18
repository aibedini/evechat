// Package gmweb is the client for the GMweb SMS gateway.
package gmweb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/evechat/evechat/internal/pii"
)

// Client sends SMS through GMweb.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// New builds the client.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 8 * time.Second},
	}
}

// Enabled reports whether the integration is configured.
func (c *Client) Enabled() bool { return c != nil && c.baseURL != "" && c.apiKey != "" }

// SendSMS delivers one text message with 2 retries and backoff.
func (c *Client) SendSMS(ctx context.Context, phone, text string) error {
	if !c.Enabled() {
		return fmt.Errorf("gmweb: integration is not configured")
	}
	payload, err := json.Marshal(map[string]string{"to": phone, "message": text})
	if err != nil {
		return err
	}
	var lastErr error
	backoff := 500 * time.Millisecond
	for attempt := 0; attempt < 3; attempt++ {
		if err := c.post(ctx, payload); err == nil {
			return nil
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	return fmt.Errorf("gmweb: send sms to %s: %w", pii.MaskPhone(phone), sanitize(lastErr, phone))
}

func (c *Client) post(ctx context.Context, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/send", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// InboundMessage is a parsed incoming SMS webhook.
type InboundMessage struct {
	From string `json:"from"`
	Text string `json:"message"`
}

// ParseInbound accepts JSON or form-encoded gateway callbacks.
func ParseInbound(r *http.Request) (InboundMessage, error) {
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") {
		var m InboundMessage
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&m); err != nil {
			return m, err
		}
		if m.From == "" {
			return m, fmt.Errorf("gmweb: missing from")
		}
		return m, nil
	}
	if err := r.ParseForm(); err != nil {
		return InboundMessage{}, err
	}
	m := InboundMessage{From: r.PostFormValue("from"), Text: r.PostFormValue("message")}
	if m.From == "" {
		m.From = r.PostFormValue("sender")
	}
	if m.Text == "" {
		m.Text = r.PostFormValue("text")
	}
	if m.From == "" {
		return m, fmt.Errorf("gmweb: missing from")
	}
	return m, nil
}

// sanitize removes the raw phone number from an error string so a transport
// error cannot leak the number into the logs.
func sanitize(err error, phone string) error {
	if err == nil || phone == "" {
		return err
	}
	msg := strings.ReplaceAll(err.Error(), phone, pii.MaskPhone(phone))
	return errors.New(msg)
}
