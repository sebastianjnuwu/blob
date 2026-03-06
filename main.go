package main

import (
	"blob/src/middleware"
	"blob/src/routes"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func main() {

	godotenv.Load()

	mux := http.NewServeMux()

	limiter := middleware.NewRateLimiterFromEnv()

	routes.RegisterRoutes(mux, limiter)

	PORT := os.Getenv("BLOB_PORT")
	if PORT == "" {
		PORT = "3000"
	}

	HOST := os.Getenv("BLOB_HOST")
	if HOST == "" {
		HOST = "localhost"
	}

	log.Println("Servidor iniciado em: http://" + HOST + ":" + PORT)
	log.Fatal(http.ListenAndServe(HOST+":"+PORT, mux))
}
