package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"gorm.io/gorm"
)

type clientFundInvestmentRepository struct {
	db *gorm.DB
}

func NewClientFundInvestmentRepository(db *gorm.DB) ClientFundInvestmentRepository {
	return &clientFundInvestmentRepository{db: db}
}

func (r *clientFundInvestmentRepository) Create(ctx context.Context, investment *model.ClientFundInvestment) error {
	return r.db.WithContext(ctx).Create(investment).Error
}

func (r *clientFundInvestmentRepository) FindByClientAndFund(ctx context.Context, clientID uint, ownerType model.OwnerType, fundID uint) ([]model.ClientFundInvestment, error) {
	var investments []model.ClientFundInvestment
	err := r.db.WithContext(ctx).
		Preload("Fund").
		Where("client_id = ? AND owner_type = ? AND fund_id = ?", clientID, ownerType, fundID).
		Order("created_at DESC").
		Find(&investments).Error
	return investments, err
}
