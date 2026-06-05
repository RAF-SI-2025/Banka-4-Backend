package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/config"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
	"github.com/stretchr/testify/require"
)

// ── Fake DividendPayout Repository ────────────────────────────────

type fakeDividendRepo struct {
	saved              []*model.DividendPayout
	saveErr            error
	allPayouts         []model.DividendPayout
	findAllErr         error
	ownershipPayouts   []model.DividendPayout
	findByOwnershipErr error
}

func (f *fakeDividendRepo) Save(_ context.Context, p *model.DividendPayout) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append(f.saved, p)
	return nil
}

func (f *fakeDividendRepo) FindAll(_ context.Context) ([]model.DividendPayout, error) {
	return f.allPayouts, f.findAllErr
}

func (f *fakeDividendRepo) FindAllByAssetOwnershipID(_ context.Context, _ uint) ([]model.DividendPayout, error) {
	return f.ownershipPayouts, f.findByOwnershipErr
}

// ── Fake AssetOwnership Repository ────────────────────────────────

type fakeDividendOwnershipRepo struct {
	ownerships []model.AssetOwnership
	findErr    error
}

func (f *fakeDividendOwnershipRepo) FindByUserId(_ context.Context, _ uint, _ model.OwnerType) ([]model.AssetOwnership, error) {
	return nil, nil
}
func (f *fakeDividendOwnershipRepo) FindByID(_ context.Context, _ uint) (*model.AssetOwnership, error) {
	return nil, nil
}
func (f *fakeDividendOwnershipRepo) FindByUserAndAsset(_ context.Context, _ uint, _ model.OwnerType, _ uint) (*model.AssetOwnership, error) {
	return nil, nil
}
func (f *fakeDividendOwnershipRepo) FindByUserAndAssetForUpdate(_ context.Context, _ uint, _ model.OwnerType, _ uint) (*model.AssetOwnership, error) {
	return nil, nil
}
func (f *fakeDividendOwnershipRepo) Upsert(_ context.Context, _ *model.AssetOwnership) error {
	return nil
}
func (f *fakeDividendOwnershipRepo) IncreaseReservedAmount(_ context.Context, _ uint, _ model.OwnerType, _ uint, _ float64) error {
	return nil
}
func (f *fakeDividendOwnershipRepo) FindAllPublic(_ context.Context, _, _ int) ([]model.AssetOwnership, int64, error) {
	return nil, 0, nil
}
func (f *fakeDividendOwnershipRepo) UpdateOTCFields(_ context.Context, _ uint, _, _ float64) error {
	return nil
}
func (f *fakeDividendOwnershipRepo) FindAllByAssetIDs(_ context.Context, _ []uint) ([]model.AssetOwnership, error) {
	return f.ownerships, f.findErr
}
func (f *fakeDividendOwnershipRepo) FindByOwnerType(_ context.Context, _ model.OwnerType) ([]model.AssetOwnership, error) {
	return f.ownerships, f.findErr
}

// ── Fake Stock Repository ──────────────────────────────────────────

type fakeDividendStockRepo struct {
	stocks  []model.Stock
	findErr error
}

func (f *fakeDividendStockRepo) FindAll(_ context.Context) ([]model.Stock, error) {
	return f.stocks, f.findErr
}
func (f *fakeDividendStockRepo) Upsert(_ context.Context, _ *model.Stock) error { return nil }
func (f *fakeDividendStockRepo) FindByAssetIDs(_ context.Context, _ []uint) ([]model.Stock, error) {
	return nil, nil
}
func (f *fakeDividendStockRepo) Count(_ context.Context) (int64, error) { return 0, nil }

// ── Fake Listing Repository ────────────────────────────────────────

type fakeDividendListingRepo struct{}

