package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ApiKey is a bearer token with permission scopes, managed by an admin.
// Only the sha256 hash is stored; the plaintext is shown once at creation.
// S3-style credentials use AccessKeyID + Secret (plaintext, required to
// verify SigV4 signatures server-side); bearer tokens use TokenHash only.
type ApiKey struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Label       string    `gorm:"type:text;not null" json:"label"`
	TokenHash   string    `gorm:"type:text;uniqueIndex" json:"-"`
	AccessKeyID string    `gorm:"type:text;uniqueIndex" json:"access_key_id,omitempty"`
	Secret      string    `gorm:"type:text" json:"-"`
	Scopes      string    `gorm:"type:text;not null" json:"scopes"`
	Active      bool      `gorm:"type:boolean;not null;default:true" json:"active"`
	CreatedAt   time.Time `gorm:"type:timestamptz;not null;autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"type:timestamptz;not null;autoUpdateTime" json:"updated_at"`
}

func (k *ApiKey) BeforeCreate(tx *gorm.DB) (err error) {
	if k.ID == uuid.Nil {
		k.ID = uuid.New()
	}
	return nil
}

// Setting is a server-side key/value (e.g. the HMAC secret for signed URLs).
type Setting struct {
	Key   string `gorm:"primaryKey" json:"key"`
	Value string `gorm:"type:text;not null" json:"-"`
}
