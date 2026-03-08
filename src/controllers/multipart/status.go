package multipart

import (
	"blob/src/database"
	"blob/src/functions"
	"blob/src/models"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// GET /blob/{uploadId}/status
func UploadStatus(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, "Missing uploadId", http.StatusBadRequest)
		return
	}
	uploadId := parts[2]
	userIDStr := r.Header.Get("X-User-ID")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "Missing or invalid X-User-ID header", http.StatusUnauthorized)
		return
	}
	var upload models.MultipartUpload
	if err := database.DB.First(&upload, "id = ?", uploadId).Error; err != nil {
		http.Error(w, "Invalid uploadId", http.StatusNotFound)
		return
	}
	if upload.UserID != userID {
		http.Error(w, "Forbidden: not your upload", http.StatusForbidden)
		return
	}
	if err := json.NewEncoder(w).Encode(upload); err != nil {
		functions.Error("failed to encode upload status json: %v", err)
	}
}
