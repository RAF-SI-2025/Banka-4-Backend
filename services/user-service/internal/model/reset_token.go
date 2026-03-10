package model

import "time"

type ResetToken struct {
	ID         uint      `gorm:"primaryKey"`
	EmployeeID uint      `gorm:"not null;index"`
	Token      string    `gorm:"not null"`
	ExpiresAt  time.Time `gorm:"not null"`
}
