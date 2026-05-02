package service

import (
	"context"
	"fmt"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
)

// OtcOfferService implements the business logic for OTC negotiations and option
// contract creation as specified in Feature 4 (OTC trading).
//
// Key invariants enforced by this service:
//
//  1. An active offer and an option contract are TWO DISTINCT entities. Negotiation
//     happens on OtcOffer; OtcOptionContract is only created upon acceptance.
//
//  2. A counter-offer UPDATES the existing OtcOffer (Amount, Price, Premium,
//     SettlementDate, ModifiedBy, LastModified). The parties never change.
//
//  3. Counter-offers alternate between parties. The same user cannot send two
//     consecutive counter-offers — the other side must respond first.
//
//  4. Only the party OPPOSITE to ModifiedBy may accept the current offer.
//
//  5. On acceptance: an OtcOptionContract is created and the premium is transferred
//     from the buyer's account to the seller's. (TODO: replace with SAGA.)
//
//  6. Seller capacity is validated per spec 3+7+2: PublicAmount must cover the
//     sum of all active negotiations and valid option contracts for the same stock.
type OtcOfferService struct {
	offerRepo          repository.OtcOfferRepository
	optionContractRepo repository.OtcOptionContractRepository
	assetOwnershipRepo repository.AssetOwnershipRepository
	stockRepo          repository.StockRepository
	bankingClient      client.BankingClient
	now                func() time.Time
}

func NewOtcOfferService(
	offerRepo repository.OtcOfferRepository,
	optionContractRepo repository.OtcOptionContractRepository,
	assetOwnershipRepo repository.AssetOwnershipRepository,
	stockRepo repository.StockRepository,
	bankingClient client.BankingClient,
) *OtcOfferService {
	return &OtcOfferService{
		offerRepo:          offerRepo,
		optionContractRepo: optionContractRepo,
		assetOwnershipRepo: assetOwnershipRepo,
		stockRepo:          stockRepo,
		bankingClient:      bankingClient,
		now:                time.Now,
	}
}

