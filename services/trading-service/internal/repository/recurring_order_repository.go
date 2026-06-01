package repository

import (
	"context"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type RecurringOrderRepository interface {
	Create(ctx context.Context, ro *model.RecurringOrder) error
	FindByID(ctx context.Context, id uint) (*model.RecurringOrder, error)
	Save(ctx context.Context, ro *model.RecurringOrder) error
	Delete(ctx context.Context, id uint) error
	FindByUser(ctx context.Context, userID uint, ownerType model.OwnerType) ([]model.RecurringOrder, error)
	FindDue(ctx context.Context, before time.Time) ([]model.RecurringOrder, error)
}
