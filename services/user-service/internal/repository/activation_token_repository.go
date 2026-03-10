package repository

import (
	"context"
	"user-service/internal/model"
)

type ActivationTokenRepository interface {
	Create(ctx context.Context, token *model.ActivationToken) error
	FindByToken(ctx context.Context, token string) (*model.ActivationToken, error)
	Delete(ctx context.Context, token *model.ActivationToken) error
}