// CreateOffer initiates a new OTC negotiation on behalf of the buyer.
func (s *OtcOfferService) CreateOffer(ctx context.Context, req dto.CreateOtcOfferRequest) (*model.OtcOffer, error) {
	buyerID, err := auth.GetSubjectFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if buyerID == req.SellerID {
		return nil, errors.BadRequestErr("cannot send an offer to yourself")
	}
	if req.SettlementDate.Before(s.now()) {
		return nil, errors.BadRequestErr("settlement date must be in the future")
	}

	if err := s.validateSellerCapacity(ctx, req.SellerID, req.StockID, req.Amount, nil); err != nil {
		return nil, err
	}

	if _, err := s.bankingClient.GetAccountByNumber(ctx, req.BuyerAccountNumber); err != nil {
		return nil, errors.BadRequestErr("buyer account number is invalid")
	}

	now := s.now()
	offer := &model.OtcOffer{
		BuyerID:            buyerID,
		SellerID:           req.SellerID,
		StockID:            req.StockID,
		Amount:             req.Amount,
		PricePerStock:      req.PricePerStock,
		Premium:            req.Premium,
		SettlementDate:     req.SettlementDate,
		BuyerAccountNumber: req.BuyerAccountNumber,
		Status:             model.OtcOfferStatusActive,
		LastModified:       now,
		ModifiedBy:         buyerID, // buyer initiated → seller's turn next
	}

	if err := s.offerRepo.Create(ctx, offer); err != nil {
		return nil, errors.InternalErr(err)
	}

	created, err := s.offerRepo.FindByID(ctx, offer.OtcOfferID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	return created, nil
}

// SendCounterOffer allows either party to update the negotiation parameters.
//
// The negotiation is a back-and-forth until one side rejects or accepts. The
// parties never change; no new offer is created — only the fields and ModifiedBy
// are updated.
func (s *OtcOfferService) SendCounterOffer(ctx context.Context, offerID uint, req dto.CounterOfferRequest) (*model.OtcOffer, error) {
	callerID, err := auth.GetSubjectFromContext(ctx)
	if err != nil {
		return nil, err
	}

	offer, err := s.offerRepo.FindByID(ctx, offerID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if offer == nil {
		return nil, errors.NotFoundErr("offer not found")
	}

	if err := s.validateParticipantAndState(offer, callerID); err != nil {
		return nil, err
	}

	if offer.ModifiedBy == callerID {
		return nil, errors.BadRequestErr("it is the other party's turn — you cannot send two consecutive counter-offers")
	}

	if req.SettlementDate.Before(s.now()) {
		return nil, errors.BadRequestErr("settlement date must be in the future")
	}

	if callerID == offer.SellerID {
		if err := s.validateSellerCapacity(ctx, offer.SellerID, offer.StockID, req.Amount, &offer.OtcOfferID); err != nil {
			return nil, err
		}
	}

	if callerID == offer.SellerID && offer.SellerAccountNumber == nil {
		if req.AccountNumber == nil {
			return nil, errors.BadRequestErr("seller_account_number is required in the seller's first counter-offer")
		}
		if _, err := s.bankingClient.GetAccountByNumber(ctx, *req.AccountNumber); err != nil {
			return nil, errors.BadRequestErr("seller account number is invalid")
		}
		offer.SellerAccountNumber = req.AccountNumber
	}

	offer.Amount = req.Amount
	offer.PricePerStock = req.PricePerStock
	offer.Premium = req.Premium
	offer.SettlementDate = req.SettlementDate
	offer.LastModified = s.now()
	offer.ModifiedBy = callerID

	if err := s.offerRepo.Save(ctx, offer); err != nil {
		return nil, errors.InternalErr(err)
	}

	updated, err := s.offerRepo.FindByID(ctx, offer.OtcOfferID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	return updated, nil
}

// AcceptOffer is called by the party OPPOSITE to ModifiedBy to accept the offer.
//
// On acceptance:
//  1. final seller capacity validation,
//  2. premium transfer from buyer's account to seller's,
//  3. OtcOptionContract is created,
//  4. seller's reserved_amount is increased by the contracted quantity,
//  5. offer status transitions to ACCEPTED with a link to the contract.
func (s *OtcOfferService) AcceptOffer(ctx context.Context, offerID uint, req dto.AcceptOfferRequest) (*model.OtcOptionContract, error) {
	callerID, err := auth.GetSubjectFromContext(ctx)
	if err != nil {
		return nil, err
	}

	offer, err := s.offerRepo.FindByID(ctx, offerID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if offer == nil {
		return nil, errors.NotFoundErr("offer not found")
	}

	if err := s.validateParticipantAndState(offer, callerID); err != nil {
		return nil, err
	}

	if offer.ModifiedBy == callerID {
		return nil, errors.BadRequestErr("you cannot accept your own offer — the other party must accept or send a counter-offer")
	}

	if callerID == offer.SellerID && offer.SellerAccountNumber == nil {
		if req.AccountNumber == nil {
			return nil, errors.BadRequestErr("seller_account_number is required when accepting")
		}
		if _, err := s.bankingClient.GetAccountByNumber(ctx, *req.AccountNumber); err != nil {
			return nil, errors.BadRequestErr("seller account number is invalid")
		}
		offer.SellerAccountNumber = req.AccountNumber
	}
	if offer.SellerAccountNumber == nil {
		return nil, errors.BadRequestErr("seller account number is missing — the seller must send a counter-offer or accept first")
	}

	if err := s.validateSellerCapacity(ctx, offer.SellerID, offer.StockID, offer.Amount, &offer.OtcOfferID); err != nil {
		return nil, err
	}

	// 1) Transfer premium: buyer → seller.
	// TODO(SAGA): Replace with proper SAGA orchestration when introduced.
	if _, err := s.bankingClient.CreatePaymentWithoutVerification(ctx, &pb.CreatePaymentRequest{
		PayerAccountNumber:     offer.BuyerAccountNumber,
		RecipientAccountNumber: *offer.SellerAccountNumber,
		Amount:                 offer.Premium,
		PaymentCode:            "289",
		Purpose:                fmt.Sprintf("OTC premium for offer #%d", offer.OtcOfferID),
	}); err != nil {
		return nil, errors.InternalErr(fmt.Errorf("premium transfer failed: %w", err))
	}

	// 2) Create the option contract.
	now := s.now()
	contract := &model.OtcOptionContract{
		OtcOfferID:     offer.OtcOfferID,
		BuyerID:        offer.BuyerID,
		SellerID:       offer.SellerID,
		StockID:        offer.StockID,
		Amount:         offer.Amount,
		StrikePrice:    offer.PricePerStock,
		Premium:        offer.Premium,
		SettlementDate: offer.SettlementDate,
	}
	if err := s.optionContractRepo.Create(ctx, contract); err != nil {
		return nil, errors.InternalErr(fmt.Errorf("option contract creation failed (premium already transferred): %w", err))
	}

	// 3) Increase the seller's reserved_amount by the contracted quantity.
	stocks, _ := s.stockRepo.FindByAssetIDs(ctx, []uint{offer.StockID})
	for i := range stocks {
		if stocks[i].StockID == offer.StockID || stocks[i].AssetID == offer.StockID {
			_ = s.assetOwnershipRepo.IncreaseReservedAmount(
				ctx, offer.SellerID, model.OwnerTypeClient, stocks[i].AssetID, float64(offer.Amount),
			)
			break
		}
	}

	// 4) Mark offer as accepted and link to the contract.
	offer.Status = model.OtcOfferStatusAccepted
	offer.OptionContractID = &contract.OtcOptionContractID
	offer.LastModified = now
	offer.ModifiedBy = callerID
	if err := s.offerRepo.Save(ctx, offer); err != nil {
		return nil, errors.InternalErr(err)
	}

	created, err := s.optionContractRepo.FindByID(ctx, contract.OtcOptionContractID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	return created, nil
}

// RejectOffer allows either party to withdraw from the negotiation at any time.
func (s *OtcOfferService) RejectOffer(ctx context.Context, offerID uint, req dto.RejectOfferRequest) (*model.OtcOffer, error) {
	callerID, err := auth.GetSubjectFromContext(ctx)
	if err != nil {
		return nil, err
	}

	offer, err := s.offerRepo.FindByID(ctx, offerID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if offer == nil {
		return nil, errors.NotFoundErr("offer not found")
	}

	if err := s.validateParticipantAndState(offer, callerID); err != nil {
		return nil, err
	}

	offer.Status = model.OtcOfferStatusRejected
	offer.LastModified = s.now()
	offer.ModifiedBy = callerID
	_ = req

	if err := s.offerRepo.Save(ctx, offer); err != nil {
		return nil, errors.InternalErr(err)
	}

	updated, err := s.offerRepo.FindByID(ctx, offer.OtcOfferID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	return updated, nil
}

// GetActiveOffersForUser returns all active negotiations in which the given user participates.
func (s *OtcOfferService) GetActiveOffersForUser(ctx context.Context, userID uint) ([]model.OtcOffer, error) {
	offers, err := s.offerRepo.FindActiveForUser(ctx, userID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	return offers, nil
}

// GetOptionContractsForUser returns all option contracts in which the given user participates.
func (s *OtcOfferService) GetOptionContractsForUser(ctx context.Context, userID uint) ([]model.OtcOptionContract, error) {
	contracts, err := s.optionContractRepo.FindForUser(ctx, userID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	return contracts, nil
}

// --- helpers ---

func (s *OtcOfferService) validateParticipantAndState(offer *model.OtcOffer, callerID uint) error {
	if callerID != offer.BuyerID && callerID != offer.SellerID {
		return errors.ForbiddenErr("you are not a participant in this negotiation")
	}
	if offer.Status != model.OtcOfferStatusActive {
		return errors.BadRequestErr("offer is not active")
	}
	return nil
}

// validateSellerCapacity enforces spec 3+7+2:
// PublicAmount >= sum(active negotiation amounts) + sum(valid option contract amounts) + requestedAmount
//
// If excludeOfferID is non-nil, that offer is excluded from the running total
// (used when updating an existing offer to avoid double-counting its current amount).
func (s *OtcOfferService) validateSellerCapacity(
	ctx context.Context,
	sellerID, stockID uint,
	requestedAmount int,
	excludeOfferID *uint,
) error {
	stocks, err := s.stockRepo.FindByAssetIDs(ctx, []uint{stockID})
	if err != nil {
		return errors.InternalErr(err)
	}
	var stock *model.Stock
	for i := range stocks {
		if stocks[i].StockID == stockID || stocks[i].AssetID == stockID {
			stock = &stocks[i]
			break
		}
	}
	if stock == nil {
		return errors.BadRequestErr("stock not found")
	}

	ownerships, err := s.assetOwnershipRepo.FindByUserId(ctx, sellerID, model.OwnerTypeClient)
	if err != nil {
		return errors.InternalErr(err)
	}

	publicAmount := 0.0
	for _, o := range ownerships {
		if o.AssetID == stock.AssetID {
			publicAmount = o.PublicAmount
			break
		}
	}

	activeOffers, err := s.offerRepo.FindActiveBySellerAndStock(ctx, sellerID, stockID, excludeOfferID)
	if err != nil {
		return errors.InternalErr(err)
	}
	reserved := 0
	for _, o := range activeOffers {
		reserved += o.Amount
	}

	activeContracts, err := s.optionContractRepo.FindActiveBySellerAndStock(ctx, sellerID, stockID, s.now())
	if err != nil {
		return errors.InternalErr(err)
	}
	for _, c := range activeContracts {
		reserved += c.Amount
	}

	totalNeeded := float64(reserved + requestedAmount)
	if publicAmount < totalNeeded {
		return errors.BadRequestErr(fmt.Sprintf(
			"seller does not have enough public shares: public=%.0f, already committed=%d, additionally requested=%d",
			publicAmount, reserved, requestedAmount,
		))
	}
	return nil
}
