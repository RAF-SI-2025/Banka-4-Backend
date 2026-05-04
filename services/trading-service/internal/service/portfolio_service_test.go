package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
	"github.com/stretchr/testify/require"
)

var errTest = errors.New("repo error")

// --- Fake repos ---

type fakeAssetOwnershipRepo struct {
	ownerships     []model.AssetOwnership
	byID           *model.AssetOwnership
	findByIDErr    error
	allPublic      []model.AssetOwnership
	allPublicTotal int64
	allPublicErr   error
	updateOTCErr   error
	upserted       []*model.AssetOwnership
	upsertErr      error
	findErr        error
}

func (r *fakeAssetOwnershipRepo) FindByUserId(_ context.Context, _ uint, _ model.OwnerType) ([]model.AssetOwnership, error) {
	return r.ownerships, r.findErr
}

func (r *fakeAssetOwnershipRepo) Upsert(_ context.Context, ownership *model.AssetOwnership) error {
	if r.upsertErr != nil {
		return r.upsertErr
	}

	copy := *ownership
	r.upserted = append(r.upserted, &copy)
	return nil
}
func (r *fakeAssetOwnershipRepo) FindByID(_ context.Context, _ uint) (*model.AssetOwnership, error) {
	return r.byID, r.findByIDErr
}
func (r *fakeAssetOwnershipRepo) FindAllPublic(_ context.Context, _, _ int) ([]model.AssetOwnership, int64, error) {
	return r.allPublic, r.allPublicTotal, r.allPublicErr
}

func (r *fakeAssetOwnershipRepo) UpdateOTCFields(_ context.Context, _ uint, _, _ float64) error {
	return r.updateOTCErr
}

func (r *fakeAssetOwnershipRepo) IncreaseReservedAmount(_ context.Context, _ uint, _ model.OwnerType, _ uint, _ float64) error {
	return nil
}

type fakeStockRepo struct {
	stocks []model.Stock
	err    error
}

func (r *fakeStockRepo) Upsert(_ context.Context, _ *model.Stock) error   { return nil }
func (r *fakeStockRepo) FindAll(_ context.Context) ([]model.Stock, error) { return nil, nil }
func (r *fakeStockRepo) FindByAssetIDs(_ context.Context, _ []uint) ([]model.Stock, error) {
	return r.stocks, r.err
}
func (r *fakeStockRepo) Count(_ context.Context) (int64, error) { return 0, nil }

type fakeOptionRepo struct {
	options []model.Option
	err     error
}

func (r *fakeOptionRepo) Upsert(_ context.Context, _ *model.Option) error { return nil }
func (r *fakeOptionRepo) FindByAssetIDs(_ context.Context, _ []uint) ([]model.Option, error) {
	return r.options, r.err
}
func (r *fakeOptionRepo) FindByStockID(_ context.Context, _ uint) ([]model.Option, error) {
	return nil, nil
}

type fakeFuturesRepo struct {
	futures []model.FuturesContract
	err     error
}

func (r *fakeFuturesRepo) FindByAssetIDs(_ context.Context, _ []uint) ([]model.FuturesContract, error) {
	return r.futures, r.err
}

type fakeForexRepo struct {
	forex []model.ForexPair
	err   error
}

func (r *fakeForexRepo) FindByAssetIDs(_ context.Context, _ []uint) ([]model.ForexPair, error) {
	return r.forex, r.err
}
func (r *fakeForexRepo) Count(_ context.Context) (int64, error)            { return int64(len(r.forex)), nil }
func (r *fakeForexRepo) Upsert(_ context.Context, _ model.ForexPair) error { return nil }
func (r *fakeForexRepo) FindAll(_ context.Context, _ repository.ListingFilter) ([]model.ForexPair, int64, error) {
	return r.forex, int64(len(r.forex)), r.err
}

// --- Helpers ---

func makeOwnership(assetID uint, ticker string, amount, avgBuyPrice float64) model.AssetOwnership {
	return model.AssetOwnership{
		UserId:         1,
		OwnerType:      model.OwnerTypeClient,
		AssetID:        assetID,
		Asset:          model.Asset{AssetID: assetID, Ticker: ticker, AssetType: model.AssetTypeStock},
		Amount:         amount,
		AvgBuyPriceRSD: avgBuyPrice,
		UpdatedAt:      time.Now(),
	}
}

