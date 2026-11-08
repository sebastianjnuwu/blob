package models

import "time"

// StorePage mirrors the published page metadata used by the Edge gateway.
type StorePage struct {
	StoreID   string    `gorm:"column:storeId"`
	BlobKey   *string   `gorm:"column:blobKey"`
	CreatedAt time.Time `gorm:"column:createdAt"`
	UpdatedAt time.Time `gorm:"column:updatedAt"`
}
