package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/client"
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
	negotiations  repository.PeerNegotiationRepository
	peers         *PeerResolver
	client        *PeerOtcClient
	tradingClient client.TradingClient
	userClient    client.UserClient
}

func NewPeerOtcService(
	negotiations repository.PeerNegotiationRepository,
	peers *PeerResolver,
	peerClient *PeerOtcClient,
	tradingClient client.TradingClient,
	userClient client.UserClient,
) *PeerOtcService {
	return &PeerOtcService{
		negotiations:  negotiations,
		peers:         peers,
		client:        peerClient,
		tradingClient: tradingClient,
		userClient:    userClient,
	}
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

// UpdateCounter handles §3.3 PUT /interbank/negotiations/:rn/:id — a peer
// bank posts a counter-offer against an ongoing negotiation owned by us.
//
// Per spec §3.3: a 409 is returned when the same party tries to counter
// twice in a row (turn violation) or when the negotiation is closed.
// Buyer/seller identities and ticker are immutable for the lifetime of
// the negotiation; only the negotiable parameters may change.
func (s *PeerOtcService) UpdateCounter(ctx context.Context, senderRouting, routingNumber int, id string, offer dto.OtcOffer) error {
	if routingNumber != s.peers.OurRoutingNumber() {
		return errors.BadRequestErr("routingNumber does not match this bank")
	}
	if err := s.validateOffer(offer); err != nil {
		return err
	}
	if offer.LastModifiedBy.RoutingNumber != senderRouting {
		return errors.UnauthorizedErr("lastModifiedBy.routingNumber does not match sender")
	}

	n, err := s.negotiations.FindByID(ctx, id)
	if err != nil {
		return errors.InternalErr(err)
	}
	if n == nil {
		return errors.NotFoundErr("negotiation not found")
	}

	if senderRouting != n.BuyerRoutingNumber && senderRouting != n.SellerRoutingNumber {
		return errors.ForbiddenErr("sender is not a party to this negotiation")
	}
	if n.Status != model.PeerNegotiationOngoing {
		return errors.ConflictErr("negotiation is not ongoing")
	}

	// Turn enforcement (§3.3): the same party cannot counter twice in a row.
	if n.LastModifiedByRouting == offer.LastModifiedBy.RoutingNumber &&
		n.LastModifiedByID == offer.LastModifiedBy.ID {
		return errors.ConflictErr("turn violation: same party cannot counter twice in a row")
	}

	// Immutable fields.
	if n.BuyerRoutingNumber != offer.BuyerID.RoutingNumber || n.BuyerID != offer.BuyerID.ID {
		return errors.BadRequestErr("buyerId cannot change during negotiation")
	}
	if n.SellerRoutingNumber != offer.SellerID.RoutingNumber || n.SellerID != offer.SellerID.ID {
		return errors.BadRequestErr("sellerId cannot change during negotiation")
	}
	if n.Ticker != offer.Ticker {
		return errors.BadRequestErr("ticker cannot change during negotiation")
	}

	// Apply counter-offer.
	n.Amount = offer.Amount
	n.PricePerStock = offer.PricePerStock
	n.PriceCurrency = offer.PriceCurrency
	n.Premium = offer.Premium
	n.PremiumCurrency = offer.PremiumCurrency
	n.SettlementDate = offer.SettlementDate
	n.LastModifiedByRouting = offer.LastModifiedBy.RoutingNumber
	n.LastModifiedByID = offer.LastModifiedBy.ID

	if err := s.negotiations.Update(ctx, n); err != nil {
		return errors.InternalErr(err)
	}

	return nil
}

// Close handles §3.5 DELETE /interbank/negotiations/:rn/:id — either party
// may withdraw from the negotiation. Operation is idempotent: closing an
// already-closed negotiation returns success without changing state.
func (s *PeerOtcService) Close(ctx context.Context, senderRouting, routingNumber int, id string) error {
	if routingNumber != s.peers.OurRoutingNumber() {
		return errors.BadRequestErr("routingNumber does not match this bank")
	}

	n, err := s.negotiations.FindByID(ctx, id)
	if err != nil {
		return errors.InternalErr(err)
	}
	if n == nil {
		return errors.NotFoundErr("negotiation not found")
	}

	if senderRouting != n.BuyerRoutingNumber && senderRouting != n.SellerRoutingNumber {
		return errors.ForbiddenErr("sender is not a party to this negotiation")
	}

	// Idempotent: leave already-closed negotiations alone.
	if n.Status != model.PeerNegotiationOngoing {
		return nil
	}

	n.Status = model.PeerNegotiationCancelled
	if err := s.negotiations.Update(ctx, n); err != nil {
		return errors.InternalErr(err)
	}

	return nil
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

// ---------------------------------------------------------------------------
// Frontend-facing operations (driven by our authenticated users via JWT).
// ---------------------------------------------------------------------------

// LocalCreateRequest is the input our users submit when initiating a
// cross-bank negotiation against a peer seller.
type LocalCreateRequest struct {
	SellerID        dto.ForeignBankId
	Ticker          string
	Amount          int
	PricePerStock   float64
	PriceCurrency   string
	Premium         float64
	PremiumCurrency string
	SettlementDate  string
}

// LocalCounterRequest is the input our users submit on counter-offer.
type LocalCounterRequest struct {
	Amount          int
	PricePerStock   float64
	PriceCurrency   string
	Premium         float64
	PremiumCurrency string
	SettlementDate  string
}

// ListAllPeerPublicStocks aggregates §3.1 public-stock listings from every
// peer in the registry. Peers that fail are skipped silently; partial
// results are returned so a single unreachable peer doesn't tank the page.
func (s *PeerOtcService) ListAllPeerPublicStocks(ctx context.Context) ([]dto.PublicStock, error) {
	var out []dto.PublicStock
	for _, peer := range s.peers.All() {
		stocks, err := s.client.PublicStock(ctx, peer.RoutingNumber)
		if err != nil {
			// Best-effort: drop this peer from the page but keep going.
			continue
		}
		out = append(out, stocks...)
	}
	return out, nil
}

// ListMyNegotiations returns every cross-bank negotiation in which the
// given local user is a party (either buyer or seller).
func (s *PeerOtcService) ListMyNegotiations(ctx context.Context, localUserID uint) ([]dto.OtcNegotiation, error) {
	rows, err := s.negotiations.ListByParty(ctx, s.peers.OurRoutingNumber(), strconv.FormatUint(uint64(localUserID), 10))
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	out := make([]dto.OtcNegotiation, 0, len(rows))
	for i := range rows {
		out = append(out, *toNegotiationDTO(&rows[i], s.peers.OurRoutingNumber()))
	}
	return out, nil
}

// CreateForLocalBuyer initiates a cross-bank negotiation: our user is the
// buyer, the seller lives on the peer. We POST §3.2 to the seller's bank,
// store a mirror row locally with IsAuthoritative=false, and return the
// authoritative id assigned by the peer.
func (s *PeerOtcService) CreateForLocalBuyer(ctx context.Context, localUserID uint, req LocalCreateRequest) (*dto.ForeignBankId, error) {
	if req.SellerID.RoutingNumber == s.peers.OurRoutingNumber() {
		return nil, errors.BadRequestErr("seller is on this bank — use the same-bank OTC API")
	}

	buyer := dto.ForeignBankId{
		RoutingNumber: s.peers.OurRoutingNumber(),
		ID:            strconv.FormatUint(uint64(localUserID), 10),
	}

	offer := dto.OtcOffer{
		BuyerID:         buyer,
		SellerID:        req.SellerID,
		Ticker:          req.Ticker,
		Amount:          req.Amount,
		PricePerStock:   req.PricePerStock,
		PriceCurrency:   req.PriceCurrency,
		Premium:         req.Premium,
		PremiumCurrency: req.PremiumCurrency,
		SettlementDate:  req.SettlementDate,
		LastModifiedBy:  buyer,
	}
	if err := s.validateOffer(offer); err != nil {
		return nil, err
	}

	remoteID, err := s.client.CreateNegotiation(ctx, offer)
	if err != nil {
		return nil, err
	}

	remoteIDValue := remoteID.ID
	mirror := &model.PeerNegotiation{
		ID:                    uuid.NewString(),
		BuyerRoutingNumber:    buyer.RoutingNumber,
		BuyerID:               buyer.ID,
		SellerRoutingNumber:   req.SellerID.RoutingNumber,
		SellerID:              req.SellerID.ID,
		Ticker:                req.Ticker,
		Amount:                req.Amount,
		PricePerStock:         req.PricePerStock,
		PriceCurrency:         req.PriceCurrency,
		Premium:               req.Premium,
		PremiumCurrency:       req.PremiumCurrency,
		SettlementDate:        req.SettlementDate,
		LastModifiedByRouting: buyer.RoutingNumber,
		LastModifiedByID:      buyer.ID,
		Status:                model.PeerNegotiationOngoing,
		IsAuthoritative:       false,
		RemoteNegotiationID:   &remoteIDValue,
	}
	if err := s.negotiations.Create(ctx, mirror); err != nil {
		return nil, errors.InternalErr(err)
	}

	return remoteID, nil
}

// SendCounterOfferAsLocal posts a counter-offer from our user against an
// existing cross-bank negotiation. negotiationID is the authoritative id
// (the seller's bank routing + their opaque id).
func (s *PeerOtcService) SendCounterOfferAsLocal(
	ctx context.Context,
	localUserID uint,
	negotiationID dto.ForeignBankId,
	req LocalCounterRequest,
) error {
	mirror, err := s.findLocalMirrorByRemote(ctx, negotiationID, localUserID)
	if err != nil {
		return err
	}

	me := dto.ForeignBankId{
		RoutingNumber: s.peers.OurRoutingNumber(),
		ID:            strconv.FormatUint(uint64(localUserID), 10),
	}

	offer := dto.OtcOffer{
		BuyerID:         dto.ForeignBankId{RoutingNumber: mirror.BuyerRoutingNumber, ID: mirror.BuyerID},
		SellerID:        dto.ForeignBankId{RoutingNumber: mirror.SellerRoutingNumber, ID: mirror.SellerID},
		Ticker:          mirror.Ticker,
		Amount:          req.Amount,
		PricePerStock:   req.PricePerStock,
		PriceCurrency:   req.PriceCurrency,
		Premium:         req.Premium,
		PremiumCurrency: req.PremiumCurrency,
		SettlementDate:  req.SettlementDate,
		LastModifiedBy:  me,
	}
	if err := s.validateOffer(offer); err != nil {
		return err
	}

	if err := s.client.UpdateCounter(ctx, negotiationID, offer); err != nil {
		return err
	}

	// Mirror the update locally so reads of "my negotiations" stay fresh.
	mirror.Amount = req.Amount
	mirror.PricePerStock = req.PricePerStock
	mirror.PriceCurrency = req.PriceCurrency
	mirror.Premium = req.Premium
	mirror.PremiumCurrency = req.PremiumCurrency
	mirror.SettlementDate = req.SettlementDate
	mirror.LastModifiedByRouting = me.RoutingNumber
	mirror.LastModifiedByID = me.ID
	if err := s.negotiations.Update(ctx, mirror); err != nil {
		return errors.InternalErr(err)
	}

	return nil
}

// WithdrawAsLocal closes a cross-bank negotiation from our side and
// notifies the peer.
func (s *PeerOtcService) WithdrawAsLocal(
	ctx context.Context,
	localUserID uint,
	negotiationID dto.ForeignBankId,
) error {
	mirror, err := s.findLocalMirrorByRemote(ctx, negotiationID, localUserID)
	if err != nil {
		return err
	}

	if err := s.client.Close(ctx, negotiationID); err != nil {
		return err
	}

	if mirror.Status != model.PeerNegotiationOngoing {
		return nil
	}
	mirror.Status = model.PeerNegotiationCancelled
	if err := s.negotiations.Update(ctx, mirror); err != nil {
		return errors.InternalErr(err)
	}

	return nil
}

// ListLocalPublicStocks serves §3.1 GET /interbank/public-stock — peer
// banks call us asking for our users' publicly-listed stocks. We pull
// from trading-service via gRPC and map to the §3.1 wire shape; every
// owner is stamped with our routing number.
func (s *PeerOtcService) ListLocalPublicStocks(ctx context.Context) ([]dto.PublicStock, error) {
	resp, err := s.tradingClient.ListPublicStocks(ctx)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	ours := s.peers.OurRoutingNumber()
	out := make([]dto.PublicStock, 0, len(resp.GetStocks()))
	for _, entry := range resp.GetStocks() {
		sellers := make([]dto.PublicStockSeller, 0, len(entry.GetSellers()))
		for _, seller := range entry.GetSellers() {
			sellers = append(sellers, dto.PublicStockSeller{
				Seller: dto.ForeignBankId{
					RoutingNumber: ours,
					ID:            strconv.FormatUint(seller.GetSellerId(), 10),
				},
				Amount: int(seller.GetAmount()),
			})
		}
		out = append(out, dto.PublicStock{
			Stock:   dto.StockDescription{Ticker: entry.GetTicker()},
			Sellers: sellers,
		})
	}

	return out, nil
}

// LookupLocalUser serves §3.7 GET /interbank/user/:rn/:id — peer banks
// resolve a foreign user id we own into a display name. routingNumber
// must match ours; id is the local uint user id encoded as decimal.
// Returns 404 when the user is not found.
func (s *PeerOtcService) LookupLocalUser(ctx context.Context, routingNumber int, id string) (*dto.UserInformation, error) {
	if routingNumber != s.peers.OurRoutingNumber() {
		return nil, errors.BadRequestErr("routingNumber does not match this bank")
	}

	userID, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return nil, errors.BadRequestErr("user id must be a positive integer")
	}

	resp, err := s.userClient.GetClientByID(ctx, userID)
	if err != nil {
		if grpcStatus, ok := status.FromError(err); ok && grpcStatus.Code() == codes.NotFound {
			return nil, errors.NotFoundErr("user not found")
		}
		return nil, errors.InternalErr(err)
	}

	return &dto.UserInformation{
		BankDisplayName: s.peers.OurBankDisplayName(),
		DisplayName:     resp.GetFullName(),
	}, nil
}

// findLocalMirrorByRemote loads our mirror row for an authoritative
// negotiation id and verifies that the calling user is a party to it.
func (s *PeerOtcService) findLocalMirrorByRemote(
	ctx context.Context,
	negotiationID dto.ForeignBankId,
	localUserID uint,
) (*model.PeerNegotiation, error) {
	userIDStr := strconv.FormatUint(uint64(localUserID), 10)
	ourRouting := s.peers.OurRoutingNumber()

	rows, err := s.negotiations.ListByParty(ctx, ourRouting, userIDStr)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	for i := range rows {
		row := &rows[i]
		if row.IsAuthoritative {
			continue
		}
		if row.RemoteNegotiationID == nil || *row.RemoteNegotiationID != negotiationID.ID {
			continue
		}
		if row.SellerRoutingNumber != negotiationID.RoutingNumber {
			continue
		}
		return row, nil
	}

	return nil, errors.NotFoundErr("negotiation not found for caller")
}
