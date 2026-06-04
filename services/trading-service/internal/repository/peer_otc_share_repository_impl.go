package repository

import (
	"context"
	"errors"

	commondb "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type peerOtcShareRepository struct {
	db *gorm.DB
}

func NewPeerOtcShareRepository(db *gorm.DB) PeerOtcShareRepository {
	return &peerOtcShareRepository{db: db}
}

func (r *peerOtcShareRepository) CreateReservation(ctx context.Context, reservation *model.PeerOtcShareReservation) error {
	return commondb.DBFromContext(ctx, r.db).Create(reservation).Error
}

func (r *peerOtcShareRepository) FindReservationByContractID(ctx context.Context, contractID string) (*model.PeerOtcShareReservation, error) {
	return r.findReservationByContractID(ctx, contractID, false)
}

func (r *peerOtcShareRepository) FindReservationByContractIDForUpdate(ctx context.Context, contractID string) (*model.PeerOtcShareReservation, error) {
	return r.findReservationByContractID(ctx, contractID, true)
}

func (r *peerOtcShareRepository) SaveReservation(ctx context.Context, reservation *model.PeerOtcShareReservation) error {
	return commondb.DBFromContext(ctx, r.db).Save(reservation).Error
}

func (r *peerOtcShareRepository) CreateCredit(ctx context.Context, credit *model.PeerOtcShareCredit) error {
	return commondb.DBFromContext(ctx, r.db).Create(credit).Error
}

func (r *peerOtcShareRepository) FindCreditByContractID(ctx context.Context, contractID string) (*model.PeerOtcShareCredit, error) {
	var credit model.PeerOtcShareCredit
	err := commondb.DBFromContext(ctx, r.db).
		Where("contract_id = ?", contractID).
		First(&credit).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &credit, nil
}

func (r *peerOtcShareRepository) findReservationByContractID(ctx context.Context, contractID string, forUpdate bool) (*model.PeerOtcShareReservation, error) {
	query := commondb.DBFromContext(ctx, r.db).Where("contract_id = ?", contractID)
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	var reservation model.PeerOtcShareReservation
	err := query.First(&reservation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &reservation, nil
}
