package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// --- Auth bypass tests ---

func TestSecurity_AuthBypass_EmptyAuthHeader(t *testing.T) {
	os.Setenv("POLYMR_USER", "admin")
	os.Setenv("POLYMR_PASS", "secret")
	defer os.Unsetenv("POLYMR_USER")
	defer os.Unsetenv("POLYMR_PASS")

	handler := BasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/agents", nil)
	req.Header.Set("Authorization", "")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("empty auth header should be rejected, got %d", rec.Code)
	}
}

func TestSecurity_AuthBypass_MalformedBasicAuth(t *testing.T) {
	os.Setenv("POLYMR_USER", "admin")
	os.Setenv("POLYMR_PASS", "secret")
	defer os.Unsetenv("POLYMR_USER")
	defer os.Unsetenv("POLYMR_PASS")

	handler := BasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	malformed := []string{
		"Basic",
		"Basic ",
		"Basic notbase64!!!",
		"Bearer token123",
		"Digest username=admin",
	}

	for _, auth := range malformed {
		req := httptest.NewRequest("GET", "/api/agents", nil)
		req.Header.Set("Authorization", auth)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code == http.StatusOK {
			t.Errorf("malformed auth %q should not grant access", auth)
		}
	}
}

func TestSecurity_AuthBypass_SQLInjectionInCredentials(t *testing.T) {
	os.Setenv("POLYMR_USER", "admin")
	os.Setenv("POLYMR_PASS", "secret")
	defer os.Unsetenv("POLYMR_USER")
	defer os.Unsetenv("POLYMR_PASS")

	handler := BasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	payloads := []struct{ user, pass string }{
		{"admin' OR '1'='1", "anything"},
		{"admin", "' OR '1'='1"},
		{"admin' --", "secret"},
		{"admin", "secret' UNION SELECT * FROM users --"},
	}

	for _, p := range payloads {
		req := httptest.NewRequest("GET", "/api/agents", nil)
		req.SetBasicAuth(p.user, p.pass)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code == http.StatusOK {
			t.Errorf("SQL injection attempt should fail: user=%q pass=%q", p.user, p.pass)
		}
	}
}

// --- IP spoofing tests ---

func TestSecurity_IPSpoofing_CfConnectingIpPriority(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Cf-Connecting-Ip", "1.1.1.1")
	req.Header.Set("X-Forwarded-For", "2.2.2.2")

	ip := extractIP(req)
	if ip != "1.1.1.1" {
		t.Errorf("CF header should take priority, got %q", ip)
	}
}

func TestSecurity_IPSpoofing_XForwardedForChain(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 172.16.0.1, 192.168.0.1")

	ip := extractIP(req)
	if ip != "10.0.0.1" {
		t.Errorf("should extract first IP from XFF chain, got %q", ip)
	}
}

// --- Rate limit bypass tests ---

func TestSecurity_RateLimit_CannotBypassWithHeaders(t *testing.T) {
	handler := RateLimitAPI(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	blocked := false
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Cf-Connecting-Ip", "attacker-ip-test")
		req.Header.Set("X-Forwarded-For", "fake-"+string(rune('a'+i)))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code == http.StatusTooManyRequests {
			blocked = true
			break
		}
	}

	if !blocked {
		t.Error("rate limiter should not be bypassed by XFF when CF header is present")
	}
}

// --- Security header tests ---

func TestSecurity_ClickjackingProtection(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	xfo := rec.Header().Get("X-Frame-Options")
	if xfo != "DENY" {
		t.Errorf("X-Frame-Options should be DENY, got %q", xfo)
	}
}

func TestSecurity_ContentTypeSniffing(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	xcto := rec.Header().Get("X-Content-Type-Options")
	if xcto != "nosniff" {
		t.Errorf("X-Content-Type-Options should be 'nosniff', got %q", xcto)
	}
}

func TestSecurity_CSPPresent(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("Content-Security-Policy header should be set")
	}
}

// --- Brute force protection ---

func TestSecurity_BruteForce_RateLimited(t *testing.T) {
	os.Setenv("POLYMR_USER", "admin")
	os.Setenv("POLYMR_PASS", "strong-password-here")
	defer os.Unsetenv("POLYMR_USER")
	defer os.Unsetenv("POLYMR_PASS")

	handler := RateLimitAuth(BasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	blocked := false
	for i := 0; i < 30; i++ {
		req := httptest.NewRequest("GET", "/api/agents", nil)
		req.SetBasicAuth("admin", "wrong-guess")
		req.Header.Set("Cf-Connecting-Ip", "brute-force-attacker")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code == http.StatusTooManyRequests {
			blocked = true
			break
		}
	}

	if !blocked {
		t.Error("brute force attack should be rate limited")
	}
}
