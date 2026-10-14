package config

import (
	"os"
	"strconv"
	"time"

	"blob/src/functions"
	"blob/src/lib/logger"
)

// Config holds all runtime configuration loaded from environment variables.
// It centralizes environment access so services don't read os.Getenv directly.
type Config struct {
	Port                   string
	Host                   string
	DatabaseURL            string
	RedisURL               string
	StoragePath            string
	TokenSecret            string
	APIKeys                string
	SignedURLSecret        string
	CORSOrigins            string
	TrustProxy             bool
	PublicURL              string
	RateLimitMax           int
	RateLimitWindowMS      int
	AllowedMimeTypes       []string
	MinChunkSize           int64
	MaxChunkSize           int64
	MaxUploadSize          int64
	MaxStorageSize         int64
	ExpiredCleanupInterval time.Duration
	TmpCleanupInterval     time.Duration
	TmpCleanupThreshold    time.Duration
	DedupEnabled           bool
	SignedURLTTL           time.Duration
	HashFallbackEnabled    bool
}

// Env is the process-wide configuration, populated by Load.
var Env *Config

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed
		}
		logger.Environment.Warn("Invalid integer for %s: %s (using default %d)", key, v, fallback)
	}
	return fallback
}

func getenvInt64(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			return parsed
		}
		logger.Environment.Warn("Invalid integer for %s: %s (using default %d)", key, v, fallback)
	}
	return fallback
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		logger.Environment.Warn("Invalid duration for %s: %s (using default %v)", key, v, fallback)
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
		logger.Environment.Warn("Invalid boolean for %s: %s (using default %t)", key, v, fallback)
	}
	return fallback
}

// Load reads environment variables into Env. It is safe to call multiple times.
func Load() *Config {
	cfg := &Config{
		Port:                   getenv("BLOB_PORT", "3000"),
		Host:                   getenv("BLOB_HOST", "localhost"),
		DatabaseURL:            getenv("BLOB_DATABASE_URL", ""),
		RedisURL:               getenv("REDIS_URL", ""),
		StoragePath:            getenv("BLOB_STORAGE_PATH", "storage/uploads"),
		TokenSecret:            getenv("BLOB_TOKEN_SECRET", ""),
		APIKeys:                getenv("BLOB_API_KEYS", ""),
		SignedURLSecret:        getenv("BLOB_SIGNED_URL_SECRET", ""),
		CORSOrigins:            getenv("BLOB_CORS_ORIGINS", "*"),
		TrustProxy:             getenvBool("BLOB_TRUST_PROXY", false),
		PublicURL:              getenv("BLOB_PUBLIC_URL", ""),
		RateLimitMax:           getenvInt("BLOB_RATE_LIMIT_MAX", 120),
		RateLimitWindowMS:      getenvInt("BLOB_RATE_LIMIT_WINDOW_MS", 60000),
		AllowedMimeTypes:       functions.SplitComma(getenv("BLOB_ALLOWED_MIME_TYPES", "image/png,image/jpeg,image/gif,image/webp")),
		MinChunkSize:           getenvInt64("BLOB_MIN_CHUNK_SIZE", 1*1024*1024),
		MaxChunkSize:           getenvInt64("BLOB_MAX_CHUNK_SIZE", 20*1024*1024),
		MaxUploadSize:          getenvInt64("BLOB_MAX_UPLOAD_SIZE_BYTES", 100*1024*1024),
		MaxStorageSize:         getenvInt64("BLOB_MAX_STORAGE_SIZE", 1024*1024*1024),
		ExpiredCleanupInterval: getenvDuration("BLOB_EXPIRED_CLEANUP_INTERVAL", 24*time.Hour),
		TmpCleanupInterval:     getenvDuration("BLOB_TMP_CLEANUP_INTERVAL", 24*time.Hour),
		TmpCleanupThreshold:    getenvDuration("BLOB_TMP_CLEANUP_THRESHOLD", 24*time.Hour),
		DedupEnabled:           getenvBool("BLOB_DEDUP_ENABLED", false),
		SignedURLTTL:           getenvDuration("BLOB_SIGNED_URL_TTL", 15*time.Minute),
		HashFallbackEnabled:    getenvBool("BLOB_HASH_FALLBACK_ENABLED", true),
	}
	Env = cfg
	return cfg
}

// ResolveStoragePath returns the base directory where blob data is stored.
func (c *Config) ResolveStoragePath() string {
	if c.StoragePath == "" {
		return "storage/uploads"
	}
	return c.StoragePath
}

// TmpStoragePath returns the storage path including the temp namespace.
func (c *Config) TmpStoragePath() string {
	return c.ResolveStoragePath() + string(os.PathSeparator) + "tmp"
}
