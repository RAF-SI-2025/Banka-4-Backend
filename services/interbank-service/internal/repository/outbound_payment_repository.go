package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
)

type OutboundPaymentRepository interface {
	Create(ctx context.Context, p *model.OutboundPayment) error
	FindByTransactionIDKey(ctx context.Context, key string) (*model.OutboundPayment, error)
}
