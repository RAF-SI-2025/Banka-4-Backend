package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type AssetOwnershipRepository interface {
	FindByIdentity(ctx context.Context, identityID uint, ownerType model.OwnerType) ([]model.AssetOwnership, error)
	FindByOwnerAndAsset(ctx context.Context, identityID uint, ownerType model.OwnerType, assetID uint) (*model.AssetOwnership, error)
	FindByOwnerAndAssetForUpdate(ctx context.Context, identityID uint, ownerType model.OwnerType, assetID uint) (*model.AssetOwnership, error)
	Upsert(ctx context.Context, ownership *model.AssetOwnership) error
}
