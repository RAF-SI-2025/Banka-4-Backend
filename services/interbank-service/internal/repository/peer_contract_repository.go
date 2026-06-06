package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
)

type PeerContractRepository interface {
	Create(ctx context.Context, contract *model.PeerContract) error
	FindByID(ctx context.Context, authorityRoutingNumber int, id string) (*model.PeerContract, error)
	FindByNegotiationID(ctx context.Context, authorityRoutingNumber int, negotiationID string) (*model.PeerContract, error)
	ListByParty(ctx context.Context, routingNumber int, partyID string) ([]model.PeerContract, error)
	Update(ctx context.Context, contract *model.PeerContract) error
	// FindActive returns all ACTIVE contracts. Settlement-date expiry is decided
	// in Go (SettlementPassed) because settlement_date is a free-form ISO-8601
	// string that cannot be reliably compared in SQL.
	FindActive(ctx context.Context) ([]model.PeerContract, error)
}
