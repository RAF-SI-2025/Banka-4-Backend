package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
)

type outboundPaymentRepository struct {
	db *gorm.DB
}

func NewOutboundPaymentRepository(database *gorm.DB) OutboundPaymentRepository {
	return &outboundPaymentRepository{db: database}
}

func (r *outboundPaymentRepository) Create(ctx context.Context, p *model.OutboundPayment) error {
	return db.DBFromContext(ctx, r.db).Create(p).Error
}

func (r *outboundPaymentRepository) FindByTransactionIDKey(ctx context.Context, key string) (*model.OutboundPayment, error) {
	var p model.OutboundPayment
	err := db.DBFromContext(ctx, r.db).Where("transaction_id_key = ?", key).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &p, err
}
