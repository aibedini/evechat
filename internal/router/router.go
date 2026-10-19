package router

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"

	"github.com/evechat/evechat/internal/apierr"
	"github.com/evechat/evechat/internal/config"
	"github.com/evechat/evechat/internal/embedfs"
	"github.com/evechat/evechat/internal/handlers"
	"github.com/evechat/evechat/internal/logx"
	"github.com/evechat/evechat/internal/middleware"
)

// Deps is everything the router needs.
type Deps struct {
	Cfg            *config.Config
	Log            zerolog.Logger
	H              *handlers.Handlers
	Tokens         *middleware.TokenManager
	APIKey         *middleware.APIKey
	WidgetLimit    *middleware.RateLimiter
	MessageLimit   *middleware.RateLimiter
	LoginLimit     *middleware.RateLimiter
	DashboardLimit *middleware.RateLimiter
}

// New builds the HTTP router.
func New(d Deps) http.Handler {
	r := chi.NewRouter()

	// Deliberately no chimw.RealIP: it rewrites r.RemoteAddr from
	// X-Forwarded-For on every request, so an unauthenticated caller could pick
	// its own IP and bypass the rate limiters, the IP blocklist, the setup
	// wizard's loopback check and the /demo/test gate. middleware.ClientIP
	// honours the proxy headers itself, and only when server.trust_proxy is on.
	r.Use(middleware.Recover)
	r.Use(middleware.RequestID)
	r.Use(requestLogger(d.Log))
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.Blocklist(d.H.State.Blocklist(), d.Cfg.Server.TrustProxy))
	r.Use(middleware.Instrument(d.H.MetricsReg))
	// Audit every authenticated mutation; handlers may add richer entries.
	middleware.MutationHook = d.H.AuditMutation

	r.Use(chimw.Compress(5))
	r.Use(middleware.SetupGate(d.H.State))

	wrap := d.H.H
	openCORS := middleware.OpenCORS()
	strictCORS := middleware.StrictCORS(d.Cfg.Server.PublicURL)

	ipKey := func(r *http.Request) string {
		return middleware.ClientIP(r, d.Cfg.Server.TrustProxy)
	}
	widgetKey := func(r *http.Request) string {
		p, _ := middleware.ProjectFrom(r.Context())
		pid := ""
		if p != nil {
			pid = p.ID
		}
		return pid + "|" + middleware.ClientIP(r, d.Cfg.Server.TrustProxy)
	}

	// ---- static assets (fonts + docs), always public ----
	r.Get("/assets/fonts/*", fontHandler)
	r.Get("/assets/openapi.yaml", openAPIHandler)
	r.Get("/api/openapi.yaml", openAPIHandler)
	r.Get("/api/docs", docsHandler)

	// Demonstration pages (local only). Every handler rejects a non-loopback
	// caller with a 404, so they never expose a project key on a public host.
	r.Get("/demo", wrap(d.H.DemoPage))
	r.Get("/demo/test", wrap(d.H.DemoTest))
	r.Get("/demo/split", wrap(d.H.DemoPage))
	r.Get("/demo/devices", wrap(d.H.DemoPage))
	r.Get("/demo/events", wrap(d.H.DemoPage))
	r.Get("/demo/demo.css", wrap(d.H.DemoAsset))
	r.Get("/demo/demo.js", wrap(d.H.DemoAsset))

	// ---- widget (open CORS, project-key auth) ----
	r.Group(func(r chi.Router) {
		r.Use(openCORS)
		r.Get("/widget.js", wrap(d.H.WidgetJS))
		r.Route("/api/widget/v1", func(r chi.Router) {
			// Bootstrap endpoints need the project key; everything else is
			// authenticated by the visitor session token, which is already
			// scoped to one project and one visitor.
			r.With(d.APIKey.Handler).Get("/config", wrap(d.H.WidgetConfig))
			r.With(d.APIKey.Handler).Get("/challenge", wrap(d.H.WidgetChallenge))
			r.With(d.APIKey.Handler, d.WidgetLimit.Middleware(widgetKey)).Post("/session", wrap(d.H.WidgetSession))
			r.Get("/events", wrap(d.H.WidgetEvents))
			r.Get("/poll", wrap(d.H.WidgetPoll))
			r.Post("/csat", wrap(d.H.WidgetCSAT))
			r.With(d.MessageLimit.Middleware(widgetKey)).Post("/conversations/{id}/messages", wrap(d.H.WidgetPostMessage))
			r.Get("/conversations/{id}/sync", wrap(d.H.WidgetSync))
		})
	})

	// ---- public channel ingest (secret auth inside the handler) ----
	r.Post("/api/channels/{id}/inbound", wrap(d.H.ChannelInbound))

	// ---- visitor websocket (token auth inside the handler) ----
	r.With(openCORS).Get("/ws/visitor", wrap(d.H.WSVisitor))

	// ---- first-run setup wizard ----
	r.Handle("/setup", setupSPAHandler())
	r.Handle("/setup/*", setupSPAHandler())
	r.Route("/api/setup", func(r chi.Router) {
		r.Get("/status", wrap(d.H.SetupStatus))
		r.Post("/check", wrap(d.H.SetupCheck))
		r.Post("/admin", wrap(d.H.SetupAdmin))
		r.Post("/project", wrap(d.H.SetupProject))
		r.Post("/integrations", wrap(d.H.SetupIntegrations))
		r.Post("/test/{kind}", wrap(d.H.SetupTest))
		r.Post("/complete", wrap(d.H.SetupComplete))
	})

	// ---- auth ----
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Use(strictCORS)
		loginKey := func(req *http.Request) string { return "login|" + ipKey(req) }
		r.With(d.LoginLimit.Middleware(loginKey)).Post("/login", wrap(d.H.Login))
		r.With(d.LoginLimit.Middleware(loginKey)).Post("/refresh", wrap(d.H.Refresh))
		r.With(d.LoginLimit.Middleware(loginKey)).Post("/forgot", wrap(d.H.ForgotPassword))
		r.With(d.LoginLimit.Middleware(loginKey)).Post("/reset", wrap(d.H.ResetPassword))
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(d.Tokens))
			r.Post("/logout", wrap(d.H.Logout))
			r.Get("/me", wrap(d.H.Me))
			r.Put("/password", wrap(d.H.ChangePassword))
		})
	})

	// ---- public health + metrics ----
	r.With(strictCORS).Get("/api/v1/health", wrap(d.H.Health))
	r.Get("/metrics", wrap(d.H.Metrics))

	// Public integration callbacks authenticate themselves with the shared
	// secret from Telegram / GMweb, so the JWT middleware is bypassed for
	// exactly these two paths.
	publicIntegrationPaths := map[string]bool{
		"/api/v1/integrations/telegram/webhook": true,
		"/api/v1/integrations/gmweb/inbound":    true,
		"/api/v1/wallboard":                     true,
	}
	authMiddleware := middleware.Auth(d.Tokens)

	// ---- dashboard API ----
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(strictCORS)
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if publicIntegrationPaths[req.URL.Path] {
					next.ServeHTTP(w, req)
					return
				}
				authMiddleware(next).ServeHTTP(w, req)
			})
		})
		r.Use(d.DashboardLimit.Middleware(func(r *http.Request) string {
			c, ok := middleware.CurrentAgent(r.Context())
			if !ok {
				return ""
			}
			return "agent|" + c.Subject
		}))

		r.Get("/config", wrap(d.H.PublicConfig))
		// Read-only wallboard feed; the token in the query is the credential.
		r.Get("/wallboard", wrap(d.H.Wallboard))

		r.Get("/settings/project", wrap(d.H.GetProjectSettings))
		r.Put("/settings/project", wrap(d.H.UpdateProjectSettings))
		r.Post("/settings/project/rotate-key", wrap(d.H.RotateAPIKey))
		r.Post("/settings/project/wallboard-token", wrap(d.H.WallboardToken))
		r.Delete("/settings/project/wallboard-token", wrap(d.H.RevokeWallboardToken))

		r.Get("/conversations", wrap(d.H.ListConversations))
		r.Get("/conversations/export", wrap(d.H.ExportConversations))
		r.Get("/conversations/{id}", wrap(d.H.GetConversation))
		r.Post("/conversations/{id}/assign", wrap(d.H.AssignConversation))
		r.Post("/conversations/{id}/claim", wrap(d.H.ClaimConversation))
		r.Post("/conversations/{id}/transfer", wrap(d.H.TransferConversation))
		r.Post("/conversations/{id}/resolve", wrap(d.H.ResolveConversation))
		r.Post("/conversations/{id}/reopen", wrap(d.H.ReopenConversation))
		r.Post("/conversations/{id}/convert-ticket", wrap(d.H.ConvertToTicket))
		r.Get("/conversations/{id}/messages", wrap(d.H.ListMessages))
		r.Post("/conversations/{id}/messages", wrap(d.H.PostMessage))
		r.Post("/conversations/{id}/note", wrap(d.H.AddNote))
		r.Post("/conversations/{id}/read", wrap(d.H.MarkConversationRead))

		r.Get("/search/messages", wrap(d.H.SearchMessages))

		r.Get("/visitors", wrap(d.H.ListVisitors))
		r.Get("/visitors/{id}", wrap(d.H.GetVisitor))
		r.Put("/visitors/{id}", wrap(d.H.UpdateVisitor))
		r.Get("/visitors/{id}/conversations", wrap(d.H.VisitorConversations))
		r.Post("/visitors/{id}/ban", wrap(d.H.BanVisitor))
		r.Delete("/visitors/{id}/ban", wrap(d.H.UnbanVisitor))
		r.Get("/visitors/{id}/export", wrap(d.H.ExportVisitor))
		r.Post("/visitors/{id}/anonymize", wrap(d.H.AnonymizeVisitor))

		r.Get("/agents", wrap(d.H.ListAgents))
		r.Post("/agents", wrap(d.H.CreateAgent))
		r.Get("/agents/{id}", wrap(d.H.GetAgent))
		r.Put("/agents/{id}", wrap(d.H.UpdateAgent))
		r.Delete("/agents/{id}", wrap(d.H.DeleteAgent))
		r.Put("/agents/{id}/password", wrap(d.H.UpdateAgentPassword))

		r.Get("/routing-rules", wrap(d.H.ListRoutingRules))
		r.Post("/routing-rules", wrap(d.H.CreateRoutingRule))
		r.Post("/routing-rules/reorder", wrap(d.H.ReorderRoutingRules))
		r.Post("/routing-rules/{id}/test", wrap(d.H.TestRoutingRule))
		r.Get("/routing-rules/{id}", wrap(d.H.GetRoutingRule))
		r.Put("/routing-rules/{id}", wrap(d.H.UpdateRoutingRule))
		r.Delete("/routing-rules/{id}", wrap(d.H.DeleteRoutingRule))

		r.Get("/canned-responses", wrap(d.H.ListCanned))
		r.Post("/canned-responses", wrap(d.H.CreateCanned))
		r.Put("/canned-responses/{id}", wrap(d.H.UpdateCanned))
		r.Delete("/canned-responses/{id}", wrap(d.H.DeleteCanned))

		r.Get("/tickets", wrap(d.H.ListTickets))
		r.Post("/tickets", wrap(d.H.CreateTicket))
		r.Get("/tickets/{id}", wrap(d.H.GetTicket))
		r.Put("/tickets/{id}", wrap(d.H.UpdateTicket))
		r.Delete("/tickets/{id}", wrap(d.H.DeleteTicket))
		r.Post("/tickets/{id}/status", wrap(d.H.SetTicketStatus))
		r.Post("/tickets/{id}/notes", wrap(d.H.AddTicketNote))

		r.Get("/knowledge-base", wrap(d.H.ListKB))
		r.Post("/knowledge-base", wrap(d.H.CreateKB))
		r.Get("/knowledge-base/search", wrap(d.H.SearchKB))
		r.Get("/knowledge-base/{id}", wrap(d.H.GetKB))
		r.Put("/knowledge-base/{id}", wrap(d.H.UpdateKB))
		r.Delete("/knowledge-base/{id}", wrap(d.H.DeleteKB))

		r.Get("/webhooks", wrap(d.H.ListWebhooks))
		r.Get("/webhooks/events", wrap(d.H.ListWebhookEvents))
		r.Post("/webhooks", wrap(d.H.CreateWebhook))
		r.Get("/webhooks/{id}", wrap(d.H.GetWebhook))
		r.Put("/webhooks/{id}", wrap(d.H.UpdateWebhook))
		r.Delete("/webhooks/{id}", wrap(d.H.DeleteWebhook))
		r.Get("/webhooks/{id}/deliveries", wrap(d.H.ListWebhookDeliveries))
		r.Post("/webhooks/{id}/test", wrap(d.H.TestWebhook))

		// Two-way channels: inbound webhooks with an outbound ack ladder.
		r.Get("/channels", wrap(d.H.ListChannels))
		r.Post("/channels", wrap(d.H.CreateChannel))
		r.Get("/channels/{id}", wrap(d.H.GetChannel))
		r.Put("/channels/{id}", wrap(d.H.UpdateChannel))
		r.Delete("/channels/{id}", wrap(d.H.DeleteChannel))
		r.Post("/channels/{id}/rotate-secret", wrap(d.H.RotateChannelSecret))
		r.Post("/channels/{id}/test", wrap(d.H.TestChannel))
		r.Post("/channels/{id}/preview", wrap(d.H.PreviewChannelMapping))
		r.Get("/channels/{id}/inbox", wrap(d.H.ListChannelInbox))
		r.Get("/channels/{id}/curl", wrap(d.H.ChannelCurl))

		r.Post("/upload", wrap(d.H.Upload))

		r.Get("/stats/overview", wrap(d.H.StatsOverview))
		r.Get("/stats/series", wrap(d.H.StatsSeries))
		r.Get("/stats/agents", wrap(d.H.StatsAgents))

		r.Get("/integrations/status", wrap(d.H.IntegrationStatus))
		r.Post("/integrations/telegram/webhook", wrap(d.H.TelegramWebhook))
		r.Post("/integrations/gmweb/inbound", wrap(d.H.GMwebInbound))
		r.Post("/integrations/telegram/set-webhook", wrap(d.H.SetTelegramWebhook))
		r.Post("/integrations/telegram/test", wrap(d.H.TestTelegram))
		r.Get("/integrations/eve/user", wrap(d.H.EVEUser))
		r.Post("/integrations/gmweb/test", wrap(d.H.TestSMS))

		r.Get("/security/blocklist", wrap(d.H.GetBlocklist))
		r.Put("/security/blocklist", wrap(d.H.UpdateBlocklist))

		r.Get("/audit", wrap(d.H.ListAudit))

		r.Get("/domains", wrap(d.H.ListDomains))
		r.Post("/domains", wrap(d.H.AddDomain))
		r.Post("/domains/{id}/verify", wrap(d.H.VerifyDomain))
		r.Delete("/domains/{id}", wrap(d.H.DeleteDomain))

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("admin", "superadmin"))
			r.Get("/diagnostics", wrap(d.H.Diagnostics))
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("superadmin"))
			r.Get("/admin/projects", wrap(d.H.ListProjects))
			r.Post("/admin/projects", wrap(d.H.CreateProject))
			r.Get("/admin/plans", wrap(d.H.ListPlans))
			r.Post("/admin/plans", wrap(d.H.UpsertPlan))
			r.Get("/admin/usage", wrap(d.H.GetUsage))
			r.Post("/admin/tenant-plan", wrap(d.H.SetTenantPlan))
		})
	})

	// ---- agent websocket ----
	r.With(strictCORS).Get("/ws/agent", wrap(d.H.WSAgent))

	// ---- uploads ----
	r.Handle("/uploads/*", uploadsHandler(d.Cfg))

	// ---- dashboard SPA ----
	r.Handle("/*", spaHandler())

	return r
}

