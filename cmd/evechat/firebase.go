package main

import (
	"os"
	"strings"

	"github.com/evechat/evechat/internal/config"
)

// applyFirebaseEnv accepts the documented un-prefixed variables, the usual
// EVECHAT_ names, and the double-underscore config tree. The JSON variable may
// contain the document itself or a path; ServiceAccountFile is explicit.
func applyFirebaseEnv(cfg *config.Config) {
	for _, key := range []string{"FIREBASE_SERVICE_ACCOUNT_JSON", "EVECHAT_FIREBASE_SERVICE_ACCOUNT_JSON", "EVECHAT_FIREBASE__SERVICE_ACCOUNT_JSON"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			if strings.HasPrefix(v, "{") {
				cfg.Firebase.ServiceAccountJSON = v
			} else {
				cfg.Firebase.ServiceAccountFile = v
			}
			cfg.Firebase.Enabled = true
			break
		}
	}
	for _, key := range []string{"FIREBASE_SERVICE_ACCOUNT_FILE", "EVECHAT_FIREBASE_SERVICE_ACCOUNT_FILE", "EVECHAT_FIREBASE__SERVICE_ACCOUNT_FILE"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			cfg.Firebase.ServiceAccountFile = v
			cfg.Firebase.Enabled = true
			break
		}
	}
	if v := strings.TrimSpace(os.Getenv("FIREBASE_SERVICE_ACCOUNT")); v != "" && cfg.Firebase.ServiceAccountFile == "" {
		cfg.Firebase.ServiceAccountFile = v
		cfg.Firebase.Enabled = true
	}
	if v := strings.TrimSpace(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")); v != "" && cfg.Firebase.ServiceAccountFile == "" {
		cfg.Firebase.ServiceAccountFile = v
		cfg.Firebase.Enabled = true
	}
	for _, key := range []string{"FIREBASE_PROJECT_ID", "EVECHAT_FIREBASE_PROJECT_ID", "EVECHAT_FIREBASE__PROJECT_ID"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			cfg.Firebase.ProjectID = v
			break
		}
	}
	for _, key := range []string{"FIREBASE_BASE_URL", "EVECHAT_FIREBASE_BASE_URL", "EVECHAT_FIREBASE__BASE_URL"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			cfg.Firebase.BaseURL = v
			break
		}
	}
	for _, key := range []string{"FIREBASE_TOKEN_URL", "EVECHAT_FIREBASE_TOKEN_URL", "EVECHAT_FIREBASE__TOKEN_URL"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			cfg.Firebase.TokenURL = v
			break
		}
	}
	if cfg.Firebase.ServiceAccountJSON != "" || cfg.Firebase.ServiceAccountFile != "" {
		cfg.Firebase.Enabled = true
	}
}
