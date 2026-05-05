package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/model"
)

type actuaryRepository struct {
	db *gorm.DB
}

func NewActuaryRepository(db *gorm.DB) ActuaryRepository {
	return &actuaryRepository{db: db}
}

func (r *actuaryRepository) FindByEmployeeID(ctx context.Context, employeeID uint) (*model.ActuaryInfo, error) {
	var actuary model.ActuaryInfo

	result := r.db.WithContext(ctx).First(&actuary, "employee_id = ?", employeeID)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}

	return &actuary, nil
}

func (r *actuaryRepository) GetAll(ctx context.Context, email, firstName, lastName, position, department, actuaryType string, active, needApproval *bool, page, pageSize int) ([]model.Employee, int64, error) {
	var employees []model.Employee
	var total int64

	query := r.db.WithContext(ctx).
		Model(&model.Employee{}).
		Preload("Position").
		Preload("Permissions").
		Preload("ActuaryInfo").
		Preload("Identity").
		Joins("JOIN actuary_infos ON actuary_infos.employee_id = employees.employee_id").
		Joins("LEFT JOIN positions ON positions.position_id = employees.position_id").
		Joins("LEFT JOIN identities ON identities.id = employees.identity_id").
		Where("actuary_infos.is_agent = ? OR actuary_infos.is_supervisor = ?", true, true)

	if email != "" {
		query = query.Where("identities.email ILIKE ?", "%"+email+"%")
	}
	if firstName != "" {
		query = query.Where("employees.first_name ILIKE ?", "%"+firstName+"%")
	}
	if lastName != "" {
		query = query.Where("employees.last_name ILIKE ?", "%"+lastName+"%")
	}
	if position != "" {
		query = query.Where("positions.title ILIKE ?", "%"+position+"%")
	}
	if department != "" {
		query = query.Where("employees.department ILIKE ?", "%"+department+"%")
	}
	if actuaryType == "agent" {
		query = query.Where("actuary_infos.is_agent = ?", true)
	}
	if actuaryType == "supervisor" {
		query = query.Where("actuary_infos.is_supervisor = ?", true)
	}
	if active != nil {
		query = query.Where("identities.active = ?", *active)
	}
	if needApproval != nil {
		query = query.Where("actuary_infos.need_approval = ?", *needApproval)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("employees.employee_id DESC").Find(&employees).Error; err != nil {
		return nil, 0, err
	}

	return employees, total, nil
}

func (r *actuaryRepository) Save(ctx context.Context, actuary *model.ActuaryInfo) error {
	return r.db.WithContext(ctx).Save(actuary).Error
}

func (r *actuaryRepository) IncrementUsedLimit(ctx context.Context, employeeID uint, amount float64) (*model.ActuaryInfo, error) {
	result := r.db.WithContext(ctx).
		Model(&model.ActuaryInfo{}).
		Where("employee_id = ? AND is_agent = ?", employeeID, true).
		UpdateColumn("used_limit", gorm.Expr("used_limit + ?", amount))
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return r.FindByEmployeeID(ctx, employeeID)
}

func (r *actuaryRepository) ResetUsedLimit(ctx context.Context, employeeID uint) error {
	return r.db.WithContext(ctx).
		Model(&model.ActuaryInfo{}).
		Where("employee_id = ?", employeeID).
		Update("used_limit", 0).
		Error
}

func (r *actuaryRepository) ResetAllUsedLimits(ctx context.Context) error {
	return r.db.WithContext(ctx).
		Model(&model.ActuaryInfo{}).
		Where("is_agent = ?", true).
		Update("used_limit", 0).
		Error
}
