package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/config"
	"github.com/evechat/evechat/internal/crypto"
	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/handlers"
	"github.com/evechat/evechat/internal/hub"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/integrations/eve"
	"github.com/evechat/evechat/internal/integrations/fcm"
	"github.com/evechat/evechat/internal/integrations/gmweb"
	"github.com/evechat/evechat/internal/integrations/smtp"
	"github.com/evechat/evechat/internal/integrations/telegram"
	"github.com/evechat/evechat/internal/logx"
	"github.com/evechat/evechat/internal/metrics"
	"github.com/evechat/evechat/internal/middleware"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
	"github.com/evechat/evechat/internal/router"
	"github.com/evechat/evechat/internal/runtimecfg"
	"github.com/evechat/evechat/internal/security"
	"github.com/evechat/evechat/internal/services"
	"github.com/evechat/evechat/internal/setup"

	_ "time/tzdata" // business-hours timezones must work in distroless images
)

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	cfgPath := "./evechat.yaml"
	registerCommonFlags(fs, &cfgPath)
	seed := fs.Bool("seed", false, "ساخت پروژه و مدیر نمونه")
	migrateOnly := fs.Bool("migrate-only", false, "فقط اجرای مهاجرت‌ها")
	showVersion := fs.Bool("version", false, "نمایش نسخه")
	healthcheck := fs.Bool("healthcheck", false, "بررسی سلامت و خروج")
	_ = fs.Parse(args)

	setup.LoadDotEnv(filepath.Join(filepath.Dir(cfgPath), ".env"))
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fatalf("%v", err)
	}
	applyFirebaseEnv(cfg)
	if *showVersion {
		fmt.Println(versionString())
		return
	}
	if err := logx.InitFile(cfg.Log.Level, cfg.Log.Pretty, cfg.Log.File, cfg.Log.MaxSizeMB, cfg.Log.MaxBackups); err != nil {
		// Logging to a file is best-effort; stderr always works.
		logx.Init(cfg.Log.Level, cfg.Log.Pretty)
		logx.L().Warn().Err(err).Msg("file logging disabled")
	}
	log := logx.L()

	if *healthcheck {
		os.Exit(probeHealth(cfg))
	}
	if err := serve(cfg, cfgPath, *migrateOnly, *seed, log); err != nil {
		log.Fatal().Err(err).Msg("startup failed")
	}
}

func probeHealth(cfg *config.Config) int {
	client := &http.Client{Timeout: 5 * time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%d/api/v1/health", cfg.Server.Port)
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck failed:", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck status:", resp.StatusCode)
		return 1
	}
	return 0
}

