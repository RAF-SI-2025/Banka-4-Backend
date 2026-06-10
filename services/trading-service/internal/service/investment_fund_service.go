package service

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	commonErrors "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
)

type InvestmentFundService struct {
	fundRepo       repository.InvestmentFundRepository
	listingRepo    repository.ListingRepository
	positionRepo   repository.ClientFundPositionRepository
	investmentRepo repository.ClientFundInvestmentRepository
	ownershipRepo  repository.AssetOwnershipRepository
	exchangeRepo   repository.ExchangeRepository
	stockRepo      repository.StockRepository
	optionRepo     repository.OptionRepository
	futuresRepo    repository.FuturesContractRepository
	forexRepo      repository.ForexRepository
	bankingClient  client.BankingClient
	userClient     client.UserServiceClient
	now            func() time.Time
	redemptionRepo repository.ClientFundRedemptionRepository
	orderService   *OrderService
}

func NewInvestmentFundService(
	fundRepo repository.InvestmentFundRepository,
	positionRepo repository.ClientFundPositionRepository,
	listingRepo repository.ListingRepository,
	investmentRepo repository.ClientFundInvestmentRepository,
	redemptionRepo repository.ClientFundRedemptionRepository,
	ownershipRepo repository.AssetOwnershipRepository,
	exchangeRepo repository.ExchangeRepository,
	stockRepo repository.StockRepository,
	optionRepo repository.OptionRepository,
	futuresRepo repository.FuturesContractRepository,
	forexRepo repository.ForexRepository,
	bankingClient client.BankingClient,
	userClient client.UserServiceClient,
	orderService *OrderService,
) *InvestmentFundService {
	return &InvestmentFundService{
		fundRepo:       fundRepo,
		positionRepo:   positionRepo,
		listingRepo:    listingRepo,
		investmentRepo: investmentRepo,
		redemptionRepo: redemptionRepo,
		ownershipRepo:  ownershipRepo,
		exchangeRepo:   exchangeRepo,
		stockRepo:      stockRepo,
		optionRepo:     optionRepo,
		futuresRepo:    futuresRepo,
		forexRepo:      forexRepo,
		bankingClient:  bankingClient,
		userClient:     userClient,
		orderService:   orderService,
		now:            time.Now,
	}
}

const pendingRedemptionBatchSize = 25
const defaultFundNAV = 1.0
const floatTolerance = 1e-9

func unitsFromPosition(position model.ClientFundPosition) float64 {
	if position.UnitsOwned > 0 {
		return position.UnitsOwned
	}
	// Backward compatibility for legacy rows created before units support.
	if position.TotalInvestedAmount > 0 {
		return position.TotalInvestedAmount
	}
	return 0
}

func calculateFundNAV(fundTotalValue, totalUnits float64) float64 {
	if totalUnits <= 0 {
		return defaultFundNAV
	}
	nav := fundTotalValue / totalUnits
	if nav <= 0 || math.IsNaN(nav) || math.IsInf(nav, 0) {
		return defaultFundNAV
	}
	return nav
}

func (s *InvestmentFundService) sumSecuritiesValue(ctx context.Context, fundID uint) (float64, error) {
	ownerships, err := s.ownershipRepo.FindByUserId(ctx, fundID, model.OwnerTypeFund)
	if err != nil {
		return 0, err
	}
	if len(ownerships) == 0 {
		return 0, nil
	}

	assetIDs := make([]uint, len(ownerships))
	for i, o := range ownerships {
		assetIDs[i] = o.AssetID
	}

	listings, err := s.listingRepo.FindByAssetIDs(ctx, assetIDs)
	if err != nil {
		return 0, err
	}

	priceInRSDByAsset := make(map[uint]float64, len(listings))
	for _, l := range listings {
		currency := "RSD"
		if l.Exchange != nil && l.Exchange.Currency != "" {
			currency = l.Exchange.Currency
		}
		priceRSD, err := s.bankingClient.ConvertCurrency(ctx, l.Price, currency, "RSD")
		if err != nil {
			return 0, err
		}
		priceInRSDByAsset[l.AssetID] = priceRSD
	}

	var total float64
	for _, o := range ownerships {
		total += o.Amount * priceInRSDByAsset[o.AssetID]
	}
	return total, nil
}

func (s *InvestmentFundService) getLiquidAssets(ctx context.Context, accountNumber string) (float64, error) {
	resp, err := s.bankingClient.GetAccountByNumber(ctx, accountNumber)
	if err != nil {
		return 0, err
	}
	if resp == nil {
		return 0, nil
	}
	return resp.AvailableBalance, nil
}

