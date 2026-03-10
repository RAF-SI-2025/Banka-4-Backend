package repository

import (
	"context"
	"user-service/internal/model"
)

type ResetTokenRepository interface {
	Create(ctx context.Context, token *model.ResetToken) error
	FindByToken(ctx context.Context, token string) (*model.ResetToken, error)
	DeleteByEmployeeID(ctx context.Context, employeeID uint) error
}
