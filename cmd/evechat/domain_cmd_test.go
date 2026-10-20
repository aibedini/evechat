package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindInstallScriptNextToConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "evechat.yaml")
	if err := os.WriteFile(cfg, []byte("server: {}\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "install.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n# --configure-domain\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, root := findInstallScript(cfg, "")
	if got != script {
		t.Fatalf("script = %q, want %q", got, script)
	}
	if root != dir {
		t.Fatalf("root = %q, want %q", root, dir)
	}
}

func TestFindInstallScriptHonoursDirFlag(t *testing.T) {
	configDir := t.TempDir()
	installDir := t.TempDir()
	cfg := filepath.Join(configDir, "evechat.yaml")
	if err := os.WriteFile(cfg, []byte("server: {}\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(installDir, "install.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n# --configure-domain\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, root := findInstallScript(cfg, installDir)
	if got != script || root != installDir {
		t.Fatalf("got (%q, %q), want (%q, %q)", got, root, script, installDir)
	}
}
