package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
)

type accountRepository struct {
	db *gorm.DB
}

func NewAccountRepository(db *gorm.DB) AccountRepository {
	return &accountRepository{db: db}
}

func (r *accountRepository) Create(ctx context.Context, account *model.Account) error {
	db := db.DBFromContext(ctx, r.db)
	return db.WithContext(ctx).Create(account).Error
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

func (r *accountRepository) FindAllByClientID(ctx context.Context, clientID uint) ([]model.Account, error) {
	var accounts []model.Account
	err := r.db.WithContext(ctx).
		Preload("Currency").
		Where("client_id = ? AND status = ?", clientID, "Active").
		Find(&accounts).Error
	return accounts, err
}

func (r *accountRepository) FindByAccountNumberAndClientID(ctx context.Context, accountNumber string, clientID uint) (*model.Account, error) {
	var account model.Account
	err := r.db.WithContext(ctx).
		Preload("Currency").
		Where("account_number = ? AND client_id = ?", accountNumber, clientID).
		First(&account).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *accountRepository) UpdateName(ctx context.Context, accountNumber string, name string) error {
	return r.db.WithContext(ctx).
		Model(&model.Account{}).
		Where("account_number = ?", accountNumber).
		Update("name", name).Error
}

func (r *accountRepository) UpdateLimits(ctx context.Context, accountNumber string, daily float64, monthly float64) error {
	db := db.DBFromContext(ctx, r.db)
	return db.WithContext(ctx).
		Model(&model.Account{}).
		Where("account_number = ?", accountNumber).
		Updates(map[string]interface{}{"daily_limit": daily, "monthly_limit": monthly}).Error
}

func (r *accountRepository) NameExistsForClient(ctx context.Context, clientID uint, name string, excludeNumber string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Account{}).
		Where("client_id = ? AND name = ? AND account_number != ?", clientID, name, excludeNumber).
		Count(&count).Error
	return count > 0, err
}

func (r *accountRepository) FindByAccountNumber(ctx context.Context, accountNumber string) (*model.Account, error) {
	db := db.DBFromContext(ctx, r.db)

	var account model.Account
	err := db.WithContext(ctx).
		Preload("Currency").
		Where("account_number = ?", accountNumber).
		First(&account).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &account, nil
}

func (r *accountRepository) UpdateBalance(ctx context.Context, account *model.Account) error {
	db := db.DBFromContext(ctx, r.db)

	return db.WithContext(ctx).Model(account).Updates(map[string]interface{}{
		"balance":           account.Balance,
		"available_balance": account.AvailableBalance,
		"daily_spending":    account.DailySpending,
		"monthly_spending":  account.MonthlySpending,
	}).Error
}
func (r *accountRepository) FindAll(ctx context.Context, query *dto.ListAccountsQuery) ([]*model.Account, int64, error) {
	var accounts []*model.Account
	var count int64

	db := r.db.WithContext(ctx).Model(&model.Account{})

	if query.ClientID != nil {
		db = db.Where("client_id = ?", *query.ClientID)
	}
	if query.AccountType != "" {
		db = db.Where("account_type = ?", query.AccountType)
	}
	if query.AccountKind != "" {
		db = db.Where("account_kind = ?", query.AccountKind)
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	if query.CurrencyID != nil {
		db = db.Where("currency_id = ?", *query.CurrencyID)
	}

	if err := db.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	offset := (query.Page - 1) * query.PageSize
	err := db.Preload("Currency").Preload("Company").
		Limit(query.PageSize).Offset(offset).Find(&accounts).Error

	return accounts, count, err
}

func (r *accountRepository) GetByAccountNumber(ctx context.Context, accountNumber string) (*model.Account, error) {
	var account model.Account
	result := r.db.WithContext(ctx).Where("account_number = ?", accountNumber).First(&account)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	return &account, result.Error
}

func (r *accountRepository) Update(ctx context.Context, account *model.Account) error {
	db := db.DBFromContext(ctx, r.db)
	return db.WithContext(ctx).Save(account).Error
}

func (r *accountRepository) FindByClientID(ctx context.Context, clientID uint) ([]model.Account, error) {
	db := db.DBFromContext(ctx, r.db)

	var accounts []model.Account
	err := db.WithContext(ctx).
		Preload("Currency").
		Where("client_id = ?", clientID).
		Find(&accounts).Error

	if err != nil {
		return nil, err
	}

	return accounts, nil
}

func (r *accountRepository) FindByAccountType(ctx context.Context, accountType model.AccountType) (*model.Account, error) {
	var account model.Account
	result := r.db.WithContext(ctx).
		Where("account_type = ?", accountType).
		First(&account)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &account, result.Error
}
