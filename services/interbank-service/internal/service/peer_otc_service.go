package service

import (
	"context"
	"fmt"
	"net/http"
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
	contracts     repository.PeerContractRepository
	peers         *PeerResolver
	client        *PeerOtcClient
	tradingClient client.TradingClient
	userClient    client.UserClient
	bankingClient client.BankingClient
	processor     *MessageProcessor
	outboundRepo  repository.OutboundMessageRepository
	txManager     repository.TransactionManager
}

func NewPeerOtcService(
	negotiations repository.PeerNegotiationRepository,
	contracts repository.PeerContractRepository,
	peers *PeerResolver,
	peerClient *PeerOtcClient,
	tradingClient client.TradingClient,
	userClient client.UserClient,
	bankingClient client.BankingClient,
	processor *MessageProcessor,
	outboundRepo repository.OutboundMessageRepository,
	txManager repository.TransactionManager,
) *PeerOtcService {
	return &PeerOtcService{
		negotiations:  negotiations,
		contracts:     contracts,
		peers:         peers,
		client:        peerClient,
		tradingClient: tradingClient,
		userClient:    userClient,
		bankingClient: bankingClient,
		processor:     processor,
		outboundRepo:  outboundRepo,
		txManager:     txManager,
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
		ID:                  uuid.NewString(),
		BuyerRoutingNumber:  offer.BuyerID.RoutingNumber,
		BuyerID:             offer.BuyerID.ID,
		SellerRoutingNumber: offer.SellerID.RoutingNumber,
		SellerID:            offer.SellerID.ID,
		Ticker:              offer.Stock.Ticker,
		BuyerAccountNumber:  offer.BuyerAccountNumber,
		Status:              model.PeerNegotiationOngoing,
		IsAuthoritative:     true,
	}
	applyNegotiableTerms(n, offer)

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

	return toPeerNegotiationDTO(n), nil
}

// UpdateCounter handles §3.3 PUT /interbank/negotiations/:rn/:id — a peer
// bank posts a counter-offer. The path :rn always carries the negotiation's
// authoritative (seller's) routing number, so the recipient may be either the
// authoritative seller's bank (buyer countered) or the buyer's mirror-holding
// bank (seller countered) — both are handled here.
//
// Per spec §3.3: a 409 is returned when the same party tries to counter
// twice in a row (turn violation) or when the negotiation is closed.
// Buyer/seller identities and ticker are immutable for the lifetime of
// the negotiation; only the negotiable parameters may change.
func (s *PeerOtcService) UpdateCounter(ctx context.Context, senderRouting, routingNumber int, id string, offer dto.OtcOffer) error {
	if err := s.validateOffer(offer); err != nil {
		return err
	}
	if offer.LastModifiedBy.RoutingNumber != senderRouting {
		return errors.UnauthorizedErr("lastModifiedBy.routingNumber does not match sender")
	}

	return s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		n, err := s.negotiations.FindByIDForUpdate(ctx, id)
		if err != nil {
			return errors.InternalErr(err)
		}
		if n == nil {
			return errors.NotFoundErr("negotiation not found")
		}

		// The path routing number must be the negotiation's authoritative
		// (seller's) routing number, regardless of which side we hold.
		if routingNumber != n.SellerRoutingNumber {
			return errors.BadRequestErr("routingNumber does not match the negotiation")
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
		if n.Ticker != offer.Stock.Ticker {
			return errors.BadRequestErr("ticker cannot change during negotiation")
		}
		if n.BuyerAccountNumber != offer.BuyerAccountNumber {
			return errors.BadRequestErr("buyerAccountNumber cannot change during negotiation")
		}

		applyNegotiableTerms(n, offer)

		if err := s.negotiations.Update(ctx, n); err != nil {
			return errors.InternalErr(err)
		}
		return nil
	})
}

// Close handles §3.5 DELETE /interbank/negotiations/:rn/:id — either party
// may withdraw from the negotiation; the path :rn is the authoritative
// (seller's) routing number, so either the authoritative or the mirror side
// may receive this. Operation is idempotent: closing an already-closed
// negotiation returns success without changing state.
func (s *PeerOtcService) Close(ctx context.Context, senderRouting, routingNumber int, id string) error {
	return s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		n, err := s.negotiations.FindByIDForUpdate(ctx, id)
		if err != nil {
			return errors.InternalErr(err)
		}
		if n == nil {
			return errors.NotFoundErr("negotiation not found")
		}

		if routingNumber != n.SellerRoutingNumber {
			return errors.BadRequestErr("routingNumber does not match the negotiation")
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
	})
}

