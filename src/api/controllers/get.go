package controllers

import (
	"encoding/json"
	"net/http"

	"blob/src/functions"
	"blob/src/lib/logger"
	"blob/src/services"
)

// GetBlobController handles GET /blob/{id}
func GetBlobController(w http.ResponseWriter, r *http.Request) {
	id, ok := parseBlobID(r)
	if !ok {
		functions.WriteJSONError(w, "Invalid blob id", http.StatusBadRequest)
		return
	}

	blob, err := services.Blobs.Get(r.Context(), id)
	if err != nil {
		functions.WriteJSONError(w, "Blob not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(blob); err != nil {
		logger.Error("failed to encode blob json: %v", err)
	}
}
