package controllers

import (
	"encoding/json"
	"net/http"

	"blob/src/functions"
	"blob/src/lib/logger"
	"blob/src/services"
)

// DeleteBlobController handles DELETE /blob/{id}
func DeleteBlobController(w http.ResponseWriter, r *http.Request) {
	id, ok := parseBlobID(r)
	if !ok {
		functions.WriteJSONError(w, "Invalid blob id", http.StatusBadRequest)
		return
	}

	if err := services.Blobs.Delete(r.Context(), id); err != nil {
		functions.WriteJSONError(w, "Blob not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"message": "Blob deleted successfully"}); err != nil {
		logger.Error("failed to encode delete success json: %v", err)
	}
}
