package repository

import (
	"context"
	stderrors "errors"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type clientFundPositionRepository struct {
	db *gorm.DB
}

func NewClientFundPositionRepository(db *gorm.DB) ClientFundPositionRepository {
	return &clientFundPositionRepository{db: db}
}

func (r *clientFundPositionRepository) FindByClientAndFund(ctx context.Context, clientID uint, ownerType model.OwnerType, fundID uint) (*model.ClientFundPosition, error) {
	var pos model.ClientFundPosition
	result := r.db.WithContext(ctx).
		Preload("Fund").
		Where("client_id = ? AND owner_type = ? AND fund_id = ?", clientID, ownerType, fundID).
		First(&pos)
	if stderrors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &pos, result.Error
}

func (r *clientFundPositionRepository) Upsert(ctx context.Context, position *model.ClientFundPosition) error {
	position.UpdatedAt = time.Now()
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "fund_id"}, {Name: "client_id"}, {Name: "owner_type"}},
			DoUpdates: clause.AssignmentColumns([]string{"units_owned", "total_invested_amount", "updated_at"}),
		}).
		Create(position).Error
}

func (r *clientFundPositionRepository) FindByClient(ctx context.Context, clientID uint, ownerType model.OwnerType) ([]model.ClientFundPosition, error) {
	var positions []model.ClientFundPosition
	result := r.db.WithContext(ctx).
		Preload("Fund").
		Where("client_id = ? AND owner_type = ?", clientID, ownerType).
		Find(&positions)
	return positions, result.Error
}

func (r *clientFundPositionRepository) FindByFund(ctx context.Context, fundID uint) ([]model.ClientFundPosition, error) {
	var positions []model.ClientFundPosition
	result := r.db.WithContext(ctx).
		Preload("Fund").
		Where("fund_id = ?", fundID).
		Find(&positions)
	return positions, result.Error
}
