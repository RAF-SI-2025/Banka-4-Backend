package model

import "common/pkg/permission"

type EmployeePermission struct {
	EmployeeID uint                  `gorm:"primaryKey"`
	Permission permission.Permission `gorm:"type:varchar(64);primaryKey"`
}

func (EmployeePermission) TableName() string {
	return "employee_permissions"
}