func (s *PeerOtcService) validateOffer(o dto.OtcOffer) error {
	if strings.TrimSpace(o.Stock.Ticker) == "" {
		return errors.BadRequestErr("ticker is required")
	}
	if o.Amount <= 0 {
		return errors.BadRequestErr("amount must be positive")
	}
	if o.PricePerUnit.Amount <= 0 {
		return errors.BadRequestErr("pricePerUnit.amount must be positive")
	}
	if o.Premium.Amount < 0 {
		return errors.BadRequestErr("premium.amount must be non-negative")
	}
	if strings.TrimSpace(o.BuyerAccountNumber) == "" {
		return errors.BadRequestErr("buyerAccountNumber is required")
	}
	if _, err := time.Parse(time.RFC3339, o.SettlementDate); err != nil {
		if _, err2 := time.Parse("2006-01-02", o.SettlementDate); err2 != nil {
			return errors.BadRequestErr("settlementDate must be ISO 8601 (date or datetime)")
		}
	}
	return nil
}

// applyNegotiableTerms copies the mutable §3.3 fields (everything except the
// fixed parties, ticker and buyer account) from a wire offer onto the model.
func applyNegotiableTerms(n *model.PeerNegotiation, o dto.OtcOffer) {
	n.Amount = o.Amount
	n.PricePerStock = o.PricePerUnit.Amount
	n.PriceCurrency = string(o.PricePerUnit.Currency)
	n.Premium = o.Premium.Amount
	n.PremiumCurrency = string(o.Premium.Currency)
	n.SettlementDate = o.SettlementDate
	n.LastModifiedByRouting = o.LastModifiedBy.RoutingNumber
	n.LastModifiedByID = o.LastModifiedBy.ID
}

// offerFromModel rebuilds the spec-shaped wire OtcOffer from the flat model.
func offerFromModel(n *model.PeerNegotiation) dto.OtcOffer {
	return dto.OtcOffer{
		Stock:              dto.StockDescription{Ticker: n.Ticker},
		SettlementDate:     n.SettlementDate,
		PricePerUnit:       dto.MonetaryValue{Currency: dto.CurrencyCode(n.PriceCurrency), Amount: n.PricePerStock},
		Premium:            dto.MonetaryValue{Currency: dto.CurrencyCode(n.PremiumCurrency), Amount: n.Premium},
		BuyerID:            dto.ForeignBankId{RoutingNumber: n.BuyerRoutingNumber, ID: n.BuyerID},
		SellerID:           dto.ForeignBankId{RoutingNumber: n.SellerRoutingNumber, ID: n.SellerID},
		Amount:             n.Amount,
		LastModifiedBy:     dto.ForeignBankId{RoutingNumber: n.LastModifiedByRouting, ID: n.LastModifiedByID},
		BuyerAccountNumber: n.BuyerAccountNumber,
	}
}

// toPeerNegotiationDTO maps the model to the §3.4 wire response (OtcOffer +
// isOngoing) served to peer banks.
func toPeerNegotiationDTO(n *model.PeerNegotiation) *dto.OtcNegotiation {
	return &dto.OtcNegotiation{
		OtcOffer:  offerFromModel(n),
		IsOngoing: n.Status == model.PeerNegotiationOngoing,
	}
}

// toNegotiationView maps the model to the richer frontend view (keeps the
// negotiation id, human status and timestamp for our own UI).
func toNegotiationView(n *model.PeerNegotiation) *dto.OtcNegotiationView {
	return &dto.OtcNegotiationView{
		ID:        dto.ForeignBankId{RoutingNumber: n.SellerRoutingNumber, ID: n.ID},
		Status:    strings.ToLower(string(n.Status)),
		UpdatedAt: n.UpdatedAt.Format(time.RFC3339),
		Offer:     offerFromModel(n),
	}
}

