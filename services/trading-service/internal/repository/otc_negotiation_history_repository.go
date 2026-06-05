package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type OtcNegotiationHistoryRepository interface {
	Create(ctx context.Context, history *model.OtcNegotiationHistory) error
	GetByOfferID(ctx context.Context, offerID uint) ([]*model.OtcNegotiationHistory, error)
}