// @Param sort_by query string false "Sort by field: name, minimum_contribution, created_at, liquid_assets, annual_return, reward_to_variability, max_drawdown, volatility"
func (s *InvestmentFundService) GetAllFunds(ctx context.Context, query dto.ListFundsQuery) (*dto.ListFundsResponse, error) {
	metricSortFields := map[string]bool{
		"annual_return":         true,
		"reward_to_variability": true,
		"max_drawdown":          true,
		"volatility":            true,
	}
	isMetricSort := metricSortFields[strings.ToLower(query.SortBy)]

	var funds []model.InvestmentFund
	var total int64
	var err error

	if isMetricSort {
		funds, total, err = s.fundRepo.FindAll(ctx, query.Name, "name", "asc", 1, math.MaxInt32)
	} else {
		funds, total, err = s.fundRepo.FindAll(ctx, query.Name, query.SortBy, query.SortDir, query.Page, query.PageSize)
	}
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	allHistories, err := s.fundRepo.GetAllPerformanceHistories(ctx, MinSnapshotsForMetrics)
	if err != nil {
		allHistories = map[uint][]model.FundPerformance{}
	}

	result := make([]dto.FundSummaryResponse, len(funds))
	for i, fund := range funds {
		secVal, err := s.sumSecuritiesValue(ctx, fund.FundID)
		if err != nil {
			return nil, commonErrors.InternalErr(err)
		}
		liquidAssets, err := s.getLiquidAssets(ctx, fund.AccountNumber)
		if err != nil {
			return nil, commonErrors.InternalErr(err)
		}

		metrics := calculateFundMetrics(allHistories[fund.FundID])
		result[i] = dto.ToFundSummaryResponse(fund, secVal, liquidAssets,
			metrics.AnnualReturn, metrics.RewardToVariability, metrics.MaxDrawdown, metrics.Volatility)
	}

	if isMetricSort {
		switch strings.ToLower(query.SortBy) {
		case "annual_return":
			sort.Slice(result, makeMetricSorter(result, func(f dto.FundSummaryResponse) *float64 { return f.AnnualReturn }, query.SortDir))
		case "reward_to_variability":
			sort.Slice(result, makeMetricSorter(result, func(f dto.FundSummaryResponse) *float64 { return f.RewardToVariability }, query.SortDir))
		case "max_drawdown":
			sort.Slice(result, makeMetricSorter(result, func(f dto.FundSummaryResponse) *float64 { return f.MaxDrawdown }, query.SortDir))
		case "volatility":
			sort.Slice(result, makeMetricSorter(result, func(f dto.FundSummaryResponse) *float64 { return f.Volatility }, query.SortDir))
		}

		total = int64(len(result))
		start := (query.Page - 1) * query.PageSize
		if start >= len(result) {
			result = []dto.FundSummaryResponse{}
		} else {
			end := start + query.PageSize
			if end > len(result) {
				end = len(result)
			}
			result = result[start:end]
		}
	}

	return &dto.ListFundsResponse{
		Data:     result,
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
	}, nil
}

func (s *InvestmentFundService) GetBankFundPositions(ctx context.Context) ([]dto.FundPositionResponse, error) {
	funds, err := s.fundRepo.GetAllInvestmentFunds(ctx)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	result := make([]dto.FundPositionResponse, 0, len(funds))

	for _, fund := range funds {
		secVal, err := s.sumSecuritiesValue(ctx, fund.FundID)
		if err != nil {
			return nil, commonErrors.InternalErr(err)
		}
		liquidAssets, err := s.getLiquidAssets(ctx, fund.AccountNumber)
		if err != nil {
			return nil, commonErrors.InternalErr(err)
		}

		fundTotalValue := liquidAssets + secVal

		var totalUnits float64
		var bankUnits float64
		var bankInvested float64
		for _, pos := range fund.Positions {
			units := unitsFromPosition(pos)
			totalUnits += units

			if pos.OwnerType == model.OwnerTypeBank {
				bankUnits += units
				bankInvested += pos.TotalInvestedAmount
			}
		}

		nav := calculateFundNAV(fundTotalValue, totalUnits)

		var bankPct float64
		if totalUnits > 0 {
			bankPct = (bankUnits / totalUnits) * 100
		} else {
			bankPct = 0
		}

		bankValue := bankUnits * nav
		profit := bankValue - bankInvested

		managerName := ""
		if manager, err := s.userClient.GetEmployeeById(ctx, uint64(fund.ManagerID)); err == nil {
			managerName = manager.FullName
		}

		result = append(result, dto.FundPositionResponse{
			FundName:       fund.Name,
			ManagerName:    managerName,
			BankSharePct:   bankPct,
			BankShareValue: bankValue,
			Profit:         profit,
		})
	}

	return result, nil
}

func (s *InvestmentFundService) GetActuaryFunds(ctx context.Context, managerID uint) ([]dto.ActuaryFundResponse, error) {
	funds, err := s.fundRepo.FindByManagerID(ctx, managerID)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	result := make([]dto.ActuaryFundResponse, len(funds))
	for i, fund := range funds {
		secVal, err := s.sumSecuritiesValue(ctx, fund.FundID)
		if err != nil {
			return nil, commonErrors.InternalErr(err)
		}
		liquidAssets, err := s.getLiquidAssets(ctx, fund.AccountNumber)
		if err != nil {
			return nil, commonErrors.InternalErr(err)
		}
		result[i] = dto.ToActuaryFundResponse(fund, secVal, liquidAssets)
	}

	return result, nil
}

