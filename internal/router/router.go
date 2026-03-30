package router

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/polymr/polymr/internal/agents"
	"github.com/polymr/polymr/internal/kali"
	"github.com/polymr/polymr/internal/notify"
	"github.com/polymr/polymr/internal/slack"
	"github.com/polymr/polymr/internal/voice"
)

var startTime = time.Now()

// SecurityHeaders adds standard security headers to every response.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'")
		next.ServeHTTP(w, r)
	})
}

func New() http.Handler {
	manager := agents.NewManager()
	manager.SlackPost = slack.PostMessage
	manager.SlackPostTS = slack.PostMessageTS
	manager.SlackThreadReply = slack.PostThreadReply
	voiceClient := voice.NewClient()

	// API router (personal-api.polymr.io)
	api := chi.NewRouter()
	api.Use(middleware.Logger)
	api.Use(middleware.Recoverer)
	api.Use(SecurityHeaders)
	api.Use(RateLimitAPI)
	api.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://personal.polymr.io"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	// Slack endpoints (verified by signing secret, no basic auth)
	api.Route("/slack", func(r chi.Router) {
		r.Use(slack.VerifyMiddleware)
		r.Post("/commands", slack.HandleSlashCommand(manager))
		r.Post("/events", slack.HandleEvent(manager))
		r.Post("/interactions", slack.HandleInteraction(manager))
	})

	// Health check handler (shared between root and /api)
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		agentCount, sessionCount, scheduleCount := manager.Status()

		frontendStatus := "ok"
		conn, err := net.DialTimeout("tcp", "localhost:3000", 2*time.Second)
		if err != nil {
			frontendStatus = "unreachable"
		} else {
			conn.Close()
		}

		uptime := time.Since(startTime).Truncate(time.Second)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":           "ok",
			"uptime":           uptime.String(),
			"frontend":         frontendStatus,
			"agents":           agentCount,
			"active_sessions":  sessionCount,
			"active_schedules": scheduleCount,
		})
	}

	api.Get("/health", healthHandler)

	// Protected API endpoints
	api.Route("/api", func(r chi.Router) {
		r.Use(RateLimitAuth)
		r.Use(AuditLog)
		r.Use(BasicAuth)
		r.Get("/health", healthHandler)
		r.Get("/agents", manager.ListAgents)
		r.Get("/agents/{agent}", manager.GetAgent)
		r.Post("/agents/{agent}/run", manager.RunAgent)
		r.Put("/agents/{agent}/model", handleModelUpdate(manager))
		r.Put("/agents/{agent}/prompt", handlePromptUpdate(manager))
		r.Get("/agents/{agent}/sessions/{session}", manager.GetSession)
		r.Post("/agents/{agent}/sessions/{session}/message", manager.SendMessage)
		r.Get("/schedules", manager.ListSchedules)
		r.Post("/schedules", manager.CreateSchedule)
		r.Delete("/schedules/{id}", manager.DeleteSchedule)
		r.Post("/voice/transcribe", handleVoiceTranscribe(voiceClient))
		r.Post("/voice/synthesize", handleVoiceSynthesize(voiceClient))
		r.Get("/kali/status", handleKaliStatus)
	})

	// Frontend proxy (personal.polymr.io → Next.js on :3000, behind basic auth)
	nextjsURL, _ := url.Parse("http://localhost:3000")
	frontendProxy := notify.WrapFrontendProxy(httputil.NewSingleHostReverseProxy(nextjsURL))

	// Host-based routing: split traffic by hostname
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Host {
		case "personal.polymr.io":
			// Static assets don't need auth or rate limiting
			if strings.HasPrefix(r.URL.Path, "/_next/") ||
				strings.HasPrefix(r.URL.Path, "/__nextjs") ||
				r.URL.Path == "/favicon.ico" {
				frontendProxy.ServeHTTP(w, r)
				return
			}
			// API requests → forward directly to api router (has its own middleware)
			if strings.HasPrefix(r.URL.Path, "/api") {
				api.ServeHTTP(w, r)
				return
			}
			// Frontend pages → auth only (separate from API rate limits)
			AuditLog(BasicAuth(frontendProxy)).ServeHTTP(w, r)
		default:
			// personal-api.polymr.io and localhost go to the API router
			api.ServeHTTP(w, r)
		}
	})
}

