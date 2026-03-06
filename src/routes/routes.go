package routes

import (
	"blob/src/middleware"
	"net/http"
)

func RegisterRoutes(mux *http.ServeMux, limiter *middleware.RateLimiter) {

	mux.HandleFunc("GET /health", HealthHandler)

	mux.Handle(
		"GET /",
		limiter.Middleware(http.HandlerFunc(GETHandler)),
	)
}
