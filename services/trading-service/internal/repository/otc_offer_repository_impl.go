package repository

import (
	"context"
	stderrors "errors"

	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type otcOfferRepositoryImpl struct {
	db *gorm.DB
}

func NewOtcOfferRepository(db *gorm.DB) OtcOfferRepository {
	return &otcOfferRepositoryImpl{db: db}
}

func (r *otcOfferRepositoryImpl) Create(ctx context.Context, offer *model.OtcOffer) error {
	return r.db.WithContext(ctx).Create(offer).Error
}

func (r *otcOfferRepositoryImpl) Save(ctx context.Context, offer *model.OtcOffer) error {
	return r.db.WithContext(ctx).Save(offer).Error
}

func (r *otcOfferRepositoryImpl) FindByID(ctx context.Context, id uint) (*model.OtcOffer, error) {
	var offer model.OtcOffer
	result := r.db.WithContext(ctx).
		Preload("Stock").
		Preload("Stock.Asset").
		First(&offer, id)
	if stderrors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &offer, nil
}

func (r *otcOfferRepositoryImpl) FindActiveForUser(ctx context.Context, userID uint) ([]model.OtcOffer, error) {
	var offers []model.OtcOffer
	err := r.db.WithContext(ctx).
		Preload("Stock").
		Preload("Stock.Asset").
		Where("(buyer_id = ? OR seller_id = ?) AND status = ?", userID, userID, model.OtcOfferStatusActive).
		Order("last_modified DESC").
		Find(&offers).Error
	return offers, err
}

func (r *otcOfferRepositoryImpl) FindActiveBySellerAndStock(ctx context.Context, sellerID, stockAssetID uint, excludeOfferID *uint) ([]model.OtcOffer, error) {
	var offers []model.OtcOffer
	q := r.db.WithContext(ctx).
		Where("seller_id = ? AND stock_asset_id = ? AND status = ?", sellerID, stockAssetID, model.OtcOfferStatusActive)
	if excludeOfferID != nil {
		q = q.Where("otc_offer_id <> ?", *excludeOfferID)
	}
	err := q.Find(&offers).Error
	return offers, err
}
