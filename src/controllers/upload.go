package controllers

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"blob/src/database"
	"blob/src/functions"
	"blob/src/models"

	"github.com/asaskevich/govalidator"
	"github.com/google/uuid"
)

func UploadBlobController(w http.ResponseWriter, r *http.Request) {

	bucket := r.FormValue("bucket")
	filename := r.FormValue("filename")
	publicStr := r.FormValue("public")
	expiresAtStr := r.FormValue("expires_at")
	metadata := r.FormValue("metadata")

	validationErrors := make(map[string]string)

	if govalidator.IsNull(bucket) {
		validationErrors["bucket"] = "bucket is required"
	} else if !govalidator.StringLength(bucket, "1", "64") {
		validationErrors["bucket"] = "bucket must be 1-64 chars"
	}

	if filename != "" && !govalidator.StringLength(filename, "1", "255") {
		validationErrors["filename"] = "filename must be 1-255 chars"
	}

	if publicStr != "" && !functions.StringInSlice(publicStr, []string{"true", "false", "0", "1"}) {
		validationErrors["public"] = "public must be true, false, 0 or 1"
	}

	if expiresAtStr != "" && !govalidator.IsRFC3339(expiresAtStr) {
		validationErrors["expires_at"] = "expires_at must be RFC3339 date"
	}

	if metadata != "" && !govalidator.IsJSON(metadata) {
		validationErrors["metadata"] = "metadata must be valid JSON"
	}

	if len(validationErrors) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"error":  "Validation failed",
			"fields": validationErrors,
		}); err != nil {
			// Optionally log: fmt.Println("failed to encode validation error json:", err)
		}
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		functions.WriteJSONError(w, "File is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	allowedMimes := os.Getenv("BLOB_ALLOWED_MIME_TYPES")
	maxUploadSize, _ := strconv.ParseInt(os.Getenv("BLOB_MAX_UPLOAD_SIZE_BYTES"), 10, 64)
	maxStorageSize, _ := strconv.ParseInt(os.Getenv("BLOB_MAX_STORAGE_SIZE"), 10, 64)

	storagePath := os.Getenv("BLOB_STORAGE_PATH")
	if storagePath == "" {
		storagePath = "storage/uploads"
	}

	// Detect MIME type real lendo os primeiros 512 bytes
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

	if !functions.IsAllowedMimeType(mime, functions.SplitComma(allowedMimes)) {
		functions.WriteJSONError(w, "MIME type not allowed", http.StatusBadRequest)
		return
	}

	if maxUploadSize > 0 && header.Size > maxUploadSize {
		functions.WriteJSONError(w, "File too large", http.StatusRequestEntityTooLarge)
		return
	}

	if maxStorageSize > 0 {
		total, err := functions.GetTotalStorageSize(storagePath)
		if err == nil && total+header.Size > maxStorageSize {
			functions.WriteJSONError(w, "Storage limit exceeded", http.StatusInsufficientStorage)
			return
		}
	}

	id := uuid.New()

	if filename == "" {
		filename = header.Filename
	}

	bucketPath := storagePath + string(os.PathSeparator) + bucket
	if err := os.MkdirAll(bucketPath, 0755); err != nil {
		functions.WriteJSONError(w, "Failed to create bucket directory", http.StatusInternalServerError)
		return
	}

	blobPath := bucketPath + string(os.PathSeparator) + id.String()

	out, err := os.Create(blobPath)
	if err != nil {
		functions.WriteJSONError(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer out.Close()

	size, err := io.Copy(out, file)
	if err != nil {
		functions.WriteJSONError(w, "Failed to write file", http.StatusInternalServerError)
		return
	}

	var metaJson map[string]interface{}
	if metadata != "" {
		if err := json.Unmarshal([]byte(metadata), &metaJson); err != nil {
			// log.Println("failed to unmarshal metadata:", err)
		}
	}

	var expiresAt *time.Time
	if expiresAtStr != "" {
		t, err := time.Parse(time.RFC3339, expiresAtStr)
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
	if publicStr != "" {
		if publicStr == "false" || publicStr == "0" {
			public = false
		}
	}

	blob := models.Blob{
		ID:        id,
		Bucket:    bucket,
		Filename:  filename,
		Mime:      mime,
		Size:      size,
		Hash:      id.String(),
		Path:      bucket + "/" + id.String(),
		Public:    &public,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}

	if metaJson != nil {
		metaBytes, _ := json.Marshal(metaJson)
		blob.Metadata = metaBytes
	}

	if err := database.DB.Create(&blob).Error; err != nil {
		functions.WriteJSONError(w, "Failed to save blob metadata", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(blob)
}
