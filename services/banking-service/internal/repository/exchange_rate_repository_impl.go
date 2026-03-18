package repository

import (
	"banking-service/internal/model"
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type exchangeRateRepository struct {
	db *gorm.DB
}

func NewExchangeRateRepository(db *gorm.DB) ExchangeRateRepository {
	return &exchangeRateRepository{db: db}
}

func (r *exchangeRateRepository) UpsertAll(ctx context.Context, rates []model.ExchangeRate) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "currency_code"}},
			DoUpdates: clause.AssignmentColumns([]string{"buy_rate", "middle_rate", "sell_rate", "provider_updated_at", "provider_next_update_at"}),
		}).
		Create(&rates).Error
}

func (r *exchangeRateRepository) GetAll(ctx context.Context) ([]model.ExchangeRate, error) {
	var rates []model.ExchangeRate
	err := r.db.WithContext(ctx).Find(&rates).Error
	return rates, err
}
