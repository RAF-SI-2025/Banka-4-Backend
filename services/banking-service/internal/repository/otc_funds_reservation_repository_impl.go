package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	commondb "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
)

type otcFundsReservationRepository struct {
	db *gorm.DB
}

func NewOtcFundsReservationRepository(db *gorm.DB) OtcFundsReservationRepository {
	return &otcFundsReservationRepository{db: db}
}

func (r *otcFundsReservationRepository) Create(ctx context.Context, reservation *model.OtcFundsReservation) error {
	return commondb.DBFromContext(ctx, r.db).Create(reservation).Error
}

func (r *otcFundsReservationRepository) FindByExecutionID(ctx context.Context, executionID string) (*model.OtcFundsReservation, error) {
	var reservation model.OtcFundsReservation
	err := commondb.DBFromContext(ctx, r.db).
		Where("execution_id = ?", executionID).
		First(&reservation).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &reservation, nil
}

func (r *otcFundsReservationRepository) Save(ctx context.Context, reservation *model.OtcFundsReservation) error {
	return commondb.DBFromContext(ctx, r.db).Save(reservation).Error
}
