package repository

import (
	"context"
	"errors"
	"user-service/internal/model"

	"gorm.io/gorm"
)

type resetTokenRepository struct {
	db *gorm.DB
}

func NewResetTokenRepository(db *gorm.DB) ResetTokenRepository {
	return &resetTokenRepository{db: db}
}

func (r *resetTokenRepository) Create(ctx context.Context, token *model.ResetToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *resetTokenRepository) FindByToken(ctx context.Context, token string) (*model.ResetToken, error) {
	var t model.ResetToken
	result := r.db.WithContext(ctx).Where("token = ?", token).First(&t)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &t, nil
}

func (r *resetTokenRepository) DeleteByEmployeeID(ctx context.Context, employeeID uint) error {
	return r.db.WithContext(ctx).Where("employee_id = ?", employeeID).Delete(&model.ResetToken{}).Error
}
