package multipart

import (
	"blob/src/database"
	"blob/src/models"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
	maxChunkSize := int64(32 << 20)
	if v := os.Getenv("BLOB_MAX_CHUNK_SIZE"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil && parsed > 0 {
			maxChunkSize = parsed
		}
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxChunkSize)
	storagePath := os.Getenv("BLOB_STORAGE_PATH")
	if storagePath == "" {
		storagePath = "storage/uploads"
	}
	tmpDir := filepath.Join(storagePath, "tmp", uploadId)
	if err := os.MkdirAll(tmpDir, 0750); err != nil {
		http.Error(w, "Failed to create temp directory", http.StatusInternalServerError)
		return
	}
	chunkPath := filepath.Join(tmpDir, fmt.Sprintf("chunk_%d", chunkIdx))
	realChunkPath, err := filepath.Abs(chunkPath)
	realTmpDir, err2 := filepath.Abs(tmpDir)
	if err != nil || err2 != nil || !strings.HasPrefix(realChunkPath, realTmpDir) {
		http.Error(w, "Invalid chunk path", http.StatusBadRequest)
		return
	}
	f, err := os.Create(realChunkPath)
	if err != nil {
		http.Error(w, "Failed to save chunk", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	minChunkSize := int64(1 * 1024 * 1024) // 1MB
	maxChunkSize = int64(20 * 1024 * 1024) // 20MB
	if v := os.Getenv("BLOB_MIN_CHUNK_SIZE"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil && parsed > 0 {
			minChunkSize = parsed
		}
	}
	if v := os.Getenv("BLOB_MAX_CHUNK_SIZE"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil && parsed > 0 {
			maxChunkSize = parsed
		}
	}
	// Verificação de integridade: hash SHA256 do chunk
	chunkHashHeader := r.Header.Get("X-Chunk-Hash")
	if chunkHashHeader == "" {
		http.Error(w, "Missing X-Chunk-Hash header", http.StatusBadRequest)
		return
	}

	// Calculate SHA256 hash while writing to file, without loading everything into memory
	limitedReader := io.LimitReader(r.Body, maxChunkSize+1)
	hasher := sha256.New()
	tee := io.TeeReader(limitedReader, hasher)
	written, err := io.Copy(f, tee)
	if err != nil {
		http.Error(w, "Failed to write chunk", http.StatusInternalServerError)
		return
	}
	if written < minChunkSize {
		http.Error(w, "Chunk too small (min "+strconv.FormatInt(minChunkSize, 10)+" bytes)", http.StatusBadRequest)
		return
	}
	if written > maxChunkSize {
		http.Error(w, "Chunk too large (max "+strconv.FormatInt(maxChunkSize, 10)+" bytes)", http.StatusBadRequest)
		return
	}
	calculatedHash := fmt.Sprintf("%x", hasher.Sum(nil))
	if !strings.EqualFold(calculatedHash, chunkHashHeader) {
		http.Error(w, "Chunk hash mismatch (integrity check failed)", http.StatusBadRequest)
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
