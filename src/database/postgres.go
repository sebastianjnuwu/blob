package database

import (
	"os"

	"blob/src/functions"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Postgres() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		functions.Error("[POSTGRES ERROR] DATABASE_URL environment variable is not set")
		os.Exit(1)
	}

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		functions.Error("[POSTGRES ERROR] %v", err)
		return
	}

	sqlDB, err := DB.DB()
	if err != nil {
		functions.Error("[POSTGRES ERROR] %v", err)
		return
	}

	if err := sqlDB.Ping(); err != nil {
		functions.Error("[POSTGRES ERROR] %v", err)
	} else {
		functions.Info("[POSTGRES] Connected successfully.")
	}
}