func (f *fakeDividendListingRepo) FindAll(_ context.Context) ([]model.Listing, error) {
	return nil, nil
}
func (f *fakeDividendListingRepo) FindStocks(_ context.Context, _ repository.ListingFilter) ([]model.Listing, int64, error) {
	return nil, 0, nil
}
func (f *fakeDividendListingRepo) FindFutures(_ context.Context, _ repository.ListingFilter) ([]model.Listing, int64, error) {
	return nil, 0, nil
}
func (f *fakeDividendListingRepo) FindOptions(_ context.Context, _ repository.ListingFilter) ([]model.Listing, int64, error) {
	return nil, 0, nil
}
func (f *fakeDividendListingRepo) FindByID(_ context.Context, _ uint, _ int) (*model.Listing, error) {
	return nil, nil
}
func (f *fakeDividendListingRepo) FindLatestDailyPriceInfo(_ context.Context, _ uint) (*model.ListingDailyPriceInfo, error) {
	return nil, nil
}
func (f *fakeDividendListingRepo) Upsert(_ context.Context, _ *model.Listing) error { return nil }
func (f *fakeDividendListingRepo) UpdatePriceAndAsk(_ context.Context, _ *model.Listing, _, _ float64) error {
	return nil
}
func (f *fakeDividendListingRepo) Count(_ context.Context) (int64, error) { return 0, nil }
func (f *fakeDividendListingRepo) CreateDailyPriceInfo(_ context.Context, _ *model.ListingDailyPriceInfo) error {
	return nil
}
func (f *fakeDividendListingRepo) FindLastDailyPriceInfo(_ context.Context, _ uint, _ time.Time) (*model.ListingDailyPriceInfo, error) {
	return nil, nil
}
func (f *fakeDividendListingRepo) FindByAssetType(_ context.Context, _ model.AssetType) ([]model.Listing, error) {
	return nil, nil
}
func (f *fakeDividendListingRepo) FindByAssetIDs(_ context.Context, _ []uint) ([]model.Listing, error) {
	return nil, nil
}

// ── Helpers ────────────────────────────────────────────────────────

// ── Helpers ────────────────────────────────────────────────────────────

func newTestDividendService(
	dividendRepo *fakeDividendRepo,
	ownershipRepo *fakeDividendOwnershipRepo,
	stockRepo *fakeDividendStockRepo,
	banking *fakeBankingClient,
) *DividendPayoutService {
	return newTestDividendServiceWithFund(dividendRepo, ownershipRepo, stockRepo, banking, nil, nil)
}

func newTestDividendServiceWithFund(
	dividendRepo *fakeDividendRepo,
	ownershipRepo *fakeDividendOwnershipRepo,
	stockRepo *fakeDividendStockRepo,
	banking *fakeBankingClient,
	fundRepo *fakeFundRepo,
	positionRepo *fakePositionRepo,
) *DividendPayoutService {
	taxRepo := &fakeTaxRepo{}
	taxSvc := NewTaxService(taxRepo, banking, &config.Configuration{
		TaxAccountNumber: "444000000000000000",
	}, fakeAuditService(nil))

	return NewDividendPayoutService(
		dividendRepo,
		ownershipRepo,
		stockRepo,
		&fakeDividendListingRepo{},
		taxSvc,
		banking,
		&config.Configuration{
			DividendAccountNumber: "444000000000000099",
		},
		fundRepo,
		positionRepo,
		nil, // orderService — not needed for basic fund dividend tests
	)
}

func makeDividendStock(dividendYield float64, price float64) model.Stock {
	return model.Stock{
		StockID:       1,
		AssetID:       10,
		DividendYield: dividendYield,
		Asset:         model.Asset{Ticker: "AAPL"},
		Listing: &model.Listing{
			AssetID:     10,
			Price:       price,
			ExchangeMIC: model.SimulatedExchangeMIC,
		},
	}
}

func makeDividendOwnership(userID uint, ownerType model.OwnerType, amount float64) model.AssetOwnership {
	return model.AssetOwnership{
		UserId:    userID,
		OwnerType: ownerType,
		AssetID:   10,
		Amount:    amount,
	}
}

// ── ProcessDividends Tests ─────────────────────────────────────────

func TestProcessDividends_SkipsZeroDividendYield(t *testing.T) {
	stockRepo := &fakeDividendStockRepo{
		stocks: []model.Stock{makeDividendStock(0, 100)},
	}
	dividendRepo := &fakeDividendRepo{}
	svc := newTestDividendService(dividendRepo, &fakeDividendOwnershipRepo{}, stockRepo, &fakeBankingClient{})

	err := svc.ProcessDividends(context.Background())
	require.NoError(t, err)
	require.Empty(t, dividendRepo.saved)
}

func TestProcessDividends_SkipsStockWithNoListing(t *testing.T) {
	stock := model.Stock{
		StockID:       2,
		AssetID:       20,
		DividendYield: 0.04,
		Asset:         model.Asset{Ticker: "MSFT"},
		Listing:       nil,
	}
	stockRepo := &fakeDividendStockRepo{stocks: []model.Stock{stock}}
	dividendRepo := &fakeDividendRepo{}
	svc := newTestDividendService(dividendRepo, &fakeDividendOwnershipRepo{}, stockRepo, &fakeBankingClient{})

	err := svc.ProcessDividends(context.Background())
	require.NoError(t, err)
	require.Empty(t, dividendRepo.saved)
}

