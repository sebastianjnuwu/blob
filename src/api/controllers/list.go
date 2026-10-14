package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"blob/src/functions"
	"blob/src/lib/logger"
	"blob/src/repository"
	"blob/src/services"
)

// ListBlobsController handles GET /blob
func ListBlobsController(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(q.Get("page_size"))
	if pageSize < 1 {
		pageSize = 20
	}

	result, err := services.Blobs.List(r.Context(), repository.ListFilter{
		Bucket:   q.Get("bucket"),
		Search:   q.Get("search"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		functions.WriteJSONError(w, "Failed to list blobs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		logger.Error("failed to encode list json: %v", err)
	}
}
