package controllers

import (
	"blob/src/database"
	"blob/src/models"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

func DownloadBlobController(w http.ResponseWriter, r *http.Request) {

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	if len(parts) < 3 || parts[0] != "blob" || parts[2] != "download" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"Invalid download URL"}`))
		return
	}

	id, err := uuid.Parse(parts[1])
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"Invalid blob id"}`))
		return
	}

	var blob models.Blob

	if err := database.DB.First(&blob, "id = ?", id).Error; err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"Blob not found"}`))
		return
	}

	// Only allow download if blob is public, hash matches, or token is valid
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
			w.Write([]byte(`{"error":"Forbidden"}`))
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
		w.Write([]byte(`{"error":"File not found on disk"}`))
		return
	}
	defer file.Close()

	// increment download counter
	database.DB.Model(&blob).UpdateColumn("download_count", blob.DownloadCount+1)

	w.Header().Set("Content-Type", blob.Mime)
	w.Header().Set("Content-Disposition", `attachment; filename="`+blob.Filename+`"`)
	w.Header().Set("Content-Length", strconv.FormatInt(blob.Size, 10))

	_, _ = io.Copy(w, file)
}
