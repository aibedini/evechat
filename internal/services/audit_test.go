package services

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestRedactRemovesSecrets is the regression test for the audit PII rule: no
// credential-looking key may reach audit_log, whatever its nesting.
func TestRedactRemovesSecrets(t *testing.T) {
	in := map[string]any{
		"password":   "hunter2",
		"api_key":    "pk_live_123",
		"secret":     "s3cr3t",
		"token":      "tok_abc",
		"jwt_secret": "jwt-value",
		"email":      "admin@example.com",
		"nested": map[string]any{
			"Password": "nested-pw",
			"keep":     "visible",
		},
		"list": []any{
			map[string]any{"token": "list-token", "name": "first"},
			map[string]any{"Authorization": "Bearer xyz", "name": "second"},
		},
	}
	out := redact(in)

	for _, key := range []string{"password", "api_key", "secret", "token", "jwt_secret"} {
		if out[key] != Redacted {
			t.Errorf("key %q was not redacted: %v", key, out[key])
		}
	}
	if out["email"] != "admin@example.com" {
		t.Errorf("non-secret key was redacted: %v", out["email"])
	}
	nested, ok := out["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested map lost its type: %T", out["nested"])
	}
	if nested["Password"] != Redacted || nested["keep"] != "visible" {
		t.Errorf("nested redaction wrong: %+v", nested)
	}
	list, ok := out["list"].([]any)
	if !ok || len(list) != 2 {
		t.Fatalf("list lost its shape: %#v", out["list"])
	}
	first := list[0].(map[string]any)
	second := list[1].(map[string]any)
	if first["token"] != Redacted {
		t.Errorf("token inside a list was not redacted: %v", first["token"])
	}
	if second["Authorization"] != Redacted {
		t.Errorf("authorization inside a list was not redacted: %v", second["Authorization"])
	}
	if first["name"] != "first" || second["name"] != "second" {
		t.Errorf("list items lost their non-secret fields: %#v", list)
	}

	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshalling redacted detail: %v", err)
	}
	for _, leaked := range []string{"hunter2", "s3cr3t", "tok_abc", "jwt-value", "list-token", "Bearer xyz", "nested-pw"} {
		if strings.Contains(string(encoded), leaked) {
			t.Errorf("serialised audit detail still contains %q: %s", leaked, encoded)
		}
	}
}

func TestRecordIsSafeWithoutARepo(t *testing.T) {
	// Auditing must never break the operation it describes.
	var nilSvc *AuditService
	nilSvc.Record(context.Background(), AuditInput{Detail: map[string]any{"password": "x"}})
}
