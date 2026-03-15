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

const TypeDeleteExpiredBlobs = "blob:delete_expired"

func handleDeleteExpiredBlobs(ctx context.Context, t *asynq.Task) error {
	return removeExpiredBlobs()
}

func StartQueueWorker() {
	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeDeleteExpiredBlobs, handleDeleteExpiredBlobs)
	mux.HandleFunc(TypeRemoveTmpChunks, handleRemoveTmpChunks)
	go func() {
		if services.AsynqServer == nil {
			functions.Error("[QUEUE ERROR] AsynqServer is nil! Did you call InitAsynq() first?")
			return
		}
		if err := services.AsynqServer.Run(mux); err != nil {
			functions.Error("[QUEUE ERROR] Worker failed: %v", err)
		}
	}()
}

func StartCleanupScheduler() {

	interval := 24 * time.Hour
	if v := os.Getenv("BLOB_EXPIRED_CLEANUP_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			interval = d
		} else {
			functions.Warn("[QUEUE] Invalid BLOB_EXPIRED_CLEANUP_INTERVAL '%s', using default 24h", v)
		}
	}

	retention := interval
	functions.Info("[QUEUE] Expired blob scheduler started (interval: %v, retention: %v)", interval, retention)
	go func() {
		for {
			if services.AsynqClient == nil {
				functions.Error("[QUEUE ERROR] AsynqClient is nil! Did you call InitAsynq() first?")
				return
			}
			task := asynq.NewTask(TypeDeleteExpiredBlobs, nil, asynq.Retention(retention))
			_, err := services.AsynqClient.Enqueue(task)
			if err != nil {
				functions.Error("[QUEUE ERROR] Failed to enqueue blob expired task: %v", err)
			}
			time.Sleep(interval)
		}
	}()
}

func removeExpiredBlobs() error {
	var expired []models.Blob
	err := database.DB.Where("expires_at IS NOT NULL AND expires_at <= ?", time.Now()).Find(&expired).Error
	if err != nil {
		functions.Error("[QUEUE] Failed to query expired blobs: %v", err)
		return err
	}
	for _, blob := range expired {
		if blob.Path != "" {
			functions.Info("[QUEUE] Attempting to remove file: %s", blob.Path)
			storagePath := os.Getenv("BLOB_STORAGE_PATH")
			if storagePath == "" {
				storagePath = "storage/uploads"
			}
			filePath := storagePath + string(os.PathSeparator) + blob.Path
			realFilePath, err := filepath.Abs(filePath)
			realStoragePath, err2 := filepath.Abs(storagePath)
			if err == nil && err2 == nil && strings.HasPrefix(realFilePath, realStoragePath) {
				if stat, statErr := os.Stat(realFilePath); statErr != nil {
					functions.Warn("[QUEUE] File stat error for %s: %v", realFilePath, statErr)
				} else {
					functions.Info("[QUEUE] File exists. Size: %d, Mode: %v", stat.Size(), stat.Mode())
				}
				err := os.Remove(realFilePath)
				if err != nil {
					functions.Warn("[QUEUE] os.Remove error for %s: %v", realFilePath, err)
				} else {
					functions.Info("[QUEUE] File removed: %s", realFilePath)
				}
			} else {
				functions.Warn("[QUEUE] Invalid file path: %s", filePath)
			}
		}
		delErr := database.DB.Delete(&blob).Error
		if delErr != nil {
			functions.Warn("[QUEUE] Failed to remove blob from DB: %v", delErr)
		} else {
			functions.Info("[QUEUE] Blob removed from DB: %s", blob.ID.String())
		}
	}
	functions.Info("[QUEUE] Expired blobs expired finished. Total: %d", len(expired))
	return nil
}