// CreateFund creates a new investment fund. Only supervisors can call this.
// A bank account is automatically created for the fund via the banking service.
func (s *InvestmentFundService) CreateFund(ctx context.Context, req dto.CreateFundRequest) (*dto.CreateFundResponse, error) {
	authCtx := auth.GetAuthFromContext(ctx)
	if authCtx == nil {
		return nil, commonErrors.UnauthorizedErr("not authenticated")
	}

	if authCtx.IdentityType != auth.IdentityEmployee {
		return nil, commonErrors.ForbiddenErr("only employees can create investment funds")
	}

	if authCtx.EmployeeID == nil {
		return nil, commonErrors.UnauthorizedErr("employee identity missing")
	}

	managerID := *authCtx.EmployeeID

	existing, err := s.fundRepo.FindByName(ctx, req.Name)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}
	if existing != nil {
		return nil, commonErrors.ConflictErr("fund name is already taken")
	}

	accountNumber, err := s.bankingClient.CreateFundAccount(ctx, req.Name, uint64(managerID))
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	fund := &model.InvestmentFund{
		Name:                        req.Name,
		Description:                 req.Description,
		MinimumContribution:         req.MinimumContribution,
		ManagerID:                   managerID,
		AccountNumber:               accountNumber,
		DividendReinvestmentPercent: req.DividendReinvestmentPercent,
		CreatedAt:                   s.now(),
	}

	if err := s.fundRepo.Create(ctx, fund); err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	return &dto.CreateFundResponse{
		FundID:                      fund.FundID,
		Name:                        fund.Name,
		Description:                 fund.Description,
		MinimumContribution:         fund.MinimumContribution,
		ManagerID:                   fund.ManagerID,
		AccountNumber:               fund.AccountNumber,
		DividendReinvestmentPercent: fund.DividendReinvestmentPercent,
		CreatedAt:                   fund.CreatedAt,
	}, nil
}

// InvestInFund handles a client or supervisor investing into a fund.
//
// Rules:
//   - Clients must use one of their own accounts.
//   - Supervisors must use a bank account.
//   - req.Amount is in the account's currency.
//   - MinimumContribution is stored in RSD, so req.Amount is converted to RSD before the check.
//   - The account is debited via ExecuteTradeSettlement (BUY direction).
//   - A ClientFundInvestment record is always created.
//   - The ClientFundPosition is created if it does not exist, or updated otherwise.
func (s *InvestmentFundService) InvestInFund(ctx context.Context, fundID uint, req dto.InvestInFundRequest) (*dto.InvestInFundResponse, error) {
	authCtx := auth.GetAuthFromContext(ctx)
	if authCtx == nil {
		return nil, commonErrors.UnauthorizedErr("not authenticated")
	}

	fund, err := s.fundRepo.FindByID(ctx, fundID)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}
	if fund == nil {
		return nil, commonErrors.NotFoundErr("fund not found")
	}

	callerID, ownerType, err := resolveCallerIdentity(authCtx)
	if err != nil {
		return nil, err
	}

	account, err := s.validateFundAccount(ctx, req.AccountNumber, authCtx)
	if err != nil {
		return nil, err
	}
	currencyCode := account.GetCurrencyCode()

	amountInRSD, err := s.bankingClient.ConvertCurrency(ctx, req.Amount, currencyCode, "RSD")
	if err != nil {
		return nil, commonErrors.ServiceUnavailableErr(err)
	}
	if amountInRSD < fund.MinimumContribution {
		return nil, commonErrors.BadRequestErr(
			fmt.Sprintf("amount %.2f %s (≈ %.2f RSD) is below the fund's minimum contribution of %.2f RSD",
				req.Amount, currencyCode, amountInRSD, fund.MinimumContribution),
		)
	}

	nav, err := s.getFundNAV(ctx, fund.FundID, fund.AccountNumber)
	if err != nil {
		return nil, err
	}
	unitsBought := amountInRSD / nav
	if unitsBought <= 0 || math.IsNaN(unitsBought) || math.IsInf(unitsBought, 0) {
		return nil, commonErrors.BadRequestErr("unable to calculate purchased fund units")
	}

	commissionExempt := authCtx.IdentityType == auth.IdentityEmployee

	_, err = s.bankingClient.CreatePaymentWithoutVerification(ctx, &pb.CreatePaymentRequest{
		PayerAccountNumber:     req.AccountNumber,
		RecipientAccountNumber: fund.AccountNumber,
		RecipientName:          fund.Name,
		Amount:                 req.Amount,
		PaymentCode:            "289",
		Purpose:                fmt.Sprintf("Investment into fund %s", fund.Name),
		CommissionExempt:       commissionExempt,
	})

	if err != nil {
		return nil, mapFundPaymentError(err)
	}

	now := s.now()

	investment := &model.ClientFundInvestment{
		ClientID:      callerID,
		OwnerType:     ownerType,
		FundID:        fundID,
		AccountNumber: req.AccountNumber,
		Amount:        amountInRSD,
		CurrencyCode:  currencyCode,
		CreatedAt:     now,
	}
	if err := s.investmentRepo.Create(ctx, investment); err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	position, err := s.positionRepo.FindByClientAndFund(ctx, callerID, ownerType, fundID)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}
	if position == nil {
		position = &model.ClientFundPosition{
			ClientID:            callerID,
			OwnerType:           ownerType,
			FundID:              fundID,
			UnitsOwned:          unitsBought,
			TotalInvestedAmount: amountInRSD,
			UpdatedAt:           now,
		}
	} else {
		position.UnitsOwned = unitsFromPosition(*position) + unitsBought
		position.TotalInvestedAmount += amountInRSD
		position.UpdatedAt = now
	}
	if err := s.positionRepo.Upsert(ctx, position); err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	return &dto.InvestInFundResponse{
		FundID:           fund.FundID,
		FundName:         fund.Name,
		InvestedNow:      req.Amount,
		CurrencyCode:     currencyCode,
		TotalInvestedRSD: position.TotalInvestedAmount,
		CreatedAt:        now,
	}, nil
}

