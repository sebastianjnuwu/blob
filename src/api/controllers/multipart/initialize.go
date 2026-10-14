package multipart

import (
	"encoding/json"
	"net/http"
	"strings"

	"blob/src/api/validators"
	"blob/src/auth"
	"blob/src/config"
	"blob/src/functions"
	"blob/src/lib/logger"
	"blob/src/services"

	"github.com/google/uuid"
)

// InitiateUpload handles POST /blob/initiate
func InitiateUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req struct {
		Bucket   string `json:"bucket"`
		Filename string `json:"filename"`
		Size     int64  `json:"size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Bucket == "" || req.Size <= 0 {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if !validators.ValidateMultipartBucket(req.Bucket) {
		http.Error(w, "Invalid bucket name", http.StatusBadRequest)
		return
	}
	if req.Filename == "" || len(req.Filename) > 255 || strings.Contains(req.Filename, "..") || strings.ContainsAny(req.Filename, "/\\") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	cfg := config.Env
	if cfg == nil {
		cfg = config.Load()
	}
	if cfg.MaxUploadSize > 0 && req.Size > cfg.MaxUploadSize {
		http.Error(w, "File too large", http.StatusRequestEntityTooLarge)
		return
	}
	if cfg.MaxStorageSize > 0 {
		total, err := functions.GetTotalStorageSize(cfg.ResolveStoragePath())
		if err == nil && total+req.Size > cfg.MaxStorageSize {
			http.Error(w, "Storage limit exceeded", http.StatusInsufficientStorage)
			return
		}
	}

	userID := auth.UserIDFromRequest(r)
	if userID == uuid.Nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	upload, err := services.Multipart.Initiate(r.Context(), services.InitiateInput{
		UserID:   userID,
		Bucket:   req.Bucket,
		Filename: req.Filename,
		Size:     req.Size,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(map[string]string{"uploadId": upload.ID.String()}); err != nil {
		logger.Error("failed to encode uploadId json: %v", err)
	}
}