func serve(cfg *config.Config, cfgPath string, migrateOnly, seed bool, log *zerolog.Logger) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d, err := db.Open(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()

	if err := db.Migrate(ctx, d, *log); err != nil {
		return err
	}
	if migrateOnly {
		log.Info().Str("db", cfg.Database.Path).Msg("migrations applied, exiting")
		return nil
	}

	reg := metrics.New(metrics.BuildInfo{
		Version: version, Commit: commit, BuildDate: buildDate,
	})
	metrics.SetDefault(reg)

	keys, err := cfg.EncryptionKeys()
	if err != nil {
		return err
	}
	enc, err := crypto.New(keys, cfg.Encryption.CurrentVersion)
	if err != nil {
		return err
	}

	r := repo.New(d)
	state, err := runtimecfg.New(ctx, r, enc, cfg.BaseURL(), *log)
	if err != nil {
		return fmt.Errorf("loading runtime settings: %w", err)
	}
	overlayRuntimeConfig(ctx, cfg, state)

	// Outbound HTTP (webhooks and integration probes) goes through this policy
	// for the rest of the process lifetime.
	security.Outbound = security.NewOutboundPolicy(cfg.Security.OutboundAllowHosts, cfg.Security.AllowPrivateOutbound)

	// Backwards compatibility: installations created before the first-run
	// wizard existed already have a superadmin; treat them as configured
	// instead of locking the operator out behind /setup.
	if !state.SetupCompleted() {
		if n, cerr := r.Agents.CountSuperadmins(ctx); cerr == nil && n > 0 {
			if err := state.CompleteSetup(ctx); err == nil {
				log.Info().Msg("legacy installation detected; setup marked complete")
			}
		}
	}

	h := hub.New(hub.HubConfig{
		PingInterval:    time.Duration(cfg.Hub.PingIntervalSeconds) * time.Second,
		PongWait:        time.Duration(cfg.Hub.PongWaitSeconds) * time.Second,
		SendBufferSize:  cfg.Hub.SendBufferSize,
		RingBufferSize:  cfg.Hub.RingBufferSize,
		MaxMessageBytes: int64(cfg.Hub.MaxMessageBytes),
	}, *log)
	go h.Run(ctx)

	notif := services.NewNotificationService(*log)
	visitors := services.NewVisitorService(r, 30*24*time.Hour, *log)
	msgs := services.NewMessageService(r, h, enc, notif, *log)
	routing := services.NewRoutingService(r, *log)
	csat := services.NewCSATScheduler(r, h, *log)
	csat.Start(ctx)
	convs := services.NewConversationService(r, h, routing, msgs, notif, csat, *log)
	realtime := services.NewRealtime(r, h, convs, msgs, *log)
	realtime.SetThrottles(
		security.NewThrottle(time.Hour), security.NewThrottle(time.Minute),
		cfg.Security.ConversationsPerHour, cfg.Security.MessagesPerConversation,
	)
	search := services.NewSearchService(r, msgs, *log)
	tickets := services.NewTicketService(r, notif, *log)
	audit := services.NewAuditService(r, *log)

	webhooks := services.NewWebhookService(r, *log)
	webhooks.Start(ctx)
	notif.Register(services.NewWebhookChannel(webhooks))

	// SMTP / email channel (config or web-wizard stored settings).
	var emailChannel *services.EmailChannel
	if cfg.SMTP.Enabled && cfg.SMTP.Host != "" {
		smtpClient := smtp.New(smtp.Config{
			Host: cfg.SMTP.Host, Port: cfg.SMTP.Port, Username: cfg.SMTP.Username,
			Password: cfg.SMTP.Password, FromName: cfg.SMTP.FromName, FromEmail: cfg.SMTP.FromEmail,
			TLSMode: cfg.SMTP.TLSMode, Timeout: cfg.SMTPTimeout(),
		})
		emailChannel = services.NewEmailChannel(smtpClient, r, cfg.BaseURL(), *log)
		notif.Register(emailChannel)
	}

	eveClient := eve.New(cfg.Integrations.EVE.BaseURL, cfg.Integrations.EVE.APIKey,
		cfg.Integrations.EVE.HMACSecret, cfg.Integrations.EVE.RequireSignature)
	gmwebClient := gmweb.New(cfg.Integrations.GMweb.BaseURL, cfg.Integrations.GMweb.APIKey)
	smsChannel := services.NewSMSChannel(gmwebClient, r, msgs, *log)
	notif.Register(smsChannel)

	// FCM push for the Android agent app. Missing/invalid credentials disable
	// delivery only; device registration stays available so the app can retry
	// after the operator fixes the service account.
	if cfg.Firebase.Enabled {
		sender, ferr := fcm.New(fcm.Config{
			ProjectID:          cfg.Firebase.ProjectID,
			ServiceAccountJSON: cfg.Firebase.ServiceAccountJSON,
			ServiceAccountFile: cfg.Firebase.ServiceAccountFile,
			BaseURL:            cfg.Firebase.BaseURL,
			TokenURL:           cfg.Firebase.TokenURL,
		})
		if ferr != nil {
			log.Warn().Err(ferr).Msg("FCM push disabled")
		} else {
			notif.Register(services.NewPushChannel(sender, r, *log))
			log.Info().Str("project_id", sender.ProjectID()).Msg("FCM push enabled")
		}
	}

	var tgBot *telegram.Bot
	if cfg.Telegram.Enabled && cfg.Telegram.BotToken != "" {
		bot, err := telegram.New(telegram.Config{
			Token:        cfg.Telegram.BotToken,
			WebhookURL:   cfg.Telegram.WebhookURL,
			Secret:       cfg.Telegram.Secret,
			DashboardURL: cfg.BaseURL(),
		}, *log)
		if err != nil {
			log.Warn().Err(err).Msg("telegram disabled")
		} else {
			tgBot = bot
			tgChannel := services.NewTelegramChannel(bot, r, msgs, convs, h, cfg.BaseURL(), *log)
			bot.SetHandlers(tgChannel.HandleInbound, tgChannel.HandleResolve, tgChannel.HandleEscalate)
			notif.Register(tgChannel)
			go func() {
				if cfg.Telegram.WebhookURL != "" {
					if werr := bot.SetWebhook(ctx); werr != nil {
						log.Warn().Err(werr).Msg("telegram setWebhook failed")
					}
				}
			}()
		}
	}

	var aiSvc *services.AIService
	if cfg.AI.Enabled && cfg.AI.ClaudeAPIKey != "" {
		aiSvc = services.NewAIService(r, msgs, h, convs, services.AIConfig{
			Enabled:      cfg.AI.Enabled,
			APIKey:       cfg.AI.ClaudeAPIKey,
			Model:        cfg.AI.Model,
			MaxTokens:    cfg.AI.MaxTokens,
			HandoffAfter: cfg.AI.HandoffAfter,
		}, *log)
		realtime.SetAI(aiSvc)
	}

	notif.Start(ctx)
	h.SetHandler(realtime)

	tokens := middleware.NewTokenManager(cfg.Auth.JWTSecret, cfg.AccessExpiry(), cfg.RefreshExpiry())
	allowHosts := []string{}
	if u := cfg.BaseURL(); u != "" {
		allowHosts = append(allowHosts, hostOf(u))
	}
	apiKeyMW := middleware.NewAPIKey(r, *log, allowHosts...)

	resetSvc := services.NewPasswordResetService(r, emailChannel, cfg.BaseURL(), cfg.Auth.BcryptCost, *log)

	// Two-way channels: inbound webhooks with a per-channel/IP limit and a 48h
	// inspector retention window.
	channels := services.NewChannelService(r, convs, msgs, *log)
	channels.SetThrottle(security.NewThrottle(time.Minute), 120)
	channels.StartRetention(ctx)

	H := &handlers.Handlers{
		Cfg:        cfg,
		Log:        *log,
		Repo:       r,
		Hub:        h,
		Tokens:     tokens,
		Enc:        enc,
		Visitors:   visitors,
		Convs:      convs,
		Msgs:       msgs,
		Routing:    routing,
		Search:     search,
		Tickets:    tickets,
		Webhooks:   webhooks,
		Channels:   channels,
		Realtime:   realtime,
		Notif:      notif,
		AI:         aiSvc,
		SMS:        smsChannel,
		State:      state,
		MetricsReg: reg,
		Audit:      audit,
		Reset:      resetSvc,
		Email:      emailChannel,
		EVE:        eveClient,
		GMweb:      gmwebClient,
		Telegram:   tgBot,
		StartedAt:  time.Now(),
		Version:    version,
		Commit:     commit,
		BuildDate:  buildDate,
		ConfigPath: cfgPath,
	}

	if seed {
		if err := seedDemo(ctx, r, cfg, log); err != nil {
			log.Error().Err(err).Msg("seed failed")
		}
	}

	if err := os.MkdirAll(cfg.Storage.UploadsDir, 0o750); err != nil {
		log.Warn().Err(err).Str("dir", cfg.Storage.UploadsDir).Msg("uploads dir not writable")
	}

	if !state.SetupCompleted() {
		token, terr := state.SetupToken(ctx, cfg.DataDir())
		if terr != nil {
			log.Warn().Err(terr).Msg("could not persist setup token")
		}
		log.Warn().Str("setup_url", cfg.BaseURL()+"/setup").Msg("first-run setup required")
		fmt.Printf("\n  ╭─ نصب اولیه لازم است ────────────────────────────────╮\n")
		fmt.Printf("  │ آدرس ویزارد : %-36s │\n", cfg.BaseURL()+"/setup")
		fmt.Printf("  │ توکن نصب    : %-36s │\n", token)
		fmt.Printf("  ╰──────────────────────────────────────────────────────╯\n\n")
	}

	handler := router.New(router.Deps{
		Cfg:            cfg,
		Log:            *log,
		H:              H,
		Tokens:         tokens,
		APIKey:         apiKeyMW,
		WidgetLimit:    middleware.NewRateLimiter(cfg.RateLimit.WidgetSessionPerMin, cfg.RateLimit.WidgetSessionPerMin),
		MessageLimit:   middleware.NewRateLimiter(cfg.RateLimit.MessagePerMin, cfg.RateLimit.MessagePerMin),
		LoginLimit:     middleware.NewRateLimiter(cfg.RateLimit.LoginPerMin, cfg.RateLimit.LoginPerMin),
		DashboardLimit: middleware.NewRateLimiter(600, 200),
	})

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadTimeout(),
		ReadTimeout:       cfg.ReadTimeout(),
		// WriteTimeout is intentionally zero: SSE and long-poll endpoints
		// legitimately keep a response open for 25s or longer.
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	lockPath := filepath.Join(cfg.DataDir(), "evechat.lock")
	_ = os.WriteFile(lockPath, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600)
	defer func() { _ = os.Remove(lockPath) }()

	go janitor(ctx, r, cfg, *log)

	log.Info().
		Str("version", version).
		Str("commit", commit).
		Str("addr", cfg.Addr()).
		Str("db", cfg.Database.Path).
		Str("public_url", cfg.BaseURL()).
		Bool("setup_complete", state.SetupCompleted()).
		Msg("⚡ EveChat starting")

	errCh := make(chan error, 1)
	go func() {
		if cfg.Server.TLS.Enabled && cfg.Server.TLS.CertFile != "" {
			errCh <- srv.ListenAndServeTLS(cfg.Server.TLS.CertFile, cfg.Server.TLS.KeyFile)
			return
		}
		errCh <- srv.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case sig := <-stop:
		log.Info().Str("signal", sig.String()).Msg("shutdown requested")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout())
	defer shutdownCancel()

	h.Shutdown(shutdownCtx)
	cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Warn().Err(err).Msg("http shutdown incomplete")
	}
	csat.Stop()
	notif.Stop()
	log.Info().Msg("EveChat stopped")
	return nil
}

