// Package doctor runs preflight and runtime diagnostics for an EveChat
// installation. Every check carries a Persian fix hint so the operator always
// knows the exact next command.
package doctor

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/config"
	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/embedfs"
	"github.com/evechat/evechat/internal/integrations/eve"
	"github.com/evechat/evechat/internal/integrations/gmweb"
	"github.com/evechat/evechat/internal/integrations/smtp"
	"github.com/evechat/evechat/internal/repo"
)

// Level is the severity of a check.
type Level int

const (
	// Fatal failures block setup and exit non-zero.
	Fatal Level = iota
	// Warn findings degrade the experience but do not block.
	Warn
	// Info is informational only.
	Info
)

func (l Level) String() string {
	switch l {
	case Fatal:
		return "fatal"
	case Warn:
		return "warn"
	default:
		return "info"
	}
}

// Result is the outcome of one check.
type Result struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

// Check is one diagnostic probe.
type Check struct {
	ID    string                           `json:"id"`
	Name  string                           `json:"name"`
	Level Level                            `json:"level"`
	Run   func(ctx context.Context) Result `json:"-"`
}

// CheckResult is a check plus its result.
type CheckResult struct {
	Check
	Result Result `json:"result"`
}

// Report is the full diagnostic run.
type Report struct {
	Checks   []CheckResult `json:"checks"`
	Summary  Summary       `json:"summary"`
	Version  string        `json:"version"`
	Go       string        `json:"go_version"`
	Duration string        `json:"duration"`
	Time     time.Time     `json:"time"`
}

// Summary counts the outcomes.
type Summary struct {
	OK    int `json:"ok"`
	Warn  int `json:"warn"`
	Fail  int `json:"fail"`
	Total int `json:"total"`
}

// Options configure a run.
type Options struct {
	ConfigPath string
	Cfg        *config.Config
	Log        zerolog.Logger
	Only       []string
	Version    string
}

// ExitCode is 0 when everything passed, 1 on fatal failures, 2 on warnings.
func (r Report) ExitCode() int {
	if r.Summary.Fail > 0 {
		return 1
	}
	if r.Summary.Warn > 0 {
		return 2
	}
	return 0
}

// JSON renders the report.
func (r Report) JSON() []byte {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return []byte(`{"error":"marshal"}`)
	}
	return b
}

// Print renders the report in Persian.
func (r Report) Print(w io.Writer, verbose bool) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "╔══════════════════════════════════════════════════════╗")
	fmt.Fprintln(w, "║            بررسی سلامت سیستم EveChat                 ║")
	fmt.Fprintln(w, "╚══════════════════════════════════════════════════════╝")
	for _, c := range r.Checks {
		icon := "✓"
		if !c.Result.OK {
			if c.Level == Fatal {
				icon = "✗"
			} else {
				icon = "!"
			}
		}
		fmt.Fprintf(w, "%s %-22s %s\n", icon, c.ID, c.Result.Message)
		if verbose && c.Result.Detail != "" {
			fmt.Fprintf(w, "    %s\n", c.Result.Detail)
		}
		if !c.Result.OK && c.Result.Fix != "" {
			fmt.Fprintf(w, "    راه‌حل: %s\n", c.Result.Fix)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "نتیجه: %d موفق، %d هشدار، %d خطا (از %d بررسی)\n",
		r.Summary.OK, r.Summary.Warn, r.Summary.Fail, r.Summary.Total)
}

// RunAll executes every check (or the requested subset) against cfg.
func RunAll(ctx context.Context, opts Options) Report {
	started := time.Now()
	rep := Report{Version: opts.Version, Go: runtime.Version(), Time: started}
	cfg := opts.Cfg
	log := opts.Log

	allowed := map[string]bool{}
	for _, id := range opts.Only {
		allowed[strings.TrimSpace(id)] = true
	}

	for _, c := range checks(cfg, opts.ConfigPath, log) {
		if len(allowed) > 0 && !allowed[c.ID] {
			continue
		}
		res := c.Run(ctx)
		if res.Message == "" {
			res.Message = "—"
		}
		rep.Checks = append(rep.Checks, CheckResult{Check: c, Result: res})
		if res.OK {
			rep.Summary.OK++
		} else if c.Level == Fatal {
			rep.Summary.Fail++
		} else {
			rep.Summary.Warn++
		}
	}
	rep.Summary.Total = len(rep.Checks)
	rep.Duration = time.Since(started).Round(time.Millisecond).String()
	return rep
}

