package controllers

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"blob/src/api/validators"
	"blob/src/config"
	"blob/src/functions"
	"blob/src/lib/logger"
	"blob/src/metrics"
	"blob/src/services"
)

// UploadBlobController handles PUT /blob
func UploadBlobController(w http.ResponseWriter, r *http.Request) {

	cfg := config.Env
	if cfg == nil {
		cfg = config.Load()
	}

	r.Body = http.MaxBytesReader(w, r.Body, cfg.MaxChunkSize)

	fields := validators.UploadFields{
		Bucket:    r.FormValue("bucket"),
		Filename:  r.FormValue("filename"),
		Public:    r.FormValue("public"),
		ExpiresAt: r.FormValue("expires_at"),
		Metadata:  r.FormValue("metadata"),
	}

	if errs := validators.ValidateUploadFields(fields); len(errs) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"error":  "Validation failed",
			"fields": errs,
		}); err != nil {
			logger.Error("failed to encode validation error json: %v", err)
		}
		return
	}

	cleanBucket := filepath.Clean(fields.Bucket)
	if fields.Bucket == "" ||
		filepath.IsAbs(fields.Bucket) ||
		strings.Contains(fields.Bucket, "\\") ||
		cleanBucket == "." ||
		cleanBucket == ".." ||
		strings.HasPrefix(cleanBucket, "../") ||
		strings.Contains(cleanBucket, "/../") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Invalid bucket",
		}); err != nil {
			functions.Error("failed to encode invalid bucket json: %v", err)
		}
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		functions.WriteJSONError(w, "File is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Detect MIME type by reading the first 512 bytes
	var mime string
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		functions.WriteJSONError(w, "Failed to seek file", http.StatusInternalServerError)
		return
	}
	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	mime = http.DetectContentType(buf[:n])
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		functions.WriteJSONError(w, "Failed to seek file", http.StatusInternalServerError)
		return
	}

	if !functions.IsAllowedMimeType(mime, cfg.AllowedMimeTypes) {
		functions.WriteJSONError(w, "MIME type not allowed", http.StatusBadRequest)
		return
	}

	if cfg.MaxUploadSize > 0 && header.Size > cfg.MaxUploadSize {
		functions.WriteJSONError(w, "File too large", http.StatusRequestEntityTooLarge)
		return
	}

	if cfg.MaxStorageSize > 0 {
		total, err := functions.GetTotalStorageSize(cfg.ResolveStoragePath())
		if err == nil && total+header.Size > cfg.MaxStorageSize {
			functions.WriteJSONError(w, "Storage limit exceeded", http.StatusInsufficientStorage)
			return
		}
	}

	filename := fields.Filename
	if filename == "" {
		filename = header.Filename
	}
	if filename != "" && (strings.Contains(filename, "..") || strings.ContainsAny(filename, "/\\")) {
		functions.WriteJSONError(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	var expiresAt *time.Time
	if fields.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, fields.ExpiresAt)
		if err != nil {
			functions.WriteJSONError(w, "invalid expires_at format", http.StatusBadRequest)
			return
		}
		if t.Before(time.Now()) {
			functions.WriteJSONError(w, "expires_at cannot be in the past", http.StatusBadRequest)
			return
		}
		expiresAt = &t
	}

	public := true
	if fields.Public == "false" || fields.Public == "0" {
		public = false
	}

	var meta map[string]interface{}
	if fields.Metadata != "" {
		if err := json.Unmarshal([]byte(fields.Metadata), &meta); err != nil {
			logger.Error("failed to unmarshal metadata: %v", err)
		}
	}

	start := time.Now()
	blob, err := services.Blobs.Upload(r.Context(), services.UploadInput{
		Bucket:    fields.Bucket,
		Filename:  filename,
		Mime:      mime,
		Size:      header.Size,
		Public:    public,
		ExpiresAt: expiresAt,
		Metadata:  meta,
		File:      file,
	})
	if err != nil {
		metrics.Default.RecordError()
		functions.WriteJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	metrics.Default.RecordUpload(blob.Size, time.Since(start))

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(blob); err != nil {
		logger.Error("failed to encode blob JSON: %v", err)
	}
}
