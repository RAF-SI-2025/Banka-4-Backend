package repository

import (
	"context"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type OtcExecutionSagaRepository interface {
	Create(ctx context.Context, saga *model.OtcExecutionSaga) error
	FindByID(ctx context.Context, sagaID uint) (*model.OtcExecutionSaga, error)
	FindByContractID(ctx context.Context, contractID uint) (*model.OtcExecutionSaga, error)
	FindByContractIDForUpdate(ctx context.Context, contractID uint) (*model.OtcExecutionSaga, error)
	FindPendingForExecution(ctx context.Context, before time.Time, limit int) ([]model.OtcExecutionSaga, error)
	Save(ctx context.Context, saga *model.OtcExecutionSaga) error
}