func checks(cfg *config.Config, configPath string, log zerolog.Logger) []Check {
	return []Check{
		{ID: "config.file", Name: "فایل تنظیمات", Level: Fatal, Run: func(ctx context.Context) Result {
			if configPath == "" {
				return Result{OK: true, Message: "تنظیمات از متغیرهای محیطی", Detail: "no --config supplied"}
			}
			f, err := os.Open(configPath)
			if err != nil {
				return Result{Message: "فایل تنظیمات خوانده نشد", Fix: "مسیر --config را بررسی کنید یا فایل را بسازید: cp evechat.example.yaml evechat.yaml", Detail: err.Error()}
			}
			_ = f.Close()
			return Result{OK: true, Message: configPath}
		}},
		{ID: "config.jwt", Name: "کلید JWT", Level: Fatal, Run: func(ctx context.Context) Result {
			if cfg.Auth.JWTSecret == "" || cfg.Auth.JWTSecret == "CHANGE_ME" {
				return Result{Message: "کلید JWT تنظیم نشده است", Fix: `export EVECHAT_AUTH__JWT_SECRET="$(openssl rand -base64 48)"`}
			}
			if len(cfg.Auth.JWTSecret) < 24 {
				return Result{Message: "کلید JWT کوتاه است", Fix: "حداقل ۲۴ کاراکتر تنظیم کنید؛ در .env مقدار EVECHAT_AUTH__JWT_SECRET را عوض کنید"}
			}
			return Result{OK: true, Message: fmt.Sprintf("%d کاراکتر", len(cfg.Auth.JWTSecret))}
		}},
		{ID: "config.keys", Name: "کلیدهای رمزنگاری", Level: Fatal, Run: func(ctx context.Context) Result {
			keys, err := cfg.EncryptionKeys()
			if err != nil {
				return Result{Message: "کلید رمزنگاری نامعتبر است", Fix: "کلید را با openssl rand -base64 32 بسازید و در EVECHAT_ENCRYPTION_KEY_V1 بگذارید", Detail: err.Error()}
			}
			if len(keys) == 0 {
				return Result{Message: "هیچ کلید رمزنگاری تنظیم نشده", Fix: "export EVECHAT_ENCRYPTION_KEY_V1=$(openssl rand -base64 32)"}
			}
			if _, ok := keys[cfg.Encryption.CurrentVersion]; !ok {
				return Result{Message: "کلید نسخه جاری موجود نیست", Fix: fmt.Sprintf("EVECHAT_ENCRYPTION_KEY_V%d را تنظیم کنید", cfg.Encryption.CurrentVersion)}
			}
			// Every key version referenced by stored messages must be loaded.
			if missing := missingKeyVersions(ctx, cfg, keys); len(missing) > 0 {
				return Result{Message: "کلید برخی پیام‌ها موجود نیست", Fix: fmt.Sprintf("کلیدهای نسخه %v را دوباره تنظیم کنید؛ پیام‌های آن نسخه قابل خواندن نیستند", missing)}
			}
			return Result{OK: true, Message: fmt.Sprintf("%d کلید بارگذاری شد", len(keys))}
		}},
		{ID: "fs.data", Name: "پوشه داده", Level: Fatal, Run: func(ctx context.Context) Result {
			dir := cfg.DataDir()
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return Result{Message: "پوشه داده ساخته نشد", Fix: "mkdir -p " + dir + " && chown evechat:evechat " + dir, Detail: err.Error()}
			}
			probe := filepath.Join(dir, ".doctor-probe")
			if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
				return Result{Message: "پوشه داده قابل نوشتن نیست", Fix: "chown -R evechat:evechat " + dir, Detail: err.Error()}
			}
			_ = os.Remove(probe)
			free := freeBytes(dir)
			if free > 0 && free < 200<<20 {
				return Result{Message: "فضای دیسک کم است", Fix: "حداقل ۲۰۰ مگابایت فضای آزاد کنید", Detail: fmt.Sprintf("%d MB free", free>>20)}
			}
			detail := ""
			if free > 0 {
				detail = fmt.Sprintf("%d MB free", free>>20)
			}
			return Result{OK: true, Message: dir, Detail: detail}
		}},
		{ID: "fs.uploads", Name: "پوشه پیوست‌ها", Level: Warn, Run: func(ctx context.Context) Result {
			dir := cfg.Storage.UploadsDir
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return Result{Message: "پوشه پیوست ساخته نشد", Fix: "mkdir -p " + dir, Detail: err.Error()}
			}
			probe := filepath.Join(dir, ".doctor-probe")
			if err := os.WriteFile(probe, []byte("ok"), 0o640); err != nil {
				return Result{Message: "پوشه پیوست قابل نوشتن نیست", Fix: "chown -R evechat:evechat " + dir, Detail: err.Error()}
			}
			_ = os.Remove(probe)
			return Result{OK: true, Message: dir}
		}},
		{ID: "fs.perms", Name: "مجوز فایل .env", Level: Warn, Run: func(ctx context.Context) Result {
			envPath := filepath.Join(cfg.DataDir(), ".env")
			if _, err := os.Stat(envPath); err != nil {
				envPath = ".env"
				if _, err2 := os.Stat(envPath); err2 != nil {
					return Result{OK: true, Message: ".env استفاده نمی‌شود"}
				}
			}
			info, err := os.Stat(envPath)
			if err != nil {
				return Result{Message: ".env خوانده نشد", Detail: err.Error()}
			}
			if info.Mode().Perm()&0o077 != 0 {
				return Result{Message: ".env برای دیگران قابل خواندن است", Fix: "chmod 600 " + envPath, Detail: info.Mode().String()}
			}
			return Result{OK: true, Message: "0600"}
		}},
		{ID: "db.open", Name: "اتصال پایگاه داده", Level: Fatal, Run: func(ctx context.Context) Result {
			d, err := db.Open(cfg)
			if err != nil {
				return Result{Message: "پایگاه داده باز نشد", Fix: "مسیر database.path و مجوزها را بررسی کنید", Detail: err.Error()}
			}
			defer d.Close()
			var integrity string
			if err := d.R.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
				return Result{Message: "integrity_check اجرا نشد", Detail: err.Error()}
			}
			if integrity != "ok" {
				return Result{Message: "پایگاه داده آسیب دیده است", Fix: "از آخرین پشتیبان بازیابی کنید: evechat restore <file>", Detail: integrity}
			}
			return Result{OK: true, Message: "سالم است"}
		}},
		{ID: "db.wal", Name: "حالت WAL", Level: Warn, Run: func(ctx context.Context) Result {
			d, err := db.Open(cfg)
			if err != nil {
				return Result{Message: "اتصال ناموفق", Detail: err.Error()}
			}
			defer d.Close()
			var mode string
			if err := d.R.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
				return Result{Message: "journal_mode خوانده نشد", Detail: err.Error()}
			}
			if strings.ToLower(mode) != "wal" {
				return Result{Message: "WAL فعال نیست", Fix: "مطمئن شوید فایل دیتابیس روی فایل‌سیستم شبکه‌ای نیست؛ سپس سرویس را ری‌استارت کنید", Detail: mode}
			}
			return Result{OK: true, Message: "wal"}
		}},
		{ID: "db.migrations", Name: "مهاجرت‌ها", Level: Warn, Run: func(ctx context.Context) Result {
			d, err := db.Open(cfg)
			if err != nil {
				return Result{Message: "اتصال ناموفق", Detail: err.Error()}
			}
			defer d.Close()
			latest, err := db.LatestMigrationVersion()
			if err != nil {
				return Result{Message: "نسخه مهاجرت خوانده نشد", Detail: err.Error()}
			}
			var current sql.NullInt64
			_ = d.R.QueryRowContext(ctx, "SELECT MAX(version) FROM schema_migrations").Scan(&current)
			if !current.Valid || int(current.Int64) < latest {
				return Result{Message: "مهاجرت‌های اجرانشده وجود دارد", Fix: "evechat --config " + cfgPathHint(cfg) + " migrate"}
			}
			return Result{OK: true, Message: fmt.Sprintf("نسخه %d", current.Int64)}
		}},
		{ID: "net.port", Name: "پورت سرور", Level: Fatal, Run: func(ctx context.Context) Result {
			addr := cfg.Addr()
			ln, err := net.Listen("tcp", addr)
			if err == nil {
				_ = ln.Close()
				return Result{OK: true, Message: addr + " آزاد است"}
			}
			if isOurs(ctx, cfg) {
				return Result{OK: true, Message: addr + " در حال استفاده توسط EveChat"}
			}
			return Result{Message: "پورت اشغال است", Fix: "پورت دیگری با server.port انتخاب کنید یا سرویس مزاحم را ببندید: ss -ltnp | grep " + fmt.Sprint(cfg.Server.Port), Detail: err.Error()}
		}},
		{ID: "net.outbound", Name: "دسترسی خروجی", Level: Warn, Run: func(ctx context.Context) Result {
			if !cfg.Telegram.Enabled || cfg.Telegram.BotToken == "" {
				return Result{OK: true, Message: "بررسی نشد (تلگرام غیرفعال)"}
			}
			req, _ := http.NewRequestWithContext(ctx, http.MethodHead, "https://api.telegram.org", nil)
			resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
			if err != nil {
				return Result{Message: "api.telegram.org در دسترس نیست", Fix: "دسترسی خروجی HTTPS به api.telegram.org را باز کنید (فایروال/پروکسی)", Detail: err.Error()}
			}
			defer resp.Body.Close()
			return Result{OK: true, Message: fmt.Sprintf("api.telegram.org (%d)", resp.StatusCode)}
		}},
		{ID: "net.publicurl", Name: "آدرس عمومی", Level: Warn, Run: func(ctx context.Context) Result {
			u, err := url.Parse(cfg.BaseURL())
			if err != nil || u.Host == "" {
				return Result{Message: "server.public_url نامعتبر است", Fix: `server.public_url: "https://chat.example.com"`}
			}
			if u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" {
				return Result{Message: "آدرس عمومی localhost است", Fix: "برای نصب واقعی server.public_url را به دامنه خود تغییر دهید"}
			}
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, cfg.BaseURL()+"/api/v1/health", nil)
			client := &http.Client{Timeout: 4 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return Result{Message: "آدرس عمومی پاسخ نداد", Fix: "DNS و پروکسی معکوس را بررسی کنید", Detail: err.Error()}
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return Result{Message: fmt.Sprintf("پاسخ %d از آدرس عمومی", resp.StatusCode), Fix: "پروکسی معکوس باید /api را به همین سرویس بفرستد"}
			}
			// The health payload must belong to this database, otherwise the
			// public URL points at a different installation.
			var health struct {
				InstanceID string `json:"instance_id"`
			}
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			_ = json.Unmarshal(body, &health)
			if local, ok, _ := localInstanceID(ctx, cfg); ok && health.InstanceID != "" && health.InstanceID != local {
				return Result{
					Message: "آدرس عمومی به نصب دیگری اشاره می‌کند",
					Fix:     "server.public_url و DNS را بررسی کنید؛ instance_id پاسخ با این پایگاه داده یکی نیست",
					Detail:  fmt.Sprintf("public=%s local=%s", health.InstanceID, local),
				}
			}
			return Result{OK: true, Message: cfg.BaseURL()}
		}},
		{ID: "time.skew", Name: "انحراف ساعت", Level: Warn, Run: func(ctx context.Context) Result {
			client := &http.Client{Timeout: 4 * time.Second}
			req, _ := http.NewRequestWithContext(ctx, http.MethodHead, "https://api.github.com", nil)
			resp, err := client.Do(req)
			if err != nil {
				return Result{OK: true, Message: "بررسی نشد (بدون اینترنت)", Detail: "offline"}
			}
			defer resp.Body.Close()
			serverTime, err := http.ParseTime(resp.Header.Get("Date"))
			if err != nil {
				return Result{OK: true, Message: "هدر Date دریافت نشد"}
			}
			skew := time.Since(serverTime)
			if skew < 0 {
				skew = -skew
			}
			if skew > 60*time.Second {
				return Result{Message: "ساعت سیستم بیش از ۶۰ ثانیه اختلاف دارد", Fix: "timedatectl set-ntp true", Detail: skew.String()}
			}
			return Result{OK: true, Message: skew.Round(time.Second).String()}
		}},
		{ID: "tls.cert", Name: "گواهی TLS", Level: Warn, Run: func(ctx context.Context) Result {
			if !cfg.Server.TLS.Enabled {
				return Result{OK: true, Message: "غیرفعال (TLS روی پروکسی)"}
			}
			cert, err := tls.LoadX509KeyPair(cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile)
			if err != nil {
				return Result{Message: "گواهی یا کلید خوانده نشد", Fix: "مسیرهای server.tls.cert_file و key_file را بررسی کنید", Detail: err.Error()}
			}
			leaf, err := parseLeaf(cert)
			if err != nil {
				return Result{Message: "گواهی نامعتبر است", Detail: err.Error()}
			}
			if time.Now().After(leaf.NotAfter) {
				return Result{Message: "گواهی منقضی شده است", Fix: "گواهی را تمدید کنید (certbot renew)", Detail: leaf.NotAfter.String()}
			}
			if time.Until(leaf.NotAfter) < 21*24*time.Hour {
				return Result{Message: "گواهی به‌زودی منقضی می‌شود", Fix: "certbot renew --dry-run", Detail: leaf.NotAfter.String()}
			}
			return Result{OK: true, Message: leaf.NotAfter.Format("2006-01-02")}
		}},
		{ID: "integration.telegram", Name: "تلگرام", Level: Warn, Run: func(ctx context.Context) Result {
			if !cfg.Telegram.Enabled || cfg.Telegram.BotToken == "" {
				return Result{OK: true, Message: "غیرفعال"}
			}
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.telegram.org/bot"+cfg.Telegram.BotToken+"/getMe", nil)
			resp, err := (&http.Client{Timeout: 6 * time.Second}).Do(req)
			if err != nil {
				return Result{Message: "اتصال به تلگرام برقرار نشد", Fix: "دسترسی خروجی به api.telegram.org را بررسی کنید", Detail: err.Error()}
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return Result{Message: "توکن ربات پذیرفته نشد", Fix: "EVECHAT_TELEGRAM__BOT_TOKEN را از BotFather دوباره بگیرید"}
			}
			return Result{OK: true, Message: "ربات متصل است"}
		}},
		{ID: "integration.eve", Name: "EVE", Level: Warn, Run: func(ctx context.Context) Result {
			client := eve.New(cfg.Integrations.EVE.BaseURL, cfg.Integrations.EVE.APIKey, cfg.Integrations.EVE.HMACSecret, cfg.Integrations.EVE.RequireSignature)
			if !client.Enabled() {
				return Result{OK: true, Message: "تنظیم نشده"}
			}
			if _, err := client.GetUserByMobile(ctx, "0000000000"); err != nil {
				return Result{Message: "EVE پاسخ نداد", Fix: "integrations.eve.base_url و api_key را بررسی کنید", Detail: err.Error()}
			}
			return Result{OK: true, Message: "متصل"}
		}},
		{ID: "integration.gmweb", Name: "GMweb", Level: Warn, Run: func(ctx context.Context) Result {
			client := gmweb.New(cfg.Integrations.GMweb.BaseURL, cfg.Integrations.GMweb.APIKey)
			if !client.Enabled() {
				return Result{OK: true, Message: "تنظیم نشده"}
			}
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(cfg.Integrations.GMweb.BaseURL, "/"), nil)
			resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
			if err != nil {
				return Result{Message: "درگاه پیامک پاسخ نداد", Fix: "integrations.gmweb.base_url را بررسی کنید", Detail: err.Error()}
			}
			_ = resp.Body.Close()
			return Result{OK: true, Message: "درگاه در دسترس است"}
		}},
		{ID: "integration.ai", Name: "دستیار هوش مصنوعی", Level: Warn, Run: func(ctx context.Context) Result {
			if !cfg.AI.Enabled || cfg.AI.ClaudeAPIKey == "" {
				return Result{OK: true, Message: "غیرفعال"}
			}
			body := `{"model":"` + cfg.AI.Model + `","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`
			req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", strings.NewReader(body))
			req.Header.Set("content-type", "application/json")
			req.Header.Set("x-api-key", cfg.AI.ClaudeAPIKey)
			req.Header.Set("anthropic-version", "2023-06-01")
			resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
			if err != nil {
				return Result{Message: "اتصال به Anthropic برقرار نشد", Fix: "دسترسی خروجی یا پروکسی را بررسی کنید", Detail: err.Error()}
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
				return Result{Message: "کلید API پذیرفته نشد", Fix: "EVECHAT_AI__CLAUDE_API_KEY را بررسی کنید", Detail: fmt.Sprint(resp.StatusCode)}
			}
			return Result{OK: true, Message: "کلید معتبر است"}
		}},
		{ID: "integration.smtp", Name: "ایمیل (SMTP)", Level: Warn, Run: func(ctx context.Context) Result {
			if !cfg.SMTP.Enabled {
				return Result{OK: true, Message: "غیرفعال"}
			}
			client := smtp.New(smtp.Config{
				Host: cfg.SMTP.Host, Port: cfg.SMTP.Port, Username: cfg.SMTP.Username, Password: cfg.SMTP.Password,
				FromName: cfg.SMTP.FromName, FromEmail: cfg.SMTP.FromEmail, TLSMode: cfg.SMTP.TLSMode,
				Timeout: cfg.SMTPTimeout(),
			})
			if err := client.Check(ctx); err != nil {
				return Result{Message: "اتصال SMTP ناموفق", Fix: "smtp.host/port/tls_mode و رمز عبور را بررسی کنید", Detail: err.Error()}
			}
			return Result{OK: true, Message: "اتصال برقرار شد"}
		}},
		{ID: "runtime.memory", Name: "حافظه مصرفی", Level: Info, Run: func(ctx context.Context) Result {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			rss := m.Sys
			if rss > 150<<20 {
				return Result{Message: "مصرف حافظه بالاست", Detail: fmt.Sprintf("%d MB", rss>>20)}
			}
			return Result{OK: true, Message: fmt.Sprintf("%d MB", rss>>20)}
		}},
		{ID: "runtime.fd", Name: "محدودیت فایل باز", Level: Warn, Run: func(ctx context.Context) Result {
			limit, ok := openFileLimit()
			if !ok {
				return Result{OK: true, Message: "نامشخص"}
			}
			if limit < 4096 {
				return Result{Message: "محدودیت فایل باز کم است", Fix: "در یونیت systemd خط LimitNOFILE=65535 را اضافه کنید", Detail: fmt.Sprint(limit)}
			}
			return Result{OK: true, Message: fmt.Sprint(limit)}
		}},
		{ID: "widget.size", Name: "اندازه ویجت", Level: Warn, Run: func(ctx context.Context) Result {
			data, err := embedfs.ReadWidget()
			if err != nil {
				return Result{Message: "ویجت در باینری نیست", Fix: "make build"}
			}
			var buf strings.Builder
			zw := gzip.NewWriter(&buf)
			_, _ = zw.Write(data)
			_ = zw.Close()
			if buf.Len() >= 16384 {
				return Result{Message: "ویجت از سقف ۱۶KB بیشتر است", Fix: "make build و بررسی بودجه اندازه", Detail: fmt.Sprintf("%d bytes gzip", buf.Len())}
			}
			return Result{OK: true, Message: fmt.Sprintf("%d bytes gzip", buf.Len())}
		}},
		{ID: "assets.dashboard", Name: "باندل پنل", Level: Warn, Run: func(ctx context.Context) Result {
			if !embedfs.HasDashboardIndex() {
				return Result{Message: "پنل در باینری نیست", Fix: "cd web/dashboard && npm ci && npm run build"}
			}
			return Result{OK: true, Message: "index.html موجود است"}
		}},
		{ID: "assets.font", Name: "فونت وزیرمتن", Level: Warn, Run: func(ctx context.Context) Result {
			if !embedfs.HasFont() {
				return Result{Message: "فونت در باینری نیست", Fix: "make fonts (یا scripts/fetch-fonts.sh) و سپس make build"}
			}
			return Result{OK: true, Message: "vazirmatn-variable.woff2"}
		}},
	}
}

