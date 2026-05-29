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
	FindByID(ctx context.Context, id string) (*model.PeerNegotiation, error)
	Update(ctx context.Context, n *model.PeerNegotiation) error
	ListByParty(ctx context.Context, routingNumber int, partyID string) ([]model.PeerNegotiation, error)
}
