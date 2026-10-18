package eve

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSanitizeMobileRemovesTheNumberFromErrors(t *testing.T) {
	raw := errors.New(`Get "https://eve.example/api/user/09121234567": dial tcp 10.0.0.1:443: i/o timeout`)
	got := sanitizeMobile(raw, "09121234567").Error()
	if strings.Contains(got, "09121234567") {
		t.Fatalf("error still contains the raw mobile number: %s", got)
	}
	if !strings.Contains(got, "091***67") {
		t.Fatalf("error lost the reference entirely: %s", got)
	}
}

func TestGetUserByMobileErrorHidesTheNumber(t *testing.T) {
	c := New("http://127.0.0.1:1", "key", "", false)
	_, err := c.GetUserByMobile(context.Background(), "09121234567")
	if err == nil {
		t.Fatal("expected a transport error")
	}
	if strings.Contains(err.Error(), "09121234567") {
		t.Fatalf("returned error leaks the mobile number: %s", err.Error())
	}
}
