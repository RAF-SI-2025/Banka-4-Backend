package model

import "time"

type RecurringOrderMode string
type RecurringOrderCadence string

const (
	RecurringOrderModeByQuantity RecurringOrderMode = "BY_QUANTITY"
	RecurringOrderModeByAmount   RecurringOrderMode = "BY_AMOUNT"
)

const (
	RecurringOrderCadenceDaily   RecurringOrderCadence = "DAILY"
	RecurringOrderCadenceWeekly  RecurringOrderCadence = "WEEKLY"
	RecurringOrderCadenceMonthly RecurringOrderCadence = "MONTHLY"
)

type RecurringOrder struct {
	RecurringOrderID uint      `gorm:"primaryKey;autoIncrement"`
	UserID           uint      `gorm:"not null;index"`
	OwnerType        OwnerType `gorm:"not null;size:10"`
	ListingID        uint      `gorm:"not null;index"`
	Listing          Listing
	Direction        OrderDirection        `gorm:"not null;size:4"`
	Mode             RecurringOrderMode    `gorm:"not null;size:20"`
	Value            float64               `gorm:"not null"`
	AccountNumber    string                `gorm:"not null;size:18"`
	Cadence          RecurringOrderCadence `gorm:"not null;size:10"`
	NextRun          time.Time             `gorm:"not null"`
	Active           bool                  `gorm:"not null;default:true"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
