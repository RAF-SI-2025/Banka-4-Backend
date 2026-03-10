package model

import "time"

type ActivationToken struct {
	ID         uint      `gorm:"primaryKey"`
	EmployeeID uint      `gorm:"not null;index"`
	Token      string    `gorm:"unique;not null"`
	ExpiresAt  time.Time `gorm:"not null"`
}
