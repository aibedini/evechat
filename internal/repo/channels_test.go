package repo

import "testing"

func TestMaskHeaders(t *testing.T) {
	in := map[string]string{
		"X-API-Key":     "super-secret",
		"Authorization": "Bearer abc",
		"X-Trace-Id":    "trace-1",
	}
	out := MaskHeaders(in)
	if out["X-API-Key"] != MaskedHeader || out["Authorization"] != MaskedHeader {
		t.Fatalf("sensitive headers were not masked: %+v", out)
	}
	if out["X-Trace-Id"] != "trace-1" {
		t.Fatalf("non-sensitive header changed: %+v", out)
	}
	if in["X-API-Key"] != "super-secret" {
		t.Fatal("MaskHeaders mutated the input map")
	}
}
