package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/evechat/evechat/internal/config"
	"github.com/evechat/evechat/internal/db"
	"github.com/evechat/evechat/internal/doctor"
	"github.com/evechat/evechat/internal/id"
	"github.com/evechat/evechat/internal/models"
	"github.com/evechat/evechat/internal/repo"
	"github.com/evechat/evechat/internal/setup"
	"github.com/evechat/evechat/internal/tui"
)

var commonPasswords = map[string]bool{
	"password": true, "12345678": true, "123456789": true, "qwerty123": true,
	"admin123": true, "11111111": true, "1234567890": true, "password1": true,
	"iloveyou": true, "welcome1": true, "abc12345": true, "letmein1": true,
	"evechat123": true, "changeme": true, "qwertyuiop": true, "1q2w3e4r": true,
	"12341234": true, "password123": true, "admin@123": true, "test1234": true,
}

func runSetupCommand(args []string) {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	cfgPath := "./evechat.yaml"
	registerCommonFlags(fs, &cfgPath)
	nonInteractive := fs.Bool("non-interactive", false, "بدون پرسش")
	adminEmail := fs.String("admin-email", "", "ایمیل مدیر")
	adminPassword := fs.String("admin-password", "", "رمز مدیر")
	adminName := fs.String("admin-name", "", "نام مدیر")
	projectName := fs.String("project-name", "", "نام پروژه")
	domains := fs.String("domains", "", "دامنه‌های مجاز (با کاما)")
	port := fs.Int("port", 0, "پورت سرور")
	publicURL := fs.String("public-url", "", "آدرس عمومی")
	dataDir := fs.String("data-dir", "", "پوشه داده")
	printCreds := fs.Bool("print-credentials", false, "نمایش اطلاعات ورود")
	asJSON := fs.Bool("json", false, "خروجی JSON")
	force := fs.Bool("force", false, "اجرای مجدد حتی اگر نصب شده باشد")
	_ = fs.Parse(args)

	setup.LoadDotEnv(filepath.Join(filepath.Dir(cfgPath), ".env"))
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fatalf("%v", err)
	}
	plan := setup.DefaultPlan(cfg, cfgPath)
	if *port > 0 {
		plan.Port = *port
	}
	if *publicURL != "" {
		plan.PublicURL = *publicURL
	}
	if *dataDir != "" {
		plan.DataDir = *dataDir
	}
	if *adminEmail != "" {
		plan.AdminEmail = *adminEmail
	}
	if *adminName != "" {
		plan.AdminName = *adminName
	}
	if *adminPassword != "" {
		plan.AdminPassword = *adminPassword
	}
	if *projectName != "" {
		plan.ProjectName = *projectName
	}
	if *domains != "" {
		for _, d := range strings.Split(*domains, ",") {
			if d = strings.TrimSpace(d); d != "" {
				plan.Domains = append(plan.Domains, d)
			}
		}
	}

	if *nonInteractive {
		runSetupNonInteractive(plan, *asJSON, *printCreds, *force)
		return
	}

	if configured(cfgPath) && !*force {
		runSetupMenu(cfg, cfgPath)
		return
	}
	runSetupWizard(plan)
}

func configured(cfgPath string) bool {
	setup.LoadDotEnv(filepath.Join(filepath.Dir(cfgPath), ".env"))
	cfg, err := config.Load(cfgPath)
	if err != nil || cfg.Validate() != nil {
		return false
	}
	d, err := db.Open(cfg)
	if err != nil {
		return false
	}
	defer d.Close()
	n, err := repo.New(d).Agents.CountSuperadmins(context.Background())
	return err == nil && n > 0
}

func runSetupNonInteractive(plan setup.Plan, asJSON, printCreds, force bool) {
	ctx := context.Background()
	if plan.AdminPassword == "" {
		plan.AdminPassword = setup.RandomPassword()
	}
	if err := plan.GenerateMissing(); err != nil {
		fatalf("%v", err)
	}
	res, err := setup.Apply(ctx, plan)
	if err != nil {
		fatalf("%v", err)
	}
	if res.AlreadyConfigured && !force {
		if asJSON {
			out, _ := json.Marshal(res)
			fmt.Println(string(out))
			return
		}
		fmt.Println("نصب قبلاً انجام شده است؛ تغییری اعمال نشد.")
		if res.ProjectKey != "" {
			fmt.Println("کلید پروژه:", res.ProjectKey)
		}
		return
	}
	if asJSON {
		out, _ := json.Marshal(res)
		fmt.Println(string(out))
		return
	}
	printSetupBox(res, printCreds || true)
}

