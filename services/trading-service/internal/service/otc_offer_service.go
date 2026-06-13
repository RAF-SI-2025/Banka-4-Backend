package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
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
//  2. A counter-offer UPDATES the existing OtcOffer (Amount, PricePerStockRSD, PremiumRSD,
//     SettlementDate, ModifiedBy, LastModified). The parties never change.
//
//  3. Counter-offers alternate between parties. The same user cannot send two
//     consecutive counter-offers — the other side must respond first.
//
//  4. Only the party OPPOSITE to ModifiedBy may accept the current offer.
//
//  5. On acceptance: processing is delegated to the OTC deal processing service,
//     which transfers the premium, activates the option contract, and creates
//     the seller reservation infrastructure.
//
//  6. Seller capacity is validated per spec 3+7+2: PublicAmount must cover the
//     sum of all active negotiations and valid option contracts for the same stock.
type OtcOfferService struct {
	offerRepo                    repository.OtcOfferRepository
	optionContractRepo           repository.OtcOptionContractRepository
	assetOwnershipRepo           repository.AssetOwnershipRepository
	stockRepo                    repository.StockRepository
	bankingClient                client.BankingClient
	userClient                   client.UserServiceClient
	mailer                       Mailer
	processingService            *OtcDealProcessingService
	otcNegotiationHistoryService OtcNegotiationHistoryService
	now                          func() time.Time
}

func NewOtcOfferService(
	offerRepo repository.OtcOfferRepository,
	optionContractRepo repository.OtcOptionContractRepository,
	assetOwnershipRepo repository.AssetOwnershipRepository,
	stockRepo repository.StockRepository,
	bankingClient client.BankingClient,
	userClient client.UserServiceClient,
	mailer Mailer,
	processingService *OtcDealProcessingService,
	otcNegotiationHistoryService OtcNegotiationHistoryService,
) *OtcOfferService {
	return &OtcOfferService{
		offerRepo:                    offerRepo,
		optionContractRepo:           optionContractRepo,
		assetOwnershipRepo:           assetOwnershipRepo,
		stockRepo:                    stockRepo,
		bankingClient:                bankingClient,
		userClient:                   userClient,
		mailer:                       mailer,
		processingService:            processingService,
		otcNegotiationHistoryService: otcNegotiationHistoryService,
		now:                          time.Now,
	}
}

