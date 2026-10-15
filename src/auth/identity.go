package auth

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// userIDNamespace is a fixed namespace used to derive stable user identifiers
// from API keys. It never changes, so a given token always maps to the same
// UUID across restarts.
var userIDNamespace = uuid.MustParse("d4c5e2f0-0000-4000-8000-000000000001")

// bearerFromRequest extracts the raw token from the Authorization header.
func bearerFromRequest(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(header[7:])
}

// UserIDFromRequest derives a stable UUID from the authenticated bearer token.
// It returns uuid.Nil when the request carries no valid token. Handlers run
// behind RequirePermission, so a nil result only happens on misuse.
func UserIDFromRequest(r *http.Request) uuid.UUID {
	token := bearerFromRequest(r)
	if token == "" || Verify(token) == nil {
		return uuid.Nil
	}
	return StableUserID(token)
}

// StableUserID derives a stable UUID from any credential string.
func StableUserID(s string) uuid.UUID {
	return uuid.NewSHA1(userIDNamespace, []byte(s))
}
