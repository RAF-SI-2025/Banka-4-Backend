package model

import "time"

type TransactionStatus string

const (
	TransactionProcessing TransactionStatus = "processing"
	TransactionCompleted  TransactionStatus = "completed"
	TransactionRejected   TransactionStatus = "rejected"
)

type Transaction struct {
	TransactionID          uint              `gorm:"primaryKey"`
	PayerAccountNumber     string            `gorm:"not null"`
	RecipientAccountNumber string            `gorm:"not null"`
	StartAmount            float64           `gorm:"not null"`
	StartCurrencyCode      CurrencyCode      `gorm:"not null; currency_code"`
	EndAmount              float64           `gorm:"not null"`
	EndCurrencyCode        CurrencyCode      `gorm:"not null; currency_code"`
	Status                 TransactionStatus `gorm:"not null"`
	CreatedAt              time.Time
}
