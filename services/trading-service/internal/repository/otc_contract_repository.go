package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type OtcContractRepository interface {
	Create(ctx context.Context, contract *model.OtcContract) error
	FindByID(ctx context.Context, contractID uint) (*model.OtcContract, error)
	FindByIDForUpdate(ctx context.Context, contractID uint) (*model.OtcContract, error)
	Save(ctx context.Context, contract *model.OtcContract) error
}
