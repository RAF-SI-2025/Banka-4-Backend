package repository

import (
	"banking-service/internal/model"
	"context"
)

type PaymentRepository interface {
	Create(ctx context.Context, payment *model.Payment) error
	GetByID(ctx context.Context, id uint) (*model.Payment, error)
	Update(ctx context.Context, payment *model.Payment) error
}
