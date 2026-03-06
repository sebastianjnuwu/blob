package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

type RateLimiter struct {
	mu      sync.Mutex
	clients map[string]*client
	max     int
	window  time.Duration
}

type client struct {
	count     int
	expiresAt time.Time
}

func NewRateLimiterFromEnv() *RateLimiter {
	maxStr := os.Getenv("BLOB_RATE_LIMIT_MAX")
	windowStr := os.Getenv("BLOB_RATE_LIMIT_WINDOW_MS")

	max, _ := strconv.Atoi(maxStr)
	windowMs, _ := strconv.Atoi(windowStr)

	return &RateLimiter{
		clients: make(map[string]*client),
		max:     max,
		window:  time.Duration(windowMs) * time.Millisecond,
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)

		rl.mu.Lock()
		c, exists := rl.clients[ip]

		if !exists || time.Now().After(c.expiresAt) {
			c = &client{
				count:     0,
				expiresAt: time.Now().Add(rl.window),
			}
			rl.clients[ip] = c
		}

		c.count++
		if c.count > rl.max {
			rl.mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(ErrorResponse{
				Error: "rate limit exceeded",
			})
			return
		}

		rl.mu.Unlock()
		next.ServeHTTP(w, r)
	})
}
