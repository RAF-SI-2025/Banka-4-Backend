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
	positionRepo   repository.ClientFundPositionRepository
	investmentRepo repository.ClientFundInvestmentRepository
	ownershipRepo  repository.AssetOwnershipRepository
	stockRepo      repository.StockRepository
	optionRepo     repository.OptionRepository
	futuresRepo    repository.FuturesContractRepository
	forexRepo      repository.ForexRepository
	bankingClient  client.BankingClient
	now            func() time.Time
}

func NewInvestmentFundService(
	fundRepo repository.InvestmentFundRepository,
	positionRepo repository.ClientFundPositionRepository,
	investmentRepo repository.ClientFundInvestmentRepository,
	ownershipRepo repository.AssetOwnershipRepository,
	stockRepo repository.StockRepository,
	optionRepo repository.OptionRepository,
	futuresRepo repository.FuturesContractRepository,
	forexRepo repository.ForexRepository,
	bankingClient client.BankingClient,
) *InvestmentFundService {
	return &InvestmentFundService{
		fundRepo:       fundRepo,
		positionRepo:   positionRepo,
		investmentRepo: investmentRepo,
		ownershipRepo:  ownershipRepo,
		stockRepo:      stockRepo,
		optionRepo:     optionRepo,
		futuresRepo:    futuresRepo,
		forexRepo:      forexRepo,
		bankingClient:  bankingClient,
		now:            time.Now,
	}
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
func (s *InvestmentFundService) GetFundSharesValueRSD(ctx context.Context, fundID uint) (float64, error) {
	ownerships, err := s.ownershipRepo.FindByUserId(ctx, fundID, model.OwnerTypeFund)
	if err != nil {
		return 0, commonErrors.InternalErr(err)
	}

	var active []model.AssetOwnership
	var assetIDs []uint
	for _, o := range ownerships {
		if o.Amount > 0 {
			active = append(active, o)
			assetIDs = append(assetIDs, o.AssetID)
		}
	}

	if len(active) == 0 {
		return 0, nil
	}

	type assetMeta struct {
		assetType dto.AssetType
		listing   *model.Listing
	}
	meta := make(map[uint]assetMeta)

	stocks, err := s.stockRepo.FindByAssetIDs(ctx, assetIDs)
	if err != nil {
		return 0, commonErrors.InternalErr(err)
	}
	for _, st := range stocks {
		meta[st.AssetID] = assetMeta{
			assetType: dto.AssetTypeStock,
			listing:   st.Listing,
		}
	}

	options, err := s.optionRepo.FindByAssetIDs(ctx, assetIDs)
	if err != nil {
		return 0, commonErrors.InternalErr(err)
	}
	for _, op := range options {
		meta[op.AssetID] = assetMeta{assetType: dto.AssetTypeOption, listing: op.Listing}
	}

	futures, err := s.futuresRepo.FindByAssetIDs(ctx, assetIDs)
	if err != nil {
		return 0, commonErrors.InternalErr(err)
	}
	for _, fc := range futures {
		meta[fc.AssetID] = assetMeta{assetType: dto.AssetTypeFutures, listing: fc.Listing}
	}

	forexPairs, err := s.forexRepo.FindByAssetIDs(ctx, assetIDs)
	if err != nil {
		return 0, commonErrors.InternalErr(err)
	}
	for _, fp := range forexPairs {
		meta[fp.AssetID] = assetMeta{assetType: dto.AssetTypeForex, listing: fp.Listing}
	}

	result := 0.0

	for _, o := range active {
		m, known := meta[o.AssetID]
		if !known {
			continue
		}

		currentPrice := 0.0
		if m.listing != nil {
			currentPrice = m.listing.Price
		}

		if m.listing == nil || m.listing.Exchange == nil || m.listing.Exchange.Currency == "" {
			return 0, commonErrors.InternalErr(fmt.Errorf("user listing does not have valid exchange/currency code"))
		}

		currency := m.listing.Exchange.Currency

		currentPriceRSD, err := s.bankingClient.ConvertCurrency(ctx, currentPrice, currency, "RSD")
		if err != nil {
			return 0, commonErrors.InternalErr(err)
		}

		result += o.Amount * currentPriceRSD
	}

	return result, nil
}

func (s *InvestmentFundService) GetFundTotalInvestedRSD(ctx context.Context, fundID uint) (float64, error) {
	positions, err := s.positionRepo.FindByFund(ctx, fundID)
	if err != nil {
		return 0, commonErrors.InternalErr(err)
	}

	var total float64
	for _, pos := range positions {
		total += pos.TotalInvestedAmount
	}
	return total, nil
}

func (s *InvestmentFundService) GetClientFundPositions(ctx context.Context, clientID uint) ([]dto.FundPositionSummaryResponse, error) {
	positions, err := s.positionRepo.FindByClient(ctx, clientID, model.OwnerTypeClient)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	result := make([]dto.FundPositionSummaryResponse, len(positions))
	for i, pos := range positions {
		result[i] = dto.ToFundPositionSummaryResponse(pos)

		fundSharesValue, err := s.GetFundSharesValueRSD(ctx, pos.FundID)
		if err != nil {
			return nil, err
		}
		fundTotalInvested, err := s.GetFundTotalInvestedRSD(ctx, pos.FundID)
		if err != nil {
			return nil, err
		}

		if fundTotalInvested == 0 {
			result[i].ClientsSharePercent = 0
		} else {
			result[i].ClientsSharePercent = pos.TotalInvestedAmount / fundTotalInvested
		}
		result[i].ClientsShareValueRSD = result[i].ClientsSharePercent * fundSharesValue
		result[i].TotalProfit = result[i].ClientsShareValueRSD - pos.TotalInvestedAmount
	}

	return result, nil
}
