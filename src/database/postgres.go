package database

import (
	"os"
	"time"

	"blob/src/lib/logger"
	"blob/src/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

// maxConnectAttempts limita quantas vezes tentamos abrir a conexão no boot.
// Depois disso o processo sai com erro para que o Docker reinicie o container,
// em vez de ficar no ar com o banco fechado e responder 404 para tudo.
const maxConnectAttempts = 30

func Postgres() {
	dsn := os.Getenv("BLOB_DATABASE_URL")
	if dsn == "" {
		logger.Database.Error("BLOB_DATABASE_URL environment variable is not set")
		os.Exit(1)
	}

	for attempt := 1; attempt <= maxConnectAttempts; attempt++ {
		db, err := openPostgres(dsn)
		if err == nil {
			DB = db
			logger.Database.Info("Connected successfully.")
			return
		}

		logger.Database.Error("connection attempt %d/%d failed: %v", attempt, maxConnectAttempts, err)
		if attempt < maxConnectAttempts {
			// Backoff crescente com teto de 5s: cobre o tempo do Postgres
			// reiniciar sem martelar o banco a cada milissegundo.
			delay := time.Duration(attempt) * time.Second
			if delay > 5*time.Second {
				delay = 5 * time.Second
			}
			time.Sleep(delay)
		}
	}

	logger.Database.Error("could not connect after %d attempts; exiting so the container restarts", maxConnectAttempts)
	os.Exit(1)
}

func openPostgres(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, err
	}

	if err := db.AutoMigrate(&models.Blob{}, &models.MultipartUpload{}, &models.ApiKey{}, &models.Setting{}); err != nil {
		sqlDB.Close()
		return nil, err
	}

	return db, nil
}