func toPeerContractDTO(c *model.PeerContract) *dto.PeerContract {
	var exercisedAt *string
	if c.ExercisedAt != nil {
		v := c.ExercisedAt.Format(time.RFC3339)
		exercisedAt = &v
	}

	return &dto.PeerContract{
		ID:            dto.ForeignBankId{RoutingNumber: c.AuthorityRoutingNumber, ID: c.ID},
		NegotiationID: dto.ForeignBankId{RoutingNumber: c.AuthorityRoutingNumber, ID: c.NegotiationID},
		BuyerID:       dto.ForeignBankId{RoutingNumber: c.BuyerRoutingNumber, ID: c.BuyerID},
		SellerID:      dto.ForeignBankId{RoutingNumber: c.SellerRoutingNumber, ID: c.SellerID},
		Ticker:        c.Ticker,
		Amount:        c.Amount,
		StrikePrice: dto.MonetaryValue{
			Currency: dto.CurrencyCode(c.StrikeCurrency),
			Amount:   c.StrikePrice,
		},
		Premium: dto.MonetaryValue{
			Currency: dto.CurrencyCode(c.PremiumCurrency),
			Amount:   c.Premium,
		},
		SettlementDate: c.SettlementDate,
		Status:         strings.ToLower(string(c.Status)),
		ExercisedAt:    exercisedAt,
		CreatedAt:      c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      c.UpdatedAt.Format(time.RFC3339),
	}
}

// ---------------------------------------------------------------------------
// Frontend-facing operations (driven by our authenticated users via JWT).
// ---------------------------------------------------------------------------

