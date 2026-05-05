package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/model"
)

type ActuaryRepository interface {
	FindByEmployeeID(ctx context.Context, employeeID uint) (*model.ActuaryInfo, error)
	GetAll(ctx context.Context, email, firstName, lastName, position, department, actuaryType string, active, needApproval *bool, page, pageSize int) ([]model.Employee, int64, error)
	Save(ctx context.Context, actuary *model.ActuaryInfo) error
	IncrementUsedLimit(ctx context.Context, employeeID uint, amount float64) (*model.ActuaryInfo, error)
	ResetUsedLimit(ctx context.Context, employeeID uint) error
	ResetAllUsedLimits(ctx context.Context) error
}
