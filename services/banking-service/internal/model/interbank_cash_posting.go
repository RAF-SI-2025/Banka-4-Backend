package model

import "time"

type InterbankCashPostingStatus string

const (
	InterbankCashPostingPrepared   InterbankCashPostingStatus = "PREPARED"
	InterbankCashPostingCommitted  InterbankCashPostingStatus = "COMMITTED"
	InterbankCashPostingRolledBack InterbankCashPostingStatus = "ROLLED_BACK"
)

type InterbankCashPosting struct {
	PostingID     string                     `gorm:"primaryKey;size:128"`
	AccountNumber string                     `gorm:"not null;size:18;index"`
	CurrencyCode  CurrencyCode               `gorm:"not null;size:4"`
	Amount        float64                    `gorm:"not null"`
	Status        InterbankCashPostingStatus `gorm:"not null;size:20"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
