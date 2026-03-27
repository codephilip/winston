package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestBasicAuth_ValidCredentials(t *testing.T) {
	os.Setenv("POLYMR_USER", "admin")
	os.Setenv("POLYMR_PASS", "secret123")
	defer os.Unsetenv("POLYMR_USER")
	defer os.Unsetenv("POLYMR_PASS")

	handler := BasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/api/agents", nil)
	req.SetBasicAuth("admin", "secret123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestBasicAuth_InvalidCredentials(t *testing.T) {
	os.Setenv("POLYMR_USER", "admin")
	os.Setenv("POLYMR_PASS", "secret123")
	defer os.Unsetenv("POLYMR_USER")
	defer os.Unsetenv("POLYMR_PASS")

	handler := BasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/agents", nil)
	req.SetBasicAuth("admin", "wrongpassword")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}

	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Error("expected WWW-Authenticate header")
	}
}

func TestBasicAuth_NoCredentials(t *testing.T) {
	os.Setenv("POLYMR_USER", "admin")
	os.Setenv("POLYMR_PASS", "secret123")
	defer os.Unsetenv("POLYMR_USER")
	defer os.Unsetenv("POLYMR_PASS")

	handler := BasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/agents", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestBasicAuth_NotConfigured(t *testing.T) {
	os.Unsetenv("POLYMR_USER")
	os.Unsetenv("POLYMR_PASS")

	handler := BasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/agents", nil)
	req.SetBasicAuth("anything", "anything")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 when auth not configured, got %d", rec.Code)
	}
}

func TestBasicAuth_WrongUsername(t *testing.T) {
	os.Setenv("POLYMR_USER", "admin")
	os.Setenv("POLYMR_PASS", "secret123")
	defer os.Unsetenv("POLYMR_USER")
	defer os.Unsetenv("POLYMR_PASS")

	handler := BasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/agents", nil)
	req.SetBasicAuth("notadmin", "secret123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestSecureCompare_Equal(t *testing.T) {
	if !secureCompare("hello", "hello") {
		t.Error("expected equal strings to match")
	}
}

func TestSecureCompare_NotEqual(t *testing.T) {
	if secureCompare("hello", "world") {
		t.Error("expected different strings to not match")
	}
}

func TestSecureCompare_Empty(t *testing.T) {
	if !secureCompare("", "") {
		t.Error("expected empty strings to match")
	}
}

func TestSecureCompare_TimingSafe(t *testing.T) {
	// Ensure nearly-identical strings don't short-circuit
	// (SHA256 hashing + constant-time compare prevents this)
	a := "password123456789"
	b := "password123456780"
	if secureCompare(a, b) {
		t.Error("nearly-identical strings should not match")
	}
}