// localInstanceID reads this installation's instance id so net.publicurl can
// tell "our own health endpoint" from "some other EveChat behind that hostname".
func localInstanceID(ctx context.Context, cfg *config.Config) (string, bool, error) {
	d, err := db.Open(cfg)
	if err != nil {
		return "", false, err
	}
	defer d.Close()
	return repo.New(d).Settings.Get(ctx, "instance_id")
}

func missingKeyVersions(ctx context.Context, cfg *config.Config, keys map[int][]byte) []int {
	d, err := db.Open(cfg)
	if err != nil {
		return nil
	}
	defer d.Close()
	rows, err := d.R.QueryContext(ctx, "SELECT DISTINCT key_version FROM messages")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var missing []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			continue
		}
		if _, ok := keys[v]; !ok {
			missing = append(missing, v)
		}
	}
	return missing
}

func isOurs(ctx context.Context, cfg *config.Config) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/api/v1/health", cfg.Server.Port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func cfgPathHint(cfg *config.Config) string {
	return "evechat.yaml"
}

func parseLeaf(cert tls.Certificate) (*x509.Certificate, error) {
	if len(cert.Certificate) == 0 {
		return nil, fmt.Errorf("certificate chain is empty")
	}
	return x509.ParseCertificate(cert.Certificate[0])
}

