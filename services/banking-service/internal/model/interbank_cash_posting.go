package model

import "time"

type InterbankCashPostingStatus string

const (
	InterbankCashPostingPrepared   InterbankCashPostingStatus = "PREPARED"
	InterbankCashPostingCommitted  InterbankCashPostingStatus = "COMMITTED"
	InterbankCashPostingRolledBack InterbankCashPostingStatus = "ROLLED_BACK"
)

type InterbankCashPosting struct {
	PostingID string `gorm:"primaryKey;size:128"`
	// AccountNumber, CurrencyCode and Amount hold the resolved values applied to
	// the local account: the account that was selected, its currency, and the
	// amount already converted into that currency (frozen at prepare time).
	AccountNumber string       `gorm:"not null;size:18;index"`
	CurrencyCode  CurrencyCode `gorm:"not null;size:4"`
	Amount        float64      `gorm:"not null"`
	// RequestedCurrencyCode and RequestedAmount preserve the original posting
	// request so retries with the same posting id can be checked for idempotency.
	RequestedCurrencyCode CurrencyCode               `gorm:"not null;size:4"`
	RequestedAmount       float64                    `gorm:"not null"`
	Status                InterbankCashPostingStatus `gorm:"not null;size:20"`
	// Payment-history metadata captured at prepare time and used to build a
	// Transaction+Payment record when the posting commits. BankingTxID is non-zero
	// only for the initiating payment leg (which already has a Transaction), so it
	// gates record creation off for that case. The rest may be empty.
	BankingTxID               uint64 `gorm:"not null;default:0"`
	CounterpartyAccountNumber string `gorm:"size:64"`
	PaymentCode               string `gorm:"size:64"`
	Purpose                   string `gorm:"size:255"`
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}