func (s *InvestmentFundService) WithdrawFromFund(ctx context.Context, fundID uint, req dto.WithdrawFromFundRequest) (*dto.WithdrawFromFundResponse, error) {
	authCtx := auth.GetAuthFromContext(ctx)
	if authCtx == nil {
		return nil, commonErrors.UnauthorizedErr("not authenticated")
	}

	fund, err := s.fundRepo.FindByID(ctx, fundID)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}
	if fund == nil {
		return nil, commonErrors.NotFoundErr("fund not found")
	}

	callerID, ownerType, err := resolveCallerIdentity(authCtx)
	if err != nil {
		return nil, err
	}

	destinationAccount, err := s.validateFundAccount(ctx, req.AccountNumber, authCtx)
	if err != nil {
		return nil, err
	}

	position, err := s.positionRepo.FindByClientAndFund(ctx, callerID, ownerType, fundID)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}
	if position == nil || unitsFromPosition(*position) <= 0 {
		return nil, commonErrors.NotFoundErr("fund position not found")
	}

	pendingAmount, err := s.redemptionRepo.SumPendingByClientAndFund(ctx, callerID, ownerType, fundID)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	nav, err := s.getFundNAV(ctx, fund.FundID, fund.AccountNumber)
	if err != nil {
		return nil, err
	}
	positionValueRSD := unitsFromPosition(*position) * nav
	if req.Amount > positionValueRSD-pendingAmount+floatTolerance {
		return nil, commonErrors.BadRequestErr("withdrawal amount exceeds available fund position")
	}

	now := s.now()
	redemption := &model.ClientFundRedemption{
		ClientID:      callerID,
		OwnerType:     ownerType,
		FundID:        fundID,
		AccountNumber: req.AccountNumber,
		Amount:        req.Amount,
		CurrencyCode:  "RSD",
		Status:        model.FundRedemptionPendingLiquidation,
		CreatedAt:     now,
	}

	liquidAssets, err := s.getLiquidAssets(ctx, fund.AccountNumber)
	if err != nil {
		return nil, commonErrors.ServiceUnavailableErr(err)
	}

	if liquidAssets < req.Amount {
		ordersCreated, err := s.liquidateFundAssets(ctx, fund, req.Amount-liquidAssets)
		if err != nil {
			return nil, err
		}
		if ordersCreated == 0 {
			return nil, commonErrors.BadRequestErr("fund has insufficient liquid assets for this withdrawal")
		}

		liquidAssets, err = s.getLiquidAssets(ctx, fund.AccountNumber)
		if err != nil {
			return nil, commonErrors.ServiceUnavailableErr(err)
		}
	}

	if liquidAssets >= req.Amount {
		return s.completeFundRedemption(ctx, fund, position, redemption, destinationAccount, authCtx.IdentityType == auth.IdentityEmployee)
	}

	if err := s.redemptionRepo.Create(ctx, redemption); err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	return &dto.WithdrawFromFundResponse{
		FundID:                   fund.FundID,
		FundName:                 fund.Name,
		DestinationAccountNumber: req.AccountNumber,
		DestinationCurrencyCode:  destinationAccount.GetCurrencyCode(),
		RequestedAmountRSD:       req.Amount,
		WithdrawnAmountRSD:       0,
		TotalInvestedRSD:         position.TotalInvestedAmount,
		Status:                   redemption.Status,
		Message:                  "Fund liquidation has started; the payout will be completed when liquidity is available",
		CreatedAt:                redemption.CreatedAt,
	}, nil
}

func (s *InvestmentFundService) completeFundRedemption(
	ctx context.Context,
	fund *model.InvestmentFund,
	position *model.ClientFundPosition,
	redemption *model.ClientFundRedemption,
	destinationAccount *pb.GetAccountByNumberResponse,
	commissionExempt bool,
) (*dto.WithdrawFromFundResponse, error) {
	nav, err := s.getFundNAV(ctx, fund.FundID, fund.AccountNumber)
	if err != nil {
		return nil, err
	}
	if redemption.Amount > unitsFromPosition(*position)*nav+floatTolerance {
		return nil, commonErrors.BadRequestErr("withdrawal amount exceeds available fund position")
	}

	_, err = s.bankingClient.CreatePaymentWithoutVerification(ctx, &pb.CreatePaymentRequest{
		PayerAccountNumber:     fund.AccountNumber,
		RecipientAccountNumber: redemption.AccountNumber,
		RecipientName:          fund.Name,
		Amount:                 redemption.Amount,
		PaymentCode:            "289",
		Purpose:                fmt.Sprintf("Withdrawal from fund %s", fund.Name),
		CommissionExempt:       commissionExempt,
	})
	if err != nil {
		return nil, mapFundPaymentError(err)
	}

	now := s.now()
	applyRedemptionToPosition(position, redemption.Amount, nav)
	position.UpdatedAt = now
	if err := s.positionRepo.Upsert(ctx, position); err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	redemption.Status = model.FundRedemptionCompleted
	redemption.CompletedAt = &now
	if redemption.ClientFundRedemptionID == 0 {
		if err := s.redemptionRepo.Create(ctx, redemption); err != nil {
			return nil, commonErrors.InternalErr(err)
		}
	} else if err := s.redemptionRepo.Update(ctx, redemption); err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	return &dto.WithdrawFromFundResponse{
		FundID:                   fund.FundID,
		FundName:                 fund.Name,
		DestinationAccountNumber: redemption.AccountNumber,
		DestinationCurrencyCode:  destinationAccount.GetCurrencyCode(),
		RequestedAmountRSD:       redemption.Amount,
		WithdrawnAmountRSD:       redemption.Amount,
		TotalInvestedRSD:         position.TotalInvestedAmount,
		Status:                   redemption.Status,
		CreatedAt:                redemption.CreatedAt,
		CompletedAt:              redemption.CompletedAt,
	}, nil
}

