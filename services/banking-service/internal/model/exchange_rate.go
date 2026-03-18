package model

import "time"

const BankCommission = 0.015

type ExchangeRate struct {
	CurrencyCode         CurrencyCode `gorm:"primaryKey;size:4"`
	BaseCurrency         CurrencyCode `gorm:"not null;default:'RSD';size:4"`
	BuyRate              float64      `gorm:"not null"`
	MiddleRate           float64      `gorm:"not null"`
	SellRate             float64      `gorm:"not null"`
	ProviderUpdatedAt    time.Time    `gorm:"not null"`
	ProviderNextUpdateAt time.Time    `gorm:"not null"`
}
