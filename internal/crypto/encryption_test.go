package crypto

import (
	"bytes"
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

func key(b byte) []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = b
	}
	return k
}

func TestEncryptDecryptTable(t *testing.T) {
	svc, err := New(map[int][]byte{1: key(1), 2: key(2)}, 2)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	cases := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"ascii", "hello world"},
		{"persian", "سلام، این یک پیام آزمایشی است"},
		{"emoji", "🎉🚀 ≤ ≥"},
		{"long", string(bytes.Repeat([]byte("a"), 4000))},
		{"json", `{"a":1,"b":["x","y"]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ct, iv, ver, err := svc.Encrypt(tc.input)
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}
			if ver != 2 {
				t.Fatalf("expected current version 2, got %d", ver)
			}
			if ct == tc.input && tc.input != "" {
				t.Fatal("ciphertext must differ from plaintext")
			}
			got, err := svc.Decrypt(ct, iv, ver)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}
			if got != tc.input {
				t.Fatalf("roundtrip mismatch: %q != %q", got, tc.input)
			}
		})
	}
}

func TestEncryptUsesFreshNonce(t *testing.T) {
	svc, err := New(map[int][]byte{1: key(9)}, 1)
	if err != nil {
		t.Fatal(err)
	}
	ct1, iv1, _, _ := svc.Encrypt("same")
	ct2, iv2, _, _ := svc.Encrypt("same")
	if ct1 == ct2 || iv1 == iv2 {
		t.Fatal("nonce reuse detected")
	}
}

func TestDecryptUnknownVersion(t *testing.T) {
	svc, err := New(map[int][]byte{1: key(1)}, 1)
	if err != nil {
		t.Fatal(err)
	}
	ct, iv, _, _ := svc.Encrypt("x")
	if _, err := svc.Decrypt(ct, iv, 7); !errors.Is(err, ErrUnknownKeyVersion) {
		t.Fatalf("expected ErrUnknownKeyVersion, got %v", err)
	}
}

func TestDecryptWrongKeyFails(t *testing.T) {
	a, _ := New(map[int][]byte{1: key(1)}, 1)
	b, _ := New(map[int][]byte{1: key(2)}, 1)
	ct, iv, ver, _ := a.Encrypt("secret")
	if _, err := b.Decrypt(ct, iv, ver); err == nil {
		t.Fatal("expected authentication failure with a different key")
	}
}

func TestNewRejectsBadKeys(t *testing.T) {
	if _, err := New(map[int][]byte{1: make([]byte, 16)}, 1); err == nil {
		t.Fatal("expected short key rejection")
	}
	if _, err := New(map[int][]byte{2: key(1)}, 1); err == nil {
		t.Fatal("expected missing current version rejection")
	}
}

func TestReEncrypt(t *testing.T) {
	svc, _ := New(map[int][]byte{1: key(1), 2: key(2)}, 2)
	ct, iv, ver, _ := svc.Encrypt("rotate me")
	// Simulate an old ciphertext by encrypting with version 1 through a second service.
	old, _ := New(map[int][]byte{1: key(1)}, 1)
	oldCT, oldIV, oldVer, _ := old.Encrypt("rotate me")
	newCT, newIV, newVer, err := svc.ReEncrypt(oldCT, oldIV, oldVer)
	if err != nil {
		t.Fatalf("ReEncrypt: %v", err)
	}
	if newVer != 2 {
		t.Fatalf("expected version 2, got %d", newVer)
	}
	got, err := svc.Decrypt(newCT, newIV, newVer)
	if err != nil || got != "rotate me" {
		t.Fatalf("re-encrypted roundtrip failed: %q %v", got, err)
	}
	_ = ct
	_ = iv
	_ = ver
}

func TestHMACSignVerify(t *testing.T) {
	body := []byte(`{"hello":"world"}`)
	sig := SignHex("s3cret", body)
	if !VerifyHex("s3cret", body, sig) {
		t.Fatal("valid signature rejected")
	}
	if VerifyHex("other", body, sig) {
		t.Fatal("signature accepted with wrong secret")
	}
	if VerifyHex("s3cret", []byte("tampered"), sig) {
		t.Fatal("signature accepted for tampered body")
	}
	wh := WebhookSignature("s3cret", 1700000000, body)
	if !VerifyWebhookSignature("s3cret", 1700000000, body, wh) {
		t.Fatal("webhook signature rejected")
	}
	if VerifyWebhookSignature("s3cret", 1700000001, body, wh) {
		t.Fatal("webhook signature accepted with wrong timestamp")
	}
}

func TestEVESignedContext(t *testing.T) {
	payload := []byte(`{"user_id":"u1"}`)
	now := time.Now().Unix()
	sig := SignEVEContext("eve-secret", payload, now)
	if !VerifyEVEContext("eve-secret", payload, now, sig, 5*time.Minute) {
		t.Fatal("valid EVE context rejected")
	}
	stale := now - 3600
	if VerifyEVEContext("eve-secret", payload, stale, SignEVEContext("eve-secret", payload, stale), 5*time.Minute) {
		t.Fatal("stale EVE context accepted")
	}
}

func TestBase64Shape(t *testing.T) {
	svc, _ := New(map[int][]byte{1: key(3)}, 1)
	ct, iv, _, _ := svc.Encrypt("shape")
	if _, err := base64.StdEncoding.DecodeString(ct); err != nil {
		t.Fatalf("ciphertext is not base64: %v", err)
	}
	if _, err := base64.StdEncoding.DecodeString(iv); err != nil {
		t.Fatalf("iv is not base64: %v", err)
	}
	nonce, _ := base64.StdEncoding.DecodeString(iv)
	if len(nonce) != 12 {
		t.Fatalf("expected 12-byte nonce, got %d", len(nonce))
	}
}
