package fcm

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func testServiceAccount(t *testing.T, tokenURL string) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	raw, err := json.Marshal(map[string]any{
		"project_id":   "demo",
		"client_email": "firebase@demo.iam.gserviceaccount.com",
		"private_key":  string(pemKey),
		"token_uri":    tokenURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestSendDataOnlyMessageAndCacheToken(t *testing.T) {
	var tokenCalls, sendCalls atomic.Int32
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			tokenCalls.Add(1)
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse token form: %v", err)
			}
			if r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
				t.Errorf("grant_type = %q", r.Form.Get("grant_type"))
			}
			if len(strings.Split(r.Form.Get("assertion"), ".")) != 3 {
				t.Errorf("assertion is not a JWT")
			}
			_, _ = w.Write([]byte(`{"access_token":"test-token","expires_in":3600}`))
		case "/v1/projects/demo/messages:send":
			sendCalls.Add(1)
			if gotAuth := r.Header.Get("Authorization"); gotAuth != "Bearer test-token" {
				t.Errorf("authorization = %q", gotAuth)
			}
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode payload: %v", err)
			}
			_, _ = w.Write([]byte(`{"name":"projects/demo/messages/1"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client, err := New(Config{
		ServiceAccountJSON: testServiceAccount(t, srv.URL+"/token"),
		BaseURL:            srv.URL + "/v1",
		HTTPClient:         srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	msg := Message{
		Token: "device-token",
		Data:  map[string]string{"source": "evechat", "v": "1", "type": "message.received", "agent_id": "a1"},
		Options: Options{
			Priority: "HIGH",
			TTL:      "3600s",
		},
	}
	if err := client.Send(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if err := client.Send(context.Background(), msg); err != nil {
		t.Fatal(err)
	}
	if tokenCalls.Load() != 1 {
		t.Fatalf("token endpoint called %d times, want 1 (cached)", tokenCalls.Load())
	}
	if sendCalls.Load() != 2 {
		t.Fatalf("send endpoint called %d times, want 2", sendCalls.Load())
	}

	message, _ := got["message"].(map[string]any)
	if message["token"] != "device-token" {
		t.Fatalf("token in payload = %#v", message["token"])
	}
	if _, ok := message["notification"]; ok {
		t.Fatal("payload must not contain a notification block")
	}
	data, _ := message["data"].(map[string]any)
	if data["source"] != "evechat" || data["agent_id"] != "a1" {
		t.Fatalf("data = %#v", data)
	}
	android, _ := message["android"].(map[string]any)
	if android["priority"] != "HIGH" || android["ttl"] != "3600s" {
		t.Fatalf("android = %#v", android)
	}
}

func TestInvalidTokenError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			_, _ = w.Write([]byte(`{"access_token":"test-token","expires_in":3600}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":404,"message":"Requested entity was not found.","status":"NOT_FOUND","details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"UNREGISTERED"}]}}`))
	}))
	defer srv.Close()

	client, err := New(Config{
		ServiceAccountJSON: testServiceAccount(t, srv.URL+"/token"),
		BaseURL:            srv.URL + "/v1",
		HTTPClient:         srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	err = client.Send(context.Background(), Message{Token: "stale", Data: map[string]string{"type": "sync"}})
	var fe *Error
	if !errors.As(err, &fe) {
		t.Fatalf("error = %v, want *fcm.Error", err)
	}
	if !fe.InvalidToken() {
		t.Fatalf("InvalidToken() = false for %#v", fe)
	}
	if fe.Code != "UNREGISTERED" {
		t.Fatalf("code = %q", fe.Code)
	}
}
