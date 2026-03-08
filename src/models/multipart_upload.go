package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type MultipartUpload struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	UserID     uuid.UUID      `gorm:"type:uuid;not null" json:"user_id"`
	Bucket     string         `gorm:"type:text;not null" json:"bucket"`
	Filename   string         `gorm:"type:text;not null" json:"filename"`
	Size       int64          `gorm:"type:bigint;not null" json:"size"`
	ChunksDone datatypes.JSON `gorm:"type:jsonb" json:"chunks_done"`
	CreatedAt  time.Time      `gorm:"type:timestamptz;not null;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time      `gorm:"type:timestamptz;not null;autoUpdateTime" json:"updated_at"`
	Completed  bool           `gorm:"type:boolean;not null;default:false" json:"completed"`
}

func (m *MultipartUpload) BeforeCreate(tx *gorm.DB) (err error) {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return
}
