package repository

import (
	"context"
	stderrors "errors"
	"time"

	commondb "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type otcOptionContractRepositoryImpl struct {
	db *gorm.DB
}

func NewOtcOptionContractRepository(db *gorm.DB) OtcOptionContractRepository {
	return &otcOptionContractRepositoryImpl{db: db}
}

func (r *otcOptionContractRepositoryImpl) Create(ctx context.Context, contract *model.OtcOptionContract) error {
	return commondb.DBFromContext(ctx, r.db).Create(contract).Error
}

func (r *otcOptionContractRepositoryImpl) Save(ctx context.Context, contract *model.OtcOptionContract) error {
	return commondb.DBFromContext(ctx, r.db).Save(contract).Error
}

func (r *otcOptionContractRepositoryImpl) FindByID(ctx context.Context, id uint) (*model.OtcOptionContract, error) {
	return r.findByID(ctx, id, false)
}

func (r *otcOptionContractRepositoryImpl) FindByIDForUpdate(ctx context.Context, id uint) (*model.OtcOptionContract, error) {
	return r.findByID(ctx, id, true)
}

func (r *otcOptionContractRepositoryImpl) FindByOfferID(ctx context.Context, offerID uint) (*model.OtcOptionContract, error) {
	var contract model.OtcOptionContract
	result := commondb.DBFromContext(ctx, r.db).
		Preload("Stock").
		Preload("Stock.Asset").
		Where("otc_offer_id = ?", offerID).
		First(&contract)
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
	err := commondb.DBFromContext(ctx, r.db).
		Preload("Stock").
		Preload("Stock.Asset").
		Preload("Stock.Listing.Exchange").
		Where("buyer_id = ? OR seller_id = ?", userID, userID).
		Order("created_at DESC").
		Find(&contracts).Error
	return contracts, err
}

func (r *otcOptionContractRepositoryImpl) FindActiveBySellerAndStock(ctx context.Context, sellerID, stockID uint, now time.Time) ([]model.OtcOptionContract, error) {
	var contracts []model.OtcOptionContract
	err := commondb.DBFromContext(ctx, r.db).
		Where("seller_id = ? AND stock_asset_id = ? AND status = ? AND settlement_date > ?", sellerID, stockID, model.OtcOptionContractStatusActive, now).
		Find(&contracts).Error
	return contracts, err
}

func (r *otcOptionContractRepositoryImpl) FindExpiredActive(ctx context.Context, before time.Time, limit int) ([]model.OtcOptionContract, error) {
	var contracts []model.OtcOptionContract
	query := commondb.DBFromContext(ctx, r.db).
		Where("status = ? AND settlement_date <= ?", model.OtcOptionContractStatusActive, before).
		Order("settlement_date ASC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&contracts).Error; err != nil {
		return nil, err
	}
	return contracts, nil
}

func (r *otcOptionContractRepositoryImpl) findByID(ctx context.Context, id uint, forUpdate bool) (*model.OtcOptionContract, error) {
	query := commondb.DBFromContext(ctx, r.db).
		Preload("Stock").
		Preload("Stock.Asset")

	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	var contract model.OtcOptionContract
	result := query.First(&contract, id)

	if stderrors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	if result.Error != nil {
		return nil, result.Error
	}

	return &contract, nil
}
func (r *otcOptionContractRepositoryImpl) FindExpiringContracts(ctx context.Context, before time.Time) ([]model.OtcOptionContract, error) {
	var contracts []model.OtcOptionContract

	err := commondb.DBFromContext(ctx, r.db).
		Where("status = ? AND settlement_date <= ?", model.OtcOptionContractStatusActive, before).
		Find(&contracts).Error

	return contracts, err
}
