package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// testAPIServer creates a minimal API server for integration testing.
// Bypasses router.New() which needs Slack tokens and agent files.
func testAPIServer(t *testing.T) http.Handler {
	t.Helper()

	os.Setenv("POLYMR_USER", "testuser")
	os.Setenv("POLYMR_PASS", "testpass")
	t.Cleanup(func() {
		os.Unsetenv("POLYMR_USER")
		os.Unsetenv("POLYMR_PASS")
	})

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	mux.Handle("/api/agents", BasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	})))

	return SecurityHeaders(RateLimitAPI(mux))
}

func TestIntegration_HealthEndpoint(t *testing.T) {
	srv := testAPIServer(t)
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("health check failed: %d", rec.Code)
	}

	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", body["status"])
	}
}

func TestIntegration_ProtectedEndpoint_NoAuth(t *testing.T) {
	srv := testAPIServer(t)
	req := httptest.NewRequest("GET", "/api/agents", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestIntegration_ProtectedEndpoint_WithAuth(t *testing.T) {
	srv := testAPIServer(t)
	req := httptest.NewRequest("GET", "/api/agents", nil)
	req.SetBasicAuth("testuser", "testpass")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestIntegration_SecurityHeaders_Present(t *testing.T) {
	srv := testAPIServer(t)
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	headers := []string{
		"X-Content-Type-Options",
		"X-Frame-Options",
		"X-XSS-Protection",
		"Referrer-Policy",
		"Content-Security-Policy",
	}

	for _, h := range headers {
		if rec.Header().Get(h) == "" {
			t.Errorf("missing security header: %s", h)
		}
	}
}

func TestIntegration_RateLimiting_Integration(t *testing.T) {
	srv := testAPIServer(t)
	blocked := false

	for i := 0; i < 20; i++ {
		req := httptest.NewRequest("GET", "/health", nil)
		req.Header.Set("Cf-Connecting-Ip", "integration-rl-test")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code == http.StatusTooManyRequests {
			blocked = true
			break
		}
	}

	if !blocked {
		t.Error("rate limiting did not engage during integration test")
	}
}

func TestIntegration_LargeRequestBody(t *testing.T) {
	srv := testAPIServer(t)

	largeBody := strings.Repeat("x", 1<<20) // 1MB
	req := httptest.NewRequest("POST", "/api/agents", strings.NewReader(largeBody))
	req.SetBasicAuth("testuser", "testpass")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code >= 500 {
		t.Errorf("large request body caused server error: %d", rec.Code)
	}
}
