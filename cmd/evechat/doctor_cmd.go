package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/evechat/evechat/internal/config"
	"github.com/evechat/evechat/internal/doctor"
	"github.com/evechat/evechat/internal/logx"
	"github.com/evechat/evechat/internal/setup"
)

func runDoctorCommand(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	cfgPath := "./evechat.yaml"
	registerCommonFlags(fs, &cfgPath)
	jsonOut := fs.Bool("json", false, "خروجی JSON")
	verbose := fs.Bool("verbose", false, "جزئیات بیشتر")
	only := fs.String("only", "", "فقط بررسی‌های مشخص (با کاما)")
	fix := fs.Bool("fix", false, "اعمال تعمیرات امن")
	_ = fs.Parse(args)

	setup.LoadDotEnv(filepath.Join(filepath.Dir(cfgPath), ".env"))
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "خطا در خواندن تنظیمات:", err)
		os.Exit(1)
	}
	logx.Init("warn", false)
	ctx := context.Background()

	if *fix {
		actions := doctor.Fix(ctx, cfg)
		if len(actions) == 0 {
			fmt.Println("تعمیر خودکاری لازم نبود.")
		}
		for _, a := range actions {
			fmt.Println("✓", a)
		}
	}

	var onlyIDs []string
	if strings.TrimSpace(*only) != "" {
		for _, id := range strings.Split(*only, ",") {
			if id = strings.TrimSpace(id); id != "" {
				onlyIDs = append(onlyIDs, id)
			}
		}
	}
	report := doctor.RunAll(ctx, doctor.Options{
		ConfigPath: cfgPath,
		Cfg:        cfg,
		Log:        *logx.L(),
		Only:       onlyIDs,
		Version:    version,
	})
	if *jsonOut {
		fmt.Println(string(report.JSON()))
	} else {
		report.Print(os.Stdout, *verbose)
	}
	os.Exit(report.ExitCode())
}
