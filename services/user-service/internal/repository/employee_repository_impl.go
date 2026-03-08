package repository

import (
	"context"
	"errors"

	"user-service/internal/model"

	"gorm.io/gorm"
)

type employeeRepository struct {
	db *gorm.DB
}

func NewEmployeeRepository(db *gorm.DB) EmployeeRepository {
	return &employeeRepository{db: db}
}
func (r *employeeRepository) Create(ctx context.Context, employee *model.Employee) error {
	return r.db.WithContext(ctx).Create(employee).Error
}

func (r *employeeRepository) FindByEmail(ctx context.Context, email string) (*model.Employee, error) {

	var employee model.Employee

	result := r.db.
		WithContext(ctx).
		Where("email = ?", email).
		First(&employee)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return &employee, result.Error
}
