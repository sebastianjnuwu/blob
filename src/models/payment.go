package models

// Payment mirrors the payment primitive needed by the Edge gateway.
type Payment struct {
	ID     string `gorm:"column:id"`
	Status string `gorm:"column:status"`
}