func TestProcessDividends_StockRepoError(t *testing.T) {
	stockRepo := &fakeDividendStockRepo{findErr: errors.New("db error")}
	svc := newTestDividendService(&fakeDividendRepo{}, &fakeDividendOwnershipRepo{}, stockRepo, &fakeBankingClient{})

	err := svc.ProcessDividends(context.Background())
	require.Error(t, err)
}

func TestProcessDividends_PaysSingleClientOwner(t *testing.T) {
	stock := makeDividendStock(4, 100.0)
	ownership := makeDividendOwnership(1, model.OwnerTypeClient, 50)

	stockRepo := &fakeDividendStockRepo{stocks: []model.Stock{stock}}
	ownershipRepo := &fakeDividendOwnershipRepo{ownerships: []model.AssetOwnership{ownership}}
	dividendRepo := &fakeDividendRepo{}
	banking := &fakeBankingClient{
		accountsResp: &pb.GetAccountsByClientIDResponse{
			Accounts: []*pb.AccountInfo{{AccountNumber: "444000000000000001"}},
		},
		paymentResp:     &pb.CreatePaymentResponse{},
		convertedAmount: 1.0,
	}

	svc := newTestDividendService(dividendRepo, ownershipRepo, stockRepo, banking)
	err := svc.ProcessDividends(context.Background())

	require.NoError(t, err)
	require.Len(t, dividendRepo.saved, 1)

	payout := dividendRepo.saved[0]
	require.Equal(t, ownership.AssetOwnershipID, payout.AssetOwnershipID)
	require.InDelta(t, 50.0, payout.GrossAmount, 0.001)
	require.InDelta(t, 50.0*0.15, payout.TaxAmount, 0.001)
	require.InDelta(t, 50.0*0.85, payout.NetAmount, 0.001)
}

func TestProcessDividends_ActuaryPaysNoTax(t *testing.T) {
	stock := makeDividendStock(4, 100.0)
	ownership := makeDividendOwnership(2, model.OwnerTypeBank, 50)
	stockRepo := &fakeDividendStockRepo{stocks: []model.Stock{stock}}
	ownershipRepo := &fakeDividendOwnershipRepo{ownerships: []model.AssetOwnership{ownership}}
	dividendRepo := &fakeDividendRepo{}
	banking := &fakeBankingClient{
		accountsResp: &pb.GetAccountsByClientIDResponse{
			Accounts: []*pb.AccountInfo{{AccountNumber: "444000000000000002"}},
		},
		paymentResp:     &pb.CreatePaymentResponse{},
		convertedAmount: 1.0,
	}

	svc := newTestDividendService(dividendRepo, ownershipRepo, stockRepo, banking)
	err := svc.ProcessDividends(context.Background())

	require.NoError(t, err)
	require.Len(t, dividendRepo.saved, 1)

	payout := dividendRepo.saved[0]
	require.Equal(t, 0.0, payout.TaxAmount)
	require.Equal(t, payout.GrossAmount, payout.NetAmount)
}

func TestProcessDividends_DividendFormula(t *testing.T) {
	// Formula: Quantity × Price × (DividendYield / 4)
	// 100 × 200 × (0.08 / 4) = 100 × 200 × 0.02 = 400
	stock := makeDividendStock(8, 200.0)
	ownership := makeDividendOwnership(3, model.OwnerTypeClient, 100)
	stockRepo := &fakeDividendStockRepo{stocks: []model.Stock{stock}}
	ownershipRepo := &fakeDividendOwnershipRepo{ownerships: []model.AssetOwnership{ownership}}
	dividendRepo := &fakeDividendRepo{}
	banking := &fakeBankingClient{
		accountsResp: &pb.GetAccountsByClientIDResponse{
			Accounts: []*pb.AccountInfo{{AccountNumber: "444000000000000003"}},
		},
		paymentResp:     &pb.CreatePaymentResponse{},
		convertedAmount: 1.0,
	}

	svc := newTestDividendService(dividendRepo, ownershipRepo, stockRepo, banking)
	err := svc.ProcessDividends(context.Background())

	require.NoError(t, err)
	require.Len(t, dividendRepo.saved, 1)
	require.InDelta(t, 400.0, dividendRepo.saved[0].GrossAmount, 0.001)
}

