package repository

import (
	"banking-service/internal/model"
	"context"
)

type ExchangeRateRepository interface {
	UpsertAll(ctx context.Context, rates []model.ExchangeRate) error
	GetAll(ctx context.Context) ([]model.ExchangeRate, error)
}
