package repository

import (
	"context"
	"user-service/internal/model"
)

type EmployeeRepository interface {
	Create(ctx context.Context, employee *model.Employee) error
	FindByEmail(ctx context.Context, email string) (*model.Employee, error)
	FindByUserName(ctx context.Context, userName string) (*model.Employee, error)
	Update(ctx context.Context, employee *model.Employee) error
	FindByID(ctx context.Context, id uint) (*model.Employee, error)
  	GetAll(ctx context.Context, email, firstName, lastName, position string, page, pageSize int) ([]model.Employee, int64, error)

}
