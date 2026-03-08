package multipart

import (
	"blob/src/database"
	"blob/src/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// PUT /blob/{uploadId}/chunk
func UploadChunk(w http.ResponseWriter, r *http.Request) {
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
	if upload.Completed {
		http.Error(w, "Upload already completed", http.StatusBadRequest)
		return
	}
	chunkIdxStr := r.Header.Get("X-Chunk-Index")
	if chunkIdxStr == "" {
		http.Error(w, "Missing X-Chunk-Index header", http.StatusBadRequest)
		return
	}
	var chunkIdx int
	if _, err := fmt.Sscanf(chunkIdxStr, "%d", &chunkIdx); err != nil {
		http.Error(w, "Invalid X-Chunk-Index header", http.StatusBadRequest)
		return
	}
	storagePath := os.Getenv("BLOB_STORAGE_PATH")
	if storagePath == "" {
		storagePath = "storage/uploads"
	}
	tmpDir := filepath.Join(storagePath, "tmp", uploadId)
	chunkPath := filepath.Join(tmpDir, fmt.Sprintf("chunk_%d", chunkIdx))
	f, err := os.Create(chunkPath)
	if err != nil {
		http.Error(w, "Failed to save chunk", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	_, err = io.Copy(f, r.Body)
	if err != nil {
		http.Error(w, "Failed to write chunk", http.StatusInternalServerError)
		return
	}

	var chunks []int
	_ = json.Unmarshal(upload.ChunksDone, &chunks)
	found := false
	for _, c := range chunks {
		if c == chunkIdx {
			found = true
			break
		}
	}

	if !found {
		chunks = append(chunks, chunkIdx)
		newChunks, _ := json.Marshal(chunks)
		database.DB.Model(&upload).Update("chunks_done", datatypes.JSON(newChunks))
	}

	w.WriteHeader(http.StatusNoContent)
}
