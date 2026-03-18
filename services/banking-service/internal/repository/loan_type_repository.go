package repository

import (
	"banking-service/internal/model"
	"context"

	"gorm.io/gorm"
)

type LoanTypeRepository interface {
	FindByID(ctx context.Context, id uint) (*model.LoanType, error)
}

type LoanTypeRepositoryImpl struct {
	db *gorm.DB
}

func NewLoanTypeRepository(db *gorm.DB) LoanTypeRepository {
	return &LoanTypeRepositoryImpl{db: db}
}

func (r *LoanTypeRepositoryImpl) FindByID(ctx context.Context, id uint) (*model.LoanType, error) {
	var loanType model.LoanType
	result := r.db.WithContext(ctx).First(&loanType, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil // Nije pronadjen
		}
		return nil, result.Error
	}
	return &loanType, nil
}
