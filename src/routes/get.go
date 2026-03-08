package routes

import (
	"encoding/json"
	"net/http"
)

func GETHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"message": "Hello, World!",
	}); err != nil {
		// Optionally log: fmt.Println("failed to encode get handler json:", err)
	}
}
