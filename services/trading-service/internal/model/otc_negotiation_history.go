package model

import "time"

type OtcNegotiationHistory struct {
	OtcNegotiationHistoryID uint `gorm:"primaryKey;autoIncrement"`

	OtcOfferID uint `gorm:"not null;index"`

	OldAmount           int
	NewAmount           int
	OldPricePerStockRSD float64 `gorm:"column:old_price_per_stock"`
	NewPricePerStockRSD float64 `gorm:"column:new_price_per_stock"`
	OldPremiumRSD       float64 `gorm:"column:old_premium"`
	NewPremiumRSD       float64 `gorm:"column:new_premium"`
	OldSettlementDate   time.Time
	NewSettlementDate   time.Time

	Timestamp  time.Time `gorm:"not null"`
	ModifiedBy uint      `gorm:"not null;index"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
