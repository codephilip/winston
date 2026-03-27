package router

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/polymr/polymr/internal/agents"
	"github.com/polymr/polymr/internal/kali"
	"github.com/polymr/polymr/internal/slack"
	"github.com/polymr/polymr/internal/voice"
)

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

	api.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Slack endpoints (verified by signing secret, no basic auth)
	api.Route("/slack", func(r chi.Router) {
		r.Use(slack.VerifyMiddleware)
		r.Post("/commands", slack.HandleSlashCommand(manager))
		r.Post("/events", slack.HandleEvent(manager))
		r.Post("/interactions", slack.HandleInteraction(manager))
	})

	// Protected API endpoints
	api.Route("/api", func(r chi.Router) {
		r.Use(RateLimitAuth)
		r.Use(AuditLog)
		r.Use(BasicAuth)
		r.Get("/agents", manager.ListAgents)
		r.Post("/agents/{agent}/run", manager.RunAgent)
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
	frontendProxy := httputil.NewSingleHostReverseProxy(nextjsURL)

	// Host-based routing: split traffic by hostname
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Host {
		case "personal.polymr.io":
			// All requests to personal.polymr.io require rate limiting + auth
			RateLimitAPI(RateLimitAuth(AuditLog(BasicAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Serve API routes directly, proxy everything else to Next.js
				if len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api" {
					api.ServeHTTP(w, r)
				} else {
					frontendProxy.ServeHTTP(w, r)
				}
			}))))).ServeHTTP(w, r)
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

func handleKaliStatus(w http.ResponseWriter, r *http.Request) {
	status, _ := kali.CheckConnectivity()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
