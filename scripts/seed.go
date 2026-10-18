// Command seed fills a development database with a demo project, team,
// knowledge base articles, routing rules and a few conversations.
//
//	go run ./scripts/seed.go --config evechat.yaml
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"

	"github.com/evechat/evechat/internal/config"
	"github.com/evechat/evechat/internal/crypto"
	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
)

func main() {
	cfgPath := flag.String("config", "./evechat.yaml", "config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	must(err)
	keys, err := cfg.EncryptionKeys()
	must(err)
	enc, err := crypto.New(keys, cfg.Encryption.CurrentVersion)
	must(err)

	d, err := db.Open(cfg)
	must(err)
	defer d.Close()
	must(db.Migrate(context.Background(), d, zerolog.Nop()))

	r := repo.New(d)
	ctx := context.Background()

	project, err := ensureProject(ctx, r)
	must(err)
	fmt.Println("project:", project.ID, "api_key:", project.APIKey)

	agents := ensureAgents(ctx, r, project.ID, cfg.Auth.BcryptCost)
	fmt.Println("agents:", len(agents))

	seedKB(ctx, r, project.ID)
	seedRules(ctx, r, project.ID, agents)
	seedConversations(ctx, r, project.ID, agents, enc)

	fmt.Println("seed complete")
}

func ensureProject(ctx context.Context, r *repo.Repo) (*models.Project, error) {
	list, err := r.Projects.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range list {
		if p.Name == "EVE Demo" {
			return p, nil
		}
	}
	p := &models.Project{
		ID:        id.NewPrefixed(id.PrefixProject),
		Name:      "EVE Demo",
		APIKey:    id.NewPrefixed("pk"),
		SecretKey: id.Token(32),
		Domains:   models.StringList{"localhost", "127.0.0.1"},
		Settings: models.JSONMap{
			"greeting":        "سلام 👋 به پشتیبانی EVE خوش آمدید. چطور می‌توانیم کمک کنیم؟",
			"offline_message": "در حال حاضر خارج از ساعات کاری هستیم؛ پیام بگذارید تا پاسخ دهیم.",
			"position":        "bottom-left",
			"colors":          models.JSONMap{"primary": "#2563eb"},
			"business_hours":  models.JSONMap{"enabled": true, "timezone": "Asia/Tehran", "start": "09:00", "end": "18:00", "days": []any{"0", "1", "2", "3", "4", "5", "6"}},
		},
		IsActive: true,
	}
	if err := r.Projects.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func ensureAgents(ctx context.Context, r *repo.Repo, projectID string, cost int) []*models.Agent {
	specs := []struct {
		email, name, role string
	}{
		{"admin@evechat.local", "مدیر سیستم", "superadmin"},
		{"sara@eve.local", "سارا محمدی", "agent"},
		{"ali@eve.local", "علی رضایی", "agent"},
	}
	out := make([]*models.Agent, 0, len(specs))
	for _, s := range specs {
		if existing, err := r.Agents.GetByEmail(ctx, projectID, s.email); err == nil {
			out = append(out, existing)
			continue
		}
		hash, err := bcryptHash("EveChat!123", cost)
		if err != nil {
			fmt.Println("hash failed:", err)
			continue
		}
		tg := ""
		a := &models.Agent{
			ID:               id.NewPrefixed(id.PrefixAgent),
			ProjectID:        projectID,
			Email:            s.email,
			PasswordHash:     hash,
			Name:             s.name,
			Role:             models.Role(s.role),
			TelegramID:       &tg,
			MaxConversations: 10,
			IsActive:         true,
			IsOnline:         s.role != "superadmin",
		}
		if err := r.Agents.Create(ctx, a); err != nil {
			fmt.Println("agent failed:", err)
			continue
		}
		out = append(out, a)
	}
	return out
}

func seedKB(ctx context.Context, r *repo.Repo, projectID string) {
	articles := []struct{ title, content string }{
		{"تمدید اشتراک", "برای تمدید اشتراک وارد پنل کاربری شوید، بخش اشتراک‌ها را باز کنید و روی تمدید بزنید. پرداخت آنی است."},
		{"بازگشت وجه", "درخواست بازگشت وجه تا ۷ روز پس از خرید از طریق پشتیبانی ثبت می‌شود و حداکثر ۷۲ ساعت کاری پردازش می‌گردد."},
		{"تغییر شماره موبایل", "برای تغییر شماره، کد یک‌بار مصرف شماره قبلی را تأیید کنید و سپس شماره جدید را ثبت کنید."},
		{"مشکل پخش ویدیو", "اگر ویدیو پخش نمی‌شود، کیفیت را روی ۴۸۰p بگذارید، VPN را خاموش کنید و مرورگر را به‌روزرسانی کنید."},
		{"پلن‌ها و قیمت‌ها", "پلن پایه ماهانه ۹۹ هزار تومان، پلن حرفه‌ای ۱۹۹ هزار تومان و پلن سازمانی بر اساس قرارداد است."},
	}
	for i, a := range articles {
		slug := fmt.Sprintf("demo-%d", i+1)
		if _, err := r.KB.GetBySlug(ctx, projectID, slug); err == nil {
			continue
		}
		excerpt := a.content
		if len(excerpt) > 120 {
			excerpt = excerpt[:120]
		}
		_ = r.KB.Create(ctx, &models.KBArticle{
			ID: id.NewPrefixed(id.PrefixKB), ProjectID: projectID, Title: a.title, Slug: slug,
			Content: a.content, Excerpt: &excerpt, Tags: models.StringList{"demo"},
			IsPublished: true,
		})
	}
	fmt.Println("kb articles: 5")
}

func seedRules(ctx context.Context, r *repo.Repo, projectID string, agents []*models.Agent) {
	agentID := ""
	if len(agents) > 1 {
		agentID = agents[1].ID
	}
	rules := []*models.RoutingRule{
		{
			ID: id.NewPrefixed(id.PrefixRule), ProjectID: projectID, Name: "صفحه پرداخت → کارشناس فروش",
			Priority: 100, IsActive: true,
			Conditions: models.JSONMap{"pages": []any{"*/billing*", "*/subscription*", "*/payment*"}},
			RouteTo:    models.JSONMap{"agent_id": agentID},
			AutoReply:  strPtr("سلام 👋 برای سؤال درباره پرداخت، همکاران فروش در خدمت شما هستند."),
		},
		{
			ID: id.NewPrefixed(id.PrefixRule), ProjectID: projectID, Name: "خارج از ساعت کاری → تلگرام",
			Priority: 50, IsActive: true,
			Conditions: models.JSONMap{"business_hours": false},
			RouteTo:    models.JSONMap{"telegram_id": "123456789"},
			AutoReply:  strPtr("پیام شما ثبت شد؛ خارج از ساعات کاری در اولین فرصت پاسخ می‌دهیم."),
		},
	}
	existing, _ := r.Rules.List(ctx, projectID, false)
	if len(existing) > 0 {
		fmt.Println("routing rules: already seeded")
		return
	}
	for _, rule := range rules {
		if err := r.Rules.Create(ctx, rule); err != nil {
			fmt.Println("rule failed:", err)
		}
	}
	fmt.Println("routing rules: 2")
}

func seedConversations(ctx context.Context, r *repo.Repo, projectID string, agents []*models.Agent, enc *crypto.Service) {
	existing, _, err := r.Conversations.List(ctx, projectID, repo.ConvFilter{Status: "all", Limit: 5})
	if err != nil || len(existing) > 0 {
		fmt.Println("conversations: already seeded")
		return
	}
	samples := []struct {
		name, msg, reply string
	}{
		{"مریم احمدی", "سلام، برای تمدید اشتراک باید چیکار کنم؟", "سلام مریم عزیز، از پنل کاربری بخش اشتراک‌ها روی تمدید بزنید. اگر مشکلی بود همینجا بفرمایید."},
		{"حسین کریمی", "ویدیو برای من پخش نمی‌شود.", "سلام حسین جان، لطفاً کیفیت را روی ۴۸۰p بگذارید و VPN را خاموش کنید."},
	}
	for i, s := range samples {
		visitorID := id.NewPrefixed(id.PrefixVisitor)
		name := s.name
		mobile := fmt.Sprintf("0912000000%d", i)
		_ = r.Visitors.Upsert(ctx, &models.Visitor{
			ID: visitorID, ProjectID: projectID, Name: &name, Phone: &mobile,
			DeviceType: strPtr("mobile"), OS: strPtr("Android"), Browser: strPtr("Chrome"),
			Metadata: models.JSONMap{"utm_source": "demo"}, SubscriptionInfo: models.JSONMap{"plan": "pro", "expiry": "2027-01-01"},
		})
		conv := &models.Conversation{
			ID: id.NewPrefixed(id.PrefixConversation), ProjectID: projectID, VisitorID: visitorID,
			Status: models.StatusActive, Channel: "web", PageURL: strPtr("https://eve.ir/pricing"), PageTitle: strPtr("تعرفه‌ها"),
		}
		if len(agents) > 2 {
			conv.AssignedAgentID = &agents[2].ID
		}
		if err := r.Conversations.Create(ctx, conv); err != nil {
			fmt.Println("conversation failed:", err)
			continue
		}
		insert(ctx, r, enc, conv, models.SenderVisitor, s.msg, visitorID)
		insert(ctx, r, enc, conv, models.SenderAgent, s.reply, "")
		_ = r.Conversations.TouchLastMessage(ctx, conv.ID, time.Now().UTC())
	}
	fmt.Println("conversations: 2")
}

func insert(ctx context.Context, r *repo.Repo, enc *crypto.Service, conv *models.Conversation, sender models.SenderType, content, senderID string) {
	ct, iv, ver, err := enc.Encrypt(content)
	if err != nil {
		return
	}
	msg := &models.Message{
		ID: id.NewPrefixed(id.PrefixMessage), ConversationID: conv.ID, SenderType: sender,
		ContentType: "text", ContentEncrypted: ct, ContentIV: iv, KeyVersion: ver,
	}
	if senderID != "" {
		msg.SenderID = &senderID
	}
	_ = r.Messages.Create(ctx, msg)
}

func strPtr(s string) *string { return &s }

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "seed error:", err)
		os.Exit(1)
	}
}

func bcryptHash(pw string, cost int) (string, error) {
	if cost <= 0 {
		cost = 10
	}
	b, err := bcrypt.GenerateFromPassword([]byte(pw), cost)
	return string(b), err
}
