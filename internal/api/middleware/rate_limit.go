package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type windowEntry struct {
	count   int
	resetAt time.Time
}

// IPRateLimiter is a fixed-window rate limiter keyed by client IP.
// It is safe for concurrent use and cleans up stale entries periodically.
type IPRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*windowEntry
	limit   int
	window  time.Duration
}

// NewIPRateLimiter creates a limiter that allows at most limit requests per
// window duration per IP. A background goroutine cleans up expired entries.
func NewIPRateLimiter(limit int, window time.Duration) *IPRateLimiter {
	rl := &IPRateLimiter{
		entries: make(map[string]*windowEntry),
		limit:   limit,
		window:  window,
	}
	go rl.cleanup()
	return rl
}

// Reset clears all per-IP windows. Intended for tests that share a process-wide
// limiter and need a hermetic starting point between cases.
func (rl *IPRateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.entries = make(map[string]*windowEntry)
}

// Allow reports whether the request from ip should be allowed through.
func (rl *IPRateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	e, ok := rl.entries[ip]
	if !ok || now.After(e.resetAt) {
		rl.entries[ip] = &windowEntry{count: 1, resetAt: now.Add(rl.window)}
		return true
	}
	e.count++
	return e.count <= rl.limit
}

func (rl *IPRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, e := range rl.entries {
			if now.After(e.resetAt) {
				delete(rl.entries, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimit returns middleware that enforces the given IPRateLimiter.
// On limit exceeded it responds 429 with a Retry-After header.
func RateLimit(limiter *IPRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			if !limiter.Allow(ip) {
				w.Header().Set("Retry-After", "60")
				Error(w, r, http.StatusTooManyRequests,
					"RATE_LIMITED", "Too many requests. Please try again later.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
