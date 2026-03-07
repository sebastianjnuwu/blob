package routes

import (
	"blob/src/controllers"
	"blob/src/functions"
	"blob/src/middleware"
	"net/http"
	"strings"
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

	// GET /blob/{id} (private)
	mux.Handle(
		"GET /blob/{id}",
		limiter.Middleware(middleware.AuthMiddleware(http.HandlerFunc(controllers.GetBlobController))),
	)
	// Custom handler for /blob/* to support dynamic download route
	mux.HandleFunc("/blob/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/download") || strings.HasSuffix(path, "/download/") {
			controllers.DownloadBlobController(w, r)
			return
		}
		// fallback: method not allowed or not found
		functions.WriteJSONMethodNotAllowed(w)
	})
	// POST /blob/{id} (private)
	mux.Handle(
		"POST /blob/{id}",
		limiter.Middleware(middleware.AuthMiddleware(http.HandlerFunc(controllers.EditBlobController))),
	)
	// DELETE /blob/{id} (private)
	mux.Handle(
		"DELETE /blob/{id}",
		limiter.Middleware(middleware.AuthMiddleware(http.HandlerFunc(controllers.DeleteBlobController))),
	)
}
