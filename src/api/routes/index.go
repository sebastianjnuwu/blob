package routes

import (
	"blob/src/api/controllers"
	multipart "blob/src/api/controllers/multipart"
	"blob/src/api/middleware"
	"blob/src/auth"
	"blob/src/functions"
	"net/http"
	"strings"
)

func RegisterRoutes(mux *http.ServeMux, limiter *middleware.RateLimiter) {

	// helper para forçar método HTTP
	methodHandler := func(method string, h http.Handler) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Method != method {
				functions.WriteJSONMethodNotAllowed(w)
				return
			}
			h.ServeHTTP(w, r)
		}
	}

	// GET / (public)
	mux.HandleFunc("/", GETHandler)

	// GET /health (public)
	mux.Handle(
		"/health",
		methodHandler("GET", limiter.Middleware(http.HandlerFunc(HealthHandler))),
	)

	// GET /version (public)
	mux.Handle(
		"/version",
		methodHandler("GET", limiter.Middleware(http.HandlerFunc(VersionHandler))),
	)

	// POST /blob/initiate (private, write)
	mux.Handle(
		"/blob/initiate",
		methodHandler("POST", limiter.Middleware(
			middleware.RequirePermission(
				http.HandlerFunc(multipart.InitiateUpload),
				auth.PermWrite,
			),
		)),
	)

	// /blob (private) - supports GET (list) and PUT (upload)
	mux.Handle("/blob", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PUT":
			limiter.Middleware(middleware.RequirePermission(http.HandlerFunc(controllers.UploadBlobController), auth.PermWrite)).ServeHTTP(w, r)
			return
		case "GET":
			limiter.Middleware(middleware.RequirePermission(http.HandlerFunc(controllers.ListBlobsController), auth.PermRead)).ServeHTTP(w, r)
			return
		default:
			functions.WriteJSONMethodNotAllowed(w)
			return
		}
	}))

	// Unified handler for dynamic /blob/* routes (download, view, get/edit/delete, sign, multipart)
	mux.HandleFunc("/blob/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// GET /blob/{id}/download (public, signed URL/hash/token authorized)
		if strings.HasSuffix(path, "/download") || strings.HasSuffix(path, "/download/") {
			limiter.Middleware(http.HandlerFunc(controllers.DownloadBlobController)).ServeHTTP(w, r)
			return
		}

		// GET /blob/{id}/view (public, signed URL/hash/token authorized)
		if strings.HasSuffix(path, "/view") || strings.HasSuffix(path, "/view/") {
			limiter.Middleware(http.HandlerFunc(controllers.ViewBlobController)).ServeHTTP(w, r)
			return
		}

		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 2 || parts[0] != "blob" {
			functions.WriteJSONMethodNotAllowed(w)
			return
		}

		// multipart + sign routes: /blob/{uploadId}/{chunk|complete|status|sign}
		if len(parts) >= 3 {
			uploadAction := parts[2]
			switch uploadAction {
			case "chunk":
				if r.Method == "PUT" {
					limiter.Middleware(middleware.RequirePermission(http.HandlerFunc(multipart.UploadChunk), auth.PermWrite)).ServeHTTP(w, r)
					return
				}
			case "complete":
				if r.Method == "POST" {
					limiter.Middleware(middleware.RequirePermission(http.HandlerFunc(multipart.CompleteUpload), auth.PermWrite)).ServeHTTP(w, r)
					return
				}
			case "status":
				if r.Method == "GET" {
					limiter.Middleware(middleware.RequirePermission(http.HandlerFunc(multipart.UploadStatus), auth.PermRead)).ServeHTTP(w, r)
					return
				}
			case "sign":
				if r.Method == "POST" {
					limiter.Middleware(middleware.RequirePermission(http.HandlerFunc(controllers.SignBlobController), auth.PermRead)).ServeHTTP(w, r)
					return
				}
			}
		}

		// id-based operations: /blob/{id}
		if len(parts) >= 2 {
			if len(parts) == 2 || (len(parts) == 3 && parts[2] == "") {
				switch r.Method {
				case "GET":
					limiter.Middleware(middleware.RequirePermission(http.HandlerFunc(controllers.GetBlobController), auth.PermRead)).ServeHTTP(w, r)
					return
				case "POST":
					limiter.Middleware(middleware.RequirePermission(http.HandlerFunc(controllers.EditBlobController), auth.PermWrite)).ServeHTTP(w, r)
					return
				case "DELETE":
					limiter.Middleware(middleware.RequirePermission(http.HandlerFunc(controllers.DeleteBlobController), auth.PermDelete)).ServeHTTP(w, r)
					return
				}
			}
		}

		// fallback: method not allowed or not found
		functions.WriteJSONMethodNotAllowed(w)
	})

	// GET /metrics (private, read)
	mux.Handle(
		"/metrics",
		methodHandler("GET", limiter.Middleware(middleware.RequirePermission(http.HandlerFunc(controllers.BlobMetricsController), auth.PermRead))),
	)

}
