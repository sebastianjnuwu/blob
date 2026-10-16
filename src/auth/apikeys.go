package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"

	"blob/src/database"
	"blob/src/lib/logger"
	"blob/src/models"
)

// hmacSecretKey is the settings key holding the HMAC secret for signed URLs.
const hmacSecretKey = "signed_url_secret"

// signedSecret caches the HMAC secret after bootstrap.
var signedSecret string

// HashToken returns the hex sha256 of a bearer token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// randomToken generates a 64-char hex token.
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Verify returns the API key when the bearer token matches an active row.
// Only hashes are stored; comparison is constant-time.
func Verify(bearerToken string) *APIKey {
	if bearerToken == "" || database.DB == nil {
		return nil
	}
	want := HashToken(bearerToken)
	var row models.ApiKey
	if err := database.DB.Where("token_hash = ? AND active = ?", want, true).First(&row).Error; err != nil {
		return nil
	}
	if row.TokenHash == nil || subtle.ConstantTimeCompare([]byte(*row.TokenHash), []byte(want)) != 1 {
		return nil
	}
	return &APIKey{Token: bearerToken, Perms: parsePermissions(row.Scopes)}
}

// S3Credentials returns the plaintext secret and permissions for an access
// key ID, for future SigV4 verification. The secret must stay server-side.
func S3Credentials(accessKeyID string) (string, map[Permission]bool, bool) {
	if accessKeyID == "" || database.DB == nil {
		return "", nil, false
	}
	var row models.ApiKey
	if err := database.DB.Where("access_key_id = ? AND active = ?", accessKeyID, true).First(&row).Error; err != nil {
		return "", nil, false
	}
	if row.Secret == "" {
		return "", nil, false
	}
	return row.Secret, parsePermissions(row.Scopes), true
}

// API keys are never auto-generated: create them with `blob admin create`.
// EnsureBootstrap creates the HMAC secret for signed URLs when missing.
// API keys are never auto-generated: create them with `blob admin create`.
func EnsureBootstrap() {
	if database.DB == nil {
		logger.Database.Error("bootstrap skipped: database unavailable")
		return
	}

	var s models.Setting
	if err := database.DB.Where("key = ?", hmacSecretKey).First(&s).Error; err != nil {
		secret, rerr := randomToken()
		if rerr != nil {
			logger.Database.Error("bootstrap HMAC secret: %v", rerr)
			return
		}
		s = models.Setting{Key: hmacSecretKey, Value: secret}
		if err := database.DB.Create(&s).Error; err != nil {
			logger.Database.Error("bootstrap HMAC secret: %v", err)
			return
		}
	}
	signedSecret = s.Value
}
