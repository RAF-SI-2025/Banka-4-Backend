package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
)

type AccountRepository interface {
	Create(ctx context.Context, account *model.Account) error
	AccountNumberExists(ctx context.Context, accountNumber string) (bool, error)
	GetByAccountNumber(ctx context.Context, accountNumber string) (*model.Account, error)
	Update(ctx context.Context, account *model.Account) error
	FindAllByClientID(ctx context.Context, clientID uint) ([]model.Account, error)
	FindByAccountNumberAndClientID(ctx context.Context, accountNumber string, clientID uint) (*model.Account, error)
	UpdateName(ctx context.Context, accountNumber string, name string) error
	UpdateLimits(ctx context.Context, accountNumber string, daily float64, monthly float64) error
	NameExistsForClient(ctx context.Context, clientID uint, name string, excludeNumber string) (bool, error)
	FindByAccountNumber(ctx context.Context, accountNumber string) (*model.Account, error)
	UpdateBalance(ctx context.Context, account *model.Account) error
	FindAll(ctx context.Context, query *dto.ListAccountsQuery) ([]*model.Account, int64, error)
	FindByClientID(ctx context.Context, clientID uint) ([]model.Account, error)
	FindByAccountType(ctx context.Context, accountType model.AccountType) (*model.Account, error)
}
