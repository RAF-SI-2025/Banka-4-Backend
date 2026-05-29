package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pkgerrors "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	pb "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
)

type PortfolioService struct {
	ownershipRepo repository.AssetOwnershipRepository
	stockRepo     repository.StockRepository
	optionRepo    repository.OptionRepository
	futuresRepo   repository.FuturesContractRepository
	forexRepo     repository.ForexRepository
	bankingClient client.BankingClient
	now           func() time.Time
	userClient    client.UserServiceClient
}

func NewPortfolioService(
	ownershipRepo repository.AssetOwnershipRepository,
	stockRepo repository.StockRepository,
	optionRepo repository.OptionRepository,
	futuresRepo repository.FuturesContractRepository,
	forexRepo repository.ForexRepository,
	bankingClient client.BankingClient,
	userClient client.UserServiceClient,
) *PortfolioService {
	return &PortfolioService{
		ownershipRepo: ownershipRepo,
		stockRepo:     stockRepo,
		optionRepo:    optionRepo,
		futuresRepo:   futuresRepo,
		forexRepo:     forexRepo,
		bankingClient: bankingClient,
		now:           time.Now,
		userClient:    userClient,
	}
}

func (s *PortfolioService) GetClientPortfolio(ctx context.Context, clientID uint) ([]dto.PortfolioAssetResponse, error) {
	ownerships, err := s.ownershipRepo.FindByUserId(ctx, clientID, model.OwnerTypeClient)
	if err != nil {
		return nil, pkgerrors.InternalErr(err)
	}

	return s.GetPortfolio(ctx, ownerships)
}

func (s *PortfolioService) GetActuaryPortfolio(ctx context.Context, actuaryID uint) ([]dto.PortfolioAssetResponse, error) {
	ownerships, err := s.ownershipRepo.FindByUserId(ctx, actuaryID, model.OwnerTypeActuary)
	if err != nil {
		return nil, pkgerrors.InternalErr(err)
	}

	return s.GetPortfolio(ctx, ownerships)
}

func (s *PortfolioService) GetWholeBankPortfolio(ctx context.Context, actuaryID uint) ([]dto.PortfolioAssetResponse, error) {
	ownerships, err := s.ownershipRepo.FindByOwnerType(ctx, model.OwnerTypeActuary)
	if err != nil {
		return nil, pkgerrors.InternalErr(err)
	}

	return s.GetPortfolio(ctx, ownerships)
}

func (s *PortfolioService) GetFundPortfolio(ctx context.Context, fundId uint) ([]dto.PortfolioAssetResponse, error) {
	ownerships, err := s.ownershipRepo.FindByUserId(ctx, fundId, model.OwnerTypeFund)
	if err != nil {
		return nil, pkgerrors.InternalErr(err)
	}

	return s.GetPortfolio(ctx, ownerships)
}

func (s *PortfolioService) GetAllActuaryProfits(ctx context.Context, page, pageSize int32, firstName, lastName string) (*dto.PaginatedActuaryProfitResponse, error) {
	resp, err := s.userClient.GetAllActuaries(ctx, page, pageSize, firstName, lastName)
	if err != nil {
		return nil, err
	}

	// 2. izvuci IDs
	ids := make([]uint64, 0, len(resp.Actuaries))
	for _, a := range resp.Actuaries {
		ids = append(ids, a.Id)
	}

	profitMap := make(map[uint64]float64)

	for _, id := range ids {
		assets, err := s.GetActuaryPortfolio(ctx, uint(id))
		if err != nil {
			return nil, err
		}

		var total float64
		for _, a := range assets {
			total += a.Profit
		}

		profitMap[id] = total
	}

	result := make([]dto.ActuaryProfitResponse, 0, len(resp.Actuaries))

	for _, a := range resp.Actuaries {
		profit := profitMap[a.Id]

		result = append(result, dto.ActuaryProfitResponse{
			FirstName: a.FirstName,
			LastName:  a.LastName,
			ProfitRSD: profit,
		})
	}

	return &dto.PaginatedActuaryProfitResponse{
		Data:     result,
		Total:    resp.Total,
		Page:     int(resp.Page),
		PageSize: int(resp.PageSize),
	}, nil
}

