package main

import (
	"blob/src/api/middleware"
	"blob/src/api/routes"
	"blob/src/auth"
	"blob/src/config"
	"blob/src/database"
	"blob/src/lib/logger"
	"blob/src/services"
	queue "blob/src/services/queue"
	"blob/src/version"
	"net/http"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

var Version = "SNAPSHOT"

func main() {

	logger.Server.Info("Starting Blob API Version: %s", Version)

	if err := godotenv.Load(); err != nil {
		logger.Environment.Error("No .env file found, relying on environment variables")
	}

	database.Redis()
	database.Postgres()

	cfg := config.Load()
	version.V = Version

	auth.EnsureBootstrap()

	services.InitAsynq()
	services.InitBlobService()
	services.InitMultipartService()
	queue.StartQueueWorker()
	queue.StartCleanupScheduler()
	queue.StartTmpCleanupScheduler()

	mux := http.NewServeMux()
	limiter := middleware.Variables()
	routes.RegisterRoutes(mux, limiter)

	corsOpts := cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "HEAD"},
		AllowedHeaders: []string{"Authorization", "Content-Type", "Accept", "Origin", "X-Chunk-Index", "X-Chunk-Hash", "X-Final-Hash", "Range"},
	}
	if cfg.CORSOrigins != "*" && cfg.CORSOrigins != "" {
		origins := strings.Split(cfg.CORSOrigins, ",")
		for i := range origins {
			origins[i] = strings.TrimSpace(origins[i])
		}
		corsOpts.AllowedOrigins = origins
	}
	// AllowCredentials cannot be combined with the "*" wildcard; auth is done
	// via the Authorization header, so cookies are not needed.
	corsOpts.AllowCredentials = len(corsOpts.AllowedOrigins) == 1 && corsOpts.AllowedOrigins[0] != "*"
	handler := cors.New(corsOpts).Handler(mux)

	logger.Server.Info("Server running at: <cyan>http://%s:%s</>.", cfg.Host, cfg.Port)
	srv := &http.Server{
		Addr:         cfg.Host + ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		logger.Server.Error("server error: %v", err)
	}

}
