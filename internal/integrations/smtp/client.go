// Package smtp is a dependency-free email client used by the notification
// channel and the doctor. It supports plain, STARTTLS and implicit TLS.
package smtp

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// Config describes the SMTP endpoint.
type Config struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromName  string
	FromEmail string
	TLSMode   string // plain | starttls | implicit
	Timeout   time.Duration
}

// Client sends mail through the configured server.
type Client struct {
	cfg Config
}

// New builds a client.
func New(cfg Config) *Client {
	if cfg.Port == 0 {
		cfg.Port = 587
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.TLSMode == "" {
		cfg.TLSMode = "starttls"
	}
	return &Client{cfg: cfg}
}

func (c *Client) addr() string { return fmt.Sprintf("%s:%d", c.cfg.Host, c.cfg.Port) }

func (c *Client) dial(ctx context.Context) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: c.cfg.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", c.addr())
	if err != nil {
		return nil, fmt.Errorf("smtp dial %s: %w", c.addr(), err)
	}
	if c.cfg.TLSMode == "implicit" {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: c.cfg.Host, MinVersion: tls.VersionTLS12})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("smtp tls handshake: %w", err)
		}
		return tlsConn, nil
	}
	return conn, nil
}

func (c *Client) auth() smtp.Auth {
	if c.cfg.Username == "" {
		return nil
	}
	return smtp.PlainAuth("", c.cfg.Username, c.cfg.Password, c.cfg.Host)
}

func (c *Client) connect(ctx context.Context) (*smtp.Client, net.Conn, error) {
	conn, err := c.dial(ctx)
	if err != nil {
		return nil, nil, err
	}
	client, err := smtp.NewClient(conn, c.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("smtp client: %w", err)
	}
	if c.cfg.TLSMode == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: c.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
				_ = client.Close()
				return nil, nil, fmt.Errorf("smtp starttls: %w", err)
			}
		} else if c.cfg.Port == 587 {
			_ = client.Close()
			return nil, nil, fmt.Errorf("smtp server does not offer STARTTLS on port 587")
		}
	}
	if auth := c.auth(); auth != nil {
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(auth); err != nil {
				_ = client.Close()
				return nil, nil, fmt.Errorf("smtp auth: %w", err)
			}
		}
	}
	return client, conn, nil
}

// Check connects, upgrades and authenticates without sending a message.
func (c *Client) Check(ctx context.Context) error {
	client, _, err := c.connect(ctx)
	if err != nil {
		return err
	}
	_ = client.Quit()
	return nil
}

// Send delivers a multipart (HTML + plain text) message.
func (c *Client) Send(ctx context.Context, to, subject, htmlBody, textBody string) error {
	if c.cfg.Host == "" {
		return fmt.Errorf("smtp: host is not configured")
	}
	from := c.cfg.FromEmail
	if from == "" {
		from = c.cfg.Username
	}
	if from == "" {
		return fmt.Errorf("smtp: from address is not configured")
	}
	client, _, err := c.connect(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = client.Quit() }()

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp MAIL: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp RCPT: %w", err)
	}
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	msg := buildMessage(c.cfg.FromName, from, to, subject, htmlBody, textBody)
	if _, err := wc.Write(msg); err != nil {
		_ = wc.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("smtp data close: %w", err)
	}
	return nil
}

func buildMessage(fromName, from, to, subject, htmlBody, textBody string) []byte {
	boundary := randomBoundary()
	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\n", formatAddress(fromName, from))
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.BEncoding.Encode("UTF-8", subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", boundary)
	fmt.Fprintf(&b, "--%s\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n", boundary)
	b.WriteString(wrapBase64(textBody))
	b.WriteString("\r\n")
	fmt.Fprintf(&b, "--%s\r\nContent-Type: text/html; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n", boundary)
	b.WriteString(wrapBase64(htmlBody))
	b.WriteString("\r\n")
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return b.Bytes()
}

func formatAddress(name, email string) string {
	if name == "" {
		return email
	}
	return mime.BEncoding.Encode("UTF-8", name) + " <" + email + ">"
}

func wrapBase64(s string) string {
	enc := base64.StdEncoding.EncodeToString([]byte(s))
	var b strings.Builder
	for i := 0; i < len(enc); i += 76 {
		end := i + 76
		if end > len(enc) {
			end = len(enc)
		}
		b.WriteString(enc[i:end])
		b.WriteString("\r\n")
	}
	return b.String()
}

func randomBoundary() string {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("evechat-%d", time.Now().UnixNano())
	}
	return "evechat-" + hex.EncodeToString(buf)
}
