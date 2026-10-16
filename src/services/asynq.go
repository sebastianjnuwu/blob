package services

import (
	"blob/src/lib/logger"
	"os"

	"github.com/hibiken/asynq"
)

var AsynqClient *asynq.Client
var AsynqServer *asynq.Server

func InitAsynq() {

	redisURL := os.Getenv("REDIS_URL")

	opt, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		logger.Queue.Error("Invalid REDIS_URL: %v", err)
		return
	}

	AsynqClient = asynq.NewClient(opt)

	AsynqServer = asynq.NewServer(
		opt,
		asynq.Config{
			Concurrency: 5,
			Queues: map[string]int{
				"default": 1,
			},
		},
	)

	logger.Queue.Info("Initialized successfully.")
}
