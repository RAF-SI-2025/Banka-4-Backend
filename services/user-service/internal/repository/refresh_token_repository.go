package repository

import (
	"context"
	"user-service/internal/model"
)

type RefreshTokenRepository interface {
	Create(ctx context.Context, token *model.RefreshToken) error
	FindByToken(ctx context.Context, token string) (*model.RefreshToken, error)
	DeleteByEmployeeID(ctx context.Context, employeeID uint) error
}
