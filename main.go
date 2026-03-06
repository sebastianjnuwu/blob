package main

import (
	"blob/src/database"
	"blob/src/functions"
	"blob/src/middleware"
	"blob/src/routes"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

func main() {
	godotenv.Load()
	database.Redis()
	database.Postgres()

	mux := http.NewServeMux()
	limiter := middleware.Variables()
	routes.RegisterRoutes(mux, limiter)

	corsOrigins := os.Getenv("BLOB_CORS_ORIGINS")
	corsOpts := cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "HEAD"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "Accept", "Origin"},
	}
	if corsOrigins != "*" && corsOrigins != "" {
		origins := strings.Split(corsOrigins, ",")
		for i := range origins {
			origins[i] = strings.TrimSpace(origins[i])
		}
		corsOpts.AllowedOrigins = origins
	}
	handler := cors.New(corsOpts).Handler(mux)

	port := os.Getenv("BLOB_PORT")
	if port == "" {
		port = "3000"
	}
	host := os.Getenv("BLOB_HOST")
	if host == "" {
		host = "localhost"
	}

	functions.Info("[SERVER] Server running at: http://%s:%s", host, port)
	http.ListenAndServe(host+":"+port, handler)
}
