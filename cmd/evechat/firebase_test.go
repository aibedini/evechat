package main

import (
	"testing"

	"github.com/evechat/evechat/internal/config"
)

func TestApplyFirebaseEnvRawAndPath(t *testing.T) {
	cfg := config.Default()
	t.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", `{"project_id":"demo"}`)
	applyFirebaseEnv(cfg)
	if !cfg.Firebase.Enabled || cfg.Firebase.ServiceAccountJSON == "" {
		t.Fatalf("raw JSON was not applied: %#v", cfg.Firebase)
	}

	cfg = config.Default()
	t.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", "")
	t.Setenv("FIREBASE_SERVICE_ACCOUNT_FILE", "/tmp/firebase.json")
	t.Setenv("FIREBASE_SERVICE_ACCOUNT", "")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	t.Setenv("FIREBASE_PROJECT_ID", "")
	applyFirebaseEnv(cfg)
	if !cfg.Firebase.Enabled || cfg.Firebase.ServiceAccountFile != "/tmp/firebase.json" {
		t.Fatalf("file path was not applied: %#v", cfg.Firebase)
	}
}