func TestProcessDividends_SkipsZeroAmountOwnership(t *testing.T) {
	stock := makeDividendStock(4, 100.0)
	ownership := makeDividendOwnership(4, model.OwnerTypeClient, 0)
	stockRepo := &fakeDividendStockRepo{stocks: []model.Stock{stock}}
	ownershipRepo := &fakeDividendOwnershipRepo{ownerships: []model.AssetOwnership{ownership}}
	dividendRepo := &fakeDividendRepo{}

	svc := newTestDividendService(dividendRepo, ownershipRepo, stockRepo, &fakeBankingClient{})
	err := svc.ProcessDividends(context.Background())

	require.NoError(t, err)
	require.Empty(t, dividendRepo.saved) // gross=0, ne ulazi u payOwner
}

func TestProcessDividends_NoAccountSkipsPayout(t *testing.T) {
	stock := makeDividendStock(4, 100.0)
	ownership := makeDividendOwnership(4, model.OwnerTypeClient, 50)
	stockRepo := &fakeDividendStockRepo{stocks: []model.Stock{stock}}
	ownershipRepo := &fakeDividendOwnershipRepo{ownerships: []model.AssetOwnership{ownership}}
	dividendRepo := &fakeDividendRepo{}
	banking := &fakeBankingClient{
		accountsResp:    &pb.GetAccountsByClientIDResponse{Accounts: []*pb.AccountInfo{}},
		convertedAmount: 1.0,
	}

	svc := newTestDividendService(dividendRepo, ownershipRepo, stockRepo, banking)
	err := svc.ProcessDividends(context.Background())

	require.NoError(t, err)
	require.Empty(t, dividendRepo.saved)
}

func TestProcessDividends_PaymentFailureDoesNotStopOtherOwners(t *testing.T) {
	stock := makeDividendStock(4, 100.0)
	ownerships := []model.AssetOwnership{
		makeDividendOwnership(5, model.OwnerTypeClient, 50),
		makeDividendOwnership(6, model.OwnerTypeClient, 50),
	}

	stockRepo := &fakeDividendStockRepo{stocks: []model.Stock{stock}}
	ownershipRepo := &fakeDividendOwnershipRepo{ownerships: ownerships}
	dividendRepo := &fakeDividendRepo{}
	banking := &fakeBankingClient{
		accountsResp: &pb.GetAccountsByClientIDResponse{
			Accounts: []*pb.AccountInfo{{AccountNumber: "444000000000000005"}},
		},
		paymentErr: errors.New("payment failed"),
	}

	svc := newTestDividendService(dividendRepo, ownershipRepo, stockRepo, banking)
	err := svc.ProcessDividends(context.Background())

	require.NoError(t, err)
}

func TestProcessDividends_SaveFailureIsNonFatal(t *testing.T) {
	stock := makeDividendStock(4, 100.0)
	ownership := makeDividendOwnership(7, model.OwnerTypeClient, 50)
	stockRepo := &fakeDividendStockRepo{stocks: []model.Stock{stock}}
	ownershipRepo := &fakeDividendOwnershipRepo{ownerships: []model.AssetOwnership{ownership}}
	dividendRepo := &fakeDividendRepo{saveErr: errors.New("db write failed")}
	banking := &fakeBankingClient{
		accountsResp: &pb.GetAccountsByClientIDResponse{
			Accounts: []*pb.AccountInfo{{AccountNumber: "444000000000000007"}},
		},
		paymentResp:     &pb.CreatePaymentResponse{},
		convertedAmount: 1.0,
	}

	svc := newTestDividendService(dividendRepo, ownershipRepo, stockRepo, banking)
	err := svc.ProcessDividends(context.Background())
	require.NoError(t, err)
}

// ── GetAllPayouts / GetPayoutsForAssetOwnership Tests ──────────────

func TestGetAllPayouts_Success(t *testing.T) {
	payouts := []model.DividendPayout{
		{DividendPayoutID: 1, AssetOwnershipID: 1, PaymentDate: time.Now()},
		{DividendPayoutID: 2, AssetOwnershipID: 2, PaymentDate: time.Now()},
	}
	dividendRepo := &fakeDividendRepo{allPayouts: payouts}
	svc := newTestDividendService(dividendRepo, &fakeDividendOwnershipRepo{}, &fakeDividendStockRepo{}, &fakeBankingClient{})

	result, err := svc.GetAllPayouts(context.Background())
	require.NoError(t, err)
	require.Len(t, result, 2)
}

