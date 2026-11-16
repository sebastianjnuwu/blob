package controllers

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"blob/src/auth"
	"blob/src/config"
	"blob/src/functions"
	"blob/src/metrics"
	"blob/src/models"
	"blob/src/services"

	"github.com/google/uuid"
)

// parseBlobID extracts and validates the blob UUID from the URL path.
func parseBlobID(r *http.Request) (uuid.UUID, bool) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "blob" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// isExpired reports whether the blob's expiration date has passed.
func isExpired(blob *models.Blob) bool {
	return blob.ExpiresAt != nil && blob.ExpiresAt.Before(time.Now())
}

// hasValidToken checks the Authorization header against configured keys.
func hasValidToken(r *http.Request) bool {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return false
	}
	return auth.Verify(strings.TrimSpace(header[7:])) != nil
}

// hasValidHash checks the legacy ?hash= authorization for private blobs.
// It can be disabled via BLOB_HASH_FALLBACK_ENABLED=false.
func hasValidHash(r *http.Request, blob *models.Blob) bool {
	if config.Env != nil && !config.Env.HashFallbackEnabled {
		return false
	}
	hashParam := strings.TrimSpace(r.URL.Query().Get("hash"))
	if blob.Hash == "" || hashParam == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(hashParam), []byte(blob.Hash)) == 1
}

// canAccessBlob evaluates all authorization paths for a private blob:
// signed URL, legacy hash and bearer token.
func canAccessBlob(r *http.Request, blob *models.Blob, action auth.Action) bool {
	if blob.Public != nil && *blob.Public {
		return true
	}
	return auth.ValidSignedURL(r, blob.ID.String(), action) ||
		hasValidHash(r, blob) ||
		hasValidToken(r)
}

func serveBlobFile(w http.ResponseWriter, r *http.Request, action auth.Action, disposition string, counterField string) {

	id, ok := parseBlobID(r)
	if !ok {
		functions.WriteJSONError(w, "Invalid blob id", http.StatusBadRequest)
		return
	}

	blob, err := services.Blobs.Get(r.Context(), id)
	if err != nil {
		functions.WriteJSONError(w, "Blob not found", http.StatusNotFound)
		return
	}

	if isExpired(blob) {
		functions.WriteJSONError(w, "Blob expired", http.StatusGone)
		return
	}

	if !canAccessBlob(r, blob, action) {
		functions.WriteJSONError(w, "Forbidden", http.StatusForbidden)
		return
	}

	start := time.Now()
	blobFile, err := services.Blobs.OpenFile(r.Context(), id, counterField)
	if err != nil {
		if os.IsNotExist(err) {
			functions.WriteJSONError(w, "File not found on disk", http.StatusNotFound)
			return
		}
		functions.WriteJSONError(w, "Failed to open file", http.StatusInternalServerError)
		return
	}
	defer blobFile.Reader.Close()

	w.Header().Set("Content-Type", blob.Mime)
	w.Header().Set("Content-Disposition", disposition+`; filename="`+blob.Filename+`"`)
	w.Header().Set("Content-Length", strconv.FormatInt(blobFile.Size, 10))
	w.Header().Set("Accept-Ranges", "bytes")
	if blob.Hash != "" {
		w.Header().Set("ETag", `"`+blob.Hash+`"`)
	}
	if blob.Public != nil && *blob.Public {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "private, max-age=3600")
	}

	http.ServeContent(w, r, blob.Filename, blobFile.ModTime, blobFile.Reader)

	if counterField == "download_count" {
		metrics.Default.RecordDownload(blobFile.Size, time.Since(start))
	} else {
		metrics.Default.RecordView(blobFile.Size)
	}
}

// DownloadBlobController handles GET /blob/{id}/download
func DownloadBlobController(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 || parts[2] != "download" {
		functions.WriteJSONError(w, "Invalid download URL", http.StatusBadRequest)
		return
	}
	serveBlobFile(w, r, auth.ActionDownload, "attachment", "download_count")
}
