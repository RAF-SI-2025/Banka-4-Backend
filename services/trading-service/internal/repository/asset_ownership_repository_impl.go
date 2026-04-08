package repository

import (
	"context"
	"errors"

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

func (r *assetOwnershipRepository) FindByIdentity(ctx context.Context, identityID uint, ownerType model.OwnerType) ([]model.AssetOwnership, error) {
	var ownerships []model.AssetOwnership
	if err := commondb.DBFromContext(ctx, r.db).
		Where("identity_id = ? AND owner_type = ?", identityID, ownerType).
		Preload("Asset").
		Find(&ownerships).Error; err != nil {
		return nil, err
	}
	return ownerships, nil
}

func (r *assetOwnershipRepository) FindByOwnerAndAsset(ctx context.Context, identityID uint, ownerType model.OwnerType, assetID uint) (*model.AssetOwnership, error) {
	return r.findByOwnerAndAsset(ctx, identityID, ownerType, assetID, false)
}

func (r *assetOwnershipRepository) FindByOwnerAndAssetForUpdate(ctx context.Context, identityID uint, ownerType model.OwnerType, assetID uint) (*model.AssetOwnership, error) {
	return r.findByOwnerAndAsset(ctx, identityID, ownerType, assetID, true)
}

func (r *assetOwnershipRepository) Upsert(ctx context.Context, ownership *model.AssetOwnership) error {
	return commondb.DBFromContext(ctx, r.db).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "identity_id"}, {Name: "owner_type"}, {Name: "asset_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"amount", "avg_buy_price", "updated_at"}),
		}).
		Create(ownership).Error
}

func (r *assetOwnershipRepository) findByOwnerAndAsset(ctx context.Context, identityID uint, ownerType model.OwnerType, assetID uint, forUpdate bool) (*model.AssetOwnership, error) {
	query := commondb.DBFromContext(ctx, r.db).
		Where("identity_id = ? AND owner_type = ? AND asset_id = ?", identityID, ownerType, assetID).
		Preload("Asset")
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	var ownership model.AssetOwnership
	if err := query.First(&ownership).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &ownership, nil
}
