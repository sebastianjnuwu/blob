package multipart

import (
	"blob/src/database"
	"blob/src/functions"
	"blob/src/models"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// POST /blob/initiate
func InitiateUpload(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Bucket   string `json:"bucket"`
		Filename string `json:"filename"`
		Size     int64  `json:"size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Bucket == "" || req.Filename == "" || req.Size <= 0 {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if !validateBucketName(req.Bucket) {
		http.Error(w, "Invalid bucket name", http.StatusBadRequest)
		return
	}
	userIDStr := r.Header.Get("X-User-ID")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "Missing or invalid X-User-ID header", http.StatusUnauthorized)
		return
	}
	upload := models.MultipartUpload{
		UserID:     userID,
		Bucket:     req.Bucket,
		Filename:   req.Filename,
		Size:       req.Size,
		ChunksDone: datatypes.JSON([]byte("[]")),
		Completed:  false,
	}
	if err := database.DB.Create(&upload).Error; err != nil {
		http.Error(w, "Failed to create upload session", http.StatusInternalServerError)
		return
	}
	storagePath := os.Getenv("BLOB_STORAGE_PATH")
	if storagePath == "" {
		storagePath = "storage/uploads"
	}
	tmpDir := filepath.Join(storagePath, "tmp", upload.ID.String())
	if err := os.MkdirAll(tmpDir, 0750); err != nil {
		http.Error(w, "Failed to create temp directory", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(map[string]string{"uploadId": upload.ID.String()}); err != nil {
		functions.Error("failed to encode uploadId json: %v", err)
	}
}