func makeListing(assetID uint, price float64) *model.Listing {
	return &model.Listing{
		ListingID: assetID,
		AssetID:   assetID,
		Price:     price,
		Exchange: &model.Exchange{
			Currency: "USD",
		},
	}
}

// --- Tests ---

func TestGetPortfolio_HappyPath_Stock(t *testing.T) {
	ownership := makeOwnership(10, "AAPL", 10, 100.0)
	ownership.PublicAmount = 5.0

	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{ownership}},
		&fakeStockRepo{stocks: []model.Stock{{StockID: 1, AssetID: 10, OutstandingShares: 1_000_000, Listing: makeListing(10, 150.0)}}},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeOrderBankingClient{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
	)

	result, err := svc.GetPortfolio(context.Background(), 1, model.OwnerTypeClient)
	require.NoError(t, err)
	require.Len(t, result, 1)

	a := result[0]
	require.Equal(t, uint(10), a.AssetID)
	require.Equal(t, dto.AssetTypeStock, a.Type)
	require.Equal(t, "AAPL", a.Ticker)
	require.Equal(t, float64(10), a.Amount)
	require.Equal(t, 150.0, a.PricePerUnitRSD)
	require.InDelta(t, (150.0-100.0)*10, a.Profit, 0.001)
	require.Equal(t, 5.0, a.PublicAmount)
}

func TestGetPortfolio_HappyPath_Option(t *testing.T) {
	ownership := makeOwnership(20, "MSFT220404C00180000", 200, 5.0)
	ownership.Asset.AssetType = model.AssetTypeOption

	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{ownership}},
		&fakeStockRepo{},
		&fakeOptionRepo{options: []model.Option{{OptionID: 1, AssetID: 20, Listing: makeListing(20, 8.0)}}},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeOrderBankingClient{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
	)

	result, err := svc.GetPortfolio(context.Background(), 1, model.OwnerTypeClient)
	require.NoError(t, err)
	require.Len(t, result, 1)

	a := result[0]
	require.Equal(t, uint(20), a.AssetID)
	require.Equal(t, dto.AssetTypeOption, a.Type)
	require.Equal(t, float64(200), a.Amount)
	require.InDelta(t, (8.0-5.0)*200, a.Profit, 0.001)
	require.Equal(t, 0.0, a.PublicAmount)
}

func TestGetPortfolio_HappyPath_Futures(t *testing.T) {
	ownership := makeOwnership(30, "CLJ22", 5, 200.0)
	ownership.Asset.AssetType = model.AssetTypeFuture

	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{ownership}},
		&fakeStockRepo{},
		&fakeOptionRepo{},
		&fakeFuturesRepo{futures: []model.FuturesContract{{FuturesContractID: 1, AssetID: 30, Listing: makeListing(30, 210.0)}}},
		&fakeForexRepo{},
		&fakeOrderBankingClient{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
	)

	result, err := svc.GetPortfolio(context.Background(), 1, model.OwnerTypeClient)
	require.NoError(t, err)
	require.Len(t, result, 1)

	a := result[0]
	require.Equal(t, uint(30), a.AssetID)
	require.Equal(t, dto.AssetTypeFutures, a.Type)
	require.Equal(t, float64(5), a.Amount)
	require.InDelta(t, (210.0-200.0)*5, a.Profit, 0.001)
}

func TestGetPortfolio_ZeroAmountFiltered(t *testing.T) {
	// Ownership with 0 amount should be excluded
	ownership := makeOwnership(10, "AAPL", 0, 100.0)

	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{ownership}},
		&fakeStockRepo{stocks: []model.Stock{{StockID: 1, AssetID: 10}}},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeOrderBankingClient{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
	)

	result, err := svc.GetPortfolio(context.Background(), 1, model.OwnerTypeClient)
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestGetPortfolio_NetAmountAfterSell(t *testing.T) {
	// After buying 10 and selling 10, net amount is 0 — position is filtered out
	ownership := makeOwnership(10, "AAPL", 0, 100.0)

	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{ownership}},
		&fakeStockRepo{stocks: []model.Stock{{StockID: 1, AssetID: 10, Listing: makeListing(10, 150.0)}}},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeOrderBankingClient{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
	)

	result, err := svc.GetPortfolio(context.Background(), 1, model.OwnerTypeClient)
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestGetPortfolio_PartialSell(t *testing.T) {
	// After buying 10 and selling 4, net amount is 6
	ownership := makeOwnership(10, "AAPL", 6, 100.0)

	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{ownership}},
		&fakeStockRepo{stocks: []model.Stock{{StockID: 1, AssetID: 10, Listing: makeListing(10, 150.0)}}},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeOrderBankingClient{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
	)

	result, err := svc.GetPortfolio(context.Background(), 1, model.OwnerTypeClient)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, float64(6), result[0].Amount)
}

