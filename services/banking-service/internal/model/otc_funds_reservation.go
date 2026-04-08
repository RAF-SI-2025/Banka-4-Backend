package model

import "time"

type OtcFundsReservationStatus string

const (
	OtcFundsReservationStatusReserved  OtcFundsReservationStatus = "RESERVED"
	OtcFundsReservationStatusReleased  OtcFundsReservationStatus = "RELEASED"
	OtcFundsReservationStatusCommitted OtcFundsReservationStatus = "COMMITTED"
	OtcFundsReservationStatusRefunded  OtcFundsReservationStatus = "REFUNDED"
)

type OtcFundsReservation struct {
	OtcFundsReservationID uint   `gorm:"primaryKey;autoIncrement"`
	ExecutionID           string `gorm:"not null;uniqueIndex;size:100"`

	BuyerAccountNumber  string       `gorm:"not null;size:18;index"`
	SellerAccountNumber string       `gorm:"not null;size:18;index"`
	TradeAmount         float64      `gorm:"not null"`
	TradeCurrencyCode   CurrencyCode `gorm:"not null;size:4"`

	SourceAmount       float64      `gorm:"not null"`
	SourceCurrencyCode CurrencyCode `gorm:"not null;size:4"`

	DestinationAmount       float64      `gorm:"not null"`
	DestinationCurrencyCode CurrencyCode `gorm:"not null;size:4"`

	Status    OtcFundsReservationStatus `gorm:"not null;size:20"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
