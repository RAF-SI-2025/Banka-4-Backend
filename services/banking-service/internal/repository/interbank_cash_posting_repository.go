package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
)

type InterbankCashPostingRepository interface {
	Create(ctx context.Context, posting *model.InterbankCashPosting) error
	FindByID(ctx context.Context, postingID string) (*model.InterbankCashPosting, error)
	Save(ctx context.Context, posting *model.InterbankCashPosting) error
}
