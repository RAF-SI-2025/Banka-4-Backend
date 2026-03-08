package repository

import (
	"context"
	"user-service/internal/model"
)

type EmployeeRepository interface {
	Create(ctx context.Context, employee *model.Employee) error
	FindByEmail(ctx context.Context, email string) (*model.Employee, error)
}
