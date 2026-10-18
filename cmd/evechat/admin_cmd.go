package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
	"github.com/evechat/evechat/internal/setup"
)

func hashPassword(pw string, cost int) (string, error) {
	if cost <= 0 {
		cost = 12
	}
	b, err := bcrypt.GenerateFromPassword([]byte(pw), cost)
	return string(b), err
}

// runUserCommand implements `evechat user add|list|passwd|disable`.
func runUserCommand(args []string) {
	fs := flag.NewFlagSet("user", flag.ExitOnError)
	cfgPath := "./evechat.yaml"
	registerCommonFlags(fs, &cfgPath)
	email := fs.String("email", "", "ایمیل کاربر")
	password := fs.String("password", "", "رمز عبور")
	name := fs.String("name", "", "نام")
	role := fs.String("role", "agent", "نقش (agent|admin|superadmin)")
	projectID := fs.String("project", "", "شناسه پروژه")
	_ = fs.Parse(args)
	sub := "list"
	if rest := fs.Args(); len(rest) > 0 {
		sub = rest[0]
	}
	setup.LoadDotEnv(filepath.Join(filepath.Dir(cfgPath), ".env"))
	cfg := mustConfig(cfgPath)
	d := mustDB(cfg)
	defer d.Close()
	r := repo.New(d)
	ctx := context.Background()

	resolveProject := func() *models.Project {
		projects, err := r.Projects.List(ctx)
		if err != nil || len(projects) == 0 {
			fatalf("پروژه‌ای وجود ندارد؛ ابتدا `evechat project add` را اجرا کنید")
		}
		if *projectID != "" {
			p, err := r.Projects.GetByID(ctx, *projectID)
			if err != nil {
				fatalf("پروژه یافت نشد: %s", *projectID)
			}
			return p
		}
		return projects[0]
	}

	switch sub {
	case "add":
		if *email == "" || !strings.Contains(*email, "@") {
			fatalf("--email معتبر لازم است")
		}
		p := resolveProject()
		if *password == "" {
			*password = setup.RandomPassword()
		}
		hash, err := hashPassword(*password, cfg.Auth.BcryptCost)
		if err != nil {
			fatalf("%v", err)
		}
		agent := &models.Agent{
			ID: id.NewPrefixed(id.PrefixAgent), ProjectID: p.ID,
			Email: strings.ToLower(*email), PasswordHash: hash, Name: firstNonEmptyStr(*name, *email),
			Role: models.Role(*role), MaxConversations: 10, IsActive: true,
		}
		if err := r.Agents.Create(ctx, agent); err != nil {
			fatalf("%v", err)
		}
		fmt.Printf("کاربر ساخته شد: %s\nرمز عبور: %s\n", agent.Email, *password)
	case "list":
		projects, _ := r.Projects.List(ctx)
		for _, p := range projects {
			agents, err := r.Agents.List(ctx, p.ID)
			if err != nil {
				continue
			}
			fmt.Printf("پروژه %s (%s):\n", p.Name, p.ID)
			for _, a := range agents {
				status := "فعال"
				if !a.IsActive {
					status = "غیرفعال"
				}
				fmt.Printf("  - %-28s %-10s %s\n", a.Email, a.Role, status)
			}
		}
	case "passwd":
		if *email == "" {
			fatalf("--email لازم است")
		}
		agents, err := r.Agents.GetByEmailAny(ctx, *email)
		if err != nil || len(agents) == 0 {
			fatalf("کاربر یافت نشد")
		}
		if *password == "" {
			*password = setup.RandomPassword()
		}
		hash, err := hashPassword(*password, cfg.Auth.BcryptCost)
		if err != nil {
			fatalf("%v", err)
		}
		if err := r.Agents.UpdatePassword(ctx, agents[0].ID, hash); err != nil {
			fatalf("%v", err)
		}
		fmt.Printf("رمز عبور %s تغییر کرد: %s\n", agents[0].Email, *password)
	case "disable":
		if *email == "" {
			fatalf("--email لازم است")
		}
		agents, err := r.Agents.GetByEmailAny(ctx, *email)
		if err != nil || len(agents) == 0 {
			fatalf("کاربر یافت نشد")
		}
		a := agents[0]
		a.IsActive = false
		if err := r.Agents.Update(ctx, a); err != nil {
			fatalf("%v", err)
		}
		fmt.Println("کاربر غیرفعال شد:", a.Email)
	default:
		fatalf("زیردستور نامعتبر: %s (add|list|passwd|disable)", sub)
	}
}

// runProjectCommand implements `evechat project add|list|key-rotate`.
func runProjectCommand(args []string) {
	fs := flag.NewFlagSet("project", flag.ExitOnError)
	cfgPath := "./evechat.yaml"
	registerCommonFlags(fs, &cfgPath)
	name := fs.String("name", "", "نام پروژه")
	domains := fs.String("domains", "", "دامنه‌ها با کاما")
	idArg := fs.String("id", "", "شناسه پروژه")
	_ = fs.Parse(args)
	sub := "list"
	if rest := fs.Args(); len(rest) > 0 {
		sub = rest[0]
	}
	setup.LoadDotEnv(filepath.Join(filepath.Dir(cfgPath), ".env"))
	cfg := mustConfig(cfgPath)
	d := mustDB(cfg)
	defer d.Close()
	r := repo.New(d)
	ctx := context.Background()

	switch sub {
	case "add":
		if strings.TrimSpace(*name) == "" {
			fatalf("--name لازم است")
		}
		project := &models.Project{
			ID: id.NewPrefixed(id.PrefixProject), Name: *name,
			APIKey: id.NewPrefixed("pk"), SecretKey: id.Token(32),
			Domains: models.StringList{}, Settings: models.JSONMap{}, IsActive: true,
		}
		for _, d := range strings.Split(*domains, ",") {
			if d = strings.TrimSpace(d); d != "" {
				project.Domains = append(project.Domains, d)
			}
		}
		if err := r.Projects.Create(ctx, project); err != nil {
			fatalf("%v", err)
		}
		fmt.Printf("پروژه ساخته شد: %s\nAPI key: %s\n", project.Name, project.APIKey)
		fmt.Println(setup.EmbedSnippet(cfg.BaseURL(), project.APIKey))
	case "list":
		projects, err := r.Projects.List(ctx)
		if err != nil {
			fatalf("%v", err)
		}
		for _, p := range projects {
			fmt.Printf("- %s (%s) api_key=%s active=%v\n", p.Name, p.ID, p.APIKey, p.IsActive)
		}
	case "key-rotate":
		projects, err := r.Projects.List(ctx)
		if err != nil || len(projects) == 0 {
			fatalf("پروژه‌ای یافت نشد")
		}
		target := projects[0]
		if *idArg != "" {
			p, err := r.Projects.GetByID(ctx, *idArg)
			if err != nil {
				fatalf("پروژه یافت نشد")
			}
			target = p
		}
		newKey := id.NewPrefixed("pk")
		if err := r.Projects.RotateAPIKey(ctx, target.ID, newKey); err != nil {
			fatalf("%v", err)
		}
		fmt.Printf("کلید جدید پروژه %s: %s\n", target.Name, newKey)
		fmt.Println("ویجت‌های نصب‌شده باید با کلید جدید به‌روزرسانی شوند.")
	default:
		fatalf("زیردستور نامعتبر: %s (add|list|key-rotate)", sub)
	}
}

var _ = os.Getenv
