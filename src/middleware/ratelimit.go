package middleware

import (
	"blob/src/database"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

type RateLimiter struct {
	max    int
	window time.Duration
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func Variables() *RateLimiter {
	max, _ := strconv.Atoi(os.Getenv("BLOB_RATE_LIMIT_MAX"))
	windowMs, _ := strconv.Atoi(os.Getenv("BLOB_RATE_LIMIT_WINDOW_MS"))
	return &RateLimiter{
		max:    max,
		window: time.Duration(windowMs) * time.Millisecond,
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		key := "ratelimit:" + ip
		count, err := database.RedisClient.Incr(database.Ctx, key).Result()
		if err == nil && count == 1 {
			database.RedisClient.PExpire(database.Ctx, key, rl.window)
		}
		if err != nil || int(count) > rl.max {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(ErrorResponse{Error: "rate limit exceeded"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
