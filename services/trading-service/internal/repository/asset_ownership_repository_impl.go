package repository

import (
	"context"
	stderrors "errors"
	"time"

	commondb "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
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
	if err := commondb.DBFromContext(ctx, r.db).
		Where("user_id = ? AND owner_type = ?", userId, ownerType).
		Preload("Asset").
		Find(&ownerships).Error; err != nil {
		return nil, err
	}
	return ownerships, nil
}

func (r *assetOwnershipRepository) FindByOwnerType(ctx context.Context, ownerType model.OwnerType) ([]model.AssetOwnership, error) {
	var ownerships []model.AssetOwnership
	if err := commondb.DBFromContext(ctx, r.db).
		Where("owner_type = ?", ownerType).
		Preload("Asset").
		Find(&ownerships).Error; err != nil {
		return nil, err
	}
	return ownerships, nil
}

func (r *assetOwnershipRepository) FindByID(ctx context.Context, id uint) (*model.AssetOwnership, error) {
	var o model.AssetOwnership
	result := commondb.DBFromContext(ctx, r.db).Preload("Asset").First(&o, id)
	if stderrors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &o, result.Error
}

func (r *assetOwnershipRepository) FindByUserAndAsset(ctx context.Context, userId uint, ownerType model.OwnerType, assetID uint) (*model.AssetOwnership, error) {
	return r.findByUserAndAsset(ctx, userId, ownerType, assetID, false)
}

func (r *assetOwnershipRepository) FindByUserAndAssetForUpdate(ctx context.Context, userId uint, ownerType model.OwnerType, assetID uint) (*model.AssetOwnership, error) {
	return r.findByUserAndAsset(ctx, userId, ownerType, assetID, true)
}

func (r *assetOwnershipRepository) Upsert(ctx context.Context, ownership *model.AssetOwnership) error {
	return commondb.DBFromContext(ctx, r.db).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "owner_type"}, {Name: "asset_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"amount", "avg_buy_price_rsd", "public_amount", "reserved_amount", "updated_at"}),
		}).
		Create(ownership).Error
}

func (r *assetOwnershipRepository) IncreaseReservedAmount(ctx context.Context, identityID uint, ownerType model.OwnerType, assetID uint, delta float64) error {
	return commondb.DBFromContext(ctx, r.db).
		Model(&model.AssetOwnership{}).
		Where("user_id = ? AND owner_type = ? AND asset_id = ?", identityID, ownerType, assetID).
		UpdateColumn("reserved_amount", gorm.Expr("reserved_amount + ?", delta)).Error
}

func (r *assetOwnershipRepository) FindAllPublic(ctx context.Context, page, pageSize int) ([]model.AssetOwnership, int64, error) {
	var ownerships []model.AssetOwnership
	var count int64

	db := commondb.DBFromContext(ctx, r.db).Model(&model.AssetOwnership{}).Where("public_amount > 0")
	if err := db.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := db.Preload("Asset").Limit(pageSize).Offset(offset).Find(&ownerships).Error
	return ownerships, count, err
}

func (r *assetOwnershipRepository) UpdateOTCFields(ctx context.Context, ownershipID uint, publicAmount, reservedAmount float64) error {
	return commondb.DBFromContext(ctx, r.db).
		Model(&model.AssetOwnership{}).
		Where("asset_ownership_id = ?", ownershipID).
		Updates(map[string]any{
			"public_amount":   publicAmount,
			"reserved_amount": reservedAmount,
			"updated_at":      time.Now(),
		}).Error
}

func (r *assetOwnershipRepository) findByUserAndAsset(ctx context.Context, userId uint, ownerType model.OwnerType, assetID uint, forUpdate bool) (*model.AssetOwnership, error) {
	query := commondb.DBFromContext(ctx, r.db).
		Where("user_id = ? AND owner_type = ? AND asset_id = ?", userId, ownerType, assetID).
		Preload("Asset")
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	var ownership model.AssetOwnership
	if err := query.First(&ownership).Error; err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &ownership, nil
}
