package slack

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"
)

func makeSignedRequest(t *testing.T, secret, body string) *http.Request {
	t.Helper()
	ts := strconv.FormatInt(time.Now().Unix(), 10)

	sigBase := fmt.Sprintf("v0:%s:%s", ts, body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(sigBase))
	sig := fmt.Sprintf("v0=%x", mac.Sum(nil))

	req := httptest.NewRequest("POST", "/slack/commands", bytes.NewBufferString(body))
	req.Header.Set("X-Slack-Request-Timestamp", ts)
	req.Header.Set("X-Slack-Signature", sig)
	return req
}

func TestVerifyMiddleware_ValidSignature(t *testing.T) {
	secret := "test-signing-secret-12345"
	os.Setenv("SLACK_SIGNING_SECRET", secret)
	defer os.Unsetenv("SLACK_SIGNING_SECRET")

	handler := VerifyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := makeSignedRequest(t, secret, "command=/marketing&text=hello")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for valid signature, got %d", rec.Code)
	}
}

func TestVerifyMiddleware_InvalidSignature(t *testing.T) {
	os.Setenv("SLACK_SIGNING_SECRET", "real-secret")
	defer os.Unsetenv("SLACK_SIGNING_SECRET")

	handler := VerifyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Sign with wrong secret
	req := makeSignedRequest(t, "wrong-secret", "command=/marketing&text=hello")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid signature, got %d", rec.Code)
	}
}

func TestVerifyMiddleware_MissingTimestamp(t *testing.T) {
	os.Setenv("SLACK_SIGNING_SECRET", "secret")
	defer os.Unsetenv("SLACK_SIGNING_SECRET")

	handler := VerifyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/slack/commands", bytes.NewBufferString("test"))
	// No timestamp header
	req.Header.Set("X-Slack-Signature", "v0=fake")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing timestamp, got %d", rec.Code)
	}
}

func TestVerifyMiddleware_ReplayAttack(t *testing.T) {
	secret := "secret"
	os.Setenv("SLACK_SIGNING_SECRET", secret)
	defer os.Unsetenv("SLACK_SIGNING_SECRET")

	handler := VerifyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	body := "command=/marketing&text=hello"

	// Use a timestamp from 10 minutes ago (outside 5-min window)
	oldTS := strconv.FormatInt(time.Now().Unix()-600, 10)
	sigBase := fmt.Sprintf("v0:%s:%s", oldTS, body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(sigBase))
	sig := fmt.Sprintf("v0=%x", mac.Sum(nil))

	req := httptest.NewRequest("POST", "/slack/commands", bytes.NewBufferString(body))
	req.Header.Set("X-Slack-Request-Timestamp", oldTS)
	req.Header.Set("X-Slack-Signature", sig)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for replay attack (old timestamp), got %d", rec.Code)
	}
}

func TestVerifyMiddleware_NoSecret(t *testing.T) {
	os.Unsetenv("SLACK_SIGNING_SECRET")

	handler := VerifyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("POST", "/slack/commands", bytes.NewBufferString("test"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Should skip verification in dev mode
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 when no secret configured (dev mode), got %d", rec.Code)
	}
}

func TestAbs(t *testing.T) {
	tests := []struct {
		input    int64
		expected int64
	}{
		{5, 5},
		{-5, 5},
		{0, 0},
		{-1, 1},
	}
	for _, tt := range tests {
		got := abs(tt.input)
		if got != tt.expected {
			t.Errorf("abs(%d) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}
