package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	expected := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":       "DENY",
		"X-XSS-Protection":      "1; mode=block",
		"Referrer-Policy":       "strict-origin-when-cross-origin",
	}

	for header, value := range expected {
		got := rec.Header().Get(header)
		if got != value {
			t.Errorf("header %s: expected %q, got %q", header, value, got)
		}
	}

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("expected Content-Security-Policy header")
	}
}

func TestStatusRecorder_CapturesStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rec, status: http.StatusOK}

	sr.WriteHeader(http.StatusNotFound)
	if sr.status != http.StatusNotFound {
		t.Errorf("expected 404, got %d", sr.status)
	}
}

func TestStatusRecorder_DefaultsToOK(t *testing.T) {
	rec := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rec, status: http.StatusOK}

	if sr.status != http.StatusOK {
		t.Errorf("expected default 200, got %d", sr.status)
	}
}
