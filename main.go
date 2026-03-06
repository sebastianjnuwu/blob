package main

import (
	"blob/src/database"
	"blob/src/functions"
	"blob/src/middleware"
	"blob/src/routes"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func main() {

	godotenv.Load()
	database.Redis()
	database.Postgres()

	mux := http.NewServeMux()
	limiter := middleware.Variables()
	routes.RegisterRoutes(mux, limiter)

	PORT := os.Getenv("BLOB_PORT")
	if PORT == "" {
		PORT = "3000"
	}
	HOST := os.Getenv("BLOB_HOST")
	if HOST == "" {
		HOST = "localhost"
	}

	functions.Info("[SERVER] Server running at: http://%s:%s", HOST, PORT)
	http.ListenAndServe(HOST+":"+PORT, mux)
}
