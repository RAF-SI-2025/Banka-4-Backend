package repository

import (
	"context"

	commondb "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"gorm.io/gorm"
)

type dividendPayoutRepository struct {
	db *gorm.DB
}

func NewDividendPayoutRepository(db *gorm.DB) DividendPayoutRepository {
	return &dividendPayoutRepository{db: db}
}

func (r *dividendPayoutRepository) Save(ctx context.Context, payout *model.DividendPayout) error {
	return commondb.DBFromContext(ctx, r.db).Create(payout).Error
}

func (r *dividendPayoutRepository) FindAll(ctx context.Context) ([]model.DividendPayout, error) {
	var payouts []model.DividendPayout
	err := commondb.DBFromContext(ctx, r.db).
		Preload("AssetOwnership").
		Preload("AssetOwnership.Asset").
		Order("payment_date DESC").
		Find(&payouts).Error
	return payouts, err
}

func (r *dividendPayoutRepository) FindAllByAssetOwnershipID(ctx context.Context, assetOwnershipID uint) ([]model.DividendPayout, error) {
	var payouts []model.DividendPayout
	err := commondb.DBFromContext(ctx, r.db).
		Where("asset_ownership_id = ?", assetOwnershipID).
		Preload("AssetOwnership").
		Preload("AssetOwnership.Asset").
		Order("payment_date DESC").
		Find(&payouts).Error
	return payouts, err
}
