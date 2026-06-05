package model

import "time"

type InterbankReservationStatus string

const (
	InterbankReservationStatusReserved   InterbankReservationStatus = "RESERVED"
	InterbankReservationStatusCommitted  InterbankReservationStatus = "COMMITTED"
	InterbankReservationStatusRolledBack InterbankReservationStatus = "ROLLED_BACK"
)

type InterbankReservation struct {
	ID                 uint                       `gorm:"primaryKey;autoIncrement"`
	PendingBankingTxID uint                       `gorm:"not null;uniqueIndex"`
	Status             InterbankReservationStatus `gorm:"not null;size:20"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
