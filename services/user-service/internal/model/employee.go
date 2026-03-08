package model

import (
	"time"
)

type Employee struct {
	EmployeeID  uint   `gorm:"primaryKey"`
	FirstName   string `gorm:"size:20;not null"`
	LastName    string `gorm:"size:100;not null"`
	Gender      string `gorm:"size:10"`
	DateOfBirth time.Time
	Email       string `gorm:"size:100;uniqueIndex"`
	PhoneNumber string `gorm:"size:20"`
	Address     string `gorm:"size:255"`
	Username    string `gorm:"size:50;uniqueIndex"`
	Password    string `gorm:"size:33"`
	Active      bool
	Department  string `gorm:"size:100"`
	PositionID  uint   `gorm:""`
	Position    Position
}
