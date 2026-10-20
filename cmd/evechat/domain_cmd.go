package main

import (
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// runDomainCommand implements `evechat domain status|set|ssl`.
//
// The actual nginx/certbot work lives in install.sh so there is exactly one
// implementation of the vhost generator. This command is the friendly CLI:
// it detects the install directory and the service port itself; the operator
// never has to pass a port.
func runDomainCommand(args []string) {
	fs := flag.NewFlagSet("domain", flag.ExitOnError)
	cfgPath := "./evechat.yaml"
	registerCommonFlags(fs, &cfgPath)
	dirFlag := fs.String("dir", "", "install directory (default: next to the config)")
	ssl := fs.Bool("ssl", false, "obtain/renew a Let's Encrypt certificate")
	email := fs.String("email", "", "ACME account email")
	domainFlag := fs.String("domain", "", "domain name")
	rest := positional(fs, args)

	sub := "status"
	if len(rest) > 0 {
		sub = rest[0]
	}
	domain := strings.TrimSpace(*domainFlag)
	if len(rest) > 1 {
		domain = strings.TrimSpace(rest[1])
	}

	switch sub {
	case "status":
		printDomainStatus(cfgPath)
	case "set":
		if domain == "" {
			fatalf("usage: evechat domain set <domain> [--ssl] [--email you@example.com]")
		}
		script, dir := findInstallScript(cfgPath, *dirFlag)
		runDomainInstaller(script, dir, domain, *ssl, *email)
	case "ssl":
		script, dir := findInstallScript(cfgPath, *dirFlag)
		runDomainInstaller(script, dir, domain, true, *email)
	default:
		fatalf("unknown domain subcommand: %s (use status|set|ssl)", sub)
	}
}

func printDomainStatus(cfgPath string) {
	cfg := mustConfig(cfgPath)
	host := ""
	if u, err := url.Parse(cfg.Server.PublicURL); err == nil {
		host = u.Hostname()
	}
	if host == "" || host == "localhost" || strings.HasPrefix(host, "127.") {
		host = "(not set)"
	}
	fmt.Printf("Domain:      %s\n", host)
	fmt.Printf("Public URL:  %s\n", cfg.Server.PublicURL)
	fmt.Printf("Listen:      %s:%d\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("Trust proxy: %v\n", cfg.Server.TrustProxy)

	if host != "" && host != "(not set)" {
		if path, notAfter, ok := certificateFor(host); ok {
			fmt.Printf("TLS:         %s (expires %s)\n", path, notAfter.Format("2006-01-02"))
		} else {
			fmt.Println("TLS:         not configured")
		}
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/api/v1/health", cfg.Server.Port))
	if err != nil {
		fmt.Println("Health:      no response")
		return
	}
	resp.Body.Close()
	fmt.Printf("Health:      HTTP %d\n", resp.StatusCode)
}

func certificateFor(domain string) (string, time.Time, bool) {
	exact := filepath.Join("/etc/letsencrypt/live", domain, "fullchain.pem")
	if path, notAfter, ok := readCertificate(exact, domain); ok {
		return path, notAfter, true
	}
	matches, _ := filepath.Glob("/etc/letsencrypt/live/*/fullchain.pem")
	for _, path := range matches {
		if path == exact {
			continue
		}
		if found, notAfter, ok := readCertificate(path, domain); ok {
			return found, notAfter, true
		}
	}
	return "", time.Time{}, false
}

func readCertificate(path, domain string) (string, time.Time, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", time.Time{}, false
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return path, time.Time{}, false
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return path, time.Time{}, false
	}
	if domain != "" && cert.VerifyHostname(domain) != nil {
		return path, time.Time{}, false
	}
	return path, cert.NotAfter, true
}

// findInstallScript looks next to the config, next to the binary, and in the
// default install directory. The returned directory is passed to install.sh so
// it updates the same instance.
func findInstallScript(cfgPath, dirFlag string) (string, string) {
	var roots []string
	absCfg := ""
	if strings.TrimSpace(dirFlag) != "" {
		roots = append(roots, dirFlag)
	}
	if abs, err := filepath.Abs(cfgPath); err == nil {
		absCfg = abs
		roots = append(roots, filepath.Dir(abs))
	}
	if exe, err := os.Executable(); err == nil {
		roots = append(roots, filepath.Dir(exe))
	}
	roots = append(roots, "/opt/evechat", ".")

	seen := map[string]bool{}
	for _, root := range roots {
		root = filepath.Clean(root)
		if root == "." && absCfg != "" {
			root = filepath.Dir(absCfg)
		}
		if abs, err := filepath.Abs(root); err == nil {
			root = abs
		}
		if seen[root] {
			continue
		}
		seen[root] = true
		script := filepath.Join(root, "install.sh")
		if st, err := os.Stat(script); err == nil && !st.IsDir() {
			data, err := os.ReadFile(script)
			if err != nil || !strings.Contains(string(data), "--configure-domain") {
				continue
			}
			return script, root
		}
	}
	fatalf("install.sh was not found for this installation; upgrade EveChat first or run the installer manually")
	return "", ""
}

func runDomainInstaller(script, dir, domain string, ssl bool, email string) {
	if os.Geteuid() != 0 {
		fatalf("run this command with sudo")
	}
	args := []string{"--configure-domain", "--yes", "--no-tui", "--dir", dir}
	if strings.TrimSpace(domain) != "" {
		args = append(args, "--domain", strings.TrimSpace(domain))
	}
	if ssl {
		args = append(args, "--ssl")
	}
	if strings.TrimSpace(email) != "" {
		args = append(args, "--email", strings.TrimSpace(email))
	}

	cmd := exec.Command(script, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fatalf("could not run %s: %v", script, err)
	}
}
