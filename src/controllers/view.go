package controllers

import (
	"net/http"
	"strings"

	"blob/src/functions"
)

// ViewBlobController handles GET /blob/{id}/view
func ViewBlobController(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 || parts[2] != "view" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte(`{"error":"Invalid view URL"}`)); err != nil {
			functions.Error("failed to write error: %v", err)
		}
		return
	}

	serveBlobFile(w, r, "inline")
}