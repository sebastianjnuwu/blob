package middleware

import (
	"net/http"
	"strings"

	"blob/src/auth"
	"blob/src/functions"
)

// bearerToken extracts the raw token from the Authorization header.
func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(header[7:])
}

// AuthMiddleware requires a valid credential: bearer token or SigV4 signature.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := auth.Verify(bearerToken(r))
		if key == nil {
			key = auth.VerifyS3(r)
		}
		if key == nil {
			functions.WriteJSONError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequirePermission wraps AuthMiddleware and additionally requires the
// authenticated key to allow every listed permission.
func RequirePermission(next http.Handler, perms ...auth.Permission) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := auth.Verify(bearerToken(r))
		if key == nil {
			key = auth.VerifyS3(r)
		}
		if key == nil {
			functions.WriteJSONError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if !auth.RequirePermission(key, perms...) {
			functions.WriteJSONError(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
