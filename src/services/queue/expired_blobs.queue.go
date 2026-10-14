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

const TypeDeleteExpiredBlobs = "blob:delete_expired"

func handleDeleteExpiredBlobs(ctx context.Context, t *asynq.Task) error {
	return removeExpiredBlobs(ctx)
}

func StartQueueWorker() {
	mux := asynq.NewServeMux()
	mux.HandleFunc(TypeDeleteExpiredBlobs, handleDeleteExpiredBlobs)
	mux.HandleFunc(TypeRemoveTmpChunks, handleRemoveTmpChunks)
	go func() {
		if services.AsynqServer == nil {
			logger.Queue.Error("AsynqServer is nil! Did you call InitAsynq() first?")
			return
		}
		if err := services.AsynqServer.Run(mux); err != nil {
			logger.Queue.Error("Worker failed: %v", err)
		}
	}()
}

func StartCleanupScheduler() {

	interval := 24 * time.Hour
	if v := os.Getenv("BLOB_EXPIRED_CLEANUP_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			interval = d
		} else {
			logger.Queue.Warn("Invalid BLOB_EXPIRED_CLEANUP_INTERVAL '%s', using default 24h", v)
		}
	}

	retention := interval
	logger.Queue.Info("Expired blob scheduler started (interval: %v, retention: %v)", interval, retention)
	go func() {
		for {
			if services.AsynqClient == nil {
				logger.Queue.Error("AsynqClient is nil! Did you call InitAsynq() first?")
				return
			}
			task := asynq.NewTask(TypeDeleteExpiredBlobs, nil, asynq.Retention(retention))
			_, err := services.AsynqClient.Enqueue(task)
			if err != nil {
				logger.Queue.Error("Failed to enqueue blob expired task: %v", err)
			}
			time.Sleep(interval)
		}
	}()
}

func removeExpiredBlobs(ctx context.Context) error {
	if config.Env == nil {
		config.Load()
	}
	removed, err := services.Blobs.PurgeExpired(ctx)
	if err != nil {
		logger.Queue.Error("Failed to purge expired blobs: %v", err)
		return err
	}
	logger.Queue.Info("Expired blobs removed. Total: %d", removed)
	return nil
}
