package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type AssetOwnershipRepository interface {
	FindByUserId(ctx context.Context, userId uint, ownerType model.OwnerType) ([]model.AssetOwnership, error)
	FindByOwnerType(ctx context.Context, ownerType model.OwnerType) ([]model.AssetOwnership, error)
	FindByID(ctx context.Context, id uint) (*model.AssetOwnership, error)
	FindByUserAndAsset(ctx context.Context, userId uint, ownerType model.OwnerType, assetID uint) (*model.AssetOwnership, error)
	FindByUserAndAssetForUpdate(ctx context.Context, userId uint, ownerType model.OwnerType, assetID uint) (*model.AssetOwnership, error)
	Upsert(ctx context.Context, ownership *model.AssetOwnership) error
	// IncreaseReservedAmount atomically adds delta to the reserved_amount for the
	// given identity+ownerType+assetID row. It is a no-op when no row matches.
	IncreaseReservedAmount(ctx context.Context, identityID uint, ownerType model.OwnerType, assetID uint, delta float64) error
	FindAllPublic(ctx context.Context, page, pageSize int) ([]model.AssetOwnership, int64, error)
	UpdateOTCFields(ctx context.Context, ownershipID uint, publicAmount, reservedAmount float64) error
}
