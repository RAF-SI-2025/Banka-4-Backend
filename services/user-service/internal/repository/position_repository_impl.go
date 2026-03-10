package repository

import (
	"context"
	"user-service/internal/model"
	
	"gorm.io/gorm"
)

type positionRepository struct {
	db *gorm.DB
}

func NewPositionRepository(db *gorm.DB) PositionRepository {
	return &positionRepository{db: db}
}

func (r *positionRepository) Exists(ctx context.Context, id uint) (bool, error) {
	var count int64

	result := r.db.
		WithContext(ctx).
		Model(&model.Position{}).
		Where("position_id = ?", id).
		Count(&count)

	if result.Error != nil {
		return false, result.Error
	}

	return count > 0, nil
}
