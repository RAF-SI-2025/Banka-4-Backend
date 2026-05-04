package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type InvestmentFundRepository interface {
	Create(ctx context.Context, fund *model.InvestmentFund) error
	FindByID(ctx context.Context, id uint) (*model.InvestmentFund, error)
	FindByAccountNumber(ctx context.Context, accountNumber string) (*model.InvestmentFund, error)
	FindByName(ctx context.Context, name string) (*model.InvestmentFund, error)
	FindHoldings(ctx context.Context, fundID uint) ([]model.AssetOwnership, error)
	GetPerformanceHistory(ctx context.Context, fundID uint, limit int) ([]model.FundPerformance, error)
	SavePerformanceSnapshot(ctx context.Context, perf *model.FundPerformance) error
	GetAllInvestmentFunds(ctx context.Context) ([]model.InvestmentFund, error)
	FindAll(ctx context.Context, name string, sortBy string, sortDir string, page int, pageSize int) ([]model.InvestmentFund, int64, error)
	FindByManagerID(ctx context.Context, managerID uint) ([]model.InvestmentFund, error)
}