// overlayRuntimeConfig applies secrets and endpoints stored by the web wizard
// on top of the YAML/env configuration. DB values win because they were set
// explicitly during setup.
func overlayRuntimeConfig(ctx context.Context, cfg *config.Config, state *runtimecfg.State) {
	if v, ok := state.Secret("integration.telegram.token"); ok && v != "" {
		cfg.Telegram.BotToken = v
		cfg.Telegram.Enabled = true
		if cfg.Telegram.Secret == "" {
			if secret, ok := state.Secret("integration.telegram.secret"); ok {
				cfg.Telegram.Secret = secret
			}
		}
		if cfg.Telegram.WebhookURL == "" {
			cfg.Telegram.WebhookURL = cfg.BaseURL() + "/api/v1/integrations/telegram/webhook"
		}
	}
	if v := state.Value("integration.eve.base_url"); v != "" {
		cfg.Integrations.EVE.BaseURL = v
	}
	if v, ok := state.Secret("integration.eve.api_key"); ok && v != "" {
		cfg.Integrations.EVE.APIKey = v
	}
	if v, ok := state.Secret("integration.eve.hmac_secret"); ok && v != "" {
		cfg.Integrations.EVE.HMACSecret = v
	}
	if v := state.Value("integration.gmweb.base_url"); v != "" {
		cfg.Integrations.GMweb.BaseURL = v
	}
	if v, ok := state.Secret("integration.gmweb.api_key"); ok && v != "" {
		cfg.Integrations.GMweb.APIKey = v
	}
	if v, ok := state.Secret("integration.ai.api_key"); ok && v != "" {
		cfg.AI.ClaudeAPIKey = v
		cfg.AI.Enabled = true
	}
	if v := state.Value("integration.ai.model"); v != "" {
		cfg.AI.Model = v
	}
	if v := state.Value("integration.smtp.host"); v != "" {
		cfg.SMTP.Enabled = true
		cfg.SMTP.Host = v
	}
	if v := state.Value("integration.smtp.port"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.SMTP.Port = n
		}
	}
	if v := state.Value("integration.smtp.username"); v != "" {
		cfg.SMTP.Username = v
	}
	if v, ok := state.Secret("integration.smtp.password"); ok {
		cfg.SMTP.Password = v
	}
	if v := state.Value("integration.smtp.from_email"); v != "" {
		cfg.SMTP.FromEmail = v
	}
	if v := state.Value("integration.smtp.from_name"); v != "" {
		cfg.SMTP.FromName = v
	}
	if v := state.Value("integration.smtp.tls_mode"); v != "" {
		cfg.SMTP.TLSMode = v
	}
	_ = ctx
}

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func janitor(ctx context.Context, r *repo.Repo, cfg *config.Config, log zerolog.Logger) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	retention := cfg.Audit.RetentionDays
	if retention <= 0 {
		retention = 180
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := r.Sessions.DeleteExpired(ctx)
			if err != nil {
				log.Warn().Err(err).Msg("session janitor failed")
			} else if n > 0 {
				log.Debug().Int64("deleted", n).Msg("expired sessions removed")
			}
			if _, err := r.Agents.PurgeResetTokens(ctx); err != nil {
				log.Warn().Err(err).Msg("reset token purge failed")
			}
			cutoffPush := time.Now().UTC().AddDate(0, 0, -60)
			if pruned, err := r.Push.PruneStale(ctx, cutoffPush); err != nil {
				log.Warn().Err(err).Msg("push device prune failed")
			} else if pruned > 0 {
				log.Debug().Int64("pruned", pruned).Msg("stale push devices pruned")
			}
			cutoff := time.Now().UTC().AddDate(0, 0, -retention).Format("2006-01-02 15:04:05")
			if pruned, err := r.Audit.Prune(ctx, cutoff); err != nil {
				log.Warn().Err(err).Msg("audit prune failed")
			} else if pruned > 0 {
				log.Debug().Int64("pruned", pruned).Msg("audit entries pruned")
			}
		}
	}
}

