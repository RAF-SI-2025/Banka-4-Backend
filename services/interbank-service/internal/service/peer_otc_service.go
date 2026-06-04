package service

import (
	"context"
	stderrors "errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
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
	exercises     repository.PeerContractExerciseRepository
	peers         *PeerResolver
	client        *PeerOtcClient
	tradingClient client.TradingClient
	userClient    client.UserClient
	bankingClient client.BankingClient
	processor     *MessageProcessor
}

type remoteCommitPendingError struct {
	err error
}

func (e *remoteCommitPendingError) Error() string {
	return "local transaction committed, remote commit is still pending: " + e.err.Error()
}

func (e *remoteCommitPendingError) Unwrap() error {
	return e.err
}

func NewPeerOtcService(
	negotiations repository.PeerNegotiationRepository,
	contracts repository.PeerContractRepository,
	exercises repository.PeerContractExerciseRepository,
	peers *PeerResolver,
	peerClient *PeerOtcClient,
	tradingClient client.TradingClient,
	userClient client.UserClient,
	bankingClient client.BankingClient,
	processor *MessageProcessor,
) *PeerOtcService {
	return &PeerOtcService{
		negotiations:  negotiations,
		contracts:     contracts,
		exercises:     exercises,
		peers:         peers,
		client:        peerClient,
		tradingClient: tradingClient,
		userClient:    userClient,
		bankingClient: bankingClient,
		processor:     processor,
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

func toPeerExerciseDTO(e *model.PeerContractExercise) *dto.PeerContractExercise {
	var completedAt *string
	if e.CompletedAt != nil {
		v := e.CompletedAt.Format(time.RFC3339)
		completedAt = &v
	}

	return &dto.PeerContractExercise{
		ID:           e.ID,
		ContractID:   dto.ForeignBankId{RoutingNumber: e.ContractAuthorityRoutingNumber, ID: e.ContractID},
		ExecutionKey: e.ExecutionKey,
		CurrentStep:  string(e.CurrentStep),
		Status:       string(e.Status),
		RetryCount:   e.RetryCount,
		LastError:    e.LastError,
		CompletedAt:  completedAt,
		CreatedAt:    e.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    e.UpdatedAt.Format(time.RFC3339),
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
	AccountNumber   string
}

type LocalAcceptRequest struct {
	AccountNumber string
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

func (s *PeerOtcService) AcceptFromPeer(ctx context.Context, senderRouting, routingNumber int, id string) (*dto.PeerContract, error) {
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
	if !n.IsAuthoritative {
		return nil, errors.BadRequestErr("accept must be sent to the authoritative seller bank")
	}
	if senderRouting != n.BuyerRoutingNumber && senderRouting != n.SellerRoutingNumber {
		return nil, errors.ForbiddenErr("sender is not a party to this negotiation")
	}
	if n.Status != model.PeerNegotiationOngoing && n.Status != model.PeerNegotiationAccepted {
		return nil, errors.ConflictErr("negotiation is not ongoing")
	}
	if n.Status == model.PeerNegotiationOngoing && n.LastModifiedByRouting == senderRouting {
		return nil, errors.ConflictErr("acceptor must be opposite to lastModifiedBy")
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

	contract, err := s.ensureContractForNegotiation(ctx, n, nil, nil, true)
	if err != nil {
		return nil, err
	}

	return toPeerContractDTO(contract), nil
}

func (s *PeerOtcService) AcceptAsLocal(ctx context.Context, localUserID uint, negotiationID dto.ForeignBankId, req LocalAcceptRequest) (*dto.PeerContract, error) {
	userIDStr := strconv.FormatUint(uint64(localUserID), 10)
	ourRouting := s.peers.OurRoutingNumber()

	if negotiationID.RoutingNumber == ourRouting {
		n, err := s.negotiations.FindByID(ctx, negotiationID.ID)
		if err != nil {
			return nil, errors.InternalErr(err)
		}
		if n == nil {
			return nil, errors.NotFoundErr("negotiation not found")
		}
		if n.BuyerRoutingNumber != ourRouting && n.SellerRoutingNumber != ourRouting {
			return nil, errors.ForbiddenErr("local user is not a party to this negotiation")
		}
		if (n.BuyerRoutingNumber == ourRouting && n.BuyerID != userIDStr) &&
			(n.SellerRoutingNumber == ourRouting && n.SellerID != userIDStr) {
			return nil, errors.ForbiddenErr("local user is not a party to this negotiation")
		}
		if n.Status != model.PeerNegotiationOngoing && n.Status != model.PeerNegotiationAccepted {
			return nil, errors.ConflictErr("negotiation is not ongoing")
		}
		if n.Status == model.PeerNegotiationOngoing && n.LastModifiedByRouting == ourRouting && n.LastModifiedByID == userIDStr {
			return nil, errors.ConflictErr("you cannot accept your own latest offer")
		}

		var buyerAccount, sellerAccount *string
		if strings.TrimSpace(req.AccountNumber) != "" {
			account := strings.TrimSpace(req.AccountNumber)
			if n.BuyerRoutingNumber == ourRouting && n.BuyerID == userIDStr {
				buyerAccount = &account
			}
			if n.SellerRoutingNumber == ourRouting && n.SellerID == userIDStr {
				sellerAccount = &account
			}
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

		contract, err := s.ensureContractForNegotiation(ctx, n, buyerAccount, sellerAccount, true)
		if err != nil {
			return nil, err
		}
		return toPeerContractDTO(contract), nil
	}

	mirror, err := s.findLocalMirrorByRemote(ctx, negotiationID, localUserID)
	if err != nil {
		return nil, err
	}
	if mirror.Status != model.PeerNegotiationOngoing && mirror.Status != model.PeerNegotiationAccepted {
		return nil, errors.ConflictErr("negotiation is not ongoing")
	}

	remoteContract, err := s.client.Accept(ctx, negotiationID)
	if err != nil {
		return nil, err
	}

	account := strings.TrimSpace(req.AccountNumber)
	var buyerAccount, sellerAccount *string
	if account != "" {
		if mirror.BuyerRoutingNumber == ourRouting && mirror.BuyerID == userIDStr {
			buyerAccount = &account
		}
		if mirror.SellerRoutingNumber == ourRouting && mirror.SellerID == userIDStr {
			sellerAccount = &account
		}
	}

	contract, err := s.ensureMirrorContract(ctx, mirror, remoteContract, buyerAccount, sellerAccount)
	if err != nil {
		return nil, err
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

func (s *PeerOtcService) ReserveSharesFromPeer(ctx context.Context, senderRouting, authorityRouting int, contractID string) error {
	contract, err := s.loadPeerContractForSellerStep(ctx, senderRouting, authorityRouting, contractID)
	if err != nil {
		return err
	}

	sellerID, err := parsePeerPartyID(contract.SellerID, "seller id")
	if err != nil {
		return err
	}

	_, err = s.tradingClient.ReservePeerOtcShares(ctx, &pb.ReservePeerOtcSharesRequest{
		ContractId: contractStorageKey(contract),
		SellerId:   uint64(sellerID),
		Ticker:     contract.Ticker,
		Amount:     float64(contract.Amount),
	})
	if err != nil {
		return errors.InternalErr(err)
	}

	return nil
}

func (s *PeerOtcService) ConsumeSharesFromPeer(ctx context.Context, senderRouting, authorityRouting int, contractID string) error {
	contract, err := s.loadPeerContractForSellerStep(ctx, senderRouting, authorityRouting, contractID)
	if err != nil {
		return err
	}

	if _, err := s.tradingClient.ConsumePeerOtcShares(ctx, contractStorageKey(contract)); err != nil {
		return errors.InternalErr(err)
	}

	return nil
}

func (s *PeerOtcService) ReleaseSharesFromPeer(ctx context.Context, senderRouting, authorityRouting int, contractID string) error {
	contract, err := s.loadPeerContractForSellerStep(ctx, senderRouting, authorityRouting, contractID)
	if err != nil {
		return err
	}

	if _, err := s.tradingClient.ReleasePeerOtcShares(ctx, contractStorageKey(contract)); err != nil {
		return errors.InternalErr(err)
	}

	return nil
}

func (s *PeerOtcService) ExerciseAsLocal(ctx context.Context, localUserID uint, contractID dto.ForeignBankId) (*dto.PeerContractExercise, error) {
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

	exercise, err := s.exercises.FindByContract(ctx, contract.AuthorityRoutingNumber, contract.ID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if exercise == nil {
		exercise = &model.PeerContractExercise{
			ContractAuthorityRoutingNumber: contract.AuthorityRoutingNumber,
			ContractID:                     contract.ID,
			ExecutionKey:                   fmt.Sprintf("peer-otc-%d-%s-%s", contract.AuthorityRoutingNumber, contract.ID, uuid.NewString()),
			CurrentStep:                    model.PeerExerciseStepInit,
			Status:                         model.PeerExerciseInProgress,
		}
		if err := s.exercises.Create(ctx, exercise); err != nil {
			return nil, errors.InternalErr(err)
		}
	}

	if exercise.Status == model.PeerExerciseCompleted {
		return toPeerExerciseDTO(exercise), nil
	}
	if exercise.Status == model.PeerExerciseFailed {
		return nil, errors.ConflictErr("peer OTC exercise has already failed")
	}

	if err := s.processPeerExercise(ctx, contract, exercise); err != nil {
		latest, latestErr := s.exercises.FindByID(ctx, exercise.ID)
		if latestErr == nil && latest != nil {
			return toPeerExerciseDTO(latest), err
		}
		return nil, err
	}

	latest, err := s.exercises.FindByID(ctx, exercise.ID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	return toPeerExerciseDTO(latest), nil
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

func (s *PeerOtcService) ensureContractForNegotiation(
	ctx context.Context,
	n *model.PeerNegotiation,
	buyerAccountNumber *string,
	sellerAccountNumber *string,
	isAuthoritative bool,
) (*model.PeerContract, error) {
	authorityRouting := n.SellerRoutingNumber
	contractID := n.ID
	if !n.IsAuthoritative && n.RemoteNegotiationID != nil {
		contractID = *n.RemoteNegotiationID
	}

	existing, err := s.contracts.FindByID(ctx, authorityRouting, contractID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if existing != nil {
		updated := false
		if buyerAccountNumber != nil && existing.BuyerAccountNumber == nil {
			existing.BuyerAccountNumber = buyerAccountNumber
			updated = true
		}
		if sellerAccountNumber != nil && existing.SellerAccountNumber == nil {
			existing.SellerAccountNumber = sellerAccountNumber
			updated = true
		}
		if updated {
			if err := s.contracts.Update(ctx, existing); err != nil {
				return nil, errors.InternalErr(err)
			}
		}
		return existing, nil
	}

	contract := &model.PeerContract{
		AuthorityRoutingNumber: authorityRouting,
		ID:                     contractID,
		NegotiationID:          contractID,
		BuyerRoutingNumber:     n.BuyerRoutingNumber,
		BuyerID:                n.BuyerID,
		SellerRoutingNumber:    n.SellerRoutingNumber,
		SellerID:               n.SellerID,
		Ticker:                 n.Ticker,
		Amount:                 n.Amount,
		StrikePrice:            n.PricePerStock,
		StrikeCurrency:         n.PriceCurrency,
		Premium:                n.Premium,
		PremiumCurrency:        n.PremiumCurrency,
		SettlementDate:         n.SettlementDate,
		BuyerAccountNumber:     buyerAccountNumber,
		SellerAccountNumber:    sellerAccountNumber,
		Status:                 model.PeerContractActive,
		IsAuthoritative:        isAuthoritative,
	}

	if err := s.contracts.Create(ctx, contract); err != nil {
		return nil, errors.InternalErr(err)
	}

	if n.Status == model.PeerNegotiationOngoing {
		n.Status = model.PeerNegotiationAccepted
		if err := s.negotiations.Update(ctx, n); err != nil {
			return nil, errors.InternalErr(err)
		}
	}

	return contract, nil
}

func (s *PeerOtcService) ensureMirrorContract(
	ctx context.Context,
	mirror *model.PeerNegotiation,
	remote *dto.PeerContract,
	buyerAccountNumber *string,
	sellerAccountNumber *string,
) (*model.PeerContract, error) {
	existing, err := s.contracts.FindByID(ctx, remote.ID.RoutingNumber, remote.ID.ID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if existing != nil {
		updated := false
		if buyerAccountNumber != nil && existing.BuyerAccountNumber == nil {
			existing.BuyerAccountNumber = buyerAccountNumber
			updated = true
		}
		if sellerAccountNumber != nil && existing.SellerAccountNumber == nil {
			existing.SellerAccountNumber = sellerAccountNumber
			updated = true
		}
		if updated {
			if err := s.contracts.Update(ctx, existing); err != nil {
				return nil, errors.InternalErr(err)
			}
		}
		return existing, nil
	}

	contract := &model.PeerContract{
		AuthorityRoutingNumber: remote.ID.RoutingNumber,
		ID:                     remote.ID.ID,
		NegotiationID:          remote.NegotiationID.ID,
		BuyerRoutingNumber:     mirror.BuyerRoutingNumber,
		BuyerID:                mirror.BuyerID,
		SellerRoutingNumber:    mirror.SellerRoutingNumber,
		SellerID:               mirror.SellerID,
		Ticker:                 remote.Ticker,
		Amount:                 remote.Amount,
		StrikePrice:            remote.StrikePrice.Amount,
		StrikeCurrency:         string(remote.StrikePrice.Currency),
		Premium:                remote.Premium.Amount,
		PremiumCurrency:        string(remote.Premium.Currency),
		SettlementDate:         remote.SettlementDate,
		BuyerAccountNumber:     buyerAccountNumber,
		SellerAccountNumber:    sellerAccountNumber,
		Status:                 model.PeerContractActive,
		IsAuthoritative:        false,
	}

	if err := s.contracts.Create(ctx, contract); err != nil {
		return nil, errors.InternalErr(err)
	}

	if mirror.Status == model.PeerNegotiationOngoing {
		mirror.Status = model.PeerNegotiationAccepted
		if err := s.negotiations.Update(ctx, mirror); err != nil {
			return nil, errors.InternalErr(err)
		}
	}

	return contract, nil
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
		Postings: []dto.Posting{
			{
				Account: personAccount(n.BuyerRoutingNumber, n.BuyerID),
				Amount:  -n.Premium,
				Asset:   monasAsset,
			},
			{
				Account: personAccount(n.SellerRoutingNumber, n.SellerID),
				Amount:  n.Premium,
				Asset:   monasAsset,
			},
			{
				Account: personAccount(n.BuyerRoutingNumber, n.BuyerID),
				Amount:  1,
				Asset:   optionAsset,
			},
			{
				Account: personAccount(n.SellerRoutingNumber, n.SellerID),
				Amount:  -1,
				Asset:   optionAsset,
			},
		},
	}
}

func (s *PeerOtcService) exerciseCashTransaction(contract *model.PeerContract, executionKey string) dto.Transaction {
	amount := float64(contract.Amount) * contract.StrikePrice
	return dto.Transaction{
		TransactionID: dto.ForeignBankId{
			RoutingNumber: s.peers.OurRoutingNumber(),
			ID:            executionKey,
		},
		Message:        "Peer OTC option exercise cash settlement",
		PaymentCode:    "289",
		PaymentPurpose: "OTC option exercise",
		Postings: []dto.Posting{
			{
				Account: personAccount(contract.BuyerRoutingNumber, contract.BuyerID),
				Amount:  -amount,
				Asset:   dto.Asset{Type: dto.AssetMonas, Body: map[string]any{"currency": contract.StrikeCurrency}},
			},
			{
				Account: personAccount(contract.SellerRoutingNumber, contract.SellerID),
				Amount:  amount,
				Asset:   dto.Asset{Type: dto.AssetMonas, Body: map[string]any{"currency": contract.StrikeCurrency}},
			},
		},
	}
}

func (s *PeerOtcService) coordinateTwoBankTransaction(ctx context.Context, peerRouting int, tx dto.Transaction, keyPrefix string) error {
	localVote, err := s.processor.PrepareLocalTransaction(ctx, &tx)
	if err != nil {
		return errors.InternalErr(err)
	}
	if localVote.Vote != dto.VoteYes {
		return errors.ConflictErr(fmt.Sprintf("local bank voted NO: %s", voteReasons(localVote)))
	}

	remotePrepared := false
	if peerRouting != s.peers.OurRoutingNumber() {
		remoteVote, err := s.client.SendNewTx(ctx, peerRouting, keyPrefix+"-new", tx)
		if err != nil {
			_, _ = s.processor.RollbackLocalTransaction(ctx, tx.TransactionID)
			return err
		}
		if remoteVote == nil || remoteVote.Vote != dto.VoteYes {
			_, _ = s.processor.RollbackLocalTransaction(ctx, tx.TransactionID)
			return errors.ConflictErr(fmt.Sprintf("peer bank voted NO: %s", voteReasonsValue(remoteVote)))
		}
		remotePrepared = true
	}

	if _, err := s.processor.CommitLocalTransaction(ctx, tx.TransactionID); err != nil {
		if remotePrepared {
			_ = s.client.SendRollbackTx(ctx, peerRouting, keyPrefix+"-rollback", tx.TransactionID)
		}
		return errors.InternalErr(err)
	}
	if remotePrepared {
		if err := s.client.SendCommitTx(ctx, peerRouting, keyPrefix+"-commit", tx.TransactionID); err != nil {
			return errors.ServiceUnavailableErr(&remoteCommitPendingError{err: err})
		}
	}

	return nil
}

func (s *PeerOtcService) prepareTwoBankTransaction(ctx context.Context, peerRouting int, tx dto.Transaction, keyPrefix string) error {
	localVote, err := s.processor.PrepareLocalTransaction(ctx, &tx)
	if err != nil {
		return errors.InternalErr(err)
	}
	if localVote.Vote != dto.VoteYes {
		return errors.ConflictErr(fmt.Sprintf("local bank voted NO: %s", voteReasons(localVote)))
	}
	if peerRouting == s.peers.OurRoutingNumber() {
		return nil
	}
	remoteVote, err := s.client.SendNewTx(ctx, peerRouting, keyPrefix+"-new", tx)
	if err != nil {
		_, _ = s.processor.RollbackLocalTransaction(ctx, tx.TransactionID)
		return err
	}
	if remoteVote == nil || remoteVote.Vote != dto.VoteYes {
		_, _ = s.processor.RollbackLocalTransaction(ctx, tx.TransactionID)
		return errors.ConflictErr(fmt.Sprintf("peer bank voted NO: %s", voteReasonsValue(remoteVote)))
	}
	return nil
}

func (s *PeerOtcService) commitTwoBankTransaction(ctx context.Context, peerRouting int, txID dto.ForeignBankId, keyPrefix string) error {
	if _, err := s.processor.CommitLocalTransaction(ctx, txID); err != nil {
		if peerRouting != s.peers.OurRoutingNumber() {
			_ = s.client.SendRollbackTx(ctx, peerRouting, keyPrefix+"-rollback", txID)
		}
		return errors.InternalErr(err)
	}
	if peerRouting != s.peers.OurRoutingNumber() {
		if err := s.client.SendCommitTx(ctx, peerRouting, keyPrefix+"-commit", txID); err != nil {
			return errors.ServiceUnavailableErr(&remoteCommitPendingError{err: err})
		}
	}
	return nil
}

func (s *PeerOtcService) rollbackTwoBankTransaction(ctx context.Context, peerRouting int, txID dto.ForeignBankId, keyPrefix string) {
	_, _ = s.processor.RollbackLocalTransaction(ctx, txID)
	if peerRouting != s.peers.OurRoutingNumber() {
		_ = s.client.SendRollbackTx(ctx, peerRouting, keyPrefix+"-rollback", txID)
	}
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

func isRemoteCommitPending(err error) bool {
	var pending *remoteCommitPendingError
	return stderrors.As(err, &pending)
}

func (s *PeerOtcService) processPeerExercise(ctx context.Context, contract *model.PeerContract, exercise *model.PeerContractExercise) error {
	for i := 0; i < 6; i++ {
		switch exercise.Status {
		case model.PeerExerciseCompleted, model.PeerExerciseFailed:
			return nil
		case model.PeerExerciseCompensating:
			return s.compensatePeerExercise(ctx, contract, exercise)
		}

		var err error
		switch exercise.CurrentStep {
		case model.PeerExerciseStepInit:
			err = s.reservePeerExerciseFunds(ctx, contract, exercise)
		case model.PeerExerciseStepFundsReserved:
			err = s.reservePeerExerciseShares(ctx, contract, exercise)
		case model.PeerExerciseStepSharesConfirmed:
			err = s.commitPeerExerciseFunds(ctx, contract, exercise)
		case model.PeerExerciseStepFundsCommitted:
			err = s.transferPeerExerciseOwnership(ctx, contract, exercise)
		case model.PeerExerciseStepOwnershipTransferred:
			err = s.completePeerExercise(ctx, contract, exercise)
		default:
			err = errors.BadRequestErr("unknown peer OTC exercise step")
		}
		if err != nil {
			return err
		}

		latest, err := s.exercises.FindByID(ctx, exercise.ID)
		if err != nil {
			return errors.InternalErr(err)
		}
		if latest == nil {
			return errors.NotFoundErr("peer OTC exercise not found")
		}
		exercise = latest
	}

	return nil
}

func (s *PeerOtcService) reservePeerExerciseFunds(ctx context.Context, contract *model.PeerContract, exercise *model.PeerContractExercise) error {
	tx := s.exerciseCashTransaction(contract, exercise.ExecutionKey)
	if err := s.prepareTwoBankTransaction(ctx, contract.SellerRoutingNumber, tx, exercise.ExecutionKey); err != nil {
		return s.failPeerExercise(ctx, exercise, err.Error())
	}

	return s.advancePeerExercise(ctx, exercise, model.PeerExerciseStepFundsReserved, model.PeerExerciseInProgress, "")
}

func (s *PeerOtcService) reservePeerExerciseShares(ctx context.Context, contract *model.PeerContract, exercise *model.PeerContractExercise) error {
	if contract.SellerRoutingNumber == s.peers.OurRoutingNumber() {
		sellerID, err := parsePeerPartyID(contract.SellerID, "seller id")
		if err != nil {
			return s.compensateAfterFundsReserved(ctx, exercise, err.Error())
		}
		_, err = s.tradingClient.ReservePeerOtcShares(ctx, &pb.ReservePeerOtcSharesRequest{
			ContractId: contractStorageKey(contract),
			SellerId:   uint64(sellerID),
			Ticker:     contract.Ticker,
			Amount:     float64(contract.Amount),
		})
		if err != nil {
			return s.compensateAfterFundsReserved(ctx, exercise, err.Error())
		}
	} else if err := s.client.ReserveShares(ctx, dto.ForeignBankId{RoutingNumber: contract.AuthorityRoutingNumber, ID: contract.ID}); err != nil {
		return s.compensateAfterFundsReserved(ctx, exercise, err.Error())
	}

	return s.advancePeerExercise(ctx, exercise, model.PeerExerciseStepSharesConfirmed, model.PeerExerciseInProgress, "")
}

func (s *PeerOtcService) commitPeerExerciseFunds(ctx context.Context, contract *model.PeerContract, exercise *model.PeerContractExercise) error {
	tx := s.exerciseCashTransaction(contract, exercise.ExecutionKey)
	if err := s.commitTwoBankTransaction(ctx, contract.SellerRoutingNumber, tx.TransactionID, exercise.ExecutionKey); err != nil {
		if isRemoteCommitPending(err) {
			exercise.LastError = err.Error()
			exercise.UpdatedAt = time.Now()
			if updateErr := s.exercises.Update(ctx, exercise); updateErr != nil {
				return errors.InternalErr(updateErr)
			}
			return errors.ServiceUnavailableErr(err)
		}
		return s.compensateAfterFundsReserved(ctx, exercise, err.Error())
	}
	return s.advancePeerExercise(ctx, exercise, model.PeerExerciseStepFundsCommitted, model.PeerExerciseInProgress, "")
}

func (s *PeerOtcService) transferPeerExerciseOwnership(ctx context.Context, contract *model.PeerContract, exercise *model.PeerContractExercise) error {
	if contract.SellerRoutingNumber == s.peers.OurRoutingNumber() {
		if _, err := s.tradingClient.ConsumePeerOtcShares(ctx, contractStorageKey(contract)); err != nil {
			return s.compensateAfterFundsCommitted(ctx, exercise, err.Error())
		}
	} else if err := s.client.ConsumeShares(ctx, dto.ForeignBankId{RoutingNumber: contract.AuthorityRoutingNumber, ID: contract.ID}); err != nil {
		return s.compensateAfterFundsCommitted(ctx, exercise, err.Error())
	}

	buyerID, err := parsePeerPartyID(contract.BuyerID, "buyer id")
	if err != nil {
		return s.compensateAfterFundsCommitted(ctx, exercise, err.Error())
	}
	if _, err := s.tradingClient.CreditPeerOtcShares(ctx, &pb.CreditPeerOtcSharesRequest{
		ContractId:      contractStorageKey(contract),
		BuyerId:         uint64(buyerID),
		Ticker:          contract.Ticker,
		Amount:          float64(contract.Amount),
		PricePerUnitRsd: contract.StrikePrice,
	}); err != nil {
		return s.compensateAfterFundsCommitted(ctx, exercise, err.Error())
	}

	return s.advancePeerExercise(ctx, exercise, model.PeerExerciseStepOwnershipTransferred, model.PeerExerciseInProgress, "")
}

func (s *PeerOtcService) completePeerExercise(ctx context.Context, contract *model.PeerContract, exercise *model.PeerContractExercise) error {
	now := time.Now()
	contract.Status = model.PeerContractExercised
	contract.ExercisedAt = &now
	if err := s.contracts.Update(ctx, contract); err != nil {
		return errors.InternalErr(err)
	}

	exercise.CurrentStep = model.PeerExerciseStepCompleted
	exercise.Status = model.PeerExerciseCompleted
	exercise.LastError = ""
	exercise.CompletedAt = &now
	exercise.UpdatedAt = now
	if err := s.exercises.Update(ctx, exercise); err != nil {
		return errors.InternalErr(err)
	}
	return nil
}

func (s *PeerOtcService) compensatePeerExercise(ctx context.Context, contract *model.PeerContract, exercise *model.PeerContractExercise) error {
	switch exercise.CurrentStep {
	case model.PeerExerciseStepFundsReserved:
		tx := s.exerciseCashTransaction(contract, exercise.ExecutionKey)
		s.rollbackTwoBankTransaction(ctx, contract.SellerRoutingNumber, tx.TransactionID, exercise.ExecutionKey)
	case model.PeerExerciseStepFundsCommitted:
		refundTx := s.exerciseCashTransaction(contract, exercise.ExecutionKey+"-refund")
		for i := range refundTx.Postings {
			refundTx.Postings[i].Amount = -refundTx.Postings[i].Amount
		}
		_ = s.coordinateTwoBankTransaction(ctx, contract.SellerRoutingNumber, refundTx, exercise.ExecutionKey+"-refund")
		if contract.SellerRoutingNumber == s.peers.OurRoutingNumber() {
			_, _ = s.tradingClient.ReleasePeerOtcShares(ctx, contractStorageKey(contract))
		} else {
			_ = s.client.ReleaseShares(ctx, dto.ForeignBankId{RoutingNumber: contract.AuthorityRoutingNumber, ID: contract.ID})
		}
	}

	exercise.Status = model.PeerExerciseFailed
	exercise.UpdatedAt = time.Now()
	if err := s.exercises.Update(ctx, exercise); err != nil {
		return errors.InternalErr(err)
	}
	return nil
}

func (s *PeerOtcService) compensateAfterFundsReserved(ctx context.Context, exercise *model.PeerContractExercise, reason string) error {
	exercise.CurrentStep = model.PeerExerciseStepFundsReserved
	exercise.Status = model.PeerExerciseCompensating
	exercise.LastError = reason
	exercise.UpdatedAt = time.Now()
	if err := s.exercises.Update(ctx, exercise); err != nil {
		return errors.InternalErr(err)
	}
	return errors.BadRequestErr(reason)
}

func (s *PeerOtcService) compensateAfterFundsCommitted(ctx context.Context, exercise *model.PeerContractExercise, reason string) error {
	exercise.CurrentStep = model.PeerExerciseStepFundsCommitted
	exercise.Status = model.PeerExerciseCompensating
	exercise.LastError = reason
	exercise.UpdatedAt = time.Now()
	if err := s.exercises.Update(ctx, exercise); err != nil {
		return errors.InternalErr(err)
	}
	return errors.BadRequestErr(reason)
}

func (s *PeerOtcService) failPeerExercise(ctx context.Context, exercise *model.PeerContractExercise, reason string) error {
	exercise.Status = model.PeerExerciseFailed
	exercise.LastError = reason
	exercise.UpdatedAt = time.Now()
	if err := s.exercises.Update(ctx, exercise); err != nil {
		return errors.InternalErr(err)
	}
	return errors.BadRequestErr(reason)
}

func (s *PeerOtcService) advancePeerExercise(ctx context.Context, exercise *model.PeerContractExercise, step model.PeerExerciseStep, statusValue model.PeerExerciseStatus, lastError string) error {
	exercise.CurrentStep = step
	exercise.Status = statusValue
	exercise.LastError = lastError
	exercise.UpdatedAt = time.Now()
	if err := s.exercises.Update(ctx, exercise); err != nil {
		return errors.InternalErr(err)
	}
	return nil
}

func (s *PeerOtcService) loadPeerContractForSellerStep(ctx context.Context, senderRouting, authorityRouting int, contractID string) (*model.PeerContract, error) {
	if authorityRouting != s.peers.OurRoutingNumber() {
		return nil, errors.BadRequestErr("contract authority routing number does not match this bank")
	}

	contract, err := s.contracts.FindByID(ctx, authorityRouting, contractID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if contract == nil {
		return nil, errors.NotFoundErr("contract not found")
	}
	if contract.BuyerRoutingNumber != senderRouting {
		return nil, errors.ForbiddenErr("only the buyer bank can perform this exercise step")
	}
	if contract.SellerRoutingNumber != s.peers.OurRoutingNumber() {
		return nil, errors.BadRequestErr("seller is not local to this bank")
	}
	if contract.Status != model.PeerContractActive {
		return nil, errors.ConflictErr("contract is not active")
	}

	return contract, nil
}

func parsePeerPartyID(id, field string) (uint, error) {
	value, err := strconv.ParseUint(id, 10, 64)
	if err != nil || value == 0 {
		return 0, errors.BadRequestErr(field + " must be a positive integer")
	}
	return uint(value), nil
}

func contractStorageKey(contract *model.PeerContract) string {
	return fmt.Sprintf("%d:%s", contract.AuthorityRoutingNumber, contract.ID)
}
