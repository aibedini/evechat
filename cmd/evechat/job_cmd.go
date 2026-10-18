package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/config"
	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/integrations/smtp"
	"github.com/evechat/evechat/internal/repo"
	"github.com/evechat/evechat/internal/services"
	"github.com/evechat/evechat/internal/setup"
)

// runJobCommand runs one scheduled job. systemd timers call it as
// `evechat job missed` and `evechat job digest`.
func runJobCommand(args []string) {
	fs := flag.NewFlagSet("job", flag.ExitOnError)
	cfgPath := "./evechat.yaml"
	registerCommonFlags(fs, &cfgPath)
	olderThan := fs.Duration("older-than", 10*time.Minute, "سن انتظار برای گفتگوهای بی‌پاسخ")
	quiet := fs.Bool("quiet", false, "فقط خروجی خطا")
	_ = fs.Parse(args)

	if fs.NArg() == 0 {
		fatalf("نام کار لازم است: missed یا digest\n  evechat job missed\n  evechat job digest")
	}
	job := fs.Arg(0)

	setup.LoadDotEnv(filepath.Join(filepath.Dir(cfgPath), ".env"))
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fatalf("%v", err)
	}
	if err := cfg.Validate(); err != nil {
		fatalf("تنظیمات نامعتبر است: %v", err)
	}

	level, levelErr := zerolog.ParseLevel(cfg.Log.Level)
	if levelErr != nil || level == zerolog.NoLevel {
		level = zerolog.InfoLevel
	}
	logger := zerolog.New(os.Stderr).Level(level).With().Timestamp().Logger()
	if *quiet {
		logger = zerolog.Nop()
	}

	d, err := db.Open(cfg)
	if err != nil {
		fatalf("اتصال به پایگاه داده ناموفق بود: %v", err)
	}
	defer d.Close()

	var client *smtp.Client
	if cfg.SMTP.Enabled && cfg.SMTP.Host != "" {
		client = smtp.New(smtp.Config{
			Host: cfg.SMTP.Host, Port: cfg.SMTP.Port, Username: cfg.SMTP.Username,
			Password: cfg.SMTP.Password, FromName: cfg.SMTP.FromName, FromEmail: cfg.SMTP.FromEmail,
			TLSMode: cfg.SMTP.TLSMode, Timeout: cfg.SMTPTimeout(),
		})
	}

	r := repo.New(d)
	svc := services.NewDigestService(r, client, cfg.Server.PublicURL, logger.With().Str("job", job).Logger())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var sent int
	switch job {
	case "missed":
		sent, err = svc.RunMissed(ctx, *olderThan)
	case "digest":
		sent, err = svc.RunDaily(ctx)
	default:
		fatalf("کار ناشناخته: %s (missed یا digest)", job)
	}
	if err != nil {
		fatalf("%v", err)
	}
	if !*quiet {
		fmt.Fprintf(os.Stdout, "%s: %d ایمیل ارسال شد\n", job, sent)
	}
}
