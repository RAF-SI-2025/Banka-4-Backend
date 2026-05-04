package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type ClientFundRedemptionRepository interface {
	Create(ctx context.Context, redemption *model.ClientFundRedemption) error
	Update(ctx context.Context, redemption *model.ClientFundRedemption) error
	FindPending(ctx context.Context, limit int) ([]model.ClientFundRedemption, error)
	SumPendingByClientAndFund(ctx context.Context, clientID uint, ownerType model.OwnerType, fundID uint) (float64, error)
}
