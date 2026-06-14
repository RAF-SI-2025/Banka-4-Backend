package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
)

type peerNegotiationRepository struct {
	db *gorm.DB
}

func NewPeerNegotiationRepository(database *gorm.DB) PeerNegotiationRepository {
	return &peerNegotiationRepository{db: database}
}

func (r *peerNegotiationRepository) Create(ctx context.Context, n *model.PeerNegotiation) error {
	return db.DBFromContext(ctx, r.db).Create(n).Error
}

func (r *peerNegotiationRepository) Upsert(ctx context.Context, n *model.PeerNegotiation) error {
	return db.DBFromContext(ctx, r.db).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "seller_routing_number"}, {Name: "id"}},
			UpdateAll: true,
		}).
		Create(n).Error
}

func (r *peerNegotiationRepository) FindByID(ctx context.Context, routingNumber int, id string) (*model.PeerNegotiation, error) {
	var n model.PeerNegotiation

	err := db.DBFromContext(ctx, r.db).
		Where("seller_routing_number = ? AND id = ?", routingNumber, id).
		First(&n).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &n, nil
}

func (r *peerNegotiationRepository) Update(ctx context.Context, n *model.PeerNegotiation) error {
	return db.DBFromContext(ctx, r.db).Save(n).Error
}

func (r *peerNegotiationRepository) FindByIDForUpdate(ctx context.Context, routingNumber int, id string) (*model.PeerNegotiation, error) {
	var n model.PeerNegotiation

	err := db.DBFromContext(ctx, r.db).
		Where("seller_routing_number = ? AND id = ?", routingNumber, id).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&n).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (r *peerNegotiationRepository) ListByParty(ctx context.Context, routingNumber int, partyID string) ([]model.PeerNegotiation, error) {
	var rows []model.PeerNegotiation

	err := db.DBFromContext(ctx, r.db).
		Where(
			"(buyer_routing_number = ? AND buyer_id = ?) OR (seller_routing_number = ? AND seller_id = ?)",
			routingNumber, partyID, routingNumber, partyID,
		).
		Order("updated_at DESC").
		Find(&rows).Error

	return rows, err
}

func (r *peerNegotiationRepository) FindOngoing(ctx context.Context) ([]model.PeerNegotiation, error) {
	var rows []model.PeerNegotiation

	err := db.DBFromContext(ctx, r.db).
		Where("status = ?", model.PeerNegotiationOngoing).
		Order("settlement_date ASC").
		Find(&rows).Error

	return rows, err
}