// seedDemo creates a demo project and superadmin on first boot.
func seedDemo(ctx context.Context, r *repo.Repo, cfg *config.Config, log *zerolog.Logger) error {
	projects, err := r.Projects.List(ctx)
	if err != nil {
		return err
	}
	var project *models.Project
	if len(projects) == 0 {
		project = &models.Project{
			ID:        id.NewPrefixed(id.PrefixProject),
			Name:      "EveChat Demo",
			APIKey:    id.NewPrefixed("pk"),
			SecretKey: id.Token(32),
			Domains:   models.StringList{},
			Settings:  models.JSONMap{},
			IsActive:  true,
		}
		if err := r.Projects.Create(ctx, project); err != nil {
			return err
		}
		log.Info().Str("project_id", project.ID).Msg("demo project created")
	} else {
		project = projects[0]
	}

	const email = "admin@evechat.local"
	if _, err := r.Agents.GetByEmail(ctx, project.ID, email); err == nil {
		log.Info().Str("email", email).Msg("superadmin already exists")
		fmt.Printf("\n────────────────────────────────────────\nExisting project api_key: %s\n────────────────────────────────────────\n\n", project.APIKey)
		return nil
	}

	password := id.Token(9)
	hash, err := handlers.HashPassword(password, cfg.Auth.BcryptCost)
	if err != nil {
		return err
	}
	admin := &models.Agent{
		ID:               id.NewPrefixed(id.PrefixAgent),
		ProjectID:        project.ID,
		Email:            email,
		PasswordHash:     hash,
		Name:             "مدیر سیستم",
		Role:             models.RoleSuperAdmin,
		MaxConversations: 50,
		IsActive:         true,
	}
	if err := r.Agents.Create(ctx, admin); err != nil {
		return err
	}
	_ = r.SaaS.SetTenantPlan(ctx, project.ID, "free")
	_ = r.Settings.Set(ctx, "setup_completed", "1")
	_ = r.Settings.Set(ctx, "setup_completed_at", time.Now().UTC().Format(time.RFC3339))

	fmt.Printf(`
──────────────────────────────────────────────────────────
  EveChat seed completed
──────────────────────────────────────────────────────────
  Dashboard : %s
  Email     : %s
  Password  : %s
  Project   : %s
  API key   : %s
──────────────────────────────────────────────────────────

`, cfg.BaseURL(), email, password, project.ID, project.APIKey)
	return nil
}

var _ = flag.NewFlagSet
