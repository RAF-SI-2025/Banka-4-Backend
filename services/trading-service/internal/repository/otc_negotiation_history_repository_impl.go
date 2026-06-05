package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"gorm.io/gorm"
)

type otcNegotiationHistoryRepositoryImpl struct {
	db *gorm.DB
}

func NewOtcNegotiationHistoryRepository(db *gorm.DB) OtcNegotiationHistoryRepository {
	return &otcNegotiationHistoryRepositoryImpl{db: db}
}

func (r *otcNegotiationHistoryRepositoryImpl) Create(ctx context.Context, history *model.OtcNegotiationHistory) error {
	return r.db.WithContext(ctx).Create(history).Error
}

func (r *otcNegotiationHistoryRepositoryImpl) GetByOfferID(ctx context.Context, offerID uint) ([]*model.OtcNegotiationHistory, error) {
	var history []*model.OtcNegotiationHistory
	if err := r.db.WithContext(ctx).Where("otc_offer_id = ?", offerID).Find(&history).Error; err != nil {
		return nil, err
	}
	return history, nil
}
