package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type ClientFundPositionRepository interface {
	FindByClientAndFund(ctx context.Context, clientID uint, ownerType model.OwnerType, fundID uint) (*model.ClientFundPosition, error)
	FindByClient(ctx context.Context, clientID uint, ownerType model.OwnerType) ([]model.ClientFundPosition, error)
	FindByFund(ctx context.Context, fundID uint) ([]model.ClientFundPosition, error)
	Upsert(ctx context.Context, position *model.ClientFundPosition) error
}
