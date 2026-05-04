package model

import "time"

// FundPerformance stores daily snapshots for performance charts.
type FundPerformance struct {
	ID           uint      `gorm:"primaryKey;autoIncrement"`
	FundID       uint      `gorm:"not null;index"`
	Date         time.Time `gorm:"not null;index"`
	FundValue    float64   `gorm:"not null"` // total market value of holdings
	LiquidAssets float64   `gorm:"not null"` // cash on fund's account
	Profit       float64   `gorm:"not null"` // fund value - total invested
	CreatedAt    time.Time
}
