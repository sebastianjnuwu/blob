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

	// 1. Validate fields
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
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":  "Validation failed",
			"fields": validationErrors,
		})
		return
	}

	// 2. Parse multipart form
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		functions.WriteJSONError(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		functions.WriteJSONError(w, "File is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 3. Check file constraints
	allowedMimes := os.Getenv("BLOB_ALLOWED_MIME_TYPES")
	maxUploadSize, _ := strconv.ParseInt(os.Getenv("BLOB_MAX_UPLOAD_SIZE_BYTES"), 10, 64)
	maxStorageSize, _ := strconv.ParseInt(os.Getenv("BLOB_MAX_STORAGE_SIZE"), 10, 64)
	storagePath := os.Getenv("BLOB_STORAGE_PATH")
	if storagePath == "" {
		storagePath = "storage/uploads"
	}

	mime := header.Header.Get("Content-Type")
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

	// 4. Save file
	id := uuid.New()
	if filename == "" {
		filename = header.Filename
	}
	bucketPath := storagePath + string(os.PathSeparator) + bucket
	os.MkdirAll(bucketPath, 0755)
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

	// 5. Parse metadata
	var metaJson map[string]interface{}
	if metadata != "" {
		json.Unmarshal([]byte(metadata), &metaJson)
	}

	// 6. Parse expires_at
	var expiresAt *time.Time
	if expiresAtStr != "" {
		t, err := time.Parse(time.RFC3339, expiresAtStr)
		if err == nil {
			expiresAt = &t
		}
	}

	// 7. Parse public
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
