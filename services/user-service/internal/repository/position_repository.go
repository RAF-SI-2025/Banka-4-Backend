package repository

import (
	"context"
)

type PositionRepository interface {
	Exists(ctx context.Context, id uint) (bool, error)
}