// CreateOffer initiates a new OTC negotiation on behalf of the buyer.
func (s *OtcOfferService) CreateOffer(ctx context.Context, req dto.CreateOtcOfferRequest) (*model.OtcOffer, error) {
	buyerID, err := auth.GetSubjectFromContext(ctx)
	if err != nil {
		return nil, err
	}

	assetOwnership, err := s.assetOwnershipRepo.FindByID(ctx, req.AssetOwnershipID)
	if assetOwnership == nil {
		return nil, errors.BadRequestErr("invalid asset ownership id")
	}
	if err != nil {
		return nil, err
	}

	if assetOwnership.OwnerType != model.OwnerTypeClient {
		return nil, errors.BadRequestErr("the provided asset does not belong to a client")
	}

	if assetOwnership.Asset.AssetType != model.AssetTypeStock {
		return nil, errors.BadRequestErr("the asset is not a stock")
	}

	if buyerID == assetOwnership.UserId {
		return nil, errors.BadRequestErr("cannot send an offer to yourself")
	}

	if req.SettlementDate.Before(s.now()) {
		return nil, errors.BadRequestErr("settlement date must be in the future")
	}

	if err := s.validateSellerCapacity(ctx, assetOwnership.UserId, assetOwnership.AssetID, req.Amount, nil); err != nil {
		return nil, err
	}

	buyerAccount, err := s.bankingClient.GetAccountByNumber(ctx, req.BuyerAccountNumber)
	if err != nil {
		return nil, errors.BadRequestErr("buyer account number is invalid")
	}
	if uint(buyerAccount.ClientId) != buyerID {
		return nil, errors.BadRequestErr("the provided account does not belong to you")
	}

	now := s.now()
	offer := &model.OtcOffer{
		BuyerID:            buyerID,
		SellerID:           assetOwnership.UserId,
		StockAssetID:       assetOwnership.AssetID,
		Amount:             req.Amount,
		PricePerStockRSD:   req.PricePerStockRSD,
		PremiumRSD:         req.PremiumRSD,
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
	s.sendEmailToUser(
		ctx,
		offer.SellerID,
		"New OTC Offer",
		fmt.Sprintf(
			"You have received a new OTC offer #%d.",
			offer.OtcOfferID,
		),
	)

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
		if err := s.validateSellerCapacity(ctx, offer.SellerID, offer.StockAssetID, req.Amount, &offer.OtcOfferID); err != nil {
			return nil, err
		}
	}

	if callerID == offer.SellerID && offer.SellerAccountNumber == nil {
		if req.AccountNumber == nil {
			return nil, errors.BadRequestErr("account_number is required in the seller's first counter-offer")
		}

		sellerAccount, err := s.bankingClient.GetAccountByNumber(ctx, *req.AccountNumber)
		if err != nil {
			return nil, errors.BadRequestErr("seller account number is invalid")
		}
		if uint(sellerAccount.ClientId) != offer.SellerID {
			return nil, errors.BadRequestErr("the provided account does not belong to you")
		}

		offer.SellerAccountNumber = req.AccountNumber
	}

	oldOffer := *offer

	offer.Amount = req.Amount
	offer.PricePerStockRSD = req.PricePerStockRSD
	offer.PremiumRSD = req.PremiumRSD
	offer.SettlementDate = req.SettlementDate
	offer.LastModified = s.now()
	offer.ModifiedBy = callerID

	if err := s.offerRepo.Save(ctx, offer); err != nil {
		return nil, errors.InternalErr(err)
	}

	if err := s.otcNegotiationHistoryService.CreateNegotiationHistory(ctx, offerID, &oldOffer, offer, callerID); err != nil {
		log.Printf("Failed creating negotiation history for offer id %d", offerID)
	}

	updated, err := s.offerRepo.FindByID(ctx, offer.OtcOfferID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	var recipientID uint

	if callerID == offer.BuyerID {
		recipientID = offer.SellerID
	} else {
		recipientID = offer.BuyerID
	}

	s.sendEmailToUser(
		ctx,
		recipientID,
		"OTC Counter Offer",
		fmt.Sprintf(
			"A new counter-offer has been submitted for OTC offer #%d.",
			offer.OtcOfferID,
		),
	)
	return updated, nil
}

// AcceptOffer is called by the party OPPOSITE to ModifiedBy to accept the offer.
//
// On acceptance, the service validates the current negotiation state and then
// delegates the actual agreement finalization to the OTC processing layer.
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
		sellerAccount, err := s.bankingClient.GetAccountByNumber(ctx, *req.AccountNumber)
		if err != nil {
			return nil, errors.BadRequestErr("seller account number is invalid")
		}

		if uint(sellerAccount.ClientId) != offer.SellerID {
			return nil, errors.BadRequestErr("the provided account does not belong to you")
		}

		offer.SellerAccountNumber = req.AccountNumber
		if err := s.offerRepo.Save(ctx, offer); err != nil {
			return nil, errors.InternalErr(err)
		}
	}

	if offer.SellerAccountNumber == nil {
		return nil, errors.BadRequestErr("seller account number is missing — the seller must send a counter-offer or accept first")
	}

	if err := s.validateSellerCapacity(ctx, offer.SellerID, offer.StockAssetID, offer.Amount, &offer.OtcOfferID); err != nil {
		return nil, err
	}

	contract, err := s.processingService.FinalizeAgreement(
		ctx,
		offer.OtcOfferID,
		callerID,
	)
	if err != nil {
		return nil, err
	}
	var recipientID uint

	if callerID == offer.BuyerID {
		recipientID = offer.SellerID
	} else {
		recipientID = offer.BuyerID
	}

	s.sendEmailToUser(
		ctx,
		recipientID,
		"OTC Offer Accepted",
		fmt.Sprintf(
			"OTC offer #%d has been accepted.",
			offer.OtcOfferID,
		),
	)
	return contract, nil
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
	var recipientID uint

	if callerID == offer.BuyerID {
		recipientID = offer.SellerID
	} else {
		recipientID = offer.BuyerID
	}

	s.sendEmailToUser(
		ctx,
		recipientID,
		"OTC Offer Rejected",
		fmt.Sprintf(
			"OTC offer #%d has been rejected.",
			offer.OtcOfferID,
		),
	)
	return updated, nil
}