func TestGetAllPayouts_RepoError(t *testing.T) {
	dividendRepo := &fakeDividendRepo{findAllErr: errors.New("db error")}
	svc := newTestDividendService(dividendRepo, &fakeDividendOwnershipRepo{}, &fakeDividendStockRepo{}, &fakeBankingClient{})

	result, err := svc.GetAllPayouts(context.Background())
	require.Error(t, err)
	require.Nil(t, result)
}

func TestGetPayoutsForAssetOwnership_Success(t *testing.T) {
	payouts := []model.DividendPayout{
		{DividendPayoutID: 1, AssetOwnershipID: 42},
	}
	dividendRepo := &fakeDividendRepo{ownershipPayouts: payouts}
	svc := newTestDividendService(dividendRepo, &fakeDividendOwnershipRepo{}, &fakeDividendStockRepo{}, &fakeBankingClient{})

	result, err := svc.GetPayoutsForAssetOwnership(context.Background(), 42)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, uint(42), result[0].AssetOwnershipID)
}

func TestGetPayoutsForAssetOwnership_RepoError(t *testing.T) {
	dividendRepo := &fakeDividendRepo{findByOwnershipErr: errors.New("db error")}
	svc := newTestDividendService(dividendRepo, &fakeDividendOwnershipRepo{}, &fakeDividendStockRepo{}, &fakeBankingClient{})

	result, err := svc.GetPayoutsForAssetOwnership(context.Background(), 42)
	require.Error(t, err)
	require.Nil(t, result)
}

// ── Fund Dividend Tests ────────────────────────────────────────────────

func makeFundWithReinvestPct(pct float64) *model.InvestmentFund {
	return &model.InvestmentFund{
		FundID:                      42,
		Name:                        "TestFund",
		AccountNumber:               "444000099999000001",
		ManagerID:                   10,
		DividendReinvestmentPercent: &pct,
	}
}

func TestProcessDividends_FundReceivesGrossDividend(t *testing.T) {
	// Fund owns 100 shares of a stock at price 100, DividendYield=4%
	// gross = 100 * 100 * (4/400) = 100
	stock := makeDividendStock(4, 100.0)
	ownership := model.AssetOwnership{
		UserId:    42, // FundID
		OwnerType: model.OwnerTypeFund,
		AssetID:   10,
		Amount:    100,
	}
	stockRepo := &fakeDividendStockRepo{stocks: []model.Stock{stock}}
	ownershipRepo := &fakeDividendOwnershipRepo{ownerships: []model.AssetOwnership{ownership}}
	dividendRepo := &fakeDividendRepo{}
	banking := &fakeBankingClient{
		accountsResp:    &pb.GetAccountsByClientIDResponse{Accounts: []*pb.AccountInfo{{AccountNumber: "444000100000000001"}}},
		paymentResp:     &pb.CreatePaymentResponse{},
		convertedAmount: 1.0,
	}
	fundRepo := &fakeFundRepo{findByIDResult: makeFundWithReinvestPct(50)}
	positionRepo := &fakePositionRepo{findByFundRes: []model.ClientFundPosition{}}

	svc := newTestDividendServiceWithFund(dividendRepo, ownershipRepo, stockRepo, banking, fundRepo, positionRepo)
	err := svc.ProcessDividends(context.Background())

	require.NoError(t, err)
	require.Len(t, dividendRepo.saved, 1)
	payout := dividendRepo.saved[0]
	require.InDelta(t, 100.0, payout.GrossAmount, 0.001)
	require.Equal(t, 0.0, payout.TaxAmount)
	require.Equal(t, payout.GrossAmount, payout.NetAmount)
	require.Equal(t, "444000099999000001", payout.AccountNumber)
}

