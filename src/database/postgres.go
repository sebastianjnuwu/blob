package database

import (
	"os"

	"blob/src/lib/logger"
	"blob/src/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Postgres() {
	dsn := os.Getenv("BLOB_DATABASE_URL")
	if dsn == "" {
		logger.Database.Error("[POSTGRES ERROR] BLOB_DATABASE_URL environment variable is not set")
		os.Exit(1)
	}

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Database.Error("[POSTGRES ERROR] %v", err)
		return
	}

	sqlDB, err := DB.DB()
	if err != nil {
		logger.Database.Error("[POSTGRES ERROR] %v", err)
		return
	}

	if err := sqlDB.Ping(); err != nil {
		logger.Database.Error("[POSTGRES ERROR] %v", err)
		return
	}

	if err := DB.AutoMigrate(&models.Blob{}, &models.MultipartUpload{}); err != nil {
		logger.Database.Error("[POSTGRES ERROR] AutoMigrate: %v", err)
		return
	}

	logger.Database.Info("[POSTGRES] Connected successfully.")
}
