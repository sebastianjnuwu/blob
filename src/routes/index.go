package routes

import (
	"blob/src/controllers"
	multipart "blob/src/controllers/multipart"
	"blob/src/functions"
	"blob/src/middleware"
	"net/http"
	"strings"
)

func RegisterRoutes(mux *http.ServeMux, limiter *middleware.RateLimiter) {

	// GET / (public)
	mux.HandleFunc("/", GETHandler)

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

	// POST /blob/initiate (private)
	mux.Handle(
		"POST /blob/initiate",
		limiter.Middleware(
			middleware.AuthMiddleware(
				http.HandlerFunc(multipart.InitiateUpload),
			),
		),
	)

	// PUT /blob/{uploadId}/chunk (private)
	mux.Handle(
		"PUT /blob/{uploadId}/chunk",
		limiter.Middleware(
			middleware.AuthMiddleware(
				http.HandlerFunc(multipart.UploadChunk),
			),
		),
	)

	// POST /blob/{uploadId}/complete (private)
	mux.Handle(
		"POST /blob/{uploadId}/complete",
		limiter.Middleware(
			middleware.AuthMiddleware(
				http.HandlerFunc(multipart.CompleteUpload),
			),
		),
	)

	// GET /blob/{uploadId}/status (private)
	mux.Handle(
		"GET /blob/{uploadId}/status",
		limiter.Middleware(
			middleware.AuthMiddleware(
				http.HandlerFunc(multipart.UploadStatus),
			),
		),
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

		// GET /blob/{id}/download (public)
		if strings.HasSuffix(path, "/download") || strings.HasSuffix(path, "/download/") {
			controllers.DownloadBlobController(w, r)
			return
		}

		// GET /blob/{id}/view (public)
		if strings.HasSuffix(path, "/view") || strings.HasSuffix(path, "/view/") {
			controllers.ViewBlobController(w, r)
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

	// GET /metrics (private)
	mux.Handle(
		"GET /metrics",
		limiter.Middleware(middleware.AuthMiddleware(http.HandlerFunc(controllers.BlobMetricsController))),
	)

}
