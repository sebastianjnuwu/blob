package services

import (
	"blob/src/database"
	"blob/src/functions"
	"blob/src/models"
	"blob/src/services"
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hibiken/asynq"
)

const TypeRemoveTmpChunks = "blob:remove_tmp_chunks"

func handleRemoveTmpChunks(ctx context.Context, t *asynq.Task) error {
	return removeOldTmpChunks()
}

func removeOldTmpChunks() error {
	threshold := 24 * time.Hour
	if v := os.Getenv("BLOB_TMP_CLEANUP_THRESHOLD"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			threshold = d
		} else {
			functions.Warn("[TMP CLEANUP] Invalid BLOB_TMP_CLEANUP_THRESHOLD '%s': %v (using default 24h)", v, err)
		}
	}
	cutoff := time.Now().Add(-threshold)
	var uploads []models.MultipartUpload
	db := database.DB.Where("(completed = ? OR completed IS NULL) AND created_at < ?", false, cutoff).Find(&uploads)
	if db.Error != nil {
		return db.Error
	}
	functions.Info("[TMP CLEANUP] Found %d unfinished uploads", len(uploads))
	storagePath := os.Getenv("BLOB_STORAGE_PATH")
	if storagePath == "" {
		storagePath = "storage/uploads"
	}
	for _, upload := range uploads {
		tmpDir := filepath.Join(storagePath, "tmp", upload.ID.String())
		functions.Info("[TMP CLEANUP] Checking tmp dir: %s", tmpDir)
		realTmpDir, err := filepath.Abs(tmpDir)
		realStoragePath, err2 := filepath.Abs(storagePath)
		if err != nil || err2 != nil || !strings.HasPrefix(realTmpDir, realStoragePath) {
			functions.Warn("[TMP CLEANUP] Invalid tmp dir: %s", tmpDir)
		} else if _, err := os.Stat(realTmpDir); os.IsNotExist(err) {
			functions.Warn("[TMP CLEANUP] Tmp dir does not exist: %s", realTmpDir)
		} else if err != nil {
			functions.Warn("[TMP CLEANUP] Error checking tmp dir %s: %v", realTmpDir, err)
		} else {
			if err := os.RemoveAll(realTmpDir); err != nil {
				functions.Warn("[TMP CLEANUP] Failed to remove %s: %v", realTmpDir, err)
			} else {
				functions.Info("[TMP CLEANUP] Removed %s", realTmpDir)
			}
		}

		if !upload.Completed {
			database.DB.Delete(&upload)
			functions.Info("[TMP CLEANUP] Deleted DB record for upload %s", upload.ID.String())
		} else {
			functions.Warn("[TMP CLEANUP] Skipped DB delete for upload %s (completed=true)", upload.ID.String())
		}
	}
	return nil
}

func StartTmpCleanupScheduler() {
	interval := 24 * time.Hour
	if v := os.Getenv("BLOB_TMP_CLEANUP_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			interval = d
		} else {
			functions.Warn("[TMP CLEANUP] Invalid BLOB_TMP_CLEANUP_INTERVAL '%s': %v (using default 24h)", v, err)
		}
	}

	retention := interval
	functions.Info("[QUEUE] Tmp chunk cleanup scheduler started (interval: %v, retention: %v)", interval, retention)
	go func() {
		for {
			task := asynq.NewTask(TypeRemoveTmpChunks, nil, asynq.Retention(retention))
			if services.AsynqClient != nil {
				_, err := services.AsynqClient.Enqueue(task)
				if err != nil {
					functions.Error("[QUEUE ERROR] Failed to enqueue tmp cleanup task: %v", err)
				}
			}
			time.Sleep(interval)
		}
	}()
}
