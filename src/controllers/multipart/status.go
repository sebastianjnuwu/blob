package multipart

import (
	"blob/src/database"
	"blob/src/functions"
	"blob/src/models"
	"encoding/json"
	"net/http"
	"strconv"
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
	// Calculate missing_chunks
	var chunksDone []int
	_ = json.Unmarshal(upload.ChunksDone, &chunksDone)
	chunkMap := make(map[int]bool)
	for _, idx := range chunksDone {
		chunkMap[idx] = true
	}
	chunkSize := 102400 // default 100 KB
	if cs := r.URL.Query().Get("chunk_size"); cs != "" {
		// Permite override via query param opcional
		if v, err := strconv.Atoi(cs); err == nil && v > 0 {
			chunkSize = v
		}
	}
	totalChunks := int((upload.Size + int64(chunkSize) - 1) / int64(chunkSize))
	var missingChunks []int
	for i := 0; i < totalChunks; i++ {
		if !chunkMap[i] {
			missingChunks = append(missingChunks, i)
		}
	}
	resp := map[string]interface{}{
		"upload":         upload,
		"missing_chunks": missingChunks,
		"chunk_size":     chunkSize,
		"total_chunks":   totalChunks,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		functions.Error("failed to encode upload status json: %v", err)
	}
}