func TestGetPortfolio_EmptyOwnerships(t *testing.T) {
	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{}},
		&fakeStockRepo{},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeOrderBankingClient{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
	)

	result, err := svc.GetPortfolio(context.Background(), 1, model.OwnerTypeClient)
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestGetPortfolio_RepoError(t *testing.T) {
	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{findErr: errTest},
		&fakeStockRepo{},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeOrderBankingClient{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
	)

	_, err := svc.GetPortfolio(context.Background(), 1, model.OwnerTypeClient)
	require.Error(t, err)
}

func TestGetPortfolio_NegativeProfit_NoTax(t *testing.T) {
	// Buy at 200, current price 150 -> negative profit, no tax
	ownership := makeOwnership(10, "AAPL", 20, 200.0)

	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{ownership}},
		&fakeStockRepo{stocks: []model.Stock{{StockID: 1, AssetID: 10, Listing: makeListing(10, 150.0)}}},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeOrderBankingClient{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
	)

	result, err := svc.GetPortfolio(context.Background(), 1, model.OwnerTypeClient)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.InDelta(t, (150.0-200.0)*20, result[0].Profit, 0.001)
	require.Equal(t, float64(20), result[0].Amount)
}

func TestGetPortfolio_MultipleAssets_ProfitAccumulation(t *testing.T) {
	// Two stocks with different profit/loss — verifies values the handler will sum.
	// AAPL: buy 10 @ 100, now 150 -> profit = +500
	// MSFT: buy 5  @ 300, now 250 -> profit = -250
	// Expected total if summed: +250
	ownershipA := makeOwnership(10, "AAPL", 10, 100.0)
	ownershipB := makeOwnership(20, "MSFT", 5, 300.0)
	ownershipB.Asset.AssetID = 20
	ownershipB.Asset.Ticker = "MSFT"

	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{ownershipA, ownershipB}},
		&fakeStockRepo{stocks: []model.Stock{
			{StockID: 1, AssetID: 10, Listing: makeListing(10, 150.0)},
			{StockID: 2, AssetID: 20, Listing: makeListing(20, 250.0)},
		}},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeOrderBankingClient{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
	)

	result, err := svc.GetPortfolio(context.Background(), 1, model.OwnerTypeClient)
	require.NoError(t, err)
	require.Len(t, result, 2)

	var total float64
	for _, a := range result {
		total += a.Profit
	}
	require.InDelta(t, 250.0, total, 0.001)
}

func TestGetPortfolio_EmptyPortfolio_ZeroProfit(t *testing.T) {
	// Empty ownership list -> GetPortfolio returns empty -> handler sums to 0
	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{}},
		&fakeStockRepo{},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeOrderBankingClient{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
	)

	result, err := svc.GetPortfolio(context.Background(), 1, model.OwnerTypeActuary)
	require.NoError(t, err)
	require.Empty(t, result)

	var total float64
	for _, a := range result {
		total += a.Profit
	}
	require.InDelta(t, 0.0, total, 0.001)
}