func TestProcessDividends_FundDistributesToClients(t *testing.T) {
	// Gross = 400.  Reinvest 50% → 200.  Payout 50% → 200 to one client.
	stock := makeDividendStock(8, 200.0) // 100 shares * 200 * (8/400) = 400
	ownership := model.AssetOwnership{
		UserId:    42,
		OwnerType: model.OwnerTypeFund,
		AssetID:   10,
		Amount:    100,
	}
	stockRepo := &fakeDividendStockRepo{stocks: []model.Stock{stock}}
	ownershipRepo := &fakeDividendOwnershipRepo{ownerships: []model.AssetOwnership{ownership}}
	dividendRepo := &fakeDividendRepo{}
	banking := &fakeBankingClient{
		accountsResp:    &pb.GetAccountsByClientIDResponse{Accounts: []*pb.AccountInfo{{AccountNumber: "444000100000000001"}}},
		paymentResp:     &pb.CreatePaymentResponse{},
		convertedAmount: 1.0,
	}
	clientUnits := 10.0
	positionRepo := &fakePositionRepo{
		findByFundRes: []model.ClientFundPosition{
			{ClientID: 1, OwnerType: model.OwnerTypeClient, UnitsOwned: clientUnits},
		},
	}
	fundRepo := &fakeFundRepo{findByIDResult: makeFundWithReinvestPct(50)}

	svc := newTestDividendServiceWithFund(dividendRepo, ownershipRepo, stockRepo, banking, fundRepo, positionRepo)
	err := svc.ProcessDividends(context.Background())

	require.NoError(t, err)
	// Only the fund-level payout is persisted; client distributions go via banking payments
	require.Len(t, dividendRepo.saved, 1)
	require.InDelta(t, 400.0, dividendRepo.saved[0].GrossAmount, 0.001)
}

func TestProcessDividends_FundWithZeroReinvestment(t *testing.T) {
	// 0% reinvestment — everything goes to clients
	stock := makeDividendStock(4, 100.0) // 100 shares → gross=100
	ownership := model.AssetOwnership{UserId: 42, OwnerType: model.OwnerTypeFund, AssetID: 10, Amount: 100}
	stockRepo := &fakeDividendStockRepo{stocks: []model.Stock{stock}}
	ownershipRepo := &fakeDividendOwnershipRepo{ownerships: []model.AssetOwnership{ownership}}
	dividendRepo := &fakeDividendRepo{}
	banking := &fakeBankingClient{
		accountsResp:    &pb.GetAccountsByClientIDResponse{Accounts: []*pb.AccountInfo{{AccountNumber: "444000100000000001"}}},
		paymentResp:     &pb.CreatePaymentResponse{},
		convertedAmount: 1.0,
	}
	positionRepo := &fakePositionRepo{
		findByFundRes: []model.ClientFundPosition{
			{ClientID: 1, OwnerType: model.OwnerTypeClient, UnitsOwned: 5},
		},
	}
	fundRepo := &fakeFundRepo{findByIDResult: makeFundWithReinvestPct(0)}

	svc := newTestDividendServiceWithFund(dividendRepo, ownershipRepo, stockRepo, banking, fundRepo, positionRepo)
	require.NoError(t, svc.ProcessDividends(context.Background()))

	// Only fund-level payout persisted; client payment goes via banking client
	require.Len(t, dividendRepo.saved, 1)
	require.InDelta(t, 100.0, dividendRepo.saved[0].GrossAmount, 0.001)
}

func TestProcessDividends_FundWithFullReinvestment(t *testing.T) {
	// 100% reinvestment — nothing goes to clients (orderService is nil so no orders placed)
	stock := makeDividendStock(4, 100.0)
	ownership := model.AssetOwnership{UserId: 42, OwnerType: model.OwnerTypeFund, AssetID: 10, Amount: 100}
	stockRepo := &fakeDividendStockRepo{stocks: []model.Stock{stock}}
	ownershipRepo := &fakeDividendOwnershipRepo{ownerships: []model.AssetOwnership{ownership}}
	dividendRepo := &fakeDividendRepo{}
	banking := &fakeBankingClient{
		paymentResp:     &pb.CreatePaymentResponse{},
		convertedAmount: 1.0,
	}
	positionRepo := &fakePositionRepo{
		findByFundRes: []model.ClientFundPosition{
			{ClientID: 1, OwnerType: model.OwnerTypeClient, UnitsOwned: 5},
		},
	}
	fundRepo := &fakeFundRepo{findByIDResult: makeFundWithReinvestPct(100)}

	svc := newTestDividendServiceWithFund(dividendRepo, ownershipRepo, stockRepo, banking, fundRepo, positionRepo)
	require.NoError(t, svc.ProcessDividends(context.Background()))

	// Only fund-level payout, no client payouts
	require.Len(t, dividendRepo.saved, 1)
	require.InDelta(t, 100.0, dividendRepo.saved[0].GrossAmount, 0.001)
}

