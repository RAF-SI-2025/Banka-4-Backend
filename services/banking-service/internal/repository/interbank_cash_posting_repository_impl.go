package repository

import (
	"context"
	"errors"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
	"gorm.io/gorm"
)

type interbankCashPostingRepository struct {
	db *gorm.DB
}

func NewInterbankCashPostingRepository(database *gorm.DB) InterbankCashPostingRepository {
	return &interbankCashPostingRepository{db: database}
}

func (r *interbankCashPostingRepository) Create(ctx context.Context, posting *model.InterbankCashPosting) error {
	return db.DBFromContext(ctx, r.db).Create(posting).Error
}

func (r *interbankCashPostingRepository) FindByID(ctx context.Context, postingID string) (*model.InterbankCashPosting, error) {
	var posting model.InterbankCashPosting
	err := db.DBFromContext(ctx, r.db).Where("posting_id = ?", postingID).First(&posting).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &posting, nil
}

func (r *interbankCashPostingRepository) Save(ctx context.Context, posting *model.InterbankCashPosting) error {
	return db.DBFromContext(ctx, r.db).Save(posting).Error
}
