package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

type rateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rlEntry
}

type rlEntry struct {
	count     int
	resetAt   time.Time
}

var loginLimiter = &rateLimiter{entries: make(map[string]*rlEntry)}

func RateLimit(maxAttempts int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			if idx := strings.LastIndex(ip, ":"); idx != -1 {
				ip = ip[:idx]
			}

			loginLimiter.mu.Lock()
			e, ok := loginLimiter.entries[ip]
			if !ok || time.Now().After(e.resetAt) {
				e = &rlEntry{count: 0, resetAt: time.Now().Add(window)}
				loginLimiter.entries[ip] = e
			}
			e.count++
			blocked := e.count > maxAttempts
			loginLimiter.mu.Unlock()

			if blocked {
				http.Error(w, `{"error":"demasiados intentos, espera un momento"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
