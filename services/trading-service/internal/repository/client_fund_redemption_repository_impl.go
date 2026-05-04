package repository

import (
	"context"
	"database/sql"

	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type clientFundRedemptionRepository struct {
	db *gorm.DB
}

func NewClientFundRedemptionRepository(db *gorm.DB) ClientFundRedemptionRepository {
	return &clientFundRedemptionRepository{db: db}
}

func (r *clientFundRedemptionRepository) Create(ctx context.Context, redemption *model.ClientFundRedemption) error {
	return r.db.WithContext(ctx).Create(redemption).Error
}

func (r *clientFundRedemptionRepository) Update(ctx context.Context, redemption *model.ClientFundRedemption) error {
	return r.db.WithContext(ctx).Save(redemption).Error
}

func (r *clientFundRedemptionRepository) FindPending(ctx context.Context, limit int) ([]model.ClientFundRedemption, error) {
	var redemptions []model.ClientFundRedemption
	query := r.db.WithContext(ctx).
		Preload("Fund").
		Where("status = ?", model.FundRedemptionPendingLiquidation).
		Order("created_at ASC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&redemptions).Error
	return redemptions, err
}

func (r *clientFundRedemptionRepository) SumPendingByClientAndFund(ctx context.Context, clientID uint, ownerType model.OwnerType, fundID uint) (float64, error) {
	var total sql.NullFloat64
	err := r.db.WithContext(ctx).
		Model(&model.ClientFundRedemption{}).
		Select("SUM(amount)").
		Where("client_id = ? AND owner_type = ? AND fund_id = ? AND status = ?", clientID, ownerType, fundID, model.FundRedemptionPendingLiquidation).
		Scan(&total).Error
	if err != nil {
		return 0, err
	}
	if !total.Valid {
		return 0, nil
	}
	return total.Float64, nil
}