func (s *PortfolioService) GetPortfolio(ctx context.Context, ownerships []model.AssetOwnership) ([]dto.PortfolioAssetResponse, error) {
	// Filter to positive positions and collect asset IDs
	var active []model.AssetOwnership
	var assetIDs []uint
	for _, o := range ownerships {
		if o.Amount > 0 {
			active = append(active, o)
			assetIDs = append(assetIDs, o.AssetID)
		}
	}

	if len(active) == 0 {
		return []dto.PortfolioAssetResponse{}, nil
	}

	// Determine asset types; listing is preloaded on each asset type
	type assetMeta struct {
		assetType dto.AssetType
		listing   *model.Listing
	}
	meta := make(map[uint]assetMeta)
	optionData := make(map[uint]*dto.OptionSpecificAssetData)

	stocks, err := s.stockRepo.FindByAssetIDs(ctx, assetIDs)
	if err != nil {
		return nil, pkgerrors.InternalErr(err)
	}
	for _, st := range stocks {
		meta[st.AssetID] = assetMeta{
			assetType: dto.AssetTypeStock,
			listing:   st.Listing,
		}
	}

	options, err := s.optionRepo.FindByAssetIDs(ctx, assetIDs)
	if err != nil {
		return nil, pkgerrors.InternalErr(err)
	}
	for _, op := range options {
		meta[op.AssetID] = assetMeta{assetType: dto.AssetTypeOption, listing: op.Listing}
		optionData[op.AssetID] = &dto.OptionSpecificAssetData{
			StrikePrice:    op.StrikePrice,
			OptionType:     string(op.OptionType),
			SettlementDate: op.SettlementDate,
		}
	}

	futures, err := s.futuresRepo.FindByAssetIDs(ctx, assetIDs)
	if err != nil {
		return nil, pkgerrors.InternalErr(err)
	}
	for _, fc := range futures {
		meta[fc.AssetID] = assetMeta{assetType: dto.AssetTypeFutures, listing: fc.Listing}
	}

	forexPairs, err := s.forexRepo.FindByAssetIDs(ctx, assetIDs)
	if err != nil {
		return nil, pkgerrors.InternalErr(err)
	}
	for _, fp := range forexPairs {
		meta[fp.AssetID] = assetMeta{assetType: dto.AssetTypeForex, listing: fp.Listing}
	}

	var result []dto.PortfolioAssetResponse

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
			return nil, pkgerrors.InternalErr(fmt.Errorf("user listing does not have valid exchange/currency code"))
		}

		currency := m.listing.Exchange.Currency

		// Convert current price to RSD for consistent profit calculation
		currentPriceRSD, err := s.toRSD(ctx, currentPrice, currency)
		if err != nil {
			return nil, pkgerrors.InternalErr(err)
		}

		profit := (currentPriceRSD - o.AvgBuyPriceRSD) * o.Amount

		var ticker string
		if o.Asset.Ticker != "" {
			ticker = o.Asset.Ticker
		}

		result = append(result, dto.PortfolioAssetResponse{
			OwnershipID:     o.AssetOwnershipID,
			AssetID:         o.AssetID,
			Type:            m.assetType,
			Ticker:          ticker,
			Amount:          o.Amount,
			PricePerUnitRSD: currentPriceRSD,
			AvgBuyPriceRSD:  o.AvgBuyPriceRSD,
			LastModified:    o.UpdatedAt,
			Profit:          profit,
			PublicAmount:    o.PublicAmount,
			OptionData:      optionData[o.AssetID],
		})
	}

	return result, nil
}

func (s *PortfolioService) toRSD(ctx context.Context, amount float64, currency string) (float64, error) {
	if currency == "RSD" {
		return amount, nil
	}
	return s.bankingClient.ConvertCurrency(ctx, amount, currency, "RSD")
}

