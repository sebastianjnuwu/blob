package middleware

import (
	"blob/src/functions"
	"net/http"
	"os"
	"strings"
)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if !strings.HasPrefix(token, "Bearer ") {
			functions.WriteJSONError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		expected := os.Getenv("BLOB_TOKEN_SECRET")
		if expected == "" || token[7:] != expected {
			functions.WriteJSONError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