func requestLogger(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/health" || strings.HasPrefix(r.URL.Path, "/uploads/") ||
				strings.HasPrefix(r.URL.Path, "/assets/") || r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			logx.WithReq(r.Context()).Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", ww.Status()).
				Int("bytes", ww.BytesWritten()).
				Dur("dur", time.Since(start)).
				Msg("request")
		})
	}
}

// fontHandler serves the self-hosted Vazirmatn files with immutable caching and
// permissive CORS (the widget loads them from customer origins).
func fontHandler(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/assets/fonts/")
	if name == "" || strings.Contains(name, "..") {
		http.NotFound(w, r)
		return
	}
	data, err := fs.ReadFile(embedfs.Assets(), path.Join("fonts", name))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sum := sha256.Sum256(data)
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	switch {
	case strings.HasSuffix(name, ".woff2"):
		w.Header().Set("Content-Type", "font/woff2")
	case strings.HasSuffix(name, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
	case strings.HasSuffix(name, ".txt"):
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	_, _ = w.Write(data)
}

func openAPIHandler(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(embedfs.Assets(), "openapi.yaml")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(data)
}

func docsHandler(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(embedfs.Assets(), "docs.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func uploadsHandler(cfg *config.Config) http.Handler {
	root := http.Dir(cfg.Storage.UploadsDir)
	fileServer := http.FileServer(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "..") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; img-src 'self' data:")
		if isImagePath(r.URL.Path) {
			w.Header().Set("Content-Disposition", "inline")
		} else {
			w.Header().Set("Content-Disposition", "attachment")
		}
		w.Header().Set("Cache-Control", "public, max-age=3600")
		d, err := root.Open(r.URL.Path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if st, serr := d.Stat(); serr == nil && st.IsDir() {
			_ = d.Close()
			http.NotFound(w, r)
			return
		}
		_ = d.Close()
		fileServer.ServeHTTP(w, r)
	})
}

func isImagePath(p string) bool {
	lp := strings.ToLower(p)
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp", ".gif"} {
		if strings.HasSuffix(lp, ext) {
			return true
		}
	}
	return false
}

func spaHandler() http.Handler {
	dash := embedfs.Dashboard()
	fileServer := http.FileServer(http.FS(dash))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if strings.HasPrefix(r.URL.Path, "/api") || strings.HasPrefix(r.URL.Path, "/ws") {
			apierr.Write(w, apierr.NotFound("مسیر یافت نشد"))
			return
		}
		if r.URL.Path == "/favicon.ico" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// Serve a real file when it exists.
		if f, err := dash.Open(p); err == nil {
			st, serr := f.Stat()
			_ = f.Close()
			if serr == nil && !st.IsDir() {
				if strings.HasPrefix(p, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		// Unknown paths with an extension are missing assets, not SPA routes.
		if idx := strings.LastIndex(p, "."); idx > 0 && !strings.HasSuffix(p, ".html") {
			http.NotFound(w, r)
			return
		}
		index, err := fs.ReadFile(dash, "index.html")
		if err != nil {
			http.Error(w, "dashboard not built", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
}

func setupSPAHandler() http.Handler {
	setupFS := embedfs.Setup()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/setup")
		p = strings.TrimPrefix(p, "/")
		if p == "" || !strings.Contains(p, ".") {
			p = "index.html"
		}
		data, err := fs.ReadFile(setupFS, p)
		if err != nil {
			data, err = fs.ReadFile(setupFS, "index.html")
			if err != nil {
				http.Error(w, "setup bundle not built", http.StatusServiceUnavailable)
				return
			}
			p = "index.html"
		}
		switch {
		case strings.HasSuffix(p, ".js"):
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		case strings.HasSuffix(p, ".css"):
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		default:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		}
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(data)
	})
}