type fundLiquidationCandidate struct {
	listing  model.Listing
	amount   float64
	priceRSD float64
}

func (s *InvestmentFundService) liquidateFundAssets(ctx context.Context, fund *model.InvestmentFund, targetRSD float64) (int, error) {
	ownerships, err := s.ownershipRepo.FindByUserId(ctx, fund.FundID, model.OwnerTypeFund)
	if err != nil {
		return 0, commonErrors.InternalErr(err)
	}
	if len(ownerships) == 0 {
		return 0, nil
	}

	assetIDs := make([]uint, 0, len(ownerships))
	ownershipByAssetID := make(map[uint]model.AssetOwnership, len(ownerships))
	for _, ownership := range ownerships {
		if ownership.Amount <= 0 {
			continue
		}
		assetIDs = append(assetIDs, ownership.AssetID)
		ownershipByAssetID[ownership.AssetID] = ownership
	}
	if len(assetIDs) == 0 {
		return 0, nil
	}

	listings, err := s.listingRepo.FindByAssetIDs(ctx, assetIDs)
	if err != nil {
		return 0, commonErrors.InternalErr(err)
	}

	candidates := make([]fundLiquidationCandidate, 0, len(listings))
	for _, listing := range listings {
		ownership, ok := ownershipByAssetID[listing.AssetID]
		if !ok || listing.Price <= 0 {
			continue
		}

		currency := "RSD"
		if listing.Exchange != nil && listing.Exchange.Currency != "" {
			currency = listing.Exchange.Currency
		}
		priceRSD, err := s.bankingClient.ConvertCurrency(ctx, listing.Price, currency, "RSD")
		if err != nil {
			return 0, commonErrors.ServiceUnavailableErr(err)
		}
		if priceRSD <= 0 {
			continue
		}

		candidates = append(candidates, fundLiquidationCandidate{
			listing:  listing,
			amount:   ownership.Amount,
			priceRSD: priceRSD,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].amount*candidates[i].priceRSD > candidates[j].amount*candidates[j].priceRSD
	})
	if len(candidates) == 0 {
		return 0, nil
	}
	if s.orderService == nil {
		return 0, commonErrors.ServiceUnavailableErr(fmt.Errorf("order service unavailable"))
	}

	remaining := targetRSD
	ordersCreated := 0
	for _, candidate := range candidates {
		if remaining <= 0 {
			break
		}

		availableQuantity := uint(math.Floor(candidate.amount))
		if availableQuantity == 0 {
			continue
		}

		quantity := uint(math.Ceil(remaining / candidate.priceRSD))
		if quantity == 0 {
			quantity = 1
		}
		if quantity > availableQuantity {
			quantity = availableQuantity
		}

		order, err := s.orderService.CreateFundLiquidationOrder(ctx, fund, candidate.listing.ListingID, quantity)
		if err != nil {
			return ordersCreated, err
		}

		ordersCreated++
		remaining -= float64(quantity) * candidate.priceRSD

		if order.Status == model.OrderStatusApproved {
			if err := s.orderService.processOrder(ctx, order); err != nil {
				log.Printf("[fund-redemptions] liquidation order %d processing failed: %v", order.OrderID, err)
			}
		}
	}

	return ordersCreated, nil
}

func (s *InvestmentFundService) ProcessPendingRedemptions(ctx context.Context) error {
	if s.redemptionRepo == nil {
		return nil
	}

	redemptions, err := s.redemptionRepo.FindPending(ctx, pendingRedemptionBatchSize)
	if err != nil {
		return commonErrors.InternalErr(err)
	}

	for i := range redemptions {
		if err := s.processPendingRedemption(ctx, &redemptions[i]); err != nil {
			log.Printf("[fund-redemptions] failed to process redemption %d: %v", redemptions[i].ClientFundRedemptionID, err)
		}
	}

	return nil
}

