package controllers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"blob/src/database"
	"blob/src/models"

	"github.com/google/uuid"
)

// EditBlobController handles POST /blob/{id}
func EditBlobController(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 || parts[2] == "" {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Missing blob id"}); err != nil {
			// Optionally log: fmt.Println("failed to encode edit error json:", err)
		}
		return
	}
	idStr := parts[2]
	id, err := uuid.Parse(idStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Invalid blob id"}); err != nil {
			// Optionally log: fmt.Println("failed to encode edit error json:", err)
		}
		return
	}

	var blob models.Blob
	result := database.DB.First(&blob, "id = ?", id)
	if result.Error != nil {
		w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Blob not found"}); err != nil {
			// Optionally log: fmt.Println("failed to encode edit error json:", err)
		}
		return
	}

	var req struct {
		Metadata  map[string]interface{} `json:"metadata"`
		Public    *bool                  `json:"public"`
		ExpiresAt *string                `json:"expires_at"`
		Filename  *string                `json:"filename"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON body"}); err != nil {
			// Optionally log: fmt.Println("failed to encode edit error json:", err)
		}
		return
	}

	if req.Metadata != nil {
		metaBytes, err := json.Marshal(req.Metadata)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			if err := json.NewEncoder(w).Encode(map[string]string{"error": "Metadata must be valid JSON"}); err != nil {
				// Optionally log: fmt.Println("failed to encode edit error json:", err)
			}
			return
		}
		blob.Metadata = metaBytes
	}
	if req.Public != nil {
		blob.Public = req.Public
	}
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			if err := json.NewEncoder(w).Encode(map[string]string{"error": "expires_at must be RFC3339 date"}); err != nil {
				// Optionally log: fmt.Println("failed to encode edit error json:", err)
			}
			return
		}
		if t.Before(time.Now().UTC()) {
			w.WriteHeader(http.StatusBadRequest)
			if err := json.NewEncoder(w).Encode(map[string]string{"error": "expires_at cannot be in the past"}); err != nil {
				// Optionally log: fmt.Println("failed to encode edit error json:", err)
			}
			return
		}
		blob.ExpiresAt = &t
	}
	if req.Filename != nil && *req.Filename != "" {
		blob.Filename = *req.Filename
	}
	blob.UpdatedAt = time.Now().UTC()

	// Recalculate hash only if blob is private and filename or metadata changed
	if blob.Public != nil && !*blob.Public && (req.Filename != nil || req.Metadata != nil) {
		var hashInput string
		if blob.Filename != "" {
			hashInput += blob.Filename
		}
		if blob.Metadata != nil {
			hashInput += string(blob.Metadata)
		}
		// Add a random salt to ensure uniqueness
		salt := fmt.Sprintf("%d", time.Now().UnixNano())
		hashInput += salt
		sha := sha256.New()
		if _, err := sha.Write([]byte(hashInput)); err != nil {
			// Optionally log: fmt.Println("failed to write hash input:", err)
		}
		blob.Hash = hex.EncodeToString(sha.Sum(nil))
	}

	if err := database.DB.Save(&blob).Error; err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update blob"}); err != nil {
			// Optionally log: fmt.Println("failed to encode edit error json:", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(blob); err != nil {
		// Optionally log: fmt.Println("failed to encode edit blob json:", err)
	}
}
