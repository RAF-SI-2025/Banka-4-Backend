package model

import "time"

type DividendPayout struct {
	DividendPayoutID uint `gorm:"primaryKey;autoIncrement"`
	AssetOwnershipID uint `gorm:"not null;index"`
	AssetOwnership   AssetOwnership
	Quantity         float64   `gorm:"not null"`
	PricePerShare    float64   `gorm:"not null"`
	GrossAmount      float64   `gorm:"not null"`
	TaxAmount        float64   `gorm:"not null;default:0"`
	NetAmount        float64   `gorm:"not null"`
	CurrencyCode     string    `gorm:"not null;size:10"`
	AccountNumber    string    `gorm:"not null"`
	PaymentDate      time.Time `gorm:"not null;index"`
}