func TestExerciseOption_Success(t *testing.T) {
	optionOwnership := makeOwnership(20, "AAPL:CALL:150.00", 200, 12.0)
	optionOwnership.OwnerType = model.OwnerTypeActuary
	optionOwnership.Asset.AssetType = model.AssetTypeOption

	ownershipRepo := &fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{optionOwnership}}
	optionRepo := &fakeOptionRepo{options: []model.Option{
		{
			AssetID:        20,
			OptionType:     model.OptionTypeCall,
			StrikePrice:    150,
			ContractSize:   100,
			SettlementDate: time.Now().Add(24 * time.Hour),
			Listing:        makeListing(20, 15),
			Stock: model.Stock{
				AssetID: 10,
				Asset:   model.Asset{AssetID: 10, Ticker: "AAPL", AssetType: model.AssetTypeStock},
				Listing: makeListing(10, 190),
			},
		},
	}}
	bankingClient := &fakeOrderBankingClient{
		accountResp: &pb.GetAccountByNumberResponse{
			AccountNumber:    "444000100000000001",
			AccountType:      "Bank",
			CurrencyCode:     "USD",
			AvailableBalance: 1_000_000,
		},
		settlementResp: &pb.ExecuteTradeSettlementResponse{
			SourceAmount:            30000,
			SourceCurrencyCode:      "USD",
			DestinationAmount:       30000,
			DestinationCurrencyCode: "USD",
		},
	}

	svc := NewPortfolioService(
		ownershipRepo,
		&fakeStockRepo{},
		optionRepo,
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		bankingClient,
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
	)
	svc.now = func() time.Time { return time.Date(2025, 6, 4, 10, 0, 0, 0, time.UTC) }

	resp, err := svc.ExerciseOption(context.Background(), 1, model.OwnerTypeActuary, 20, "444000100000000001")
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, uint(20), resp.OptionAssetID)
	require.Equal(t, uint(10), resp.StockAssetID)
	require.Equal(t, uint(1), resp.ExercisedContracts)
	require.Equal(t, 100.0, resp.PurchasedShares)
	require.Equal(t, 15000.0, resp.TotalCost)
	require.Equal(t, uint(1), resp.RemainingContracts)
	require.Equal(t, 100.0, resp.RemainingOptionShares)
	require.Len(t, ownershipRepo.upserted, 2)

	var stockOwnership *model.AssetOwnership
	var optionOwnershipUpdate *model.AssetOwnership
	for _, ownership := range ownershipRepo.upserted {
		switch ownership.AssetID {
		case 10:
			stockOwnership = ownership
		case 20:
			optionOwnershipUpdate = ownership
		}
	}

	require.NotNil(t, stockOwnership)
	require.Equal(t, 100.0, stockOwnership.Amount)
	require.Equal(t, 150.0, stockOwnership.AvgBuyPriceRSD)

	require.NotNil(t, optionOwnershipUpdate)
	require.Equal(t, 100.0, optionOwnershipUpdate.Amount)
}

func TestExerciseOption_ExpiredOption(t *testing.T) {
	optionOwnership := makeOwnership(20, "AAPL:CALL:150.00", 100, 12.0)
	optionOwnership.OwnerType = model.OwnerTypeActuary
	optionOwnership.Asset.AssetType = model.AssetTypeOption

	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{optionOwnership}},
		&fakeStockRepo{},
		&fakeOptionRepo{options: []model.Option{
			{
				AssetID:        20,
				OptionType:     model.OptionTypeCall,
				StrikePrice:    150,
				ContractSize:   100,
				SettlementDate: time.Date(2025, 6, 3, 10, 0, 0, 0, time.UTC),
				Stock: model.Stock{
					AssetID: 10,
					Listing: makeListing(10, 190),
				},
			},
		}},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeOrderBankingClient{accountResp: &pb.GetAccountByNumberResponse{AccountType: "Bank"}},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
	)
	svc.now = func() time.Time { return time.Date(2025, 6, 4, 10, 0, 0, 0, time.UTC) }

	_, err := svc.ExerciseOption(context.Background(), 1, model.OwnerTypeActuary, 20, "444000100000000001")
	require.Error(t, err)
	require.Contains(t, err.Error(), "expired option")
}

func TestExerciseOption_NotInTheMoney(t *testing.T) {
	optionOwnership := makeOwnership(20, "AAPL:CALL:150.00", 100, 12.0)
	optionOwnership.OwnerType = model.OwnerTypeActuary
	optionOwnership.Asset.AssetType = model.AssetTypeOption

	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{optionOwnership}},
		&fakeStockRepo{},
		&fakeOptionRepo{options: []model.Option{
			{
				AssetID:        20,
				OptionType:     model.OptionTypeCall,
				StrikePrice:    150,
				ContractSize:   100,
				SettlementDate: time.Now().Add(24 * time.Hour),
				Stock: model.Stock{
					AssetID: 10,
					Listing: makeListing(10, 140),
				},
			},
		}},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeOrderBankingClient{accountResp: &pb.GetAccountByNumberResponse{AccountType: "Bank"}},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
	)

	_, err := svc.ExerciseOption(context.Background(), 1, model.OwnerTypeActuary, 20, "444000100000000001")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not in the money")
}

