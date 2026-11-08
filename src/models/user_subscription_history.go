package models

import "time"

// UserSubscriptionHistory mirrors the subscription primitive needed by Edge.
type UserSubscriptionHistory struct {
	ID         string     `gorm:"column:id"`
	UserID     string     `gorm:"column:userId"`
	PaymentID  *string    `gorm:"column:paymentId"`
	ChangeType string     `gorm:"column:changeType"`
	ExpiresAt  *time.Time `gorm:"column:expiresAt"`
	CreatedAt  time.Time  `gorm:"column:createdAt"`
}
