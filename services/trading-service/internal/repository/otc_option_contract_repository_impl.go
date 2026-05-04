package repository

import (
	"context"
	stderrors "errors"
	"time"

	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type otcOptionContractRepositoryImpl struct {
	db *gorm.DB
}

func NewOtcOptionContractRepository(db *gorm.DB) OtcOptionContractRepository {
	return &otcOptionContractRepositoryImpl{db: db}
}

func (r *otcOptionContractRepositoryImpl) Create(ctx context.Context, contract *model.OtcOptionContract) error {
	return r.db.WithContext(ctx).Create(contract).Error
}

func (r *otcOptionContractRepositoryImpl) Save(ctx context.Context, contract *model.OtcOptionContract) error {
	return r.db.WithContext(ctx).Save(contract).Error
}

func (r *otcOptionContractRepositoryImpl) FindByID(ctx context.Context, id uint) (*model.OtcOptionContract, error) {
	var contract model.OtcOptionContract
	result := r.db.WithContext(ctx).
		Preload("Stock").
		Preload("Stock.Asset").
		First(&contract, id)
	if stderrors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &contract, nil
}

func (r *otcOptionContractRepositoryImpl) FindForUser(ctx context.Context, userID uint) ([]model.OtcOptionContract, error) {
	var contracts []model.OtcOptionContract
	err := r.db.WithContext(ctx).
		Preload("Stock").
		Preload("Stock.Asset").
		Where("buyer_id = ? OR seller_id = ?", userID, userID).
		Order("created_at DESC").
		Find(&contracts).Error
	return contracts, err
}

func (r *otcOptionContractRepositoryImpl) FindActiveBySellerAndStock(ctx context.Context, sellerID, stockID uint, now time.Time) ([]model.OtcOptionContract, error) {
	var contracts []model.OtcOptionContract
	err := r.db.WithContext(ctx).
		Where("seller_id = ? AND stock_asset_id = ? AND is_exercised = false AND settlement_date > ?", sellerID, stockID, now).
		Find(&contracts).Error
	return contracts, err
}
