package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type recurringOrderRepositoryImpl struct {
	db *gorm.DB
}

func NewRecurringOrderRepository(db *gorm.DB) RecurringOrderRepository {
	return &recurringOrderRepositoryImpl{db: db}
}

func (r *recurringOrderRepositoryImpl) Create(ctx context.Context, ro *model.RecurringOrder) error {
	return r.db.WithContext(ctx).Create(ro).Error
}

func (r *recurringOrderRepositoryImpl) FindByID(ctx context.Context, id uint) (*model.RecurringOrder, error) {
	var ro model.RecurringOrder
	result := r.db.WithContext(ctx).Preload("Listing").Preload("Listing.Asset").First(&ro, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &ro, result.Error
}

func (r *recurringOrderRepositoryImpl) Save(ctx context.Context, ro *model.RecurringOrder) error {
	return r.db.WithContext(ctx).Save(ro).Error
}

func (r *recurringOrderRepositoryImpl) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.RecurringOrder{}, id).Error
}

func (r *recurringOrderRepositoryImpl) FindByUser(ctx context.Context, userID uint, ownerType model.OwnerType) ([]model.RecurringOrder, error) {
	var orders []model.RecurringOrder
	err := r.db.WithContext(ctx).
		Preload("Listing").
		Preload("Listing.Asset").
		Where("user_id = ? AND owner_type = ?", userID, ownerType).
		Order("created_at DESC").
		Find(&orders).Error
	return orders, err
}

func (r *recurringOrderRepositoryImpl) FindDue(ctx context.Context, before time.Time) ([]model.RecurringOrder, error) {
	var orders []model.RecurringOrder
	err := r.db.WithContext(ctx).
		Preload("Listing").
		Preload("Listing.Asset").
		Where("active = ? AND next_run <= ?", true, before).
		Order("next_run ASC").
		Find(&orders).Error
	return orders, err
}
