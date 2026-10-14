package services

import (
	"blob/src/config"
	"blob/src/lib/logger"
	"blob/src/services"
	"context"
	"os"
	"time"

	"github.com/hibiken/asynq"
)

const TypeRemoveTmpChunks = "blob:remove_tmp_chunks"

func handleRemoveTmpChunks(ctx context.Context, t *asynq.Task) error {
	return removeOldTmpChunks(ctx)
}

func removeOldTmpChunks(ctx context.Context) error {
	threshold := 24 * time.Hour
	if v := os.Getenv("BLOB_TMP_CLEANUP_THRESHOLD"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			threshold = d
		} else {
			logger.Queue.Warn("[TMP CLEANUP] Invalid BLOB_TMP_CLEANUP_THRESHOLD '%s': %v (using default 24h)", v, err)
		}
	}
	if config.Env == nil {
		config.Load()
	}
	cutoff := time.Now().Add(-threshold)
	removed, err := services.Multipart.CleanupAbandoned(ctx, cutoff)
	if err != nil {
		logger.Queue.Error("[TMP CLEANUP] Failed to clean up tmp chunks: %v", err)
		return err
	}
	logger.Queue.Info("[TMP CLEANUP] Removed %d abandoned uploads", removed)
	return nil
}

func StartTmpCleanupScheduler() {
	interval := 24 * time.Hour
	if v := os.Getenv("BLOB_TMP_CLEANUP_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			interval = d
		} else {
			logger.Queue.Warn("[TMP CLEANUP] Invalid BLOB_TMP_CLEANUP_INTERVAL '%s': %v (using default 24h)", v, err)
		}
	}

	retention := interval
	logger.Queue.Info("[QUEUE] Tmp chunk cleanup scheduler started (interval: %v, retention: %v)", interval, retention)
	go func() {
		for {
			task := asynq.NewTask(TypeRemoveTmpChunks, nil, asynq.Retention(retention))
			if services.AsynqClient != nil {
				_, err := services.AsynqClient.Enqueue(task)
				if err != nil {
					logger.Queue.Error("[QUEUE ERROR] Failed to enqueue tmp cleanup task: %v", err)
				}
			}
			time.Sleep(interval)
		}
	}()
}
