package routes

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"blob/src/config"
	"blob/src/database"
	"blob/src/lib/logger"
)

// HealthHandler handles GET /health, reporting the status of each dependency.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	healthy := true

	components := map[string]string{"status": "ok"}

	if database.DB != nil {
		if sqlDB, err := database.DB.DB(); err == nil {
			if err := sqlDB.Ping(); err != nil {
				components["database"] = "unreachable"
				healthy = false
			} else {
				components["database"] = "ok"
			}
		}
	} else {
		components["database"] = "not_initialized"
		healthy = false
	}

	if database.RedisClient != nil {
		if err := database.RedisClient.Ping(database.Ctx).Err(); err != nil {
			components["redis"] = "unreachable"
			healthy = false
		} else {
			components["redis"] = "ok"
		}
	} else {
		components["redis"] = "not_initialized"
		healthy = false
	}

	storagePath := config.Env.ResolveStoragePath()
	if _, err := os.Stat(storagePath); err != nil {
		components["storage"] = "unreachable"
		healthy = false
	} else {
		components["storage"] = "ok"
	}

	components["timestamp"] = time.Now().UTC().Format(time.RFC3339)

	code := http.StatusOK
	status := "ok"
	if !healthy {
		code = http.StatusServiceUnavailable
		status = "degraded"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     status,
		"components": components,
	}); err != nil {
		logger.Error("failed to encode health json: %v", err)
	}
}
