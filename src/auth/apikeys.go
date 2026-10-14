package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"

	"blob/src/config"
	"blob/src/database"
	"blob/src/lib/logger"
	"blob/src/models"
)

// hmacSecretKey is the settings key holding the HMAC secret for signed URLs.
const hmacSecretKey = "signed_url_secret"

// signedSecret caches the HMAC secret after bootstrap.
var signedSecret string

// hashToken returns the hex sha256 of a bearer token.
func hashToken(token string) string {
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
	want := hashToken(bearerToken)
	var row models.ApiKey
	if err := database.DB.Where("token_hash = ? AND active = ?", want, true).First(&row).Error; err != nil {
		return nil
	}
	if subtle.ConstantTimeCompare([]byte(row.TokenHash), []byte(want)) != 1 {
		return nil
	}
	return &APIKey{Token: bearerToken, Perms: parsePermissions(row.Scopes)}
}

// EnsureBootstrap creates the HMAC secret and the first admin key when empty.
// The admin plaintext is printed once; it is never stored.
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

	var n int64
	database.DB.Model(&models.ApiKey{}).Where("active = ?", true).Count(&n)
	if n > 0 {
		return
	}

	token, rerr := randomToken()
	if rerr != nil {
		logger.Database.Error("bootstrap admin key: %v", rerr)
		return
	}
	label := "admin"
	if config.Env != nil && config.Env.AdminEmail != "" {
		label = config.Env.AdminEmail
	}
	if err := database.DB.Create(&models.ApiKey{
		Label:     label,
		TokenHash: hashToken(token),
		Scopes:    "all",
		Active:    true,
	}).Error; err != nil {
		logger.Database.Error("bootstrap admin key: %v", err)
		return
	}
	logger.Database.Info("bootstrap admin key (shown once): %s", token)
}
