package controllers

import (
	"encoding/json"
	"net/http"
	"strings"

	"blob/src/database"
	"blob/src/models"

	"github.com/google/uuid"
)

// GetBlobController handles GET /blob/{id}
func GetBlobController(w http.ResponseWriter, r *http.Request) {

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 || parts[2] == "" {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Missing blob id"}); err != nil {
			// Optionally log: fmt.Println("failed to encode get error json:", err)
		}
		return
	}
	idStr := parts[2]
	id, err := uuid.Parse(idStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Invalid blob id"}); err != nil {
			// Optionally log: fmt.Println("failed to encode get error json:", err)
		}
		return
	}

	var blob models.Blob
	result := database.DB.First(&blob, "id = ?", id)
	if result.Error != nil {
		w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Blob not found"}); err != nil {
			// Optionally log: fmt.Println("failed to encode get error json:", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(blob); err != nil {
		// Optionally log: fmt.Println("failed to encode blob json:", err)
	}
}
