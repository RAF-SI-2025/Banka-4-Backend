package db

import (
	"context"
	"gorm.io/gorm"
)

type TxContextKey struct{}

func DBFromContext(ctx context.Context, defaultDB *gorm.DB) *gorm.DB {
    tx, ok := ctx.Value(TxContextKey{}).(*gorm.DB)
    if ok && tx != nil {
        return tx.WithContext(ctx)
    }
    return defaultDB.WithContext(ctx)
}
