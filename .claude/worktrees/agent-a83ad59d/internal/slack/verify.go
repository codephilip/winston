package slack

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

// VerifyMiddleware validates that incoming requests are actually from Slack
// using the signing secret.
func VerifyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signingSecret := os.Getenv("SLACK_SIGNING_SECRET")
		if signingSecret == "" {
			// Skip verification in dev if no secret configured
			next.ServeHTTP(w, r)
			return
		}

		// Read and buffer the body so downstream handlers can re-read it
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		// Check timestamp to prevent replay attacks (5 min window)
		tsStr := r.Header.Get("X-Slack-Request-Timestamp")
		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid timestamp", http.StatusUnauthorized)
			return
		}
		if abs(time.Now().Unix()-ts) > 300 {
			http.Error(w, "request too old", http.StatusUnauthorized)
			return
		}

		// Compute expected signature
		sigBasestring := fmt.Sprintf("v0:%s:%s", tsStr, string(body))
		mac := hmac.New(sha256.New, []byte(signingSecret))
		mac.Write([]byte(sigBasestring))
		expected := fmt.Sprintf("v0=%x", mac.Sum(nil))

		actual := r.Header.Get("X-Slack-Signature")
		if !hmac.Equal([]byte(expected), []byte(actual)) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
