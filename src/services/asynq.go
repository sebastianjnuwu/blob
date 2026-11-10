package services

import (
	"blob/src/lib/logger"
	"os"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

var AsynqClient *asynq.Client
var AsynqServer *asynq.Server

func InitAsynq() {
	redisURL := os.Getenv("REDIS_URL")

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		logger.Queue.Error("Invalid REDIS_URL: %v", err)
		return
	}

	clientOpt := asynq.RedisClientOpt{
		Network:   opt.Network,
		Addr:      opt.Addr,
		Username:  opt.Username,
		Password:  opt.Password,
		DB:        opt.DB,
		TLSConfig: opt.TLSConfig,
	}

	AsynqClient = asynq.NewClient(clientOpt)

	AsynqServer = asynq.NewServer(
		clientOpt,
		asynq.Config{
			Concurrency: 5,
			Queues: map[string]int{
				"default": 1,
			},
		},
	)

	logger.Queue.Info("Initialized successfully.")
}
