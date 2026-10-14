package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"blob/src/config"
)

// SignedURLPrefix is the query parameter name for the HMAC signature.
const SignedURLPrefix = "signature"

// Action identifies what a signed URL is allowed to do.
type Action string

const (
	ActionDownload Action = "download"
	ActionView     Action = "view"
)

// Sign generates an HMAC-SHA256 signature for an object access request.
func Sign(secret string, id string, action Action, expires time.Time) string {
	payload := fmt.Sprintf("%s:%s:%d", id, action, expires.Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature checks a signature from the URL against the expected one for
// id/action. It is constant-time on the signature comparison.
func VerifySignature(secret string, id string, action Action, expiresStr, signature string, now time.Time) bool {
	if signature == "" {
		return false
	}
	expires, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil || now.Unix() > expires {
		return false
	}
	expected := Sign(secret, id, action, time.Unix(expires, 0))
	return hmac.Equal([]byte(expected), []byte(strings.ToLower(strings.TrimSpace(signature))))
}

// secret returns the HMAC secret for signed URLs, bootstrapped in the database.
func secret() string {
	return signedSecret
}

// Secret returns the HMAC secret used for signed URLs.
func Secret() string { return secret() }

// DefaultTTL is the lifetime of generated signed URLs.
func DefaultTTL() time.Duration {
	if config.Env != nil {
		return config.Env.SignedURLTTL
	}
	return 15 * time.Minute
}

// ParseURL extracts the id, action, expires and signature from a download/view
// request query string.
func ParseURL(r *http.Request) (id string, action Action, expires, signature string) {
	q := r.URL.Query()
	return q.Get("id"),
		Action(q.Get("action")),
		q.Get("expires"),
		q.Get(SignedURLPrefix)
}

// ValidSignedURL verifies a full request against the configured secret.
func ValidSignedURL(r *http.Request, id string, action Action) bool {
	q := r.URL.Query()
	return VerifySignature(secret(), id, action, q.Get("expires"), q.Get(SignedURLPrefix), time.Now())
}
