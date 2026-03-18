package repository

import (
	"banking-service/internal/model"
	"common/pkg/db"
	"context"

	"gorm.io/gorm"
)

type accountRepository struct {
	db *gorm.DB
}

func NewAccountRepository(db *gorm.DB) AccountRepository {
	return &accountRepository{db: db}
}

func (r *accountRepository) Create(ctx context.Context, account *model.Account) error {
	return r.db.WithContext(ctx).Create(account).Error
}

func (r *accountRepository) AccountNumberExists(ctx context.Context, accountNumber string) (bool, error) {
	var count int64

	err := r.db.WithContext(ctx).
		Model(&model.Account{}).
		Where("account_number = ?", accountNumber).
		Count(&count).
		Error

	return count > 0, err
}

func (r *accountRepository) GetByAccountNumber(ctx context.Context, accountNumber string) (*model.Account, error) {
	db := db.DBFromContext(ctx, r.db)

	var account model.Account
	if err := db.WithContext(ctx).First(&account, accountNumber).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *accountRepository) Update(ctx context.Context, account *model.Account) error {
	db := db.DBFromContext(ctx, r.db)

	return db.WithContext(ctx).Save(account).Error
}