func (s *InvestmentFundService) processPendingRedemption(ctx context.Context, redemption *model.ClientFundRedemption) error {
	fund := &redemption.Fund
	if fund.FundID == 0 {
		found, err := s.fundRepo.FindByID(ctx, redemption.FundID)
		if err != nil {
			return commonErrors.InternalErr(err)
		}
		if found == nil {
			return commonErrors.NotFoundErr("fund not found")
		}
		fund = found
	}

	liquidAssets, err := s.getLiquidAssets(ctx, fund.AccountNumber)
	if err != nil {
		return commonErrors.ServiceUnavailableErr(err)
	}
	if liquidAssets < redemption.Amount {
		return nil
	}

	position, err := s.positionRepo.FindByClientAndFund(ctx, redemption.ClientID, redemption.OwnerType, redemption.FundID)
	if err != nil {
		return commonErrors.InternalErr(err)
	}
	if position == nil {
		return commonErrors.BadRequestErr("withdrawal amount exceeds available fund position")
	}

	nav, err := s.getFundNAV(ctx, redemption.FundID, fund.AccountNumber)
	if err != nil {
		return err
	}
	positionValueRSD := unitsFromPosition(*position) * nav
	if positionValueRSD < redemption.Amount-floatTolerance {
		return commonErrors.BadRequestErr("withdrawal amount exceeds available fund position")
	}

	destinationAccount, err := s.bankingClient.GetAccountByNumber(ctx, redemption.AccountNumber)
	if err != nil {
		return commonErrors.ServiceUnavailableErr(err)
	}

	_, err = s.completeFundRedemption(ctx, fund, position, redemption, destinationAccount, redemption.OwnerType == model.OwnerTypeBank)
	return err
}

func (s *InvestmentFundService) validateFundAccount(ctx context.Context, accountNumber string, authCtx *auth.AuthContext) (*pb.GetAccountByNumberResponse, error) {
	account, err := s.bankingClient.GetAccountByNumber(ctx, accountNumber)
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			return nil, commonErrors.NotFoundErr("account not found")
		}
		return nil, commonErrors.ServiceUnavailableErr(err)
	}
	if account == nil {
		return nil, commonErrors.NotFoundErr("account not found")
	}

	switch authCtx.IdentityType {
	case auth.IdentityClient:
		if authCtx.ClientID == nil || uint64(*authCtx.ClientID) != account.GetClientId() {
			return nil, commonErrors.ForbiddenErr("account does not belong to you")
		}
	case auth.IdentityEmployee:
		if account.GetAccountType() != "Bank" {
			return nil, commonErrors.BadRequestErr("supervisors must use a bank account for fund transactions")
		}
	}

	return account, nil
}

func mapFundPaymentError(err error) error {
	st, ok := status.FromError(err)
	if ok {
		switch st.Code() {
		case codes.NotFound:
			return commonErrors.NotFoundErr(st.Message())
		case codes.FailedPrecondition, codes.InvalidArgument:
			return commonErrors.BadRequestErr(st.Message())
		}
	}
	return commonErrors.ServiceUnavailableErr(err)
}

func resolveCallerIdentity(authCtx *auth.AuthContext) (uint, model.OwnerType, error) {
	switch authCtx.IdentityType {
	case auth.IdentityClient:
		if authCtx.ClientID == nil {
			return 0, "", commonErrors.UnauthorizedErr("not authenticated")
		}
		return *authCtx.ClientID, model.OwnerTypeClient, nil
	case auth.IdentityEmployee:
		if authCtx.EmployeeID == nil {
			return 0, "", commonErrors.UnauthorizedErr("not authenticated")
		}
		return *authCtx.EmployeeID, model.OwnerTypeBank, nil
	default:
		return 0, "", commonErrors.UnauthorizedErr("unknown identity type")
	}
}
func (s *InvestmentFundService) getFundSharesValueRSD(ctx context.Context, fundID uint) (float64, error) {
	securitiesValue, err := s.sumSecuritiesValue(ctx, fundID)
	if err != nil {
		return 0, commonErrors.InternalErr(err)
	}
	return securitiesValue, nil
}

func (s *InvestmentFundService) getFundTotalUnits(ctx context.Context, fundID uint) (float64, error) {
	positions, err := s.positionRepo.FindByFund(ctx, fundID)
	if err != nil {
		return 0, commonErrors.InternalErr(err)
	}

	var totalUnits float64
	for _, pos := range positions {
		totalUnits += unitsFromPosition(pos)
	}
	return totalUnits, nil
}

func (s *InvestmentFundService) getFundNAV(ctx context.Context, fundID uint, accountNumber string) (float64, error) {
	liquidAssets, err := s.getLiquidAssets(ctx, accountNumber)
	if err != nil {
		return 0, commonErrors.InternalErr(err)
	}
	sharesValue, err := s.getFundSharesValueRSD(ctx, fundID)
	if err != nil {
		return 0, commonErrors.InternalErr(err)
	}
	totalUnits, err := s.getFundTotalUnits(ctx, fundID)
	if err != nil {
		return 0, err
	}
	return calculateFundNAV(liquidAssets+sharesValue, totalUnits), nil
}

