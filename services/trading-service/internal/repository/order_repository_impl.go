package repository

import (
	"context"
	"errors"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type orderRepositoryImpl struct {
	db *gorm.DB
}

func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &orderRepositoryImpl{db: db}
}

func (r *orderRepositoryImpl) Create(ctx context.Context, order *model.Order) error {
	return r.db.WithContext(ctx).Create(order).Error
}

func (r *orderRepositoryImpl) FindByID(ctx context.Context, id uint) (*model.Order, error) {
	var order model.Order
	result := r.db.WithContext(ctx).Preload("Listing").Preload("Listing.Asset").First(&order, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return &order, result.Error
}

func (r *orderRepositoryImpl) Save(ctx context.Context, order *model.Order) error {
	return r.db.WithContext(ctx).Save(order).Error
}

func (r *orderRepositoryImpl) FindAll(ctx context.Context, page, pageSize int, userID *uint, ownerType *model.OwnerType, status *model.OrderStatus, direction *model.OrderDirection, isDone *bool) ([]model.Order, int64, error) {
	var orders []model.Order
	var count int64

	db := r.db.WithContext(ctx).Model(&model.Order{})

	if userID != nil {
		db = db.Where("user_id = ?", *userID)
	}
	if ownerType != nil {
		db = db.Where("owner_type = ?", *ownerType)
	}
	if status != nil {
		db = db.Where("status = ?", *status)
	}
	if direction != nil {
		db = db.Where("direction = ?", *direction)
	}
	if isDone != nil {
		db = db.Where("is_done = ?", *isDone)
	}

	if err := db.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := db.Preload("Listing").Preload("Listing.Asset").Limit(pageSize).Offset(offset).Find(&orders).Error
	return orders, count, err
}

func (r *orderRepositoryImpl) FindReadyForExecution(ctx context.Context, before time.Time, limit int) ([]model.Order, error) {
	var orders []model.Order

	query := r.db.WithContext(ctx).
		Preload("Listing").
		Preload("Listing.Asset").
		Where("status = ?", model.OrderStatusApproved).
		Where("is_done = ?", false).
		Where("next_execution_at IS NOT NULL").
		Where("next_execution_at <= ?", before).
		Order("next_execution_at ASC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&orders).Error; err != nil {
		return nil, err
	}

	return orders, nil
}

func (r *orderRepositoryImpl) FindUserOrders(ctx context.Context, userID uint, ownerType model.OwnerType, query dto.UserOrdersQuery) ([]model.Order, int64, error) {
	var orders []model.Order
	var count int64

	db := r.db.WithContext(ctx).Model(&model.Order{}).
		Where("order_owner_user_id = ? AND order_owner_type = ?", userID, ownerType)

	// Apply filters
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	if query.OrderType != nil {
		db = db.Where("order_type = ?", *query.OrderType)
	}

	if query.FromDate != nil {
		start := query.FromDate.Truncate(24 * time.Hour)
		db = db.Where("created_at >= ?", start)
	}

	if query.ToDate != nil {
		end := query.ToDate.Truncate(24 * time.Hour).Add(24 * time.Hour)
		db = db.Where("created_at < ?", end)
	}

	// Asset type filter requires join with listings and assets
	if query.AssetType != nil {
		db = db.Joins("JOIN listings ON listings.listing_id = orders.listing_id").
			Joins("JOIN assets ON assets.asset_id = listings.asset_id").
			Where("assets.asset_type = ?", *query.AssetType)
	}

	// Count total
	if err := db.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	// Pagination
	offset := (query.Page - 1) * query.PageSize
	err := db.
		Preload("Listing").
		Preload("Listing.Asset").
		Order("created_at DESC").
		Limit(query.PageSize).
		Offset(offset).
		Find(&orders).Error

	return orders, count, err
}
