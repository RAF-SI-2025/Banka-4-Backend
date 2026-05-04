package repository

import (
	"context"
	stderrors "errors"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type assetOwnershipRepository struct {
	db *gorm.DB
}

func NewAssetOwnershipRepository(db *gorm.DB) AssetOwnershipRepository {
	return &assetOwnershipRepository{db: db}
}

func (r *assetOwnershipRepository) FindByUserId(ctx context.Context, userId uint, ownerType model.OwnerType) ([]model.AssetOwnership, error) {
	var ownerships []model.AssetOwnership
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND owner_type = ?", userId, ownerType).
		Preload("Asset").
		Find(&ownerships).Error; err != nil {
		return nil, err
	}
	return ownerships, nil
}

func (r *assetOwnershipRepository) FindByID(ctx context.Context, id uint) (*model.AssetOwnership, error) {
	var o model.AssetOwnership
	result := r.db.WithContext(ctx).Preload("Asset").First(&o, id)
	if stderrors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &o, result.Error
}

func (r *assetOwnershipRepository) Upsert(ctx context.Context, ownership *model.AssetOwnership) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "owner_type"}, {Name: "asset_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"amount", "avg_buy_price_rsd", "updated_at"}),
		}).
		Create(ownership).Error
}

func (r *assetOwnershipRepository) IncreaseReservedAmount(ctx context.Context, identityID uint, ownerType model.OwnerType, assetID uint, delta float64) error {
	return r.db.WithContext(ctx).
		Model(&model.AssetOwnership{}).
		Where("user_id = ? AND owner_type = ? AND asset_id = ?", identityID, ownerType, assetID).
		UpdateColumn("reserved_amount", gorm.Expr("reserved_amount + ?", delta)).Error
}

func (r *assetOwnershipRepository) FindAllPublic(ctx context.Context, page, pageSize int) ([]model.AssetOwnership, int64, error) {
	var ownerships []model.AssetOwnership
	var count int64

	db := r.db.WithContext(ctx).Model(&model.AssetOwnership{}).Where("public_amount > 0")
	if err := db.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := db.Preload("Asset").Limit(pageSize).Offset(offset).Find(&ownerships).Error
	return ownerships, count, err
}

func (r *assetOwnershipRepository) UpdateOTCFields(ctx context.Context, ownershipID uint, publicAmount, reservedAmount float64) error {
	return r.db.WithContext(ctx).
		Model(&model.AssetOwnership{}).
		Where("asset_ownership_id = ?", ownershipID).
		Updates(map[string]any{
			"public_amount":   publicAmount,
			"reserved_amount": reservedAmount,
			"updated_at":      time.Now(),
		}).Error
}
