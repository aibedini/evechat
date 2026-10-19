package handlers

import (
	"testing"

	"github.com/evechat/evechat/internal/repo"
)

func TestMergeChannelHeadersKeepsMaskedValues(t *testing.T) {
	stored := map[string]string{"X-Api-Key": "raw-secret", "X-Tenant": "acme"}
	out, err := mergeChannelHeaders(map[string]string{
		"X-API-Key": repo.MaskedHeader,
		"X-Tenant":  "acme-2",
	}, stored)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if out["X-Api-Key"] != "raw-secret" {
		t.Fatalf("masked header lost its stored value: %#v", out)
	}
	if out["X-Tenant"] != "acme-2" {
		t.Fatalf("editable header was not updated: %#v", out)
	}
}

func TestMergeChannelHeadersRejectsMaskWithoutStoredValue(t *testing.T) {
	if _, err := mergeChannelHeaders(map[string]string{"X-API-Key": repo.MaskedHeader}, nil); err == nil {
		t.Fatal("a mask without a stored value must be rejected")
	}
}

func TestValidateChannelHeadersRejectsUnsafeInput(t *testing.T) {
	cases := []map[string]string{
		{"Host": "evil.example"},
		{"X-Test": "line\r\nInjected: 1"},
		{"Bad Header": "x"},
	}
	for _, in := range cases {
		if _, err := validateChannelHeaders(in); err == nil {
			t.Fatalf("unsafe header accepted: %#v", in)
		}
	}
}
