package repository

import (
	"context"

	commondb "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"gorm.io/gorm"
)

type GormTransactionManager struct {
	db *gorm.DB
}

func NewGormTransactionManager(db *gorm.DB) TransactionManager {
	return &GormTransactionManager{db: db}
}

func (m *GormTransactionManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if existingTx, ok := ctx.Value(commondb.TxContextKey{}).(*gorm.DB); ok && existingTx != nil {
		return fn(ctx)
	}

	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := context.WithValue(ctx, commondb.TxContextKey{}, tx)
		return fn(txCtx)
	})
}
