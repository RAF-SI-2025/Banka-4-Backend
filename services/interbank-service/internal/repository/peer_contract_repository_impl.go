package repository

import (
	"context"
	"errors"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
	"gorm.io/gorm"
)

type peerContractRepository struct {
	db *gorm.DB
}

func NewPeerContractRepository(database *gorm.DB) PeerContractRepository {
	return &peerContractRepository{db: database}
}

func (r *peerContractRepository) Create(ctx context.Context, contract *model.PeerContract) error {
	return db.DBFromContext(ctx, r.db).Create(contract).Error
}

func (r *peerContractRepository) FindByID(ctx context.Context, authorityRoutingNumber int, id string) (*model.PeerContract, error) {
	var contract model.PeerContract
	err := db.DBFromContext(ctx, r.db).
		Where("authority_routing_number = ? AND id = ?", authorityRoutingNumber, id).
		First(&contract).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &contract, nil
}

func (r *peerContractRepository) FindByNegotiationID(ctx context.Context, authorityRoutingNumber int, negotiationID string) (*model.PeerContract, error) {
	var contract model.PeerContract
	err := db.DBFromContext(ctx, r.db).
		Where("authority_routing_number = ? AND negotiation_id = ?", authorityRoutingNumber, negotiationID).
		First(&contract).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &contract, nil
}

func (r *peerContractRepository) ListByParty(ctx context.Context, routingNumber int, partyID string) ([]model.PeerContract, error) {
	var rows []model.PeerContract
	err := db.DBFromContext(ctx, r.db).
		Where(
			"(buyer_routing_number = ? AND buyer_id = ?) OR (seller_routing_number = ? AND seller_id = ?)",
			routingNumber, partyID, routingNumber, partyID,
		).
		Order("updated_at DESC").
		Find(&rows).Error
	return rows, err
}

func (r *peerContractRepository) Update(ctx context.Context, contract *model.PeerContract) error {
	return db.DBFromContext(ctx, r.db).Save(contract).Error
}

func (r *peerContractRepository) FindActive(ctx context.Context) ([]model.PeerContract, error) {
	var rows []model.PeerContract

	err := db.DBFromContext(ctx, r.db).
		Where("status = ?", model.PeerContractActive).
		Order("settlement_date ASC").
		Find(&rows).Error

	return rows, err
}
