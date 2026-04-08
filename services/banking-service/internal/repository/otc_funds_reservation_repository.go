package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
)

type OtcFundsReservationRepository interface {
	Create(ctx context.Context, reservation *model.OtcFundsReservation) error
	FindByExecutionID(ctx context.Context, executionID string) (*model.OtcFundsReservation, error)
	Save(ctx context.Context, reservation *model.OtcFundsReservation) error
}
