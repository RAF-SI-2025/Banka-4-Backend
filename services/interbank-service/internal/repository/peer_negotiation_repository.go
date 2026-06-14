package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
)

// PeerNegotiationRepository persists cross-bank OTC negotiations and serves
// reads driven by both inbound peer requests (§3.4 GET, §3.3 PUT) and our
// users' outbound traffic.
type PeerNegotiationRepository interface {
	Create(ctx context.Context, n *model.PeerNegotiation) error
	// Upsert writes the row by (seller_routing_number, id), replacing all
	// non-PK columns on conflict. Used for mirror rows (IsAuthoritative=false)
	// so that a peer bank resetting its sequence doesn't strand stale rows.
	Upsert(ctx context.Context, n *model.PeerNegotiation) error
	// FindByID / FindByIDForUpdate key on the negotiation's full identity
	// (authoritative seller routing number + id); id alone is not unique.
	FindByID(ctx context.Context, routingNumber int, id string) (*model.PeerNegotiation, error)
	FindByIDForUpdate(ctx context.Context, routingNumber int, id string) (*model.PeerNegotiation, error)
	Update(ctx context.Context, n *model.PeerNegotiation) error
	ListByParty(ctx context.Context, routingNumber int, partyID string) ([]model.PeerNegotiation, error)
	// FindOngoing returns all ONGOING negotiations. Settlement-date expiry is
	// decided in Go (SettlementPassed), not in SQL.
	FindOngoing(ctx context.Context) ([]model.PeerNegotiation, error)
}