func printSetupBox(res *setup.Result, showPassword bool) {
	password := "••••••••••••"
	if showPassword && res.AdminPassword != "" {
		password = res.AdminPassword
	}
	tui.Title("EveChat نصب شد ✓")
	tui.Box("", []string{
		"پنل:        " + res.DashboardURL,
		"ایمیل:      " + res.AdminEmail,
		"رمز عبور:   " + password,
		"کلید پروژه: " + res.ProjectKey,
	})
	fmt.Println()
	fmt.Println("کد نصب — این کد را قبل از </body> سایت‌تان بچسبانید:")
	fmt.Println(res.EmbedSnippet)
	fmt.Println()
}

func runSetupWizard(plan setup.Plan) {
	ctx := context.Background()
	tui.Title("نصب و راه‌اندازی EveChat")
	fmt.Println("چند دقیقه‌ی دیگر چت پشتیبانی‌تان آماده است. اول بررسی می‌کنیم سرور سالم است.")
	fmt.Println()

	// Step 1 — preflight
	tui.Step(1, 8, "بررسی پیش‌نیازها")
	cfg, err := config.Load(plan.ConfigPath)
	if err != nil {
		fatalf("%v", err)
	}
	report := doctor.RunAll(ctx, doctor.Options{
		ConfigPath: plan.ConfigPath, Cfg: cfg, Only: []string{"fs.data", "net.port", "time.skew"},
	})
	for _, c := range report.Checks {
		if c.Result.OK {
			tui.OK(c.Name + ": " + c.Result.Message)
		} else if c.Level == doctor.Fatal {
			tui.Fail(c.Name + ": " + c.Result.Message)
			if c.Result.Fix != "" {
				fmt.Println("   راه‌حل:", c.Result.Fix)
			}
			fatalf("پیش‌نیازها برقرار نیستند")
		} else {
			tui.Warn(c.Name + ": " + c.Result.Message)
		}
	}

	// Step 2 — data dirs
	tui.Step(2, 8, "مسیر داده‌ها")
	fmt.Println("داده‌های این نصب (پایگاه داده و پیوست‌ها) اینجا ذخیره می‌شود. برای نصب واقعی پوشه‌ای با دسترسی محدود انتخاب کنید.")
	plan.DataDir = tui.Ask("پوشه داده‌ها", plan.DataDir, func(v string) error {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("مسیر نمی‌تواند خالی باشد")
		}
		if err := os.MkdirAll(v, 0o750); err != nil {
			return fmt.Errorf("ساخت پوشه ممکن نشد: %v", err)
		}
		return nil
	})
	_ = os.MkdirAll(filepath.Join(plan.DataDir, "uploads"), 0o750)
	tui.OK("پوشه داده آماده است: " + plan.DataDir)

	// Step 3 — address
	tui.Step(3, 8, "آدرس و پورت")
	fmt.Println("پورت باید آزاد باشد. اگر پشت nginx/Caddy هستید، آدرس عمومی https بدهید و به سؤال بعدی «بله» بگویید.")
	plan.Port = atoiOr(tui.Ask("پورت سرور", strconv.Itoa(plan.Port), func(v string) error {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("پورت باید بین ۱ و ۶۵۵۳۵ باشد")
		}
		return nil
	}), plan.Port)
	plan.PublicURL = tui.Ask("آدرس عمومی (public_url)", plan.PublicURL, func(v string) error {
		if !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
			return fmt.Errorf("آدرس باید با http:// یا https:// شروع شود")
		}
		return nil
	})
	plan.TrustProxy = tui.AskYesNo("پشت پروکسی معکوس (nginx/Caddy) هستید؟", true)

	// Step 4 — keys
	tui.Step(4, 8, "کلیدهای امنیتی")
	fmt.Println("این کلیدها پیام‌ها و نشست‌ها را محافظت می‌کنند. اگر گم شوند، پیام‌های قدیمی قابل بازیابی نیستند.")
	choice := tui.Menu("کلیدهای امنیتی", []tui.MenuItem{
		{Label: "تولید خودکار کلیدها (پیشنهادی)"},
		{Label: "وارد کردن کلیدهای موجود"},
	})
	if choice == 1 {
		plan.JWTSecret = tui.AskSecret("JWT secret (خالی = بدون تغییر)")
		plan.EncryptionKey = tui.AskSecret("Encryption key نسخه ۱ (base64 ۳۲ بایت)")
	}
	plan.GenerateSecrets()
	tui.Box("کلیدهای تولیدشده", []string{
		"JWT_SECRET=" + plan.JWTSecret,
		"EVECHAT_ENCRYPTION_KEY_V1=" + plan.EncryptionKey,
	})
	tui.Warn("این کلیدها را جای امنی نگه دارید. با گم شدن کلید رمزنگاری، پیام‌های قدیمی غیرقابل بازیابی می‌شوند.")

	// Step 5 — admin
	tui.Step(5, 8, "حساب مدیر")
	fmt.Println("این حساب به همه‌چیز دسترسی دارد. رمز را جای امنی نگه دارید.")
	plan.AdminName = tui.Ask("نام مدیر", plan.AdminName, nil)
	plan.AdminEmail = tui.Ask("ایمیل مدیر", plan.AdminEmail, func(v string) error {
		if !strings.Contains(v, "@") {
			return fmt.Errorf("ایمیل معتبر نیست")
		}
		return nil
	})
	if tui.AskYesNo("رمز عبور به‌صورت خودکار تولید شود؟", true) {
		plan.AdminPassword = setup.RandomPassword()
		tui.OK("رمز تولید شد: " + plan.AdminPassword)
	} else {
		for {
			pw := tui.AskSecret("رمز عبور (حداقل ۱۰ کاراکتر)")
			if err := validatePassword(pw); err != nil {
				tui.Fail(err.Error())
				continue
			}
			again := tui.AskSecret("تکرار رمز عبور")
			if again != pw {
				tui.Fail("رمزها یکسان نیستند")
				continue
			}
			plan.AdminPassword = pw
			break
		}
	}

	// Step 6 — project
	tui.Step(6, 8, "اولین پروژه")
	fmt.Println("پروژه یعنی یک سایت یا سرویس شما. «دامنه‌های مجاز»: فقط از این سایت‌ها ویجت کار می‌کند. اگر خالی بگذارید از هر سایتی کار می‌کند؛ این فقط برای آزمایش است.")
	plan.ProjectName = tui.Ask("نام پروژه", plan.ProjectName, nil)
	domainsRaw := tui.Ask("دامنه‌های مجاز (با کاما، خالی = حالت توسعه)", "", nil)
	if strings.TrimSpace(domainsRaw) == "" {
		tui.Warn("بدون دامنه، ویجت روی همه دامنه‌ها اجرا می‌شود (فقط برای توسعه)")
	} else {
		for _, d := range strings.Split(domainsRaw, ",") {
			d = strings.TrimSpace(d)
			if d != "" && !strings.Contains(d, ".") && d != "localhost" {
				tui.Warn("دامنه نامعتبر نادیده گرفته شد: " + d)
				continue
			}
			if d != "" {
				plan.Domains = append(plan.Domains, d)
			}
		}
	}

	// Step 7 — integrations (optional)
	tui.Step(7, 8, "یکپارچه‌سازی‌ها (اختیاری)")
	fmt.Println("همه اختیاری‌اند و بعداً هم از تنظیمات می‌شود اضافه کرد.")
	selected := tui.MultiSelect("کدام سرویس‌ها را می‌خواهید؟", []string{
		"تلگرام (ربات پشتیبانی)",
		"EVE (شناسایی کاربر)",
		"GMweb (پیامک)",
		"دستیار هوش مصنوعی",
		"ایمیل (SMTP)",
	}, nil)
	for _, idx := range selected {
		switch idx {
		case 0:
			plan.TelegramToken = tui.AskSecret("توکن ربات تلگرام")
			if plan.TelegramToken != "" {
				plan.TelegramEnabled = true
				plan.TelegramSecret = setup.RandomSecret(24)
				ok, message := setup.TestTelegram(ctx, plan.TelegramToken)
				reportProbe("تلگرام", ok, message)
			}
		case 1:
			plan.EVEBaseURL = tui.Ask("آدرس EVE", "", nil)
			plan.EVEAPIKey = tui.AskSecret("کلید EVE")
			plan.EVESecret = tui.AskSecret("HMAC secret (اختیاری)")
			if plan.EVEBaseURL != "" {
				ok, message := setup.TestEVE(ctx, plan.EVEBaseURL, plan.EVEAPIKey)
				reportProbe("EVE", ok, message)
			}
		case 2:
			plan.GMwebBaseURL = tui.Ask("آدرس GMweb", "", nil)
			plan.GMwebAPIKey = tui.AskSecret("کلید GMweb")
			if plan.GMwebBaseURL != "" {
				ok, message := setup.TestURL(ctx, plan.GMwebBaseURL)
				reportProbe("GMweb", ok, message)
			}
		case 3:
			plan.AIKey = tui.AskSecret("کلید Anthropic")
			if plan.AIKey != "" {
				plan.AIEnabled = true
				ok, message := setup.TestAnthropic(ctx, plan.AIKey, plan.AIModel)
				reportProbe("هوش مصنوعی", ok, message)
			}
		case 4:
			plan.SMTPEnabled = true
			plan.SMTPHost = tui.Ask("SMTP host", "", nil)
			plan.SMTPPort = atoiOr(tui.Ask("SMTP port", "587", nil), 587)
			plan.SMTPUser = tui.Ask("SMTP username", "", nil)
			plan.SMTPPassword = tui.AskSecret("SMTP password")
			plan.SMTPFrom = tui.Ask("آدرس فرستنده", plan.SMTPUser, nil)
			plan.SMTPFromName = tui.Ask("نام فرستنده", "EveChat", nil)
			if plan.SMTPHost != "" {
				ok, message := setup.TestSMTP(ctx, setup.SMTPProbe{
					Host: plan.SMTPHost, Port: plan.SMTPPort, Username: plan.SMTPUser,
					Password: plan.SMTPPassword, From: plan.SMTPFrom, TLSMode: plan.SMTPTLSMode,
				})
				reportProbe("ایمیل", ok, message)
			}
		}
	}

	// Step 8 — apply
	tui.Step(8, 8, "آماده است!")
	res, err := setup.Apply(ctx, plan)
	if err != nil {
		fatalf("%v", err)
	}
	printSetupBox(res, true)
	fmt.Println("گام بعدی: ./evechat --config " + plan.ConfigPath + " serve")
}

