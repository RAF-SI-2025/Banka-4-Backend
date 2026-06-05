package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	commondb "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
)

type interbankReservationRepository struct {
	db *gorm.DB
}

func NewInterbankReservationRepository(db *gorm.DB) InterbankReservationRepository {
	return &interbankReservationRepository{db: db}
}

func (r *interbankReservationRepository) Create(ctx context.Context, res *model.InterbankReservation) error {
	return commondb.DBFromContext(ctx, r.db).Create(res).Error
}

func (r *interbankReservationRepository) FindByPendingBankingTxID(ctx context.Context, bankingTxID uint) (*model.InterbankReservation, error) {
	var res model.InterbankReservation
	err := commondb.DBFromContext(ctx, r.db).
		Where("pending_banking_tx_id = ?", bankingTxID).
		First(&res).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &res, err
}

func (r *interbankReservationRepository) Save(ctx context.Context, res *model.InterbankReservation) error {
	return commondb.DBFromContext(ctx, r.db).Save(res).Error
}