func TestProcessDividends_FundNotFound_SkipsPayout(t *testing.T) {
	stock := makeDividendStock(4, 100.0)
	ownership := model.AssetOwnership{UserId: 99, OwnerType: model.OwnerTypeFund, AssetID: 10, Amount: 100}
	stockRepo := &fakeDividendStockRepo{stocks: []model.Stock{stock}}
	ownershipRepo := &fakeDividendOwnershipRepo{ownerships: []model.AssetOwnership{ownership}}
	dividendRepo := &fakeDividendRepo{}
	banking := &fakeBankingClient{}
	// findByIDResult=nil means fund not found
	fundRepo := &fakeFundRepo{findByIDResult: nil}

	svc := newTestDividendServiceWithFund(dividendRepo, ownershipRepo, stockRepo, banking, fundRepo, nil)
	// Should log error and continue, not bubble up at the top level
	require.NoError(t, svc.ProcessDividends(context.Background()))
	require.Empty(t, dividendRepo.saved)
}

func TestProcessDividends_FundClientProportionalSplit(t *testing.T) {
	// Two clients: 1 unit and 3 units → 25% and 75% of payout
	// gross=100, reinvest=50% → payout=50
	// client1 gets 50*0.25=12.5, client2 gets 50*0.75=37.5
	stock := makeDividendStock(4, 100.0)
	ownership := model.AssetOwnership{UserId: 42, OwnerType: model.OwnerTypeFund, AssetID: 10, Amount: 100}
	stockRepo := &fakeDividendStockRepo{stocks: []model.Stock{stock}}
	ownershipRepo := &fakeDividendOwnershipRepo{ownerships: []model.AssetOwnership{ownership}}
	dividendRepo := &fakeDividendRepo{}
	banking := &fakeBankingClient{
		accountsResp:    &pb.GetAccountsByClientIDResponse{Accounts: []*pb.AccountInfo{{AccountNumber: "444000100000000001"}}},
		paymentResp:     &pb.CreatePaymentResponse{},
		convertedAmount: 1.0,
	}
	positionRepo := &fakePositionRepo{
		findByFundRes: []model.ClientFundPosition{
			{ClientID: 1, OwnerType: model.OwnerTypeClient, UnitsOwned: 1},
			{ClientID: 2, OwnerType: model.OwnerTypeClient, UnitsOwned: 3},
		},
	}
	fundRepo := &fakeFundRepo{findByIDResult: makeFundWithReinvestPct(50)}

	svc := newTestDividendServiceWithFund(dividendRepo, ownershipRepo, stockRepo, banking, fundRepo, positionRepo)
	require.NoError(t, svc.ProcessDividends(context.Background()))

	// Only fund-level payout persisted; client distributions go via banking payments
	require.Len(t, dividendRepo.saved, 1)
	require.InDelta(t, 100.0, dividendRepo.saved[0].GrossAmount, 0.001)
}

func TestResolveTargetAccount_FundOwnership(t *testing.T) {
	ownership := model.AssetOwnership{UserId: 42, OwnerType: model.OwnerTypeFund}
	fundRepo := &fakeFundRepo{findByIDResult: &model.InvestmentFund{
		FundID:        42,
		AccountNumber: "444000099999000001",
	}}
	banking := &fakeBankingClient{}

	svc := newTestDividendServiceWithFund(&fakeDividendRepo{}, &fakeDividendOwnershipRepo{}, &fakeDividendStockRepo{}, banking, fundRepo, nil)
	accNum, currency, err := svc.resolveTargetAccount(context.Background(), ownership, "USD")

	require.NoError(t, err)
	require.Equal(t, "444000099999000001", accNum)
	require.Equal(t, "RSD", currency)
}

func TestResolveTargetAccount_FundNotFound(t *testing.T) {
	ownership := model.AssetOwnership{UserId: 99, OwnerType: model.OwnerTypeFund}
	fundRepo := &fakeFundRepo{findByIDResult: nil}
	banking := &fakeBankingClient{}

	svc := newTestDividendServiceWithFund(&fakeDividendRepo{}, &fakeDividendOwnershipRepo{}, &fakeDividendStockRepo{}, banking, fundRepo, nil)
	_, _, err := svc.resolveTargetAccount(context.Background(), ownership, "USD")
	require.Error(t, err)
}