func reportProbe(name string, ok bool, message string) {
	if ok {
		tui.OK(name + ": " + message)
		return
	}
	tui.Warn(name + ": تست ناموفق — " + message)
}

func validatePassword(pw string) error {
	if len(pw) < 10 {
		return fmt.Errorf("رمز عبور باید حداقل ۱۰ کاراکتر باشد")
	}
	if commonPasswords[strings.ToLower(pw)] {
		return fmt.Errorf("این رمز عبور بسیار رایج است")
	}
	return nil
}

func atoiOr(s string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return n
}

// runSetupMenu is the re-run menu shown when the instance is already configured.
func runSetupMenu(cfg *config.Config, cfgPath string) {
	tui.Title("مدیریت نصب EveChat")
	for {
		choice := tui.Menu("چه کاری انجام شود؟", []tui.MenuItem{
			{Label: "تغییر تنظیمات سرور"},
			{Label: "افزودن کاربر پشتیبان"},
			{Label: "افزودن پروژه"},
			{Label: "پیکربندی یکپارچه‌سازی‌ها"},
			{Label: "چرخش کلید رمزنگاری"},
			{Label: "تست سلامت سیستم"},
			{Label: "نمایش کد embed"},
			{Label: "خروج"},
		})
		switch choice {
		case 0:
			port := atoiOr(tui.Ask("پورت سرور", strconv.Itoa(cfg.Server.Port), nil), cfg.Server.Port)
			publicURL := tui.Ask("آدرس عمومی", cfg.BaseURL(), nil)
			if err := setup.PatchServerSettings(cfgPath, port, publicURL); err != nil {
				tui.Fail(err.Error())
			} else {
				tui.OK("تنظیمات ذخیره شد؛ برای اعمال، سرویس را ری‌استارت کنید")
			}
		case 1:
			addAgentInteractive(cfg, cfgPath)
		case 2:
			addProjectInteractive(cfg, cfgPath)
		case 3:
			tui.Warn("یکپارچه‌سازی‌ها را از پنل (تنظیمات ← یکپارچه‌سازی‌ها) پیکربندی کنید")
		case 4:
			tui.Warn("کلید جدید را در .env با نام EVECHAT_ENCRYPTION_KEY_V" + strconv.Itoa(cfg.Encryption.CurrentVersion+1) + " قرار دهید و سپس: evechat key rotate")
		case 5:
			report := doctor.RunAll(context.Background(), doctor.Options{ConfigPath: cfgPath, Cfg: cfg, Version: version})
			report.Print(os.Stdout, true)
		case 6:
			printEmbedSnippet(cfg, cfgPath)
		case 7, -1:
			return
		}
	}
}

