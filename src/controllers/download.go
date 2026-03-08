package controllers

import (
	"blob/src/database"
	"blob/src/functions"
	"blob/src/models"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DownloadBlobController handles GET /blob/{id}/download
func DownloadBlobController(w http.ResponseWriter, r *http.Request) {

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	if len(parts) < 3 || parts[0] != "blob" || parts[2] != "download" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte(`{"error":"Invalid download URL"}`)); err != nil {
			functions.Error("failed to write error: %v", err)
		}
		return
	}

	id, err := uuid.Parse(parts[1])
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte(`{"error":"Invalid blob id"}`)); err != nil {
			functions.Error("failed to write error: %v", err)
		}
		return
	}

	var blob models.Blob

	if err := database.DB.First(&blob, "id = ?", id).Error; err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		if _, err := w.Write([]byte(`{"error":"Blob not found"}`)); err != nil {
			functions.Error("failed to write error: %v", err)
		}
		return
	}

	if blob.ExpiresAt != nil && blob.ExpiresAt.Before(time.Now()) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusGone)
		if _, err := w.Write([]byte(`{"error":"Blob expired"}`)); err != nil {
			functions.Error("failed to write blob expired json: %v", err)
		}
		return
	}

	if blob.Public != nil && !*blob.Public {

		hashParam := strings.TrimSpace(r.URL.Query().Get("hash"))
		dbHash := strings.TrimSpace(blob.Hash)

		token := r.Header.Get("Authorization")
		expected := os.Getenv("BLOB_TOKEN_SECRET")

		hasValidToken :=
			expected != "" &&
				strings.HasPrefix(token, "Bearer ") &&
				token[7:] == expected

		hashValid :=
			dbHash != "" &&
				hashParam != "" &&
				hashParam == dbHash

		if !hashValid && !hasValidToken {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			if _, err := w.Write([]byte(`{"error":"Forbidden"}`)); err != nil {
				functions.Error("failed to write forbidden json: %v", err)
			}
			return
		}
	}

	storagePath := os.Getenv("BLOB_STORAGE_PATH")
	if storagePath == "" {
		storagePath = "storage/uploads"
	}

	filePath := filepath.Join(storagePath, blob.Path)

	file, err := os.Open(filePath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		if _, err := w.Write([]byte(`{"error":"File not found on disk"}`)); err != nil {
			functions.Error("failed to write file not found json: %v", err)
		}
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	database.DB.Model(&blob).UpdateColumn("download_count", blob.DownloadCount+1)

	w.Header().Set("Content-Type", blob.Mime)
	w.Header().Set("Content-Disposition", `attachment; filename="`+blob.Filename+`"`)
	w.Header().Set("Content-Length", strconv.FormatInt(blob.Size, 10))

	http.ServeContent(
		w,
		r,
		blob.Filename,
		info.ModTime(),
		file,
	)
}
