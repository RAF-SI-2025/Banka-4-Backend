package model

import (
	"common/pkg/permission"
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
	Password    string `gorm:"size:255"`
	Active      bool
	Department  string `gorm:"size:100"`
	PositionID  uint   `gorm:""`
	Position    Position
	Permissions []EmployeePermission `gorm:"foreignKey:EmployeeID"`
}

func (e *Employee) HasPermission(p permission.Permission) bool {
	for _, ep := range e.Permissions {
		if ep.Permission == p {
			return true
		}
	}
	return false
}
