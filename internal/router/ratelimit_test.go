package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractIP_CloudflareHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Cf-Connecting-Ip", "1.2.3.4")
	req.Header.Set("X-Forwarded-For", "5.6.7.8")

	got := extractIP(req)
	if got != "1.2.3.4" {
		t.Errorf("expected CF IP, got %q", got)
	}
}

func TestExtractIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")

	got := extractIP(req)
	if got != "1.2.3.4" {
		t.Errorf("expected first XFF IP, got %q", got)
	}
}

func TestExtractIP_XForwardedForSingle(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1")

	got := extractIP(req)
	if got != "10.0.0.1" {
		t.Errorf("expected single XFF IP, got %q", got)
	}
}

func TestExtractIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	// httptest sets RemoteAddr to "192.0.2.1:1234"
	got := extractIP(req)
	if got != "192.0.2.1" {
		t.Errorf("expected RemoteAddr without port, got %q", got)
	}
}

func TestRateLimitAPI_AllowsNormalTraffic(t *testing.T) {
	handler := RateLimitAPI(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/agents", nil)
	req.Header.Set("Cf-Connecting-Ip", "rate-test-normal")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for first request, got %d", rec.Code)
	}
}

func TestRateLimitAPI_BlocksExcessiveTraffic(t *testing.T) {
	handler := RateLimitAPI(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust the burst limit (5 for API limiter)
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest("GET", "/api/agents", nil)
		req.Header.Set("Cf-Connecting-Ip", "rate-test-flood")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code == http.StatusTooManyRequests {
			return // Test passed — rate limit kicked in
		}
	}

	t.Error("rate limiter did not trigger after 20 requests")
}

func TestRateLimitAuth_BlocksBruteForce(t *testing.T) {
	handler := RateLimitAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust the burst limit (10 for auth limiter)
	for i := 0; i < 30; i++ {
		req := httptest.NewRequest("POST", "/api/agents", nil)
		req.Header.Set("Cf-Connecting-Ip", "brute-force-test")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code == http.StatusTooManyRequests {
			return // Test passed
		}
	}

	t.Error("auth rate limiter did not trigger after 30 requests")
}

func TestRateLimitAPI_DifferentIPsIndependent(t *testing.T) {
	handler := RateLimitAPI(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Make requests from two different IPs
	for _, ip := range []string{"ip-independent-a", "ip-independent-b"} {
		req := httptest.NewRequest("GET", "/api/agents", nil)
		req.Header.Set("Cf-Connecting-Ip", ip)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("first request from %s should succeed, got %d", ip, rec.Code)
		}
	}
}
