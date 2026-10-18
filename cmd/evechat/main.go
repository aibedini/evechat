// Command evechat is the single-binary EveChat server and operations tool.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/evechat/evechat/internal/setup"
)

// commandNames lists every subcommand accepted by the binary.
var commandNames = map[string]bool{
	"serve": true, "setup": true, "doctor": true, "migrate": true, "backup": true,
	"restore": true, "update": true, "user": true, "project": true, "key": true,
	"healthcheck": true, "version": true, "job": true,
}

// splitCommand finds the subcommand anywhere in args so that both
// `evechat setup --config x` and `evechat --config x setup` work (the installer
// and systemd unit use the second form). Values of flags are skipped.
func splitCommand(args []string) (string, []string, bool) {
	valueFlags := map[string]bool{
		"--config": true, "-config": true, "--data-dir": true, "-data-dir": true,
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if valueFlags[a] {
			i++
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		if commandNames[a] {
			rest := make([]string, 0, len(args)-1)
			rest = append(rest, args[:i]...)
			rest = append(rest, args[i+1:]...)
			return a, rest, true
		}
		return "", nil, false
	}
	return "", nil, false
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		runServe(nil)
		return
	}

	cmd, rest, ok := splitCommand(args)
	if !ok {
		if strings.HasPrefix(args[0], "-") {
			switch args[0] {
			case "-h", "--help", "help":
				printHelp()
				return
			case "-v", "--version":
				fmt.Println(versionString())
				return
			case "--healthcheck", "-healthcheck":
				runHealthcheckCommand(args[1:])
				return
			}
			// Bare flags mean `serve` (backwards compatibility).
			runServe(args)
			return
		}
		fmt.Fprintf(os.Stderr, "دستور ناشناخته: %s\n\n", args[0])
		printHelp()
		os.Exit(2)
	}

	switch cmd {
	case "-h", "--help", "help":
		printHelp()
	case "version":
		fmt.Println(versionString())
	case "serve":
		runServe(rest)
	case "setup":
		runSetupCommand(rest)
	case "doctor":
		runDoctorCommand(rest)
	case "migrate":
		runMigrateCommand(rest)
	case "backup":
		runBackupCommand(rest)
	case "restore":
		runRestoreCommand(rest)
	case "update":
		runUpdateCommand(rest)
	case "user":
		runUserCommand(rest)
	case "project":
		runProjectCommand(rest)
	case "key":
		runKeyCommand(rest)
	case "healthcheck":
		runHealthcheckCommand(rest)
	case "job":
		runJobCommand(rest)
	}
}

func printHelp() {
	fmt.Print(`EveChat — سرور پشتیبانی آنلاین

استفاده:
  evechat [serve] [گزینه‌ها]        اجرای سرور (پیش‌فرض)
  evechat setup [گزینه‌ها]          ویزارد نصب و راه‌اندازی
  evechat doctor [گزینه‌ها]         بررسی سلامت سیستم
  evechat migrate                  اجرای مهاجرت‌ها و خروج
  evechat backup <مسیر>            پشتیبان‌گیری آنلاین
  evechat restore <فایل>           بازیابی از پشتیبان
  evechat update [--check]         به‌روزرسانی باینری
  evechat user add|list|passwd|disable
  evechat project add|list|key-rotate
  evechat key rotate|status
  evechat healthcheck              بررسی سلامت (برای داکر/systemd)
  evechat version                  نمایش نسخه

گزینه‌های سرور:
  --config <path>     مسیر فایل تنظیمات (پیش‌فرض ./evechat.yaml)
  --seed              ساخت پروژه و مدیر نمونه
  --migrate-only      فقط اجرای مهاجرت‌ها

گزینه‌های setup:
  --non-interactive   بدون پرسش (برای نصب خودکار)
  --admin-email --admin-password --admin-name
  --project-name --domains --port --public-url --data-dir
  --print-credentials --json

گزینه‌های doctor:
  --json --verbose --only <id,id> --fix

مستندات: https://github.com/aibedini/evechat
`)
}

// configPathFromArgs extracts --config from a raw flag list (used before any
// FlagSet exists so .env can be loaded first).
func configPathFromArgs(args []string) string {
	path := "./evechat.yaml"
	for i, a := range args {
		if a == "--config" || a == "-config" {
			if i+1 < len(args) {
				path = args[i+1]
			}
		}
		if strings.HasPrefix(a, "--config=") {
			path = strings.TrimPrefix(a, "--config=")
		}
	}
	return path
}

// loadEnvForConfig loads <dir>/.env next to the config file (if any).
func loadEnvForConfig(args []string) string {
	path := configPathFromArgs(args)
	dir := filepath.Dir(path)
	setup.LoadDotEnv(filepath.Join(dir, ".env"))
	if dir != "." {
		setup.LoadDotEnv(".env")
	}
	return path
}

// registerCommonFlags wires the flags shared by every command.
func registerCommonFlags(fs *flag.FlagSet, cfgPath *string) {
	fs.StringVar(cfgPath, "config", "./evechat.yaml", "مسیر فایل تنظیمات")
	fs.StringVar(cfgPath, "c", "./evechat.yaml", "مسیر فایل تنظیمات (کوتاه)")
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "خطا: "+format+"\n", args...)
	os.Exit(1)
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
