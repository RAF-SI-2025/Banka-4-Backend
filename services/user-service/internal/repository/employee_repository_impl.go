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

func (r *employeeRepository) FindByUserName(ctx context.Context, userName string) (*model.Employee, error) {
	var employee model.Employee
	result := r.db.
		WithContext(ctx).
		Where("username = ?", userName).
		First(&employee)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &employee, result.Error
}

func (r *employeeRepository) Update(ctx context.Context, employee *model.Employee) error {
	return r.db.WithContext(ctx).Save(employee).Error
}

func (r *employeeRepository) FindByID(ctx context.Context, id uint) (*model.Employee, error) {
	var e model.Employee
	result := r.db.WithContext(ctx).First(&e, id)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	if result.Error != nil {
		return nil, result.Error
	}
	return &e, nil
}

func (r *employeeRepository) GetAll(ctx context.Context, email, firstName, lastName, position string, page, pageSize int) ([]model.Employee, int64, error) {
	var employees []model.Employee
	var total int64

	query := r.db.WithContext(ctx).
    Model(&model.Employee{}).
    Preload("Position").
    Joins("LEFT JOIN positions ON positions.position_id = employees.position_id")

	// Filter
	if email != "" {
		query = query.Where("email ILIKE ?", "%"+email+"%")
	}
	if firstName != "" {
		query = query.Where("first_name ILIKE ?", "%"+firstName+"%")
	}
	if lastName != "" {
		query = query.Where("last_name ILIKE ?", "%"+lastName+"%")
	}
	if position != "" {
		query = query.Where("positions.name ILIKE ?", "%"+position+"%")
	}

	// Get total
	if err := query.Model(&model.Employee{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Pagination
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("employee_id DESC").Find(&employees).Error; err != nil {
		return nil, 0, err
	}

	return employees, total, nil
}
