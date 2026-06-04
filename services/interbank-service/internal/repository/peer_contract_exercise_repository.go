package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
)

type PeerContractExerciseRepository interface {
	Create(ctx context.Context, exercise *model.PeerContractExercise) error
	FindByID(ctx context.Context, id uint) (*model.PeerContractExercise, error)
	FindByContract(ctx context.Context, authorityRoutingNumber int, contractID string) (*model.PeerContractExercise, error)
	Update(ctx context.Context, exercise *model.PeerContractExercise) error
}