func applyRedemptionToPosition(position *model.ClientFundPosition, amountRSD, nav float64) {
	currentUnits := unitsFromPosition(*position)
	if currentUnits <= 0 || nav <= 0 {
		position.UnitsOwned = 0
		position.TotalInvestedAmount = 0
		return
	}

	currentValue := currentUnits * nav
	if currentValue <= 0 {
		position.UnitsOwned = 0
		position.TotalInvestedAmount = 0
		return
	}

	redeemedUnits := amountRSD / nav
	if redeemedUnits > currentUnits {
		redeemedUnits = currentUnits
	}

	costReductionRatio := amountRSD / currentValue
	if costReductionRatio > 1 {
		costReductionRatio = 1
	}

	// Always persist explicit units after first redemption, including legacy rows.
	position.UnitsOwned = currentUnits - redeemedUnits
	if position.UnitsOwned < floatTolerance {
		position.UnitsOwned = 0
	}

	position.TotalInvestedAmount -= position.TotalInvestedAmount * costReductionRatio
	if position.TotalInvestedAmount < floatTolerance {
		position.TotalInvestedAmount = 0
	}
}

func (s *InvestmentFundService) GetClientFundPositions(ctx context.Context, clientID uint) ([]dto.FundPositionSummaryResponse, error) {
	positions, err := s.positionRepo.FindByClient(ctx, clientID, model.OwnerTypeClient)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	result := make([]dto.FundPositionSummaryResponse, len(positions))
	for i, pos := range positions {
		result[i] = dto.ToFundPositionSummaryResponse(pos)

		fund, err := s.fundRepo.FindByID(ctx, pos.FundID)
		if err != nil || fund == nil {
			return nil, commonErrors.InternalErr(err)
		}

		liquidAssets, err := s.getLiquidAssets(ctx, fund.AccountNumber)
		if err != nil {
			return nil, commonErrors.InternalErr(err)
		}

		sharesValue, err := s.getFundSharesValueRSD(ctx, pos.FundID)
		if err != nil {
			return nil, commonErrors.InternalErr(err)
		}

		fundTotalValue := sharesValue + liquidAssets
		fundTotalUnits, err := s.getFundTotalUnits(ctx, pos.FundID)
		if err != nil {
			return nil, err
		}

		if fundTotalUnits == 0 {
			result[i].ClientsSharePercent = 0
		} else {
			result[i].ClientsSharePercent = (unitsFromPosition(pos) / fundTotalUnits) * 100
		}
		result[i].ClientsShareValueRSD = (result[i].ClientsSharePercent * fundTotalValue) / 100
		result[i].TotalProfit = result[i].ClientsShareValueRSD - pos.TotalInvestedAmount
	}

	return result, nil
}

func (s *InvestmentFundService) GetFundDetail(ctx context.Context, fundID uint) (*dto.FundDetailResponse, error) {
	// 1. Fund base info
	fund, err := s.fundRepo.FindByID(ctx, fundID)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}
	if fund == nil {
		return nil, commonErrors.NotFoundErr("investment fund not found")
	}

	securitiesValue, err := s.sumSecuritiesValue(ctx, fund.FundID)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}
	liquidAssets, err := s.getLiquidAssets(ctx, fund.AccountNumber)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	fundValue := liquidAssets + securitiesValue

	holdings, err := s.fundRepo.FindHoldings(ctx, fundID)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	holdingsResp := make([]dto.SecurityHoldingResponse, 0, len(holdings))

	// Batch fetch listings by asset IDs
	assetIDs := make([]uint, len(holdings))
	for i, h := range holdings {
		assetIDs[i] = h.AssetID
	}
	listings, err := s.listingRepo.FindByAssetIDs(ctx, assetIDs)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}
	listingMap := make(map[uint]*model.Listing)
	for i := range listings {
		listingMap[listings[i].AssetID] = &listings[i]
	}

	var totalInvested float64
	for _, pos := range fund.Positions {
		totalInvested += pos.TotalInvestedAmount
	}

	for _, h := range holdings {
		listing, ok := listingMap[h.AssetID]
		if !ok {
			continue
		}
		dailyInfo, _ := s.listingRepo.FindLastDailyPriceInfo(ctx, listing.ListingID, time.Now())
		currentPrice := listing.Price

		exchangeMic := listing.Exchange.MicCode
		exchange, err := s.exchangeRepo.FindByMicCode(ctx, exchangeMic)
		if err != nil {
			return nil, err
		}
		listingCurrency := exchange.Currency

		change := 0.0
		volume := uint64(0)
		if dailyInfo != nil {
			change = dailyInfo.Change
			volume = uint64(dailyInfo.Volume)
		}

		holdingsResp = append(holdingsResp, dto.SecurityHoldingResponse{
			Ticker:            h.Asset.Ticker,
			Price:             currentPrice,
			Amount:            h.Amount,
			Currency:          listingCurrency,
			Change:            change,
			Volume:            volume,
			InitialMarginCost: listing.MaintenanceMargin,
			AcquisitionDate:   h.UpdatedAt,
		})
	}

	profit := fundValue - totalInvested

	managerName := fmt.Sprintf("Manager %d", fund.ManagerID)
	if s.userClient != nil {

		resp, err := s.userClient.GetEmployeeById(ctx, uint64(fund.ManagerID))
		if err == nil && resp != nil {
			managerName = resp.GetFullName()
		}
	}

	allSnapshots, err := s.fundRepo.GetPerformanceHistory(ctx, fundID, 0)
	if err != nil {
		allSnapshots = []model.FundPerformance{}
	}

	for i, j := 0, len(allSnapshots)-1; i < j; i, j = i+1, j-1 {
		allSnapshots[i], allSnapshots[j] = allSnapshots[j], allSnapshots[i]
	}

	metrics := calculateFundMetrics(allSnapshots)

	displayHistory := allSnapshots
	if len(displayHistory) > 12 {
		displayHistory = displayHistory[len(displayHistory)-12:]
	}

	perfResp := make([]dto.FundPerformanceEntry, len(displayHistory))
	for i, p := range displayHistory {
		perfResp[i] = dto.FundPerformanceEntry{
			Date:         p.Date,
			Value:        p.FundValue,
			Profit:       p.Profit,
			LiquidAssets: p.LiquidAssets,
		}
	}

	allHistories, err := s.fundRepo.GetAllPerformanceHistories(ctx, MinSnapshotsForMetrics)
	if err != nil {
		allHistories = map[uint][]model.FundPerformance{}
	}
	avgHistory := averagePerformanceHistory(allHistories)
	avgHistoryResp := make([]dto.FundPerformanceEntry, len(avgHistory))
	for i, p := range avgHistory {
		avgHistoryResp[i] = dto.FundPerformanceEntry{
			Date:         p.Date,
			Value:        p.FundValue,
			Profit:       p.Profit,
			LiquidAssets: p.LiquidAssets,
		}
	}

	return &dto.FundDetailResponse{
		ID:                          fund.FundID,
		Name:                        fund.Name,
		Description:                 fund.Description,
		Manager:                     managerName,
		FundValue:                   fundValue,
		MinInvestment:               fund.MinimumContribution,
		Profit:                      profit,
		LiquidAssets:                liquidAssets,
		DividendReinvestmentPercent: fund.DividendReinvestmentPercent,
		Holdings:                    holdingsResp,
		PerformanceHistory:          perfResp,
		AnnualReturn:                metrics.AnnualReturn,
		RewardToVariability:         metrics.RewardToVariability,
		MaxDrawdown:                 metrics.MaxDrawdown,
		Volatility:                  metrics.Volatility,
		AverageMarketHistory:        avgHistoryResp,
	}, nil
}

