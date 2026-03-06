package database

import (
	"context"
	"os"

	"blob/src/functions"

	"github.com/redis/go-redis/v9"
)

var RedisClient *redis.Client
var Ctx = context.Background()

func Redis() {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		functions.Error("[REDIS ERROR] REDIS_URL environment variable is not set")
		os.Exit(1)
	}

	opt, err := redis.ParseURL(redisURL)

	if err != nil {
		functions.Error("[REDIS ERROR] Invalid URL: %v", err)
		return
	}

	RedisClient = redis.NewClient(opt)

	if err := RedisClient.Ping(Ctx).Err(); err != nil {
		functions.Error("[REDIS ERROR] %v", err)
	} else {
		functions.Info("[REDIS] Connected successfully.")
	}
}
