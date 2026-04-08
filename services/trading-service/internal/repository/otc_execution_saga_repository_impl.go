package repository

import (
	"context"
	"errors"
	"time"

	commondb "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type otcExecutionSagaRepository struct {
	db *gorm.DB
}

func NewOtcExecutionSagaRepository(db *gorm.DB) OtcExecutionSagaRepository {
	return &otcExecutionSagaRepository{db: db}
}

func (r *otcExecutionSagaRepository) Create(ctx context.Context, saga *model.OtcExecutionSaga) error {
	return commondb.DBFromContext(ctx, r.db).Create(saga).Error
}

func (r *otcExecutionSagaRepository) FindByID(ctx context.Context, sagaID uint) (*model.OtcExecutionSaga, error) {
	var saga model.OtcExecutionSaga
	if err := commondb.DBFromContext(ctx, r.db).
		Preload("Contract").
		Preload("Contract.Asset").
		First(&saga, sagaID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &saga, nil
}

func (r *otcExecutionSagaRepository) FindByContractID(ctx context.Context, contractID uint) (*model.OtcExecutionSaga, error) {
	return r.findByContractID(ctx, contractID, false)
}

func (r *otcExecutionSagaRepository) FindByContractIDForUpdate(ctx context.Context, contractID uint) (*model.OtcExecutionSaga, error) {
	return r.findByContractID(ctx, contractID, true)
}

func (r *otcExecutionSagaRepository) FindPendingForExecution(ctx context.Context, before time.Time, limit int) ([]model.OtcExecutionSaga, error) {
	var sagas []model.OtcExecutionSaga
	query := commondb.DBFromContext(ctx, r.db).
		Preload("Contract").
		Preload("Contract.Asset").
		Where("status IN ?", []model.OtcExecutionStatus{model.OtcExecutionStatusInProgress, model.OtcExecutionStatusCompensating}).
		Where("next_retry_at IS NULL OR next_retry_at <= ?", before).
		Order("updated_at ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&sagas).Error; err != nil {
		return nil, err
	}

	return sagas, nil
}

func (r *otcExecutionSagaRepository) Save(ctx context.Context, saga *model.OtcExecutionSaga) error {
	return commondb.DBFromContext(ctx, r.db).Save(saga).Error
}

func (r *otcExecutionSagaRepository) findByContractID(ctx context.Context, contractID uint, forUpdate bool) (*model.OtcExecutionSaga, error) {
	query := commondb.DBFromContext(ctx, r.db).
		Preload("Contract").
		Preload("Contract.Asset").
		Where("contract_id = ?", contractID)
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	var saga model.OtcExecutionSaga
	if err := query.First(&saga).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &saga, nil
}
