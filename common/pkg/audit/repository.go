package audit

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type Repository interface {
	Save(ctx context.Context, entry *AuditLog) error
	GetAll(ctx context.Context, actionType string, performedByEmployeeID *uint, dateFrom, dateTo *time.Time, page, pageSize int) ([]AuditLog, int64, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Save(ctx context.Context, entry *AuditLog) error {
	return r.db.WithContext(ctx).Create(entry).Error
}

func (r *repository) GetAll(ctx context.Context, actionType string, performedByEmployeeID *uint, dateFrom, dateTo *time.Time, page, pageSize int) ([]AuditLog, int64, error) {
	var entries []AuditLog
	var total int64

	query := r.db.WithContext(ctx).Model(&AuditLog{})

	if actionType != "" {
		query = query.Where("action_type = ?", actionType)
	}
	if performedByEmployeeID != nil {
		query = query.Where("performed_by_employee_id = ?", *performedByEmployeeID)
	}
	if dateFrom != nil {
		query = query.Where("created_at >= ?", *dateFrom)
	}
	if dateTo != nil {
		query = query.Where("created_at <= ?", *dateTo)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&entries).Error; err != nil {
		return nil, 0, err
	}

	return entries, total, nil
}
