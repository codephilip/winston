package router

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"os"
)

// BasicAuth middleware protects routes with username/password.
// Credentials are read from POLYMR_USER and POLYMR_PASS env vars.
func BasicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedUser := os.Getenv("POLYMR_USER")
		expectedPass := os.Getenv("POLYMR_PASS")

		if expectedUser == "" || expectedPass == "" {
			// No auth configured — block everything
			http.Error(w, "Auth not configured", http.StatusForbidden)
			return
		}

		user, pass, ok := r.BasicAuth()
		if !ok || !secureCompare(user, expectedUser) || !secureCompare(pass, expectedPass) {
			attemptedUser := user
			if attemptedUser == "" {
				attemptedUser = "[no-user]"
			}
			LogFailedAuth(r, attemptedUser)
			w.Header().Set("WWW-Authenticate", `Basic realm="Winston"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func secureCompare(a, b string) bool {
	ha := sha256.Sum256([]byte(a))
	hb := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ha[:], hb[:]) == 1
}
