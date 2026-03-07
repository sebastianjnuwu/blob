package routes

import (
	"blob/src/controllers"
	"blob/src/functions"
	"blob/src/middleware"
	"net/http"
)

func RegisterRoutes(mux *http.ServeMux, limiter *middleware.RateLimiter) {

	// Default handler for undefined routes
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		functions.WriteJSONMethodNotAllowed(w)
	})

	// GET /health (public)
	mux.Handle(
		"GET /health",
		limiter.Middleware(http.HandlerFunc(HealthHandler)),
	)

	// GET / (public)
	mux.Handle(
		"GET /",
		limiter.Middleware(http.HandlerFunc(GETHandler)),
	)

	// PUT /blob (private)
	mux.Handle(
		"PUT /blob",
		limiter.Middleware(middleware.AuthMiddleware(http.HandlerFunc(controllers.UploadBlobController))),
	)

	// GET /blob (private)
	mux.Handle(
		"GET /blob",
		limiter.Middleware(middleware.AuthMiddleware(http.HandlerFunc(controllers.ListBlobsController))),
	)
}