func (s *InvestmentFundService) DeleteFund(ctx context.Context, fundID uint) error {
	authCtx := auth.GetAuthFromContext(ctx)
	if authCtx == nil {
		return commonErrors.UnauthorizedErr("not authenticated")
	}

	if authCtx.IdentityType != auth.IdentityEmployee {
		return commonErrors.ForbiddenErr("only employees can delete investment funds")
	}

	fund, err := s.fundRepo.FindByID(ctx, fundID)
	if err != nil {
		return commonErrors.InternalErr(err)
	}
	if fund == nil {
		return commonErrors.NotFoundErr("investment fund not found")
	}

	positions, err := s.positionRepo.FindByFund(ctx, fundID)
	if err != nil {
		return commonErrors.InternalErr(err)
	}
	if len(positions) > 0 {
		return commonErrors.ConflictErr("cannot delete fund with active client positions")
	}

	if err := s.fundRepo.Delete(ctx, fundID); err != nil {
		return commonErrors.InternalErr(err)
	}

	return nil
}

func (s *InvestmentFundService) TransferFunds(ctx context.Context, fromManagerID uint, toManagerID uint) (int, error) {
	count, err := s.fundRepo.UpdateManagerID(ctx, fromManagerID, toManagerID)
	if err != nil {
		return 0, commonErrors.InternalErr(err)
	}
	return int(count), nil
}

func (s *InvestmentFundService) CalculateAndSaveDailyHistory(ctx context.Context) error {
	funds, err := s.fundRepo.GetAllInvestmentFunds(ctx)
	if err != nil {
		return err
	}

	for _, fund := range funds {
		liquidAssets, err := s.getLiquidAssets(ctx, fund.AccountNumber)
		if err != nil {
			continue
		}

		secVal, err := s.sumSecuritiesValue(ctx, fund.FundID)
		if err != nil {
			continue
		}

		fundValue := liquidAssets + secVal

		var totalInvested float64
		for _, pos := range fund.Positions {
			totalInvested += pos.TotalInvestedAmount
		}

		profit := fundValue - totalInvested

		perf := &model.FundPerformance{
			FundID:       fund.FundID,
			Date:         s.now(),
			FundValue:    fundValue,
			Profit:       profit,
			LiquidAssets: liquidAssets,
		}

		if err := s.fundRepo.SavePerformanceSnapshot(ctx, perf); err != nil {
			log.Printf("failed to get liquid assets for fund %d: %v", fund.FundID, err)
			// Ako pukne čuvanje za jedan fond, samo nastavljamo dalje
			continue
		}
	}

	return nil
}

func makeMetricSorter(
	funds []dto.FundSummaryResponse,
	get func(dto.FundSummaryResponse) *float64,
	dir string,
) func(i, j int) bool {
	desc := strings.ToLower(dir) == "desc"
	return func(i, j int) bool {
		vi := get(funds[i])
		vj := get(funds[j])
		// nil ide na kraj uvek
		if vi == nil && vj == nil {
			return false
		}
		if vi == nil {
			return false
		}
		if vj == nil {
			return true
		}
		if desc {
			return *vi > *vj
		}
		return *vi < *vj
	}
}
