package repository

import (
	"banking-service/internal/model"
	"context"

	"gorm.io/gorm"
)

type LoanRepository interface {
	CreateRequest(ctx context.Context, request *model.LoanRequest) error
	FindByClientID(ctx context.Context, clientID uint, sortByAmountDesc bool) ([]model.LoanRequest, error)
	FindByIDAndClientID(ctx context.Context, id uint, clientID uint) (*model.LoanRequest, error)
}

type LoanRepositoryImpl struct {
	db *gorm.DB
}

func NewLoanRepository(db *gorm.DB) LoanRepository {
	return &LoanRepositoryImpl{db: db}
}

func (r *LoanRepositoryImpl) FindByClientID(ctx context.Context, clientID uint, sortByAmountDesc bool) ([]model.LoanRequest, error) {
	var loans []model.LoanRequest

	query := r.db.WithContext(ctx).Where("client_id = ?", clientID).Preload("LoanType")

	if sortByAmountDesc {
		query = query.Order("amount DESC")
	} else {
		query = query.Order("amount ASC")
	}

	if err := query.Find(&loans).Error; err != nil {
		return nil, err
	}
	return loans, nil
}

func (r *LoanRepositoryImpl) FindByIDAndClientID(ctx context.Context, id uint, clientID uint) (*model.LoanRequest, error) {
	var loan model.LoanRequest
	if err := r.db.WithContext(ctx).Where("id = ? AND client_id = ?", id, clientID).Preload("LoanType").First(&loan).Error; err != nil {
		return nil, err
	}
	return &loan, nil
}

func (r *LoanRepositoryImpl) CreateRequest(ctx context.Context, request *model.LoanRequest) error {
	return r.db.WithContext(ctx).Create(request).Error
}
