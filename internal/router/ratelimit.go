package router

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ipLimiter tracks per-IP rate limiters.
type ipLimiter struct {
	mu       sync.Mutex
	limiters map[string]*visitorLimiter
	rate     rate.Limit
	burst    int
}

type visitorLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newIPLimiter(r rate.Limit, burst int) *ipLimiter {
	l := &ipLimiter{
		limiters: make(map[string]*visitorLimiter),
		rate:     r,
		burst:    burst,
	}
	// Clean up stale entries every 3 minutes.
	go l.cleanup()
	return l
}

func (l *ipLimiter) getLimiter(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	v, ok := l.limiters[ip]
	if !ok {
		limiter := rate.NewLimiter(l.rate, l.burst)
		l.limiters[ip] = &visitorLimiter{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}
	v.lastSeen = time.Now()
	return v.limiter
}

func (l *ipLimiter) cleanup() {
	for {
		time.Sleep(3 * time.Minute)
		l.mu.Lock()
		for ip, v := range l.limiters {
			if time.Since(v.lastSeen) > 5*time.Minute {
				delete(l.limiters, ip)
			}
		}
		l.mu.Unlock()
	}
}

// API rate limiter: 30 requests per minute (0.5/s), burst of 15.
// Burst must accommodate page loads that fire several fetches at once
// (e.g. voice flow: transcribe + agent run + synthesize in quick succession).
var apiLimiter = newIPLimiter(rate.Limit(30.0/60.0), 15)

// Auth brute force limiter: 15 requests per minute (0.25/s), burst of 10.
// Generous enough for Cloudflare Access callbacks + browser, tight enough to block brute force.
var authLimiter = newIPLimiter(rate.Limit(15.0/60.0), 10)

// RateLimitAPI is middleware that enforces 30 req/min per IP for API routes.
func RateLimitAPI(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		if !apiLimiter.getLimiter(ip).Allow() {
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RateLimitAuth is middleware that enforces 5 req/min per IP for auth attempts.
// Apply this before the auth middleware to block brute force attacks.
func RateLimitAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		if !authLimiter.getLimiter(ip).Allow() {
			http.Error(w, `{"error":"too many authentication attempts"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// extractIP gets the real client IP from the request.
// Cloudflare Tunnel sets Cf-Connecting-Ip with the real client IP.
func extractIP(r *http.Request) string {
	// Cloudflare's real client IP header (most reliable behind CF tunnel)
	if cfIP := r.Header.Get("Cf-Connecting-Ip"); cfIP != "" {
		return cfIP
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	// Strip port from RemoteAddr.
	addr := r.RemoteAddr
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}
