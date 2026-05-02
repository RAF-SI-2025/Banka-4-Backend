package service

import (
	"context"
	"fmt"
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
	bankingClient  client.BankingClient
	userClient     client.UserServiceClient
	now            func() time.Time
}

func NewInvestmentFundService(
	fundRepo repository.InvestmentFundRepository,
	positionRepo repository.ClientFundPositionRepository,
	listingRepo repository.ListingRepository,
	investmentRepo repository.ClientFundInvestmentRepository,
	ownershipRepo repository.AssetOwnershipRepository,
	bankingClient client.BankingClient,
	userClient client.UserServiceClient,
) *InvestmentFundService {
	return &InvestmentFundService{
		fundRepo:       fundRepo,
		positionRepo:   positionRepo,
		listingRepo:    listingRepo,
		investmentRepo: investmentRepo,
		ownershipRepo:  ownershipRepo,
		bankingClient:  bankingClient,
		userClient:     userClient,
		now:            time.Now,
	}
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

func (s *InvestmentFundService) GetAllFunds(ctx context.Context, query dto.ListFundsQuery) (*dto.ListFundsResponse, error) {
	funds, total, err := s.fundRepo.FindAll(ctx, query.Name, query.SortBy, query.SortDir, query.Page, query.PageSize)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
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
		result[i] = dto.ToFundSummaryResponse(fund, secVal, liquidAssets)
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

		var totalInvested float64
		var bankInvested float64
		for _, pos := range fund.Positions {
			totalInvested += pos.TotalInvestedAmount

			if pos.OwnerType == model.OwnerTypeActuary {
				bankInvested += pos.TotalInvestedAmount
			}
		}

		var bankPct float64
		if totalInvested > 0 {
			bankPct = (bankInvested / totalInvested) * 100
		} else {
			bankPct = 0
		}

		bankValue := bankPct * (liquidAssets + secVal)
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
		Name:                req.Name,
		Description:         req.Description,
		MinimumContribution: req.MinimumContribution,
		ManagerID:           managerID,
		AccountNumber:       accountNumber,
		CreatedAt:           s.now(),
	}

	if err := s.fundRepo.Create(ctx, fund); err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	return &dto.CreateFundResponse{
		FundID:              fund.FundID,
		Name:                fund.Name,
		Description:         fund.Description,
		MinimumContribution: fund.MinimumContribution,
		ManagerID:           fund.ManagerID,
		AccountNumber:       fund.AccountNumber,
		CreatedAt:           fund.CreatedAt,
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
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.NotFound:
				return nil, commonErrors.NotFoundErr(st.Message())
			case codes.FailedPrecondition:
				return nil, commonErrors.BadRequestErr(st.Message())
			}
		}
		return nil, commonErrors.ServiceUnavailableErr(err)
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
			TotalInvestedAmount: amountInRSD,
			UpdatedAt:           now,
		}
	} else {
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

func (s *InvestmentFundService) validateFundAccount(ctx context.Context, accountNumber string, authCtx *auth.AuthContext) (*pb.GetAccountByNumberResponse, error) {
	account, err := s.bankingClient.GetAccountByNumber(ctx, accountNumber)
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			return nil, commonErrors.NotFoundErr("account not found")
		}
		return nil, commonErrors.ServiceUnavailableErr(err)
	}

	switch authCtx.IdentityType {
	case auth.IdentityClient:
		if authCtx.ClientID == nil || uint64(*authCtx.ClientID) != account.GetClientId() {
			return nil, commonErrors.ForbiddenErr("account does not belong to you")
		}
	case auth.IdentityEmployee:
		if account.GetAccountType() != "Bank" {
			return nil, commonErrors.BadRequestErr("supervisors must use a bank account for fund investments")
		}
	}

	return account, nil
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
		return *authCtx.EmployeeID, model.OwnerTypeActuary, nil
	default:
		return 0, "", commonErrors.UnauthorizedErr("unknown identity type")
	}
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
		marketValue := h.Amount * currentPrice
		fundValue += marketValue

		change := 0.0
		volume := uint64(0)
		if dailyInfo != nil {
			change = dailyInfo.Change
			volume = uint64(dailyInfo.Volume)
		}

		holdingsResp = append(holdingsResp, dto.SecurityHoldingResponse{
			Ticker:            h.Asset.Ticker,
			Price:             currentPrice,
			Change:            change,
			Volume:            volume,
			InitialMarginCost: listing.MaintenanceMargin,
			AcquisitionDate:   h.UpdatedAt,
		})
	}

	profit := fundValue - totalInvested

	// 6. Performance history (last 12 entries)
	perfHistory, err := s.fundRepo.GetPerformanceHistory(ctx, fundID, 12)
	if err != nil {
		perfHistory = []model.FundPerformance{}
	}
	perfResp := make([]dto.FundPerformanceEntry, len(perfHistory))
	for i, p := range perfHistory {
		perfResp[i] = dto.FundPerformanceEntry{
			Date:         p.Date,
			Value:        p.FundValue,
			Profit:       p.Profit,
			LiquidAssets: p.LiquidAssets,
		}
	}

	managerName := fmt.Sprintf("Manager %d", fund.ManagerID)
	if s.userClient != nil {

		resp, err := s.userClient.GetEmployeeById(ctx, uint64(fund.ManagerID))
		if err == nil && resp != nil {
			managerName = resp.GetFullName()
		}
	}

	return &dto.FundDetailResponse{
		ID:                 fund.FundID,
		Name:               fund.Name,
		Description:        fund.Description,
		Manager:            managerName,
		FundValue:          fundValue,
		MinInvestment:      fund.MinimumContribution,
		Profit:             profit,
		LiquidAssets:       liquidAssets,
		Holdings:           holdingsResp,
		PerformanceHistory: perfResp,
	}, nil
}
