package router

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// auditEntry represents a single audit log line.
type auditEntry struct {
	Timestamp string `json:"timestamp"`
	IP        string `json:"ip"`
	User      string `json:"user"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Status    int    `json:"status"`
	UserAgent string `json:"user_agent"`
}

var auditLogger *log.Logger

func init() {
	logPath := os.Getenv("AUDIT_LOG_PATH")
	if logPath == "" {
		logPath = filepath.Join(os.Getenv("HOME"), "Library", "Logs", "polymr-audit.log")
	}
	f, err := os.OpenFile(logPath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		log.Printf("[audit] WARNING: could not open audit log: %v", err)
		// Fall back to stderr so we don't lose audit events.
		auditLogger = log.New(os.Stderr, "[audit] ", 0)
		return
	}
	auditLogger = log.New(f, "", 0)
}

// statusRecorder wraps http.ResponseWriter to capture the status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// AuditLog is middleware that logs every authenticated request as JSON.
func AuditLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		user, _, _ := r.BasicAuth()
		if user == "" {
			user = "-"
		}

		entry := auditEntry{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			IP:        extractIP(r),
			User:      user,
			Method:    r.Method,
			Path:      r.URL.Path,
			Status:    rec.status,
			UserAgent: r.UserAgent(),
		}

		data, _ := json.Marshal(entry)
		auditLogger.Println(string(data))
	})
}

// LogFailedAuth writes a failed authentication attempt to the audit log.
func LogFailedAuth(r *http.Request, attemptedUser string) {
	entry := auditEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		IP:        extractIP(r),
		User:      attemptedUser,
		Method:    r.Method,
		Path:      r.URL.Path,
		Status:    http.StatusUnauthorized,
		UserAgent: r.UserAgent(),
	}

	data, _ := json.Marshal(entry)
	auditLogger.Println(string(data))
}
