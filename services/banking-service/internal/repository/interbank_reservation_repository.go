package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
)

type InterbankReservationRepository interface {
	Create(ctx context.Context, r *model.InterbankReservation) error
	FindByPendingBankingTxID(ctx context.Context, bankingTxID uint) (*model.InterbankReservation, error)
	Save(ctx context.Context, r *model.InterbankReservation) error
}
