package gmweb

import (
	"errors"
	"strings"
	"testing"
)

func TestSanitizeRemovesTheNumberFromErrors(t *testing.T) {
	raw := errors.New("Post \"https://gmweb.example/send\": dial tcp: connection refused")
	got := sanitize(raw, "09121234567").Error()
	if strings.Contains(got, "09121234567") {
		t.Fatalf("error still contains the raw phone number: %s", got)
	}
}
