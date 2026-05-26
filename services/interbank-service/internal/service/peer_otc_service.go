package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/repository"
)

// PeerOtcService implements the cross-bank OTC negotiation lifecycle.
//
// Authoritative state lives on the seller's bank (§3.2). When a peer's
// buyer initiates against our seller, we are authoritative; we generate
// the id and store the row. When our buyer initiates against a peer
// seller, we hold a mirror row and the peer is authoritative.
type PeerOtcService struct {
	negotiations repository.PeerNegotiationRepository
	peers        *PeerResolver
}

func NewPeerOtcService(negotiations repository.PeerNegotiationRepository, peers *PeerResolver) *PeerOtcService {
	return &PeerOtcService{negotiations: negotiations, peers: peers}
}

// CreateFromPeer handles §3.2 POST /interbank/negotiations — a peer bank's
// buyer initiates a negotiation against a seller in our bank. We are
// authoritative and assign the id.
func (s *PeerOtcService) CreateFromPeer(ctx context.Context, senderRouting int, offer dto.OtcOffer) (dto.ForeignBankId, error) {
	if err := s.validateOffer(offer); err != nil {
		return dto.ForeignBankId{}, err
	}

	// The sender must own the lastModifiedBy id — peers cannot impersonate.
	if offer.LastModifiedBy.RoutingNumber != senderRouting {
		return dto.ForeignBankId{}, errors.UnauthorizedErr("lastModifiedBy.routingNumber does not match sender")
	}

	// Seller must live in our bank (otherwise the peer should have addressed
	// the seller's actual bank, not us).
	if offer.SellerID.RoutingNumber != s.peers.OurRoutingNumber() {
		return dto.ForeignBankId{}, errors.BadRequestErr("sellerId.routingNumber does not match this bank")
	}

	n := &model.PeerNegotiation{
		ID:                    uuid.NewString(),
		BuyerRoutingNumber:    offer.BuyerID.RoutingNumber,
		BuyerID:               offer.BuyerID.ID,
		SellerRoutingNumber:   offer.SellerID.RoutingNumber,
		SellerID:              offer.SellerID.ID,
		Ticker:                offer.Ticker,
		Amount:                offer.Amount,
		PricePerStock:         offer.PricePerStock,
		PriceCurrency:         offer.PriceCurrency,
		Premium:               offer.Premium,
		PremiumCurrency:       offer.PremiumCurrency,
		SettlementDate:        offer.SettlementDate,
		LastModifiedByRouting: offer.LastModifiedBy.RoutingNumber,
		LastModifiedByID:      offer.LastModifiedBy.ID,
		Status:                model.PeerNegotiationOngoing,
		IsAuthoritative:       true,
	}

	if err := s.negotiations.Create(ctx, n); err != nil {
		return dto.ForeignBankId{}, errors.InternalErr(err)
	}

	return dto.ForeignBankId{
		RoutingNumber: s.peers.OurRoutingNumber(),
		ID:            n.ID,
	}, nil
}

// GetByID handles §3.4 GET /interbank/negotiations/:rn/:id — returns the
// stored negotiation. The :rn path parameter is expected to match this
// bank's routing number when we are authoritative.
func (s *PeerOtcService) GetByID(ctx context.Context, routingNumber int, id string) (*dto.OtcNegotiation, error) {
	if routingNumber != s.peers.OurRoutingNumber() {
		return nil, errors.BadRequestErr("routingNumber does not match this bank")
	}

	n, err := s.negotiations.FindByID(ctx, id)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if n == nil {
		return nil, errors.NotFoundErr("negotiation not found")
	}

	return toNegotiationDTO(n, s.peers.OurRoutingNumber()), nil
}

func (s *PeerOtcService) validateOffer(o dto.OtcOffer) error {
	if strings.TrimSpace(o.Ticker) == "" {
		return errors.BadRequestErr("ticker is required")
	}

	if o.Amount <= 0 {
		return errors.BadRequestErr("amount must be positive")
	}

	if o.PricePerStock <= 0 {
		return errors.BadRequestErr("pricePerStock must be positive")
	}

	if o.Premium < 0 {
		return errors.BadRequestErr("premium must be non-negative")
	}

	if _, err := time.Parse(time.RFC3339, o.SettlementDate); err != nil {
		// Accept either full RFC 3339 or a bare YYYY-MM-DD; reject anything else.
		if _, err2 := time.Parse("2006-01-02", o.SettlementDate); err2 != nil {
			return errors.BadRequestErr("settlementDate must be ISO 8601 (date or datetime)")
		}
	}

	return nil
}

// toNegotiationDTO maps the persistence model back to the wire-shape used
// on §3.4 responses.
func toNegotiationDTO(n *model.PeerNegotiation, ourRouting int) *dto.OtcNegotiation {
	idRouting := ourRouting
	idValue := n.ID
	if !n.IsAuthoritative && n.RemoteNegotiationID != nil {
		// Mirror rows expose the authoritative bank's id.
		idValue = *n.RemoteNegotiationID
		idRouting = n.SellerRoutingNumber
	}

	return &dto.OtcNegotiation{
		ID:        dto.ForeignBankId{RoutingNumber: idRouting, ID: idValue},
		Status:    strings.ToLower(string(n.Status)),
		UpdatedAt: n.UpdatedAt.Format(time.RFC3339),
		Offer: dto.OtcOffer{
			BuyerID:         dto.ForeignBankId{RoutingNumber: n.BuyerRoutingNumber, ID: n.BuyerID},
			SellerID:        dto.ForeignBankId{RoutingNumber: n.SellerRoutingNumber, ID: n.SellerID},
			Ticker:          n.Ticker,
			Amount:          n.Amount,
			PricePerStock:   n.PricePerStock,
			PriceCurrency:   n.PriceCurrency,
			Premium:         n.Premium,
			PremiumCurrency: n.PremiumCurrency,
			SettlementDate:  n.SettlementDate,
			LastModifiedBy:  dto.ForeignBankId{RoutingNumber: n.LastModifiedByRouting, ID: n.LastModifiedByID},
		},
	}
}
