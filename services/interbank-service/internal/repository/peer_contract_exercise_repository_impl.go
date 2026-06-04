package repository

import (
	"context"
	"errors"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
	"gorm.io/gorm"
)

type peerContractExerciseRepository struct {
	db *gorm.DB
}

func NewPeerContractExerciseRepository(database *gorm.DB) PeerContractExerciseRepository {
	return &peerContractExerciseRepository{db: database}
}

func (r *peerContractExerciseRepository) Create(ctx context.Context, exercise *model.PeerContractExercise) error {
	return db.DBFromContext(ctx, r.db).Create(exercise).Error
}

func (r *peerContractExerciseRepository) FindByID(ctx context.Context, id uint) (*model.PeerContractExercise, error) {
	var exercise model.PeerContractExercise
	err := db.DBFromContext(ctx, r.db).First(&exercise, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &exercise, nil
}

func (r *peerContractExerciseRepository) FindByContract(ctx context.Context, authorityRoutingNumber int, contractID string) (*model.PeerContractExercise, error) {
	var exercise model.PeerContractExercise
	err := db.DBFromContext(ctx, r.db).
		Where("contract_authority_routing_number = ? AND contract_id = ?", authorityRoutingNumber, contractID).
		First(&exercise).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &exercise, nil
}

func (r *peerContractExerciseRepository) Update(ctx context.Context, exercise *model.PeerContractExercise) error {
	return db.DBFromContext(ctx, r.db).Save(exercise).Error
}
