package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
)

type PreparedTransactionRepository interface {
	Create(ctx context.Context, tx *model.PreparedTransaction) error
	FindByID(ctx context.Context, routingNumber int, id string) (*model.PreparedTransaction, error)
	Update(ctx context.Context, tx *model.PreparedTransaction) error
}