// Fix applies only the safe repairs: create directories, tighten .env
// permissions, run pending migrations and re-register the Telegram webhook.
// It never touches keys, data or configuration values.
func Fix(ctx context.Context, cfg *config.Config) []string {
	actions := []string{}
	if dir := cfg.DataDir(); dir != "" {
		if err := os.MkdirAll(dir, 0o750); err == nil {
			actions = append(actions, "ساخت پوشه داده: "+dir)
		}
	}
	if dir := cfg.Storage.UploadsDir; dir != "" {
		if err := os.MkdirAll(dir, 0o750); err == nil {
			actions = append(actions, "ساخت پوشه پیوست‌ها: "+dir)
		}
	}
	for _, candidate := range []string{".env", filepath.Join(cfg.DataDir(), ".env")} {
		if info, err := os.Stat(candidate); err == nil {
			if info.Mode().Perm()&0o077 != 0 {
				if err := os.Chmod(candidate, 0o600); err == nil {
					actions = append(actions, "محدود کردن دسترسی .env به 0600: "+candidate)
				}
			}
		}
	}
	if d, err := db.Open(cfg); err == nil {
		if err := db.Migrate(ctx, d, zerolog.Nop()); err == nil {
			actions = append(actions, "اجرای مهاجرت‌های پایگاه داده")
		}
		_ = d.Close()
	}
	if cfg.Telegram.Enabled && cfg.Telegram.BotToken != "" && cfg.Telegram.WebhookURL != "" {
		params := "url=" + url.QueryEscape(cfg.Telegram.WebhookURL)
		if cfg.Telegram.Secret != "" {
			params += "&secret_token=" + url.QueryEscape(cfg.Telegram.Secret)
		}
		endpoint := "https://api.telegram.org/bot" + cfg.Telegram.BotToken + "/setWebhook?" + params
		if resp, err := (&http.Client{Timeout: 8 * time.Second}).Get(endpoint); err == nil {
			_ = resp.Body.Close()
			actions = append(actions, "ثبت مجدد وب‌هوک تلگرام")
		}
	}
	return actions
}
