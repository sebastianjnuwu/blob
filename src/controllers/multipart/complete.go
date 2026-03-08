package multipart

import (
	"blob/src/database"
	"blob/src/models"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// POST /blob/{uploadId}/complete
func CompleteUpload(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Missing uploadId"}); err != nil {
			// Optionally log: fmt.Println("failed to encode error json (missing uploadId):", err)
		}
		return
	}
	uploadId := parts[1]

	userIDStr := r.Header.Get("X-User-ID")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Missing or invalid X-User-ID header"}); err != nil {
			// Optionally log: fmt.Println("failed to encode error json (missing user id):", err)
		}
		return
	}

	var upload models.MultipartUpload
	if err := database.DB.First(&upload, "id = ?", uploadId).Error; err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Invalid uploadId"}); err != nil {
			// Optionally log: fmt.Println("failed to encode error json (invalid uploadId):", err)
		}
		return
	}

	if upload.UserID != userID {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Forbidden: not your upload"}); err != nil {
			// Optionally log: fmt.Println("failed to encode error json (forbidden):", err)
		}
		return
	}

	if upload.Completed {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Upload already completed"}); err != nil {
			// Optionally log: fmt.Println("failed to encode error json (already completed):", err)
		}
		return
	}

	var chunks []int
	if err := json.Unmarshal(upload.ChunksDone, &chunks); err != nil || len(chunks) == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "No chunks uploaded"}); err != nil {
			// Optionally log: fmt.Println("failed to encode error json (no chunks):", err)
		}
		return
	}
	sort.Ints(chunks)

	storagePath := os.Getenv("BLOB_STORAGE_PATH")
	if storagePath == "" {
		storagePath = "storage/uploads"
	}
	tmpDir := filepath.Join(storagePath, "tmp", uploadId)
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create temp directory"}); err != nil {
			// Optionally log: fmt.Println("failed to encode error json (temp dir):", err)
		}
		return
	}

	finalPath := filepath.Join(tmpDir, "final")
	fFinal, err := os.Create(finalPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create final file"}); err != nil {
			// Optionally log: fmt.Println("failed to encode error json (final file):", err)
		}
		return
	}

	for _, idx := range chunks {
		chunkPath := filepath.Join(tmpDir, fmt.Sprintf("chunk_%d", idx))
		f, err := os.Open(chunkPath)
		if err != nil {
			fFinal.Close()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			if err := json.NewEncoder(w).Encode(map[string]string{"error": "Missing chunk"}); err != nil {
				// Optionally log: fmt.Println("failed to encode error json (missing chunk):", err)
			}
			return
		}
		if _, err := io.Copy(fFinal, f); err != nil {
			f.Close()
			fFinal.Close()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			if err := json.NewEncoder(w).Encode(map[string]string{"error": "Failed to write chunk"}); err != nil {
				// Optionally log: fmt.Println("failed to encode error json (write chunk):", err)
			}
			return
		}
		f.Close()
	}

	// Detect MIME type real
	if _, err := fFinal.Seek(0, 0); err != nil {
		fFinal.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Failed to seek final file"}); err != nil {
			// Optionally log: fmt.Println("failed to encode error json (seek final file):", err)
		}
		return
	}
	buf := make([]byte, 512)
	n, _ := fFinal.Read(buf)
	mime := http.DetectContentType(buf[:n])
	if _, err := fFinal.Seek(0, 0); err != nil {
		fFinal.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Failed to seek final file (2)"}); err != nil {
			// Optionally log: fmt.Println("failed to encode error json (seek final file 2):", err)
		}
		return
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, fFinal); err != nil {
		fFinal.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to hash file"})
		return
	}
	hash := hex.EncodeToString(hasher.Sum(nil))
	fFinal.Close()

	bucketPath := filepath.Join(storagePath, upload.Bucket)
	if err := os.MkdirAll(bucketPath, 0755); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create bucket directory"})
		return
	}

	finalID := uploadId
	destPath := filepath.Join(bucketPath, finalID)
	if err := os.Rename(finalPath, destPath); err != nil {
		in, errIn := os.Open(finalPath)
		if errIn == nil {
			out, errOut := os.Create(destPath)
			if errOut == nil {
				_, _ = io.Copy(out, in)
				out.Close()
			}
			in.Close()
			os.Remove(finalPath)
		}
	}

	blobUUID, err := uuid.Parse(finalID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid UUID"})
		return
	}

	bl := models.Blob{
		ID:        blobUUID,
		Bucket:    upload.Bucket,
		Filename:  upload.Filename,
		Mime:      mime,
		Size:      upload.Size,
		Hash:      hash,
		Path:      upload.Bucket + "/" + finalID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := database.DB.Create(&bl).Error; err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save blob"})
		return
	}

	database.DB.Model(&upload).Update("completed", true)
	if err := os.RemoveAll(tmpDir); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to remove temp directory"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":         bl.ID.String(),
		"bucket":     bl.Bucket,
		"filename":   bl.Filename,
		"size":       bl.Size,
		"hash":       bl.Hash,
		"created_at": bl.CreatedAt,
		"updated_at": bl.UpdatedAt,
	})
}
