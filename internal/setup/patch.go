package setup

import (
	"fmt"
	"os"
	"strings"
)

// GenerateMissing fills secrets and defaults that were not supplied.
func (p *Plan) GenerateMissing() error {
	if strings.TrimSpace(p.AdminEmail) == "" {
		p.AdminEmail = "admin@evechat.local"
	}
	if !strings.Contains(p.AdminEmail, "@") {
		return fmt.Errorf("ایمیل مدیر معتبر نیست: %s", p.AdminEmail)
	}
	if p.AdminName == "" {
		p.AdminName = "مدیر سیستم"
	}
	if p.ProjectName == "" {
		p.ProjectName = "EVE"
	}
	if p.Port <= 0 {
		p.Port = 3000
	}
	p.GenerateSecrets()
	return nil
}

// PatchServerSettings rewrites only the port/public_url lines of an existing
// YAML file, keeping every other line (and comment) untouched.
func PatchServerSettings(path string, port int, publicURL string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	inServer := false
	foundPort := false
	foundURL := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(line, "server:") {
			inServer = true
			continue
		}
		if inServer {
			if !strings.HasPrefix(line, " ") {
				inServer = false
				continue
			}
			if strings.HasPrefix(trimmed, "port:") {
				lines[i] = fmt.Sprintf("  port: %d", port)
				foundPort = true
			}
			if strings.HasPrefix(trimmed, "public_url:") {
				lines[i] = fmt.Sprintf("  public_url: %q", publicURL)
				foundURL = true
			}
		}
	}
	if !foundPort || !foundURL {
		insert := []string{}
		if !foundPort {
			insert = append(insert, fmt.Sprintf("  port: %d", port))
		}
		if !foundURL {
			insert = append(insert, fmt.Sprintf("  public_url: %q", publicURL))
		}
		lines = append(lines, insert...)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o640)
}
