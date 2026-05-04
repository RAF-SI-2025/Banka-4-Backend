package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type ClientFundInvestmentRepository interface {
	Create(ctx context.Context, investment *model.ClientFundInvestment) error
	FindByClientAndFund(ctx context.Context, clientID uint, ownerType model.OwnerType, fundID uint) ([]model.ClientFundInvestment, error)
}
