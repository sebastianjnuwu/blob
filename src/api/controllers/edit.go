package controllers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"blob/src/functions"
	"blob/src/lib/logger"
	"blob/src/services"
)

// EditBlobController handles POST /blob/{id}
func EditBlobController(w http.ResponseWriter, r *http.Request) {
	id, ok := parseBlobID(r)
	if !ok {
		functions.WriteJSONError(w, "Invalid blob id", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req struct {
		Metadata  map[string]interface{} `json:"metadata"`
		Public    *bool                  `json:"public"`
		ExpiresAt *string                `json:"expires_at"`
		Bucket    *string                `json:"bucket"`
		Filename  *string                `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		functions.WriteJSONError(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	input := services.EditInput{Metadata: req.Metadata, Public: req.Public}
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			functions.WriteJSONError(w, "expires_at must be RFC3339 date", http.StatusBadRequest)
			return
		}
		if t.Before(time.Now().UTC()) {
			functions.WriteJSONError(w, "expires_at cannot be in the past", http.StatusBadRequest)
			return
		}
		input.ExpiresAt = &t
	}

	if req.Bucket != nil && *req.Bucket != "" {
		if strings.Contains(*req.Bucket, "..") {
			functions.WriteJSONError(w, "Invalid bucket name", http.StatusBadRequest)
			return
		}
		if !regexp.MustCompile(`^[a-zA-Z0-9/_-]+$`).MatchString(*req.Bucket) {
			functions.WriteJSONError(w, "Bucket can only contain letters, numbers, '/', '-', and '_'", http.StatusBadRequest)
			return
		}
		input.Bucket = req.Bucket
	}
	if req.Filename != nil && *req.Filename != "" {
		if len(*req.Filename) > 255 {
			functions.WriteJSONError(w, "filename must be 1-255 chars", http.StatusBadRequest)
			return
		}
		if strings.Contains(*req.Filename, "..") || strings.ContainsAny(*req.Filename, "/\\") {
			functions.WriteJSONError(w, "Invalid filename", http.StatusBadRequest)
			return
		}
		input.Filename = req.Filename
	}

	blob, err := services.Blobs.Edit(r.Context(), id, input)
	if err != nil {
		functions.WriteJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(blob); err != nil {
		logger.Error("failed to encode edit blob json: %v", err)
	}
}