func TestExerciseOption_PutOptionRejected(t *testing.T) {
	optionOwnership := makeOwnership(20, "AAPL:PUT:150.00", 100, 12.0)
	optionOwnership.OwnerType = model.OwnerTypeActuary
	optionOwnership.Asset.AssetType = model.AssetTypeOption

	svc := NewPortfolioService(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{optionOwnership}},
		&fakeStockRepo{},
		&fakeOptionRepo{options: []model.Option{
			{
				AssetID:        20,
				OptionType:     model.OptionTypePut,
				StrikePrice:    150,
				ContractSize:   100,
				SettlementDate: time.Now().Add(24 * time.Hour),
				Stock: model.Stock{
					AssetID: 10,
					Listing: makeListing(10, 140),
				},
			},
		}},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeOrderBankingClient{accountResp: &pb.GetAccountByNumberResponse{AccountType: "Bank"}},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
	)

	_, err := svc.ExerciseOption(context.Background(), 1, model.OwnerTypeActuary, 20, "444000100000000001")
	require.Error(t, err)
	require.Contains(t, err.Error(), "only call options can be exercised")
}

// --- GetClientPortfolio / GetActuaryPortfolio ---

func newPortfolioSvc(ownershipRepo *fakeAssetOwnershipRepo, stockRepo *fakeStockRepo, userClient *fakeUserServiceClient) *PortfolioService {
	return NewPortfolioService(ownershipRepo, stockRepo, &fakeOptionRepo{}, &fakeFuturesRepo{}, &fakeForexRepo{}, &fakeOrderBankingClient{}, userClient)
}

func TestGetClientPortfolio_ResolvesIdentityID(t *testing.T) {
	const clientID = uint64(5)
	const identityID = uint64(42)
	ownership := makeOwnership(10, "AAPL", 10, 100.0)
	ownership.UserId = uint(identityID)

	svc := newPortfolioSvc(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{ownership}},
		&fakeStockRepo{stocks: []model.Stock{{StockID: 1, AssetID: 10, OutstandingShares: 500, Listing: makeListing(10, 150.0)}}},
		&fakeUserServiceClient{clientResp: &pb.GetClientByIdResponse{Id: clientID, IdentityId: identityID}},
	)

	result, err := svc.GetClientPortfolio(context.Background(), uint(clientID))
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, "AAPL", result[0].Ticker)
}

func TestGetClientPortfolio_ClientNotFound(t *testing.T) {
	svc := newPortfolioSvc(
		&fakeAssetOwnershipRepo{},
		&fakeStockRepo{},
		&fakeUserServiceClient{},
	)

	result, err := svc.GetClientPortfolio(context.Background(), 999)
	require.NoError(t, err)
	require.Empty(t, result)
}

func TestGetActuaryPortfolio_ResolvesIdentityID(t *testing.T) {
	const actuaryID = uint64(7)
	const identityID = uint64(55)
	ownership := makeOwnership(20, "MSFT", 5, 200.0)
	ownership.UserId = uint(identityID)
	ownership.OwnerType = model.OwnerTypeActuary

	svc := newPortfolioSvc(
		&fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{ownership}},
		&fakeStockRepo{stocks: []model.Stock{{StockID: 2, AssetID: 20, OutstandingShares: 1000, Listing: makeListing(20, 250.0)}}},
		&fakeUserServiceClient{employeeResp: &pb.GetEmployeeByIdResponse{Id: actuaryID, IdentityId: identityID}},
	)

	result, err := svc.GetActuaryPortfolio(context.Background(), uint(actuaryID))
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, "MSFT", result[0].Ticker)
}

func TestGetActuaryPortfolio_ActuaryNotFound(t *testing.T) {
	svc := newPortfolioSvc(
		&fakeAssetOwnershipRepo{},
		&fakeStockRepo{},
		&fakeUserServiceClient{},
	)

	result, err := svc.GetActuaryPortfolio(context.Background(), 99)
	require.NoError(t, err)
	require.Empty(t, result)
}
