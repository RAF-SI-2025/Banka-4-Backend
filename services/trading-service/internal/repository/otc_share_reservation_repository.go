package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type OtcShareReservationRepository interface {
	Create(ctx context.Context, reservation *model.OtcShareReservation) error
	FindByContractID(ctx context.Context, contractID uint) (*model.OtcShareReservation, error)
	FindByContractIDForUpdate(ctx context.Context, contractID uint) (*model.OtcShareReservation, error)
	SumActiveReservedBySellerAsset(ctx context.Context, sellerIdentityID uint, sellerOwnerType model.OwnerType, assetID uint, excludeContractID *uint) (float64, error)
	Save(ctx context.Context, reservation *model.OtcShareReservation) error
}