// LocalCreateRequest is the input our users submit when initiating a
// cross-bank negotiation against a peer seller.
type LocalCreateRequest struct {
	SellerID           dto.ForeignBankId
	Ticker             string
	Amount             int
	PricePerStock      float64
	PriceCurrency      string
	Premium            float64
	PremiumCurrency    string
	SettlementDate     string
	BuyerAccountNumber string
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

type LocalAcceptRequest struct{}

type LocalExerciseRequest struct {
	BuyerAccountNumber string
}

// ListAllPeerPublicStocks aggregates §3.1 public-stock listings from every
// peer in the registry. Peers that fail are skipped silently; partial
// results are returned so a single unreachable peer doesn't tank the page.
func (s *PeerOtcService) ListAllPeerPublicStocks(ctx context.Context) ([]dto.PublicStock, error) {
	out := make([]dto.PublicStock, 0)
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
func (s *PeerOtcService) ListMyNegotiations(ctx context.Context, localUserID uint) ([]dto.OtcNegotiationView, error) {
	rows, err := s.negotiations.ListByParty(ctx, s.peers.OurRoutingNumber(), strconv.FormatUint(uint64(localUserID), 10))
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	out := make([]dto.OtcNegotiationView, 0, len(rows))
	for i := range rows {
		out = append(out, *toNegotiationView(&rows[i]))
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
		Stock:              dto.StockDescription{Ticker: req.Ticker},
		SettlementDate:     req.SettlementDate,
		PricePerUnit:       monetary(req.PriceCurrency, req.PricePerStock),
		Premium:            monetary(req.PremiumCurrency, req.Premium),
		BuyerID:            buyer,
		SellerID:           req.SellerID,
		Amount:             req.Amount,
		LastModifiedBy:     buyer,
		BuyerAccountNumber: req.BuyerAccountNumber,
	}
	if err := s.validateOffer(offer); err != nil {
		return nil, err
	}

	remoteID, err := s.client.CreateNegotiation(ctx, offer)
	if err != nil {
		return nil, err
	}

	mirror := &model.PeerNegotiation{
		ID:                  remoteID.ID,
		BuyerRoutingNumber:  buyer.RoutingNumber,
		BuyerID:             buyer.ID,
		SellerRoutingNumber: req.SellerID.RoutingNumber,
		SellerID:            req.SellerID.ID,
		Ticker:              req.Ticker,
		BuyerAccountNumber:  req.BuyerAccountNumber,
		Status:              model.PeerNegotiationOngoing,
		IsAuthoritative:     false,
	}
	applyNegotiableTerms(mirror, offer)
	if err := s.negotiations.Create(ctx, mirror); err != nil {
		return nil, errors.InternalErr(err)
	}

	return remoteID, nil
}

// SendCounterOfferAsLocal posts a counter-offer from our user against an
// existing cross-bank negotiation. Our user may be the buyer (we hold a mirror
// row, peer is the authoritative seller) or the seller (we hold the
// authoritative row, peer is the buyer); the counter is sent to whichever bank
// is the opposing party. negotiationID is the authoritative id
// (the seller's bank routing + their opaque id).
func (s *PeerOtcService) SendCounterOfferAsLocal(
	ctx context.Context,
	localUserID uint,
	negotiationID dto.ForeignBankId,
	req LocalCounterRequest,
) error {
	n, err := s.findLocalNegotiation(ctx, negotiationID, localUserID)
	if err != nil {
		return err
	}

	me := dto.ForeignBankId{
		RoutingNumber: s.peers.OurRoutingNumber(),
		ID:            strconv.FormatUint(uint64(localUserID), 10),
	}

	if n.Status != model.PeerNegotiationOngoing {
		return errors.ConflictErr("negotiation is not ongoing")
	}
	// Turn rule: you cannot counter your own latest offer.
	if n.LastModifiedByRouting == me.RoutingNumber && n.LastModifiedByID == me.ID {
		return errors.ConflictErr("turn violation: you cannot counter your own latest offer")
	}

	offer := dto.OtcOffer{
		Stock:              dto.StockDescription{Ticker: n.Ticker},
		SettlementDate:     req.SettlementDate,
		PricePerUnit:       monetary(req.PriceCurrency, req.PricePerStock),
		Premium:            monetary(req.PremiumCurrency, req.Premium),
		BuyerID:            dto.ForeignBankId{RoutingNumber: n.BuyerRoutingNumber, ID: n.BuyerID},
		SellerID:           dto.ForeignBankId{RoutingNumber: n.SellerRoutingNumber, ID: n.SellerID},
		Amount:             req.Amount,
		LastModifiedBy:     me,
		BuyerAccountNumber: n.BuyerAccountNumber,
	}
	if err := s.validateOffer(offer); err != nil {
		return err
	}

	// Notify the opposing bank (§3.3). The path id is always the authoritative
	// (seller's) routing + opaque id; the request goes to whichever bank is the
	// other party.
	authoritativeID := dto.ForeignBankId{RoutingNumber: n.SellerRoutingNumber, ID: n.ID}
	if err := s.client.UpdateCounter(ctx, s.opposingRouting(n, me), authoritativeID, offer); err != nil {
		return err
	}

	// Reflect the update on our local copy (authoritative or mirror).
	applyNegotiableTerms(n, offer)
	if err := s.negotiations.Update(ctx, n); err != nil {
		return errors.InternalErr(err)
	}

	return nil
}

func (s *PeerOtcService) AcceptFromPeer(ctx context.Context, senderRouting, routingNumber int, id string) (*dto.PeerContract, error) {
	ourRouting := s.peers.OurRoutingNumber()

	// Lock the negotiation row so concurrent accepts serialize on its status.
	// The contract-existence fast-path below runs after the lock is released;
	// true dedup of concurrent accepts is guaranteed by the deterministic accept
	// transaction id (peer-otc-accept-<seller>-<id>) — PrepareLocalTransaction
	// short-circuits on the existing PreparedTransaction row, and the contract's
	// composite primary key rejects a duplicate insert — so a second concurrent
	// accept cannot double-charge the premium or double-reserve shares.
	var n *model.PeerNegotiation
	if err := s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		locked, err := s.negotiations.FindByIDForUpdate(ctx, id)
		if err != nil {
			return errors.InternalErr(err)
		}
		if locked == nil {
			return errors.NotFoundErr("negotiation not found")
		}
		// The path {rn} must name the negotiation's authoritative (seller's) bank —
		// the shared id both banks use — and we must be a party to it. We may be the
		// authority (seller) or the non-authority (buyer): the accepting party's bank
		// forwards the accept to us, the opposing party's bank, and we form the TX.
		if routingNumber != locked.SellerRoutingNumber {
			return errors.BadRequestErr("routingNumber does not identify this negotiation")
		}
		if ourRouting != locked.BuyerRoutingNumber && ourRouting != locked.SellerRoutingNumber {
			return errors.ForbiddenErr("this bank is not a party to this negotiation")
		}
		if senderRouting != locked.BuyerRoutingNumber && senderRouting != locked.SellerRoutingNumber {
			return errors.ForbiddenErr("sender is not a party to this negotiation")
		}
		if locked.Status != model.PeerNegotiationOngoing && locked.Status != model.PeerNegotiationAccepted {
			return errors.ConflictErr("negotiation is not ongoing")
		}
		if locked.Status == model.PeerNegotiationOngoing && locked.LastModifiedByRouting == senderRouting {
			return errors.ConflictErr("acceptor must be opposite to lastModifiedBy")
		}
		n = locked
		return nil
	}); err != nil {
		return nil, err
	}

	existing, err := s.contracts.FindByID(ctx, n.SellerRoutingNumber, n.ID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if existing != nil {
		return toPeerContractDTO(existing), nil
	}

	if err := s.coordinateAcceptTransaction(ctx, n); err != nil {
		return nil, err
	}

	contract, err := s.contracts.FindByID(ctx, n.SellerRoutingNumber, n.ID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if contract == nil {
		return nil, errors.InternalErr(fmt.Errorf("contract not created after accept"))
	}
	return toPeerContractDTO(contract), nil
}

func (s *PeerOtcService) AcceptAsLocal(ctx context.Context, localUserID uint, negotiationID dto.ForeignBankId, _ LocalAcceptRequest) (*dto.PeerContract, error) {
	userIDStr := strconv.FormatUint(uint64(localUserID), 10)
	ourRouting := s.peers.OurRoutingNumber()

	var n *model.PeerNegotiation
	if negotiationID.RoutingNumber == ourRouting {
		var err error
		n, err = s.negotiations.FindByID(ctx, negotiationID.ID)
		if err != nil {
			return nil, errors.InternalErr(err)
		}
		if n == nil {
			return nil, errors.NotFoundErr("negotiation not found")
		}
	} else {
		var err error
		n, err = s.findLocalMirrorByRemote(ctx, negotiationID, localUserID)
		if err != nil {
			return nil, err
		}
	}

	isOurBuyer := n.BuyerRoutingNumber == ourRouting && n.BuyerID == userIDStr
	isOurSeller := n.SellerRoutingNumber == ourRouting && n.SellerID == userIDStr
	if !isOurBuyer && !isOurSeller {
		return nil, errors.ForbiddenErr("local user is not a party to this negotiation")
	}
	if n.Status != model.PeerNegotiationOngoing && n.Status != model.PeerNegotiationAccepted {
		return nil, errors.ConflictErr("negotiation is not ongoing")
	}
	if n.Status == model.PeerNegotiationOngoing && n.LastModifiedByRouting == ourRouting && n.LastModifiedByID == userIDStr {
		return nil, errors.ConflictErr("you cannot accept your own latest offer")
	}

	existing, err := s.contracts.FindByID(ctx, negotiationID.RoutingNumber, negotiationID.ID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if existing != nil {
		return toPeerContractDTO(existing), nil
	}

	// Always forward the accept to the opposing party's bank — the accepting
	// bank never forms the transaction itself; the counterparty does. The path
	// carries the authoritative (seller's) routing number, the shared negotiation
	// id both banks use, while the destination is the opposing party's bank.
	me := dto.ForeignBankId{RoutingNumber: ourRouting, ID: userIDStr}
	if _, err := s.client.Accept(ctx, s.opposingRouting(n, me), negotiationID); err != nil {
		return nil, err
	}

	contract, err := s.contracts.FindByID(ctx, negotiationID.RoutingNumber, negotiationID.ID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if contract == nil {
		return nil, errors.InternalErr(fmt.Errorf("contract not created after accept"))
	}
	return toPeerContractDTO(contract), nil
}

func (s *PeerOtcService) ListMyContracts(ctx context.Context, localUserID uint) ([]dto.PeerContract, error) {
	rows, err := s.contracts.ListByParty(ctx, s.peers.OurRoutingNumber(), strconv.FormatUint(uint64(localUserID), 10))
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	out := make([]dto.PeerContract, 0, len(rows))
	for i := range rows {
		out = append(out, *toPeerContractDTO(&rows[i]))
	}
	return out, nil
}

func (s *PeerOtcService) ExerciseAsLocal(ctx context.Context, localUserID uint, contractID dto.ForeignBankId, buyerAccountNumber string) (*dto.PeerContract, error) {
	contract, err := s.contracts.FindByID(ctx, contractID.RoutingNumber, contractID.ID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if contract == nil {
		return nil, errors.NotFoundErr("contract not found")
	}

	userIDStr := strconv.FormatUint(uint64(localUserID), 10)
	ourRouting := s.peers.OurRoutingNumber()
	if contract.BuyerRoutingNumber != ourRouting || contract.BuyerID != userIDStr {
		return nil, errors.ForbiddenErr("only the local buyer may exercise this peer OTC contract")
	}
	if contract.Status != model.PeerContractActive {
		return nil, errors.ConflictErr("contract is not active")
	}
	if SettlementPassed(contract.SettlementDate) {
		return nil, errors.ConflictErr("option contract has expired")
	}
	if strings.TrimSpace(buyerAccountNumber) == "" {
		return nil, errors.BadRequestErr("buyerAccountNumber is required to exercise")
	}

	executionKey := fmt.Sprintf("peer-otc-exercise-%d-%s-%s", contract.AuthorityRoutingNumber, contract.ID, uuid.NewString())
	tx := s.exerciseTransaction(contract, buyerAccountNumber, executionKey)

	if err := s.coordinateTwoBankTransaction(ctx, contract.SellerRoutingNumber, tx, executionKey); err != nil {
		return nil, err
	}

	updated, err := s.contracts.FindByID(ctx, contractID.RoutingNumber, contractID.ID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	return toPeerContractDTO(updated), nil
}

// WithdrawAsLocal closes a cross-bank negotiation from our side and notifies
// the opposing party's bank (§3.5). Works whether our user is the buyer
// (mirror row) or the seller (authoritative row).
func (s *PeerOtcService) WithdrawAsLocal(
	ctx context.Context,
	localUserID uint,
	negotiationID dto.ForeignBankId,
) error {
	n, err := s.findLocalNegotiation(ctx, negotiationID, localUserID)
	if err != nil {
		return err
	}

	me := dto.ForeignBankId{
		RoutingNumber: s.peers.OurRoutingNumber(),
		ID:            strconv.FormatUint(uint64(localUserID), 10),
	}
	authoritativeID := dto.ForeignBankId{RoutingNumber: n.SellerRoutingNumber, ID: n.ID}
	if err := s.client.Close(ctx, s.opposingRouting(n, me), authoritativeID); err != nil {
		return err
	}

	if n.Status != model.PeerNegotiationOngoing {
		return nil
	}
	n.Status = model.PeerNegotiationCancelled
	if err := s.negotiations.Update(ctx, n); err != nil {
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

	row, err := s.negotiations.FindByID(ctx, negotiationID.ID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if row == nil || row.IsAuthoritative || row.SellerRoutingNumber != negotiationID.RoutingNumber {
		return nil, errors.NotFoundErr("negotiation not found for caller")
	}
	if row.BuyerID != userIDStr && row.SellerID != userIDStr {
		return nil, errors.NotFoundErr("negotiation not found for caller")
	}
	return row, nil
}

// findLocalNegotiation loads a negotiation by its authoritative id and verifies
// the calling local user is a party — buyer or seller, mirror or authoritative.
// Unlike findLocalMirrorByRemote it does not require a mirror row, so it serves
// the seller side of a cross-bank negotiation too.
func (s *PeerOtcService) findLocalNegotiation(
	ctx context.Context,
	negotiationID dto.ForeignBankId,
	localUserID uint,
) (*model.PeerNegotiation, error) {
	userIDStr := strconv.FormatUint(uint64(localUserID), 10)
	ourRouting := s.peers.OurRoutingNumber()

	row, err := s.negotiations.FindByID(ctx, negotiationID.ID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	// The negotiation id's routing number is always the authoritative
	// (seller's) bank, so it must match the stored seller routing.
	if row == nil || row.SellerRoutingNumber != negotiationID.RoutingNumber {
		return nil, errors.NotFoundErr("negotiation not found for caller")
	}
	isLocalBuyer := row.BuyerRoutingNumber == ourRouting && row.BuyerID == userIDStr
	isLocalSeller := row.SellerRoutingNumber == ourRouting && row.SellerID == userIDStr
	if !isLocalBuyer && !isLocalSeller {
		return nil, errors.NotFoundErr("negotiation not found for caller")
	}
	return row, nil
}

// opposingRouting returns the routing number of the party that is NOT the given
// local user — i.e. the bank a counter/close notification must be sent to.
func (s *PeerOtcService) opposingRouting(n *model.PeerNegotiation, me dto.ForeignBankId) int {
	if n.BuyerRoutingNumber == me.RoutingNumber && n.BuyerID == me.ID {
		return n.SellerRoutingNumber // we are the buyer → notify the seller
	}
	return n.BuyerRoutingNumber // we are the seller → notify the buyer
}

// monetary is a small constructor for the wire MonetaryValue shape.
func monetary(currency string, amount float64) dto.MonetaryValue {
	return dto.MonetaryValue{Currency: dto.CurrencyCode(currency), Amount: amount}
}

func (s *PeerOtcService) coordinateAcceptTransaction(ctx context.Context, n *model.PeerNegotiation) error {
	tx := s.acceptTransaction(n)
	peerRouting := n.BuyerRoutingNumber
	if peerRouting == s.peers.OurRoutingNumber() {
		peerRouting = n.SellerRoutingNumber
	}
	return s.coordinateTwoBankTransaction(ctx, peerRouting, tx, fmt.Sprintf("peer-otc-accept-%d-%s", n.SellerRoutingNumber, n.ID))
}

func (s *PeerOtcService) acceptTransaction(n *model.PeerNegotiation) dto.Transaction {
	negotiationID := dto.ForeignBankId{RoutingNumber: n.SellerRoutingNumber, ID: n.ID}
	optionAsset := dto.Asset{
		Type: dto.AssetOption,
		Body: map[string]any{
			"negotiationId": map[string]any{
				"routingNumber": negotiationID.RoutingNumber,
				"id":            negotiationID.ID,
			},
			"stock": map[string]any{
				"ticker": n.Ticker,
			},
			"pricePerUnit": map[string]any{
				"currency": n.PriceCurrency,
				"amount":   n.PricePerStock,
			},
			"settlementDate": n.SettlementDate,
			"amount":         float64(n.Amount),
		},
	}
	monasAsset := dto.Asset{
		Type: dto.AssetMonas,
		Body: map[string]any{"currency": n.PremiumCurrency},
	}

	return dto.Transaction{
		TransactionID: dto.ForeignBankId{
			RoutingNumber: s.peers.OurRoutingNumber(),
			ID:            fmt.Sprintf("peer-otc-accept-%d-%s", n.SellerRoutingNumber, n.ID),
		},
		Message:        "Peer OTC option premium and contract acceptance",
		PaymentCode:    "289",
		PaymentPurpose: "OTC option premium",
		// §3.6 accept postings. Sign convention (§2.8): amount < 0 is a CREDIT
		// (asset leaves the account), amount > 0 is a DEBIT (asset enters).
		Postings: []dto.Posting{
			// posting 1: buyer ACCOUNT MONAS CREDIT (amount < 0) — buyer pays the premium
			{
				Account: dto.TxAccount{Type: dto.TxAccountAccount, Num: &[]string{n.BuyerAccountNumber}[0]},
				Amount:  -n.Premium,
				Asset:   monasAsset,
			},
			// posting 2: seller PERSON MONAS DEBIT (amount > 0) — seller receives the premium (resolved by ClientId on seller's bank)
			{
				Account: personAccount(n.SellerRoutingNumber, n.SellerID),
				Amount:  n.Premium,
				Asset:   monasAsset,
			},
			// posting 3: seller PERSON OPTION CREDIT (amount < 0) — seller gives the option (shares reserved)
			{
				Account: personAccount(n.SellerRoutingNumber, n.SellerID),
				Amount:  -1,
				Asset:   optionAsset,
			},
			// posting 4: buyer PERSON OPTION DEBIT (amount > 0) — buyer receives the option
			{
				Account: personAccount(n.BuyerRoutingNumber, n.BuyerID),
				Amount:  1,
				Asset:   optionAsset,
			},
		},
	}
}

func (s *PeerOtcService) exerciseTransaction(contract *model.PeerContract, buyerAccountNumber, executionKey string) dto.Transaction {
	amount := float64(contract.Amount) * contract.StrikePrice
	contractID := dto.ForeignBankId{RoutingNumber: contract.AuthorityRoutingNumber, ID: contract.ID}
	stockAsset := dto.Asset{Type: dto.AssetStock, Body: map[string]any{"ticker": contract.Ticker}}
	monasAsset := dto.Asset{Type: dto.AssetMonas, Body: map[string]any{"currency": contract.StrikeCurrency}}

	return dto.Transaction{
		TransactionID: dto.ForeignBankId{
			RoutingNumber: s.peers.OurRoutingNumber(),
			ID:            executionKey,
		},
		Message:        "Peer OTC option exercise",
		PaymentCode:    "289",
		PaymentPurpose: "OTC option exercise",
		// §2.7.2 option-execution postings. Sign convention (§2.8): amount < 0 is
		// a CREDIT (asset leaves the account), amount > 0 is a DEBIT (asset enters).
		Postings: []dto.Posting{
			// posting 1: buyer ACCOUNT MONAS CREDIT (amount < 0) — buyer pays π·k strike
			{
				Account: dto.TxAccount{Type: dto.TxAccountAccount, Num: &[]string{buyerAccountNumber}[0]},
				Amount:  -amount,
				Asset:   monasAsset,
			},
			// posting 2: option OPTION MONAS DEBIT (amount > 0) — strike paid into the pseudo-account (forwarded to seller on commit)
			{
				Account: dto.TxAccount{Type: dto.TxAccountOption, ID: &contractID},
				Amount:  amount,
				Asset:   monasAsset,
			},
			// posting 3: option OPTION STOCK CREDIT (amount < 0) — reserved shares leave the pseudo-account
			{
				Account: dto.TxAccount{Type: dto.TxAccountOption, ID: &contractID},
				Amount:  -float64(contract.Amount),
				Asset:   stockAsset,
			},
			// posting 4: buyer PERSON STOCK DEBIT (amount > 0) — buyer receives k shares
			{
				Account: personAccount(contract.BuyerRoutingNumber, contract.BuyerID),
				Amount:  float64(contract.Amount),
				Asset:   stockAsset,
			},
		},
	}
}

func (s *PeerOtcService) coordinateTwoBankTransaction(ctx context.Context, peerRouting int, tx dto.Transaction, keyPrefix string) error {
	newTxKey := keyPrefix + "-new"

	// Step 1: prepare locally + enqueue NEW_TX outbox row atomically.
	_, localVote, newTxMsg, err := s.processor.PrepareAndEnqueueNewTx(ctx, &tx, peerRouting, newTxKey, model.FlowTypeOTC, 0)
	if err != nil {
		return errors.InternalErr(err)
	}
	if localVote.Vote != dto.VoteYes {
		return errors.ConflictErr(fmt.Sprintf("local bank voted NO: %s", voteReasons(localVote)))
	}

	if peerRouting == s.peers.OurRoutingNumber() {
		// Same-bank: commit locally, no remote messaging or outbox row needed.
		_, _, err = s.processor.CommitAndEnqueueFollowUp(ctx, tx.TransactionID, peerRouting, keyPrefix+"-commit", model.FlowTypeOTC)
		if err != nil {
			return errors.InternalErr(err)
		}
		return nil
	}

	// Step 2: try to send NEW_TX to peer synchronously (optimistic path).
	remoteVote, sendErr := s.client.SendNewTx(ctx, peerRouting, newTxKey, tx)
	if sendErr != nil {
		// Network failure — outbox worker will retry NEW_TX; caller gets 503.
		return errors.ServiceUnavailableErr(sendErr)
	}
	_ = s.outboundRepo.MarkSent(ctx, newTxMsg.ID, http.StatusOK, nil)

	if remoteVote == nil || remoteVote.Vote != dto.VoteYes {
		// Step 3a: peer voted NO — the peer never prepared, so it has no
		// PreparedTransaction to roll back. Sending ROLLBACK_TX would get a 500
		// and exhaust retries pointlessly. Just roll back our own side directly
		// and cancel the now-redundant NEW_TX outbox row.
		_, _ = s.processor.RollbackLocalTransaction(ctx, tx.TransactionID)
		if newTxMsg != nil {
			_ = s.outboundRepo.Cancel(ctx, newTxMsg.ID)
		}
		return errors.ConflictErr(fmt.Sprintf("peer bank voted NO: %s", voteReasonsValue(remoteVote)))
	}

	// Step 3b: peer voted YES — commit + enqueue COMMIT_TX atomically; try sync send.
	commitKey := keyPrefix + "-commit"
	_, commitMsg, err := s.processor.CommitAndEnqueueFollowUp(ctx, tx.TransactionID, peerRouting, commitKey, model.FlowTypeOTC)
	if err != nil {
		return errors.InternalErr(err)
	}
	if commitMsg != nil {
		if err := s.client.SendCommitTx(ctx, peerRouting, commitKey, tx.TransactionID); err == nil {
			_ = s.outboundRepo.MarkSent(ctx, commitMsg.ID, http.StatusNoContent, nil)
		}
		// If sync send failed: outbox worker retries. Local is already committed — return success.
	}
	return nil
}

func personAccount(routing int, id string) dto.TxAccount {
	return dto.TxAccount{
		Type: dto.TxAccountPerson,
		ID:   &dto.ForeignBankId{RoutingNumber: routing, ID: id},
	}
}

func voteReasons(vote dto.TransactionVote) string {
	if len(vote.Reasons) == 0 {
		return "no reason provided"
	}
	parts := make([]string, 0, len(vote.Reasons))
	for _, reason := range vote.Reasons {
		parts = append(parts, string(reason.Reason))
	}
	return strings.Join(parts, ",")
}

func voteReasonsValue(vote *dto.TransactionVote) string {
	if vote == nil {
		return "no vote returned"
	}
	return voteReasons(*vote)
}

// SettlementPassed returns true when the settlement date string (ISO 8601
// date or datetime) represents a point in time that has already passed.
func SettlementPassed(settlementDate string) bool {
	if t, err := time.Parse(time.RFC3339, settlementDate); err == nil {
		return time.Now().After(t)
	}
	if t, err := time.Parse("2006-01-02", settlementDate); err == nil {
		return time.Now().After(t.Add(24 * time.Hour))
	}
	return false
}
