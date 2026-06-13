package repository

import (
	"context"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type TaxRepository interface {
	FindAllAccumulatedTax(ctx context.Context, accountNumbers []string, page, pageSize int) ([]model.AccumulatedTax, int64, error)

	FindAccumulatedTaxByAccountNumber(ctx context.Context, accountNumber string) (*model.AccumulatedTax, error)

	FindAllPositiveAccumulatedTax(ctx context.Context) ([]model.AccumulatedTax, error)

	AddTaxOwed(ctx context.Context, accountNumber string, employeeID *uint, amount float64, currencyCode string) error

	// ReduceTaxOwed subtracts amount from an existing accumulated-tax row, clamped
	// at zero. It never creates a row: a loss with no prior gains has nothing to
	// offset.
	ReduceTaxOwed(ctx context.Context, accountNumber string, employeeID *uint, amount float64) error

	ClearTax(ctx context.Context, accountNumber string, clearedAt time.Time) error

	RecordCollectionResult(ctx context.Context, collection *model.TaxCollection, clearTax bool, clearedAmount float64, clearedAt time.Time) error

	SaveAccumulatedTax(ctx context.Context, tax *model.AccumulatedTax) error

	CreateTaxCollection(ctx context.Context, collection *model.TaxCollection) error

	FindTaxCollectionsByAccountNumber(ctx context.Context, accountNumber string) ([]model.TaxCollection, error)

	FindLatestTaxCollection(ctx context.Context, accountNumber string) (*model.TaxCollection, error)

	FindAccumulatedTaxByEmployeeID(ctx context.Context, employeeID uint) ([]model.AccumulatedTax, error)

	FindAccumulatedTaxByClientAccountNumbers(ctx context.Context, accountNumbers []string) ([]model.AccumulatedTax, error)
}
