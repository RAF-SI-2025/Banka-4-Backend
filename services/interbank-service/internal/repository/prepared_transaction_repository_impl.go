package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
)

type preparedTransactionRepository struct {
	db *gorm.DB
}

func NewPreparedTransactionRepository(database *gorm.DB) PreparedTransactionRepository {
	return &preparedTransactionRepository{db: database}
}

func (r *preparedTransactionRepository) Create(ctx context.Context, tx *model.PreparedTransaction) error {
	return db.DBFromContext(ctx, r.db).WithContext(ctx).Create(tx).Error
}

func (r *preparedTransactionRepository) FindByID(ctx context.Context, routingNumber int, id string) (*model.PreparedTransaction, error) {
	var tx model.PreparedTransaction

	err := db.DBFromContext(ctx, r.db).WithContext(ctx).
		Where("routing_number = ? AND id = ?", routingNumber, id).
		First(&tx).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &tx, nil
}

func (r *preparedTransactionRepository) Update(ctx context.Context, tx *model.PreparedTransaction) error {
	return db.DBFromContext(ctx, r.db).WithContext(ctx).Save(tx).Error
}
