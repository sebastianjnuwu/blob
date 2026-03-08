package controllers

import (
	"encoding/json"
	"net/http"
	"strings"

	"blob/src/database"
	"blob/src/functions"
	"blob/src/models"

	"github.com/google/uuid"
)

// GetBlobController handles GET /blob/{id}
func GetBlobController(w http.ResponseWriter, r *http.Request) {

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 || parts[2] == "" {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Missing blob id"}); err != nil {
			functions.Error("failed to encode get error json: %v", err)
		}
		return
	}
	idStr := parts[2]
	id, err := uuid.Parse(idStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Invalid blob id"}); err != nil {
			functions.Error("failed to encode get error json: %v", err)
		}
		return
	}

	var blob models.Blob
	result := database.DB.First(&blob, "id = ?", id)
	if result.Error != nil {
		w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Blob not found"}); err != nil {
			functions.Error("failed to encode get error json: %v", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(blob); err != nil {
		functions.Error("failed to encode blob json: %v", err)
	}
}