// GetActiveOffersForUser returns all active negotiations in which the given user participates.
func (s *OtcOfferService) GetActiveOffersForUser(ctx context.Context, userID uint) ([]dto.OtcOfferResponse, error) {
	offers, err := s.offerRepo.FindActiveForUser(ctx, userID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if len(offers) == 0 {
		return []dto.OtcOfferResponse{}, nil
	}

	// --- 1. Collect AssetIDs ---
	assetIDSet := make(map[uint]struct{})
	for _, o := range offers {
		assetIDSet[o.StockAssetID] = struct{}{}
	}
	assetIDs := make([]uint, 0, len(assetIDSet))
	for id := range assetIDSet {
		assetIDs = append(assetIDs, id)
	}

	// --- 2. Fetch stocks ---
	stocks, err := s.stockRepo.FindByAssetIDs(ctx, assetIDs)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	// --- 3. Build market data map ---
	type marketData struct {
		price    float64
		currency string
	}
	dataMap := make(map[uint]marketData)
	for _, stock := range stocks {
		if stock.Listing != nil {
			md := marketData{
				price: stock.Listing.Price,
			}
			if stock.Listing.Exchange != nil {
				md.currency = stock.Listing.Exchange.Currency
			}
			dataMap[stock.AssetID] = md
		}
	}

	// --- 4. Convert to DTO ---
	resp := dto.ToOtcOfferResponseList(offers)

	// --- 5. Inject market data ---
	for i := range resp {
		if md, ok := dataMap[resp[i].StockAssetID]; ok {
			resp[i].CurrentPrice = &md.price
			resp[i].ListingCurrency = md.currency

			// Convert to RSD if not already in RSD
			if md.currency != "RSD" {
				priceRSD, err := s.bankingClient.ConvertCurrency(ctx, md.price, md.currency, "RSD")
				if err != nil {
					resp[i].CurrentPriceRSD = nil
				} else {
					resp[i].CurrentPriceRSD = &priceRSD
				}
			} else {
				// Already in RSD, reuse the same value
				resp[i].CurrentPriceRSD = &md.price
			}
		} else {
			resp[i].CurrentPrice = nil
			resp[i].ListingCurrency = ""
			resp[i].CurrentPriceRSD = nil
		}
	}

	return resp, nil
}

func (s *OtcOfferService) GetOptionContractsForUser(
	ctx context.Context,
	userID uint,
) ([]dto.OtcOptionContractResponse, error) {

	contracts, err := s.optionContractRepo.FindForUser(ctx, userID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if len(contracts) == 0 {
		return []dto.OtcOptionContractResponse{}, nil
	}

	// --- 1. Collect AssetIDs + UserIDs ---
	assetIDSet := make(map[uint]struct{})
	userIDSet := make(map[uint64]struct{})

	for _, c := range contracts {
		assetIDSet[c.StockAssetID] = struct{}{}

		userIDSet[uint64(c.BuyerID)] = struct{}{}
		userIDSet[uint64(c.SellerID)] = struct{}{}
	}

	assetIDs := make([]uint, 0, len(assetIDSet))
	for id := range assetIDSet {
		assetIDs = append(assetIDs, id)
	}

	userIDs := make([]uint64, 0, len(userIDSet))
	for id := range userIDSet {
		userIDs = append(userIDs, id)
	}

	// --- 2. Fetch stocks ---
	stocks, err := s.stockRepo.FindByAssetIDs(ctx, assetIDs)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	// --- 3. Build market data map ---
	type marketData struct {
		price    float64
		currency string
	}
	dataMap := make(map[uint]marketData)

	for _, stock := range stocks {
		if stock.Listing != nil {
			md := marketData{
				price: stock.Listing.Price,
			}
			if stock.Listing.Exchange != nil {
				md.currency = stock.Listing.Exchange.Currency
			}
			dataMap[stock.AssetID] = md
		}
	}

	// --- 4. Fetch users (BATCH) ---
	userResp, err := s.userClient.GetClientsByIds(ctx, userIDs)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	userMap := make(map[uint64]string)
	for _, u := range userResp.Clients {
		userMap[u.Id] = u.FullName
	}

	// --- 5. Convert to DTO ---
	resp := dto.ToOtcOptionContractResponseList(contracts)

	// --- 6. Inject everything ---
	for i := range resp {
		assetID := resp[i].StockAssetID

		// Market data
		if md, ok := dataMap[assetID]; ok {
			resp[i].CurrentPrice = &md.price
			resp[i].ListingCurrency = md.currency
		} else {
			resp[i].CurrentPrice = nil
			resp[i].ListingCurrency = ""
		}

		// User data
		resp[i].BuyerFullName = userMap[uint64(resp[i].BuyerID)]
		resp[i].SellerFullName = userMap[uint64(resp[i].SellerID)]

		// TODO Change this when communication with other banks is impleemented
		resp[i].BuyerBank = "Banka 4"
		resp[i].SellerBank = "Banka 4"
	}

	return resp, nil
}

// ExerciseContract allows the buyer who holds the OTC option contract to start
// or resume its settlement saga.
func (s *OtcOfferService) ExerciseContract(ctx context.Context, contractID uint) (*model.OtcExecutionSaga, error) {
	callerID, err := auth.GetSubjectFromContext(ctx)
	if err != nil {
		return nil, err
	}

	contract, err := s.optionContractRepo.FindByID(ctx, contractID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	if contract == nil {
		return nil, errors.NotFoundErr("OTC contract not found")
	}

	if callerID != contract.BuyerID {
		return nil, errors.ForbiddenErr("only the buyer may exercise this OTC contract")
	}

	return s.processingService.ExerciseContract(ctx, contractID)
}

// GetExecution returns the saga state and its per-step attempt log for an OTC
// execution, restricted to the contract's buyer or seller.
func (s *OtcOfferService) GetExecution(ctx context.Context, executionID uint) (*model.OtcExecutionSaga, []model.OtcExecutionSagaLogEntry, error) {
	callerID, err := auth.GetSubjectFromContext(ctx)
	if err != nil {
		return nil, nil, err
	}

	saga, err := s.processingService.GetExecutionStatus(ctx, executionID)
	if err != nil {
		return nil, nil, err
	}

	if callerID != saga.Contract.BuyerID && callerID != saga.Contract.SellerID {
		return nil, nil, errors.ForbiddenErr("you are not a participant in this OTC contract")
	}

	entries, err := s.processingService.GetExecutionLog(ctx, executionID)
	if err != nil {
		return nil, nil, err
	}

	return saga, entries, nil
}

// GetExecutionLog returns the per-step attempt log for an execution the caller
// already has access to (e.g. one just returned by ExerciseContract).
func (s *OtcOfferService) GetExecutionLog(ctx context.Context, executionID uint) ([]model.OtcExecutionSagaLogEntry, error) {
	return s.processingService.GetExecutionLog(ctx, executionID)
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
		if stocks[i].AssetID == stockID {
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

func (s *OtcOfferService) sendEmailToUser(
	ctx context.Context,
	userID uint,
	subject string,
	body string,
) {
	if s.mailer == nil || s.userClient == nil {
		return
	}

	resp, err := s.userClient.GetClientsByIds(ctx, []uint64{uint64(userID)})
	if err != nil || resp == nil || len(resp.Clients) == 0 {
		return
	}

	client := resp.Clients[0]
	if client == nil || client.Email == "" {
		return
	}

	_ = s.mailer.Send(client.Email, subject, body)
}