func printEmbedSnippet(cfg *config.Config, cfgPath string) {
	setup.LoadDotEnv(filepath.Join(filepath.Dir(cfgPath), ".env"))
	d, err := db.Open(cfg)
	if err != nil {
		tui.Fail(err.Error())
		return
	}
	defer d.Close()
	projects, err := repo.New(d).Projects.List(context.Background())
	if err != nil || len(projects) == 0 {
		tui.Fail("پروژه‌ای یافت نشد")
		return
	}
	fmt.Println(setup.EmbedSnippet(cfg.BaseURL(), projects[0].APIKey))
}

func addAgentInteractive(cfg *config.Config, cfgPath string) {
	setup.LoadDotEnv(filepath.Join(filepath.Dir(cfgPath), ".env"))
	d, err := db.Open(cfg)
	if err != nil {
		tui.Fail(err.Error())
		return
	}
	defer d.Close()
	r := repo.New(d)
	projects, err := r.Projects.List(context.Background())
	if err != nil || len(projects) == 0 {
		tui.Fail("ابتدا یک پروژه بسازید")
		return
	}
	project := projects[0]
	if len(projects) > 1 {
		items := make([]tui.MenuItem, len(projects))
		for i, p := range projects {
			items[i] = tui.MenuItem{Label: p.Name}
		}
		if idx := tui.Menu("پروژه", items); idx >= 0 {
			project = projects[idx]
		}
	}
	email := tui.Ask("ایمیل کاربر", "", func(v string) error {
		if !strings.Contains(v, "@") {
			return fmt.Errorf("ایمیل معتبر نیست")
		}
		return nil
	})
	name := tui.Ask("نام", email, nil)
	password := tui.AskSecret("رمز عبور (خالی = تولید خودکار)")
	if password == "" {
		password = setup.RandomPassword()
	}
	hash, err := hashPassword(password, cfg.Auth.BcryptCost)
	if err != nil {
		tui.Fail(err.Error())
		return
	}
	agent := &models.Agent{
		ID: id.NewPrefixed(id.PrefixAgent), ProjectID: project.ID,
		Email: strings.ToLower(email), PasswordHash: hash, Name: name,
		Role: models.RoleAgent, MaxConversations: 10, IsActive: true,
	}
	if err := r.Agents.Create(context.Background(), agent); err != nil {
		tui.Fail(err.Error())
		return
	}
	tui.OK("کاربر ساخته شد — رمز: " + password)
}

func addProjectInteractive(cfg *config.Config, cfgPath string) {
	setup.LoadDotEnv(filepath.Join(filepath.Dir(cfgPath), ".env"))
	d, err := db.Open(cfg)
	if err != nil {
		tui.Fail(err.Error())
		return
	}
	defer d.Close()
	r := repo.New(d)
	name := tui.Ask("نام پروژه", "", nil)
	project := &models.Project{
		ID: id.NewPrefixed(id.PrefixProject), Name: name,
		APIKey: id.NewPrefixed("pk"), SecretKey: id.Token(32),
		Domains: models.StringList{}, Settings: models.JSONMap{}, IsActive: true,
	}
	if err := r.Projects.Create(context.Background(), project); err != nil {
		tui.Fail(err.Error())
		return
	}
	tui.OK("پروژه ساخته شد: " + project.ID)
	fmt.Println(setup.EmbedSnippet(cfg.BaseURL(), project.APIKey))
}
