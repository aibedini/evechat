package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/evechat/evechat/internal/integrations/smtp"
)

// Probe results are Persian, user-facing messages for the CLI wizard.

func httpClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

// TestTelegram verifies a bot token with getMe.
func TestTelegram(ctx context.Context, token string) (bool, string) {
	if strings.TrimSpace(token) == "" {
		return false, "توکن ربات لازم است"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.telegram.org/bot"+token+"/getMe", nil)
	if err != nil {
		return false, "درخواست نامعتبر است"
	}
	resp, err := httpClient(6 * time.Second).Do(req)
	if err != nil {
		return false, "اتصال به تلگرام برقرار نشد"
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, "توکن ربات پذیرفته نشد"
	}
	var payload struct {
		Result struct {
			Username string `json:"username"`
		} `json:"result"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&payload)
	return true, "ربات @" + payload.Result.Username + " متصل شد"
}

// TestEVE checks that the EVE API answers and accepts the key.
func TestEVE(ctx context.Context, baseURL, apiKey string) (bool, string) {
	if baseURL == "" {
		return false, "آدرس EVE لازم است"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/user/0000000000", nil)
	if err != nil {
		return false, "آدرس نامعتبر است"
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := httpClient(6 * time.Second).Do(req)
	if err != nil {
		return false, "EVE پاسخ نداد"
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return false, "کلید EVE پذیرفته نشد"
	}
	return true, "EVE در دسترس است"
}

// TestURL checks that an endpoint answers at all.
func TestURL(ctx context.Context, rawURL string) (bool, string) {
	if rawURL == "" {
		return false, "آدرس لازم است"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(rawURL, "/"), nil)
	if err != nil {
		return false, "آدرس نامعتبر است"
	}
	resp, err := httpClient(5 * time.Second).Do(req)
	if err != nil {
		return false, "اتصال برقرار نشد"
	}
	_ = resp.Body.Close()
	return true, "در دسترس است"
}

// TestAnthropic sends a one-token request to validate the API key.
func TestAnthropic(ctx context.Context, apiKey, model string) (bool, string) {
	if apiKey == "" {
		return false, "کلید API لازم است"
	}
	if model == "" {
		model = "claude-haiku-4-5"
	}
	body := fmt.Sprintf(`{"model":%q,"max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`, model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", strings.NewReader(body))
	if err != nil {
		return false, "درخواست نامعتبر است"
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := httpClient(10 * time.Second).Do(req)
	if err != nil {
		return false, "اتصال به Anthropic برقرار نشد"
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return false, "کلید API پذیرفته نشد"
	}
	if resp.StatusCode >= 500 {
		return false, fmt.Sprintf("خطای سرویس (%d)", resp.StatusCode)
	}
	return true, "کلید معتبر است"
}

// SMTPProbe is the input of TestSMTP.
type SMTPProbe struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	TLSMode  string
}

// TestSMTP connects, upgrades and authenticates without sending mail.
func TestSMTP(ctx context.Context, p SMTPProbe) (bool, string) {
	client := smtp.New(smtp.Config{
		Host: p.Host, Port: p.Port, Username: p.Username, Password: p.Password,
		FromEmail: p.From, TLSMode: firstNonEmpty(p.TLSMode, "starttls"), Timeout: 10 * time.Second,
	})
	if err := client.Check(ctx); err != nil {
		return false, err.Error()
	}
	return true, "اتصال برقرار شد"
}
