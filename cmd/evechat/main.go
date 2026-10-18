// Command evechat is the single-binary EveChat server.
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
	"github.com/evechat/evechat/internal/integrations/gmweb"
	"github.com/evechat/evechat/internal/integrations/telegram"
	"github.com/evechat/evechat/internal/logx"
	"github.com/evechat/evechat/internal/middleware"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
	"github.com/evechat/evechat/internal/router"
	"github.com/evechat/evechat/internal/services"

	_ "time/tzdata" // business-hours timezones must work in distroless images
)

var version = "dev"

func main() {
	var (
		configPath  = flag.String("config", "./evechat.yaml", "path to the YAML configuration file")
		migrateOnly = flag.Bool("migrate-only", false, "apply database migrations and exit")
		seed        = flag.Bool("seed", false, "create a demo project and a superadmin, then keep running")
		showVersion = flag.Bool("version", false, "print the version and exit")
		healthcheck = flag.Bool("healthcheck", false, "probe the local health endpoint and exit 0/1")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	logx.Init(cfg.Log.Level, cfg.Log.Pretty)
	log := logx.L()

	if *healthcheck {
		os.Exit(probeHealth(cfg))
	}

	if err := run(cfg, *migrateOnly, *seed, log); err != nil {
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

func run(cfg *config.Config, migrateOnly, seed bool, log *zerolog.Logger) error {
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

	keys, err := cfg.EncryptionKeys()
	if err != nil {
		return err
	}
	enc, err := crypto.New(keys, cfg.Encryption.CurrentVersion)
	if err != nil {
		return err
	}

	r := repo.New(d)

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
	search := services.NewSearchService(r, msgs, *log)
	tickets := services.NewTicketService(r, notif, *log)

	webhooks := services.NewWebhookService(r, *log)
	webhooks.Start(ctx)
	notif.Register(services.NewWebhookChannel(webhooks))

	eveClient := eve.New(cfg.Integrations.EVE.BaseURL, cfg.Integrations.EVE.APIKey,
		cfg.Integrations.EVE.HMACSecret, cfg.Integrations.EVE.RequireSignature)
	gmwebClient := gmweb.New(cfg.Integrations.GMweb.BaseURL, cfg.Integrations.GMweb.APIKey)
	smsChannel := services.NewSMSChannel(gmwebClient, r, msgs, *log)
	notif.Register(smsChannel)

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

	H := &handlers.Handlers{
		Cfg:       cfg,
		Log:       *log,
		Repo:      r,
		Hub:       h,
		Tokens:    tokens,
		Enc:       enc,
		Visitors:  visitors,
		Convs:     convs,
		Msgs:      msgs,
		Routing:   routing,
		Search:    search,
		Tickets:   tickets,
		Webhooks:  webhooks,
		Realtime:  realtime,
		Notif:     notif,
		AI:        aiSvc,
		SMS:       smsChannel,
		EVE:       eveClient,
		GMweb:     gmwebClient,
		Telegram:  tgBot,
		StartedAt: time.Now(),
		Version:   version,
	}

	if seed {
		if err := seedDemo(ctx, r, cfg, log); err != nil {
			log.Error().Err(err).Msg("seed failed")
		}
	}

	if err := os.MkdirAll(cfg.Storage.UploadsDir, 0o750); err != nil {
		log.Warn().Err(err).Str("dir", cfg.Storage.UploadsDir).Msg("uploads dir not writable")
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
		// legitimately keep a response open for 25s-forever.
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	go janitor(ctx, r, *log)

	log.Info().
		Str("version", version).
		Str("addr", cfg.Addr()).
		Str("db", cfg.Database.Path).
		Str("public_url", cfg.BaseURL()).
		Int("ping_interval_s", cfg.Hub.PingIntervalSeconds).
		Int("ring_buffer", cfg.Hub.RingBufferSize).
		Int("send_buffer", cfg.Hub.SendBufferSize).
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

func hostOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func janitor(ctx context.Context, r *repo.Repo, log zerolog.Logger) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := r.Sessions.DeleteExpired(ctx)
			if err != nil {
				log.Warn().Err(err).Msg("session janitor failed")
				continue
			}
			if n > 0 {
				log.Debug().Int64("deleted", n).Msg("expired sessions removed")
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
  Embed snippet:
  <script>
    window.EveChat = { projectKey: "%s", apiUrl: "%s" };
  </script>
  <script async src="%s/widget.js"></script>
──────────────────────────────────────────────────────────

`, cfg.BaseURL(), email, password, project.ID, project.APIKey, project.APIKey, cfg.BaseURL(), cfg.BaseURL())
	return nil
}
