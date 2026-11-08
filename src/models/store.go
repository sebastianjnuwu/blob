package models

import "time"

// Store mirrors the read-only Store entity used by the Edge gateway.
type Store struct {
	ID        string     `gorm:"column:id"`
	OwnerID   string     `gorm:"column:ownerId"`
	Domain    *string    `gorm:"column:domain"`
	DeletedAt *time.Time `gorm:"column:deletedAt"`
}