func handleVoiceTranscribe(vc *voice.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !vc.IsConfigured() {
			http.Error(w, `{"error":"ELEVENLABS_API_KEY not configured"}`, http.StatusServiceUnavailable)
			return
		}

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, `{"error":"invalid multipart form"}`, http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("audio")
		if err != nil {
			http.Error(w, `{"error":"missing audio file"}`, http.StatusBadRequest)
			return
		}
		defer file.Close()

		audioBytes, err := io.ReadAll(file)
		if err != nil {
			http.Error(w, `{"error":"failed to read audio"}`, http.StatusInternalServerError)
			return
		}

		text, err := vc.SpeechToText(audioBytes, header.Filename)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"text": text})
	}
}

func handleVoiceSynthesize(vc *voice.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !vc.IsConfigured() {
			http.Error(w, `{"error":"ELEVENLABS_API_KEY not configured"}`, http.StatusServiceUnavailable)
			return
		}

		var req struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
			return
		}

		text := req.Text
		if len(text) > 1000 {
			text = text[:1000] + "... response truncated for audio."
		}

		audioBytes, err := vc.TextToSpeech(text)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(audioBytes)))
		w.Write(audioBytes)
	}
}

// handleModelUpdate wraps the model update endpoint with Slack notification and service restart.
func handleModelUpdate(manager *agents.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Capture response to check if the model actually changed
		rec := &responseBuffer{header: http.Header{}, code: http.StatusOK}
		manager.UpdateAgentModel(rec, r)

		// Copy response to the real writer
		for k, v := range rec.header {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.code)
		w.Write(rec.body)

		// If model changed, notify and restart
		var result struct {
			Agent    string `json:"agent"`
			OldModel string `json:"old_model"`
			NewModel string `json:"new_model"`
			Changed  bool   `json:"changed"`
		}
		if json.Unmarshal(rec.body, &result) == nil && result.Changed {
			notify.ModelChange(result.Agent, result.OldModel, result.NewModel)

			// Restart services asynchronously so the response is sent first
			go func() {
				time.Sleep(1 * time.Second)
				restartServices()
			}()
		}
	}
}

// handlePromptUpdate wraps the prompt update endpoint with Slack notification and restart.
func handlePromptUpdate(manager *agents.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID := chi.URLParam(r, "agent")
		rec := &responseBuffer{header: http.Header{}, code: http.StatusOK}
		manager.UpdateAgentPrompt(rec, r)

		for k, v := range rec.header {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.code)
		w.Write(rec.body)

		var result struct {
			Restart bool `json:"restart"`
		}
		if json.Unmarshal(rec.body, &result) == nil && result.Restart {
			notify.PromptChange(agentID)
			go func() {
				time.Sleep(1 * time.Second)
				restartServices()
			}()
		}
	}
}

// responseBuffer captures an HTTP response for inspection before forwarding.
type responseBuffer struct {
	header http.Header
	code   int
	body   []byte
}

func (rb *responseBuffer) Header() http.Header       { return rb.header }
func (rb *responseBuffer) WriteHeader(code int)       { rb.code = code }
func (rb *responseBuffer) Write(b []byte) (int, error) { rb.body = append(rb.body, b...); return len(b), nil }

// restartServices rebuilds and restarts the Go router and Next.js frontend via launchctl.
// The script is detached into its own process group so it survives this process being killed
// (restart.sh does launchctl bootout on this very service).
func restartServices() {
	projectDir := filepath.Dir(filepath.Dir(os.Args[0])) // bin/polymr → project root
	// Fall back to well-known project path if binary isn't in bin/
	if _, err := os.Stat(filepath.Join(projectDir, "go.mod")); err != nil {
		home := os.Getenv("HOME")
		projectDir = filepath.Join(home, "projects", "winston")
	}

	script := filepath.Join(projectDir, "scripts", "restart.sh")
	log.Printf("[router] triggering service restart via %s", script)

	// Run detached: own process group + nohup so it survives parent death
	cmd := exec.Command("nohup", "bash", script)
	cmd.Dir = projectDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Send output to log file instead of pipes (which break when parent dies)
	logFile, err := os.OpenFile(
		filepath.Join(os.Getenv("HOME"), "Library", "Logs", "winston-restart.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644,
	)
	if err == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		defer logFile.Close()
	}

	if err := cmd.Start(); err != nil {
		log.Printf("[router] restart failed to start: %v", err)
		return
	}
	log.Printf("[router] restart script launched (pid %d), this process will be replaced", cmd.Process.Pid)
	// Do NOT wait — the script will kill us via launchctl bootout
}

func handleKaliStatus(w http.ResponseWriter, r *http.Request) {
	status, _ := kali.CheckConnectivity()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
