package repository

import (
	"context"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type OrderRepository interface {
	Create(ctx context.Context, order *model.Order) error
	FindByID(ctx context.Context, id uint) (*model.Order, error)
	Save(ctx context.Context, order *model.Order) error
	FindAll(ctx context.Context, page, pageSize int, userID *uint, ownerType *model.OwnerType, status *model.OrderStatus, direction *model.OrderDirection, isDone *bool) ([]model.Order, int64, error)
	FindReadyForExecution(ctx context.Context, before time.Time, limit int) ([]model.Order, error)
}
