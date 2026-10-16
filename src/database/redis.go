package database

import (
	"context"
	"os"

	"blob/src/lib/logger"

	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client
var Ctx = context.Background()

func Redis() {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		logger.Database.Error("REDIS_URL environment variable is not set")
		os.Exit(1)
	}

	opt, err := redis.ParseURL(redisURL)

	if err != nil {
		logger.Database.Error("Invalid URL: %v", err)
		return
	}

	RedisClient = redis.NewClient(opt)

	if err := RedisClient.Ping(Ctx).Err(); err != nil {
		logger.Database.Error("%v", err)
	} else {
		logger.Database.Info("Connected successfully.")
	}
}