func (s *PortfolioService) ExerciseOption(ctx context.Context, userId uint, ownerType model.OwnerType, optionAssetID uint, accountNumber string) (*dto.ExerciseOptionResponse, error) {
	if ownerType != model.OwnerTypeActuary {
		return nil, pkgerrors.ForbiddenErr("only actuaries can exercise options")
	}

	account, err := s.bankingClient.GetAccountByNumber(ctx, accountNumber)
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			return nil, pkgerrors.NotFoundErr("account not found")
		}
		return nil, pkgerrors.ServiceUnavailableErr(err)
	}
	if account.AccountType != "Bank" {
		return nil, pkgerrors.BadRequestErr("employees must use a bank account")
	}

	ownerships, err := s.ownershipRepo.FindByUserId(ctx, userId, ownerType)
	if err != nil {
		return nil, pkgerrors.InternalErr(err)
	}

	optionOwnership := findOwnershipByAssetID(ownerships, optionAssetID)
	if optionOwnership == nil || optionOwnership.Amount <= 0 {
		return nil, pkgerrors.NotFoundErr("option ownership not found")
	}

	options, err := s.optionRepo.FindByAssetIDs(ctx, []uint{optionAssetID})
	if err != nil {
		return nil, pkgerrors.InternalErr(err)
	}
	if len(options) == 0 {
		return nil, pkgerrors.NotFoundErr("option not found")
	}

	option := options[0]
	if option.OptionType != model.OptionTypeCall {
		return nil, pkgerrors.BadRequestErr("only call options can be exercised")
	}

	if !option.SettlementDate.After(s.now()) {
		return nil, pkgerrors.BadRequestErr("cannot exercise an expired option")
	}

	if option.ContractSize <= 0 {
		return nil, pkgerrors.InternalErr(fmt.Errorf("option contract size must be positive"))
	}

	if option.Stock.AssetID == 0 || option.Stock.Listing == nil {
		return nil, pkgerrors.InternalErr(fmt.Errorf("underlying stock listing is not available"))
	}

	if option.Stock.Listing.Price <= option.StrikePrice {
		return nil, pkgerrors.BadRequestErr("option is not in the money")
	}

	heldContracts, err := exercisedContracts(optionOwnership.Amount, option.ContractSize)
	if err != nil {
		return nil, pkgerrors.BadRequestErr(err.Error())
	}
	exercisedContracts := uint(1)

	purchasedShares := float64(option.ContractSize)
	remainingShares := optionOwnership.Amount - purchasedShares
	totalCost := purchasedShares * option.StrikePrice

	stockCurrency, err := listingCurrency(option.Stock.Listing, option.Listing)
	if err != nil {
		return nil, pkgerrors.InternalErr(err)
	}

	settlement, err := s.bankingClient.ExecuteTradeSettlement(
		ctx,
		accountNumber,
		stockCurrency,
		pb.TradeSettlementDirection_TRADE_SETTLEMENT_DIRECTION_BUY,
		totalCost,
	)
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.NotFound:
				return nil, pkgerrors.NotFoundErr(st.Message())
			case codes.FailedPrecondition:
				return nil, pkgerrors.BadRequestErr(st.Message())
			}
		}
		return nil, pkgerrors.ServiceUnavailableErr(err)
	}

	strikePriceRSD, err := s.toRSD(ctx, option.StrikePrice, stockCurrency)
	if err != nil {
		return nil, pkgerrors.InternalErr(err)
	}

	stockOwnership := findOwnershipByAssetID(ownerships, option.Stock.AssetID)
	if stockOwnership == nil {
		stockOwnership = &model.AssetOwnership{
			UserId:    userId,
			OwnerType: ownerType,
			AssetID:   option.Stock.AssetID,
		}
	}

	newStockAmount := stockOwnership.Amount + purchasedShares
	if newStockAmount > 0 {
		stockOwnership.AvgBuyPriceRSD = (stockOwnership.AvgBuyPriceRSD*stockOwnership.Amount + strikePriceRSD*purchasedShares) / newStockAmount
	}
	stockOwnership.Amount = newStockAmount
	stockOwnership.UpdatedAt = s.now()

	optionOwnership.Amount = remainingShares
	optionOwnership.UpdatedAt = s.now()

	if err := s.ownershipRepo.Upsert(ctx, stockOwnership); err != nil {
		return nil, pkgerrors.InternalErr(err)
	}
	if err := s.ownershipRepo.Upsert(ctx, optionOwnership); err != nil {
		return nil, pkgerrors.InternalErr(err)
	}

	return &dto.ExerciseOptionResponse{
		OptionAssetID:           option.AssetID,
		StockAssetID:            option.Stock.AssetID,
		ExercisedContracts:      exercisedContracts,
		PurchasedShares:         purchasedShares,
		StrikePrice:             option.StrikePrice,
		TotalCost:               totalCost,
		RemainingOptionShares:   remainingShares,
		RemainingContracts:      heldContracts - exercisedContracts,
		SourceAmount:            settlement.GetSourceAmount(),
		SourceCurrencyCode:      settlement.GetSourceCurrencyCode(),
		DestinationAmount:       settlement.GetDestinationAmount(),
		DestinationCurrencyCode: settlement.GetDestinationCurrencyCode(),
	}, nil
}

func findOwnershipByAssetID(ownerships []model.AssetOwnership, assetID uint) *model.AssetOwnership {
	for i := range ownerships {
		if ownerships[i].AssetID == assetID {
			return &ownerships[i]
		}
	}

	return nil
}

func exercisedContracts(amount float64, contractSize int) (uint, error) {
	if contractSize <= 0 {
		return 0, fmt.Errorf("option contract size must be positive")
	}

	contracts := amount / float64(contractSize)
	if math.Abs(contracts-math.Round(contracts)) > 1e-9 {
		return 0, fmt.Errorf("option position amount is inconsistent with contract size")
	}

	return uint(math.Round(contracts)), nil
}

func listingCurrency(primary *model.Listing, fallback *model.Listing) (string, error) {
	if primary != nil && primary.Exchange != nil && primary.Exchange.Currency != "" {
		return normalizeCurrencyCode(primary.Exchange.Currency), nil
	}

	if fallback != nil && fallback.Exchange != nil && fallback.Exchange.Currency != "" {
		return normalizeCurrencyCode(fallback.Exchange.Currency), nil
	}

	return "", fmt.Errorf("listing does not have valid exchange/currency code")
}
