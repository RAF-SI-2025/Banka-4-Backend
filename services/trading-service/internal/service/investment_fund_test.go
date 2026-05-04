package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/stretchr/testify/require"
)

// ── Fake Fund Repo (extended for GetFundDetail) ───────────────────────────────

type fakeFundRepo struct {
	findByIDResult   *model.InvestmentFund
	findByIDErr      error
	findByNameResult *model.InvestmentFund
	findByNameErr    error
	createErr        error
	created          *model.InvestmentFund

	// new fields for GetFundDetail
	findHoldingsResult          []model.AssetOwnership
	findHoldingsErr             error
	getPerformanceHistoryResult []model.FundPerformance
	getPerformanceHistoryErr    error

	findAllResult       []model.InvestmentFund
	findAllTotal        int64
	findAllErr          error
	findByManagerResult []model.InvestmentFund
	findByManagerErr    error
}

func (f *fakeFundRepo) FindByName(ctx context.Context, name string) (*model.InvestmentFund, error) {
	return f.findByNameResult, f.findByNameErr
}
func (f *fakeFundRepo) FindByID(ctx context.Context, id uint) (*model.InvestmentFund, error) {
	return f.findByIDResult, f.findByIDErr
}
func (f *fakeFundRepo) FindByAccountNumber(ctx context.Context, accountNumber string) (*model.InvestmentFund, error) {
	return nil, nil
}

func (f *fakeFundRepo) GetAllInvestmentFunds(ctx context.Context) ([]model.InvestmentFund, error) {
	return f.findAllResult, f.findAllErr
}

func (f *fakeFundRepo) FindAll(ctx context.Context, name, sortBy, sortDir string, page, pageSize int) ([]model.InvestmentFund, int64, error) {
	return f.findAllResult, f.findAllTotal, f.findAllErr
}

func (f *fakeFundRepo) FindByManagerID(ctx context.Context, managerID uint) ([]model.InvestmentFund, error) {
	return f.findByManagerResult, f.findByManagerErr
}

func (f *fakeFundRepo) Create(ctx context.Context, fund *model.InvestmentFund) error {
	if f.createErr != nil {
		return f.createErr
	}
	fund.FundID = 1
	f.created = fund
	return nil
}
func (f *fakeFundRepo) FindHoldings(ctx context.Context, fundID uint) ([]model.AssetOwnership, error) {
	return f.findHoldingsResult, f.findHoldingsErr
}

func (f *fakeFundRepo) GetPerformanceHistory(ctx context.Context, fundID uint, limit int) ([]model.FundPerformance, error) {
	return f.getPerformanceHistoryResult, f.getPerformanceHistoryErr
}
func (f *fakeFundRepo) SavePerformanceSnapshot(ctx context.Context, perf *model.FundPerformance) error {
	return nil
}

// ── Fake Position / Investment Repos (unchanged) ─────────────────────────────

type fakePositionRepo struct {
	findResult      *model.ClientFundPosition
	findErr         error
	findByClientRes []model.ClientFundPosition
	findByClientErr error
	findByFundRes   []model.ClientFundPosition
	findByFundErr   error
	upsertErr       error
	upserted        *model.ClientFundPosition
}

func (f *fakePositionRepo) FindByClientAndFund(ctx context.Context, clientID uint, ownerType model.OwnerType, fundID uint) (*model.ClientFundPosition, error) {
	return f.findResult, f.findErr
}

func (f *fakePositionRepo) FindByClient(ctx context.Context, clientID uint, ownerType model.OwnerType) ([]model.ClientFundPosition, error) {
	return f.findByClientRes, f.findByClientErr
}

func (f *fakePositionRepo) FindByFund(ctx context.Context, fundID uint) ([]model.ClientFundPosition, error) {
	return f.findByFundRes, f.findByFundErr
}
func (f *fakePositionRepo) Upsert(ctx context.Context, position *model.ClientFundPosition) error {
	f.upserted = position
	return f.upsertErr
}

type fakeInvestmentRepo struct {
	createErr error
}

func (f *fakeInvestmentRepo) Create(ctx context.Context, investment *model.ClientFundInvestment) error {
	return f.createErr
}
func (f *fakeInvestmentRepo) FindByClientAndFund(ctx context.Context, clientID uint, ownerType model.OwnerType, fundID uint) ([]model.ClientFundInvestment, error) {
	return nil, nil
}
func (f *fakeInvestmentRepo) Count(ctx context.Context) (int64, error) {
	return 0, nil
}

type fakeRedemptionRepo struct {
	createErr  error
	updateErr  error
	pendingSum float64
	pendingErr error
	pending    []model.ClientFundRedemption
	findErr    error
	created    *model.ClientFundRedemption
	updated    *model.ClientFundRedemption
}

func (f *fakeRedemptionRepo) Create(ctx context.Context, redemption *model.ClientFundRedemption) error {
	if f.createErr != nil {
		return f.createErr
	}
	if redemption.ClientFundRedemptionID == 0 {
		redemption.ClientFundRedemptionID = 1
	}
	f.created = redemption
	return nil
}

func (f *fakeRedemptionRepo) Update(ctx context.Context, redemption *model.ClientFundRedemption) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updated = redemption
	return nil
}

func (f *fakeRedemptionRepo) FindPending(ctx context.Context, limit int) ([]model.ClientFundRedemption, error) {
	return f.pending, f.findErr
}

func (f *fakeRedemptionRepo) SumPendingByClientAndFund(ctx context.Context, clientID uint, ownerType model.OwnerType, fundID uint) (float64, error) {
	return f.pendingSum, f.pendingErr
}

// ── Fake Fund Banking Client ──────────────────────────────────────

type fakeFundBankingClient struct {
	createdAccountNumber string
	createFundAccountErr error
	getAccountResult     *pb.GetAccountByNumberResponse
	accountsByNumber     map[string]*pb.GetAccountByNumberResponse
	paymentErr           error
	payments             []*pb.CreatePaymentRequest
	tradeSettlementErr   error
	convertCurrencyFunc  func(amount float64, from, to string) (float64, error)
}

func (f *fakeFundBankingClient) GetAccountByNumber(_ context.Context, accountNumber string) (*pb.GetAccountByNumberResponse, error) {
	if f.accountsByNumber != nil {
		return f.accountsByNumber[accountNumber], nil
	}
	if f.getAccountResult != nil {
		return f.getAccountResult, nil
	}
	return nil, nil
}
func (f *fakeFundBankingClient) HasActiveLoan(_ context.Context, _ uint64) (*pb.HasActiveLoanResponse, error) {
	return nil, nil
}
func (f *fakeFundBankingClient) CreatePaymentWithoutVerification(_ context.Context, req *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error) {
	if f.paymentErr != nil {
		return nil, f.paymentErr
	}
	f.payments = append(f.payments, req)
	return &pb.CreatePaymentResponse{PaymentId: uint64(len(f.payments))}, nil
}
func (f *fakeFundBankingClient) GetAccountsByClientID(_ context.Context, _ uint64) (*pb.GetAccountsByClientIDResponse, error) {
	return nil, nil
}
func (f *fakeFundBankingClient) ConvertCurrency(_ context.Context, amount float64, from, to string) (float64, error) {
	if f.convertCurrencyFunc != nil {
		return f.convertCurrencyFunc(amount, from, to)
	}
	return amount, nil
}
func (f *fakeFundBankingClient) ExecuteTradeSettlement(_ context.Context, _, _ string, _ pb.TradeSettlementDirection, _ float64) (*pb.ExecuteTradeSettlementResponse, error) {
	if f.tradeSettlementErr != nil {
		return nil, f.tradeSettlementErr
	}
	return &pb.ExecuteTradeSettlementResponse{}, nil
}
func (f *fakeFundBankingClient) GetAccountCurrency(_ context.Context, _ string) (string, error) {
	return "RSD", nil
}
func (f *fakeFundBankingClient) CreateFundAccount(_ context.Context, _ string, _ uint64) (string, error) {
	return f.createdAccountNumber, f.createFundAccountErr
}

// ── Fake User Client
type fakeUserClient struct {
	getEmployeeByIDFunc func(ctx context.Context, id uint64) (*pb.GetEmployeeByIdResponse, error)
}

type fakeFundUserClient struct {
	// configurable responses
	getClientByIdResp *pb.GetClientByIdResponse
	getClientByIdErr  error

	getEmployeeByIdResp *pb.GetEmployeeByIdResponse
	getEmployeeByIdErr  error

	getAllClientsResp *pb.GetAllClientsResponse
	getAllClientsErr  error

	getAllActuariesResp *pb.GetAllActuariesResponse
	getAllActuariesErr  error

	getIdentityByUserIdResp *pb.GetIdentityByUserIdResponse
	getIdentityByUserIdErr  error
}

func (f *fakeFundUserClient) GetClientById(_ context.Context, _ uint64) (*pb.GetClientByIdResponse, error) {
	return f.getClientByIdResp, f.getClientByIdErr
}

func (f *fakeFundUserClient) GetClientByIdentityId(_ context.Context, _ uint64) (*pb.GetClientByIdResponse, error) {
	return f.getClientByIdResp, f.getClientByIdErr
}

func (f *fakeFundUserClient) GetEmployeeById(_ context.Context, _ uint64) (*pb.GetEmployeeByIdResponse, error) {
	return f.getEmployeeByIdResp, f.getEmployeeByIdErr
}

func (f *fakeFundUserClient) GetEmployeeByIdentityId(_ context.Context, _ uint64) (*pb.GetEmployeeByIdResponse, error) {
	return f.getEmployeeByIdResp, f.getEmployeeByIdErr
}

func (f *fakeFundUserClient) GetAllClients(_ context.Context, _, _ int32, _, _ string) (*pb.GetAllClientsResponse, error) {
	return f.getAllClientsResp, f.getAllClientsErr
}

func (f *fakeFundUserClient) GetAllActuaries(_ context.Context, _, _ int32, _, _ string) (*pb.GetAllActuariesResponse, error) {
	return f.getAllActuariesResp, f.getAllActuariesErr
}

func (f *fakeFundUserClient) GetIdentityByUserId(_ context.Context, _ uint64, _ string) (*pb.GetIdentityByUserIdResponse, error) {
	return f.getIdentityByUserIdResp, f.getIdentityByUserIdErr
}

// ── Helpers ───────────────────────────────────────────────────────

func fundSupervisorCtx() context.Context {
	employeeID := uint(25)
	return auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityID:   200,
		IdentityType: auth.IdentityEmployee,
		EmployeeID:   &employeeID,
	})
}

func (f *fakeUserClient) GetClientById(ctx context.Context, id uint64) (*pb.GetClientByIdResponse, error) {
	return nil, nil
}
func (f *fakeUserClient) GetClientByIdentityId(ctx context.Context, identityId uint64) (*pb.GetClientByIdResponse, error) {
	return nil, nil
}
func (f *fakeUserClient) GetEmployeeById(ctx context.Context, id uint64) (*pb.GetEmployeeByIdResponse, error) {
	if f.getEmployeeByIDFunc != nil {
		return f.getEmployeeByIDFunc(ctx, id)
	}
	return &pb.GetEmployeeByIdResponse{Id: id, FullName: fmt.Sprintf("Manager %d", id)}, nil
}
func (f *fakeUserClient) GetEmployeeByIdentityId(ctx context.Context, identityId uint64) (*pb.GetEmployeeByIdResponse, error) {
	return nil, nil
}
func (f *fakeUserClient) GetAllClients(ctx context.Context, page, pageSize int32, firstName, lastName string) (*pb.GetAllClientsResponse, error) {
	return nil, nil
}
func (f *fakeUserClient) GetAllActuaries(ctx context.Context, page, pageSize int32, firstName, lastName string) (*pb.GetAllActuariesResponse, error) {
	return nil, nil
}
func (f *fakeUserClient) GetIdentityByUserId(ctx context.Context, userID uint64, userType string) (*pb.GetIdentityByUserIdResponse, error) {
	return nil, nil
}

// ── Helper for creating service with listingRepo ───────────────────────────

func newTestFundServiceWithListing(fundRepo *fakeFundRepo, listingRepo *fakeListingRepo, bankingClient *fakeFundBankingClient, userClient *fakeUserClient) *InvestmentFundService {
	exchange := defaultExchange()
	svc := NewInvestmentFundService(fundRepo, &fakePositionRepo{}, listingRepo, &fakeInvestmentRepo{}, &fakeRedemptionRepo{}, &fakeAssetOwnershipRepo{}, &fakeExchangeRepo{exchange: exchange}, &fakeStockRepo{},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{}, bankingClient, userClient, nil)
	svc.listingRepo = listingRepo // inject listingRepo
	return svc
}

// ── Tests: GetFundDetail ───────────────────────────────────────────────────

func TestGetFundDetail_Success(t *testing.T) {
	fund := &model.InvestmentFund{
		FundID:              1,
		Name:                "Test Fund",
		Description:         "A test fund",
		MinimumContribution: 500,
		ManagerID:           10,
		AccountNumber:       "ACC123",
		Positions: []model.ClientFundPosition{
			{TotalInvestedAmount: 1500.0},
		},
	}
	bankingClient := &fakeFundBankingClient{
		getAccountResult: &pb.GetAccountByNumberResponse{AvailableBalance: 2000},
	}
	userClient := &fakeUserClient{}
	fundRepo := &fakeFundRepo{
		findByIDResult: fund,
		findHoldingsResult: []model.AssetOwnership{
			{
				AssetID:   100,
				Amount:    10,
				Asset:     model.Asset{Ticker: "AAPL"}, // not pointer
				UpdatedAt: time.Now().Add(-24 * time.Hour),
			},
			{
				AssetID:   101,
				Amount:    5,
				Asset:     model.Asset{Ticker: "GOOGL"}, // not pointer
				UpdatedAt: time.Now().Add(-48 * time.Hour),
			},
		},
		getPerformanceHistoryResult: []model.FundPerformance{
			{Date: time.Now().AddDate(0, -1, 0), FundValue: 1800},
			{Date: time.Now().AddDate(0, -2, 0), FundValue: 1700},
		},
	}
	listingRepo := &fakeListingRepo{
		byAssetIDs: []model.Listing{
			{AssetID: 100, Price: 120, MaintenanceMargin: 10, ListingID: 1000, Exchange: &model.Exchange{MicCode: "XSIM"}},
			{AssetID: 101, Price: 110, MaintenanceMargin: 8, ListingID: 1001, Exchange: &model.Exchange{MicCode: "XSIM"}},
		},
		dailyPriceInfo: &model.ListingDailyPriceInfo{Change: 2.5, Volume: 1000},
	}
	svc := newTestFundServiceWithListing(fundRepo, listingRepo, bankingClient, userClient)

	resp, err := svc.GetFundDetail(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "Test Fund", resp.Name)
	require.Equal(t, "A test fund", resp.Description)
	require.Equal(t, 500.0, resp.MinInvestment)
	require.Equal(t, "Manager 10", resp.Manager)
	require.Equal(t, 2000.0, resp.LiquidAssets)

	// fundValue = (10*120)+(5*110) = 1200+550=1750
	// profit = 1750 - 1500 = 250
	require.Equal(t, 3750.0, resp.FundValue)
	require.Equal(t, 2250.0, resp.Profit)

	require.Len(t, resp.Holdings, 2)
	require.Equal(t, "AAPL", resp.Holdings[0].Ticker)
	require.Equal(t, 120.0, resp.Holdings[0].Price)
	require.Equal(t, 2.5, resp.Holdings[0].Change)
	require.Equal(t, uint64(1000), resp.Holdings[0].Volume)
	require.Equal(t, 10.0, resp.Holdings[0].InitialMarginCost)

	require.Len(t, resp.PerformanceHistory, 2)
}

func newTestFundService(fundRepo *fakeFundRepo, ownershipRepo *fakeAssetOwnershipRepo, listingRepo *fakeListingRepo, bankingClient *fakeFundBankingClient, userClient *fakeFundUserClient) *InvestmentFundService {
	exchange := defaultExchange()
	return NewInvestmentFundService(
		fundRepo,
		&fakePositionRepo{},
		listingRepo,
		&fakeInvestmentRepo{},
		&fakeRedemptionRepo{},
		ownershipRepo,
		&fakeExchangeRepo{exchange: exchange},
		&fakeStockRepo{},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		bankingClient,
		userClient,
		nil,
	)
}

// ── CreateFund tests ──────────────────────────────────────────────

func TestCreateFund_Success(t *testing.T) {
	fundRepo := &fakeFundRepo{}
	bankingClient := &fakeFundBankingClient{createdAccountNumber: "444000112345678901"}
	svc := newTestFundService(fundRepo, &fakeAssetOwnershipRepo{}, &fakeListingRepo{}, bankingClient, &fakeFundUserClient{})

	resp, err := svc.CreateFund(fundSupervisorCtx(), validFundRequest())

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "Alpha Growth Fund", resp.Name)
	require.Equal(t, "444000112345678901", resp.AccountNumber)
	require.Equal(t, uint(25), resp.ManagerID)
	require.Equal(t, 1000.00, resp.MinimumContribution)
	require.WithinDuration(t, time.Now(), resp.CreatedAt, 5*time.Second)
}

func TestGetFundDetail_NotFound(t *testing.T) {
	fundRepo := &fakeFundRepo{findByIDResult: nil}
	svc := newTestFundServiceWithListing(fundRepo, &fakeListingRepo{}, &fakeFundBankingClient{}, &fakeUserClient{})
	_, err := svc.GetFundDetail(context.Background(), 99)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestCreateFund_Unauthenticated(t *testing.T) {
	svc := newTestFundService(&fakeFundRepo{}, &fakeAssetOwnershipRepo{}, &fakeListingRepo{}, &fakeFundBankingClient{}, &fakeFundUserClient{})

	_, err := svc.CreateFund(context.Background(), validFundRequest())

	require.Error(t, err)
	require.Contains(t, err.Error(), "not authenticated")
}

func TestGetFundDetail_RepoFindByIDError(t *testing.T) {
	fundRepo := &fakeFundRepo{findByIDErr: errors.New("db error")}
	svc := newTestFundServiceWithListing(fundRepo, &fakeListingRepo{}, &fakeFundBankingClient{}, &fakeUserClient{})
	_, err := svc.GetFundDetail(context.Background(), 1)

	require.Error(t, err)
}

func TestCreateFund_NotEmployee(t *testing.T) {
	svc := newTestFundService(&fakeFundRepo{}, &fakeAssetOwnershipRepo{}, &fakeListingRepo{}, &fakeFundBankingClient{}, &fakeFundUserClient{})

	_, err := svc.CreateFund(fundClientCtx(), validFundRequest())

	require.Error(t, err)
}

func TestGetFundDetail_HoldingsError(t *testing.T) {
	fund := &model.InvestmentFund{FundID: 1, AccountNumber: "ACC"}
	fundRepo := &fakeFundRepo{
		findByIDResult:  fund,
		findHoldingsErr: errors.New("holdings error"),
	}
	svc := newTestFundServiceWithListing(fundRepo, &fakeListingRepo{}, &fakeFundBankingClient{}, &fakeUserClient{})
	_, err := svc.GetFundDetail(context.Background(), 1)
	require.Error(t, err)
}

func TestCreateFund_BankingClientError(t *testing.T) {
	fundRepo := &fakeFundRepo{}
	bankingClient := &fakeFundBankingClient{
		createFundAccountErr: fmt.Errorf("banking service unavailable"),
	}
	svc := newTestFundService(fundRepo, &fakeAssetOwnershipRepo{}, &fakeListingRepo{}, bankingClient, &fakeFundUserClient{})

	_, err := svc.CreateFund(fundSupervisorCtx(), validFundRequest())

	require.Error(t, err)
}

func TestGetFundDetail_EmptyHoldings(t *testing.T) {
	fund := &model.InvestmentFund{FundID: 1, AccountNumber: "ACC", MinimumContribution: 100}
	fundRepo := &fakeFundRepo{
		findByIDResult:     fund,
		findHoldingsResult: []model.AssetOwnership{},
		getPerformanceHistoryResult: []model.FundPerformance{
			{Date: time.Now(), FundValue: 5000},
		},
	}
	listingRepo := &fakeListingRepo{}
	bankingClient := &fakeFundBankingClient{
		getAccountResult: &pb.GetAccountByNumberResponse{AvailableBalance: 5000},
	}
	userClient := &fakeUserClient{}
	svc := newTestFundServiceWithListing(fundRepo, listingRepo, bankingClient, userClient)

	resp, err := svc.GetFundDetail(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 5000.0, resp.FundValue)
	require.Equal(t, 5000.0, resp.Profit) // profit = 5000 - 0 = 5000
	require.Empty(t, resp.Holdings)
	require.NotEmpty(t, resp.PerformanceHistory)
}

// ── GetAllFunds tests ─────────────────────────────────────────────

func TestGetAllFunds_Success(t *testing.T) {
	fund := model.InvestmentFund{
		FundID:              1,
		Name:                "Alpha Growth Fund",
		Description:         "IT sector fund",
		MinimumContribution: 1000.0,
		ManagerID:           25,
		AccountNumber:       "444000000000000001",
		Positions: []model.ClientFundPosition{
			{TotalInvestedAmount: 300.0},
		},
	}
	ownership := model.AssetOwnership{AssetID: 10, Amount: 2.0}
	listing := model.Listing{AssetID: 10, Price: 100.0}

	fundRepo := &fakeFundRepo{findAllResult: []model.InvestmentFund{fund}, findAllTotal: 1}
	ownershipRepo := &fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{ownership}}
	listingRepo := &fakeListingRepo{byAssetIDs: []model.Listing{listing}}
	bankingClient := &fakeFundBankingClient{
		getAccountResult: &pb.GetAccountByNumberResponse{AvailableBalance: 500.0},
	}
	svc := newTestFundService(fundRepo, ownershipRepo, listingRepo, bankingClient, &fakeFundUserClient{})

	resp, err := svc.GetAllFunds(fundSupervisorCtx(), dto.ListFundsQuery{Page: 1, PageSize: 10})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, int64(1), resp.Total)
	require.Len(t, resp.Data, 1)
	// securitiesValue = 2.0 * 100.0 = 200.0
	// fundValue = 500 (liquid) + 200 (securities) = 700
	require.Equal(t, 700.0, resp.Data[0].FundValue)
	// profit = 700 - 300 (invested) = 400
	require.Equal(t, 400.0, resp.Data[0].Profit)
	require.Equal(t, 500.0, resp.Data[0].LiquidAssets)
	require.Equal(t, 1, resp.Page)
	require.Equal(t, 10, resp.PageSize)
}

func TestGetAllFunds_Empty(t *testing.T) {
	fundRepo := &fakeFundRepo{findAllResult: []model.InvestmentFund{}, findAllTotal: 0}
	svc := newTestFundService(fundRepo, &fakeAssetOwnershipRepo{}, &fakeListingRepo{}, &fakeFundBankingClient{}, &fakeFundUserClient{})

	resp, err := svc.GetAllFunds(fundSupervisorCtx(), dto.ListFundsQuery{Page: 1, PageSize: 10})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, int64(0), resp.Total)
	require.Empty(t, resp.Data)
}

func TestGetAllFunds_RepoError(t *testing.T) {
	fundRepo := &fakeFundRepo{findAllErr: errors.New("db error")}
	svc := newTestFundService(fundRepo, &fakeAssetOwnershipRepo{}, &fakeListingRepo{}, &fakeFundBankingClient{}, &fakeFundUserClient{})

	_, err := svc.GetAllFunds(fundSupervisorCtx(), dto.ListFundsQuery{Page: 1, PageSize: 10})

	require.Error(t, err)
}

func TestGetAllFunds_OwnershipRepoError(t *testing.T) {
	fund := model.InvestmentFund{FundID: 1, Name: "Fund", AccountNumber: "444000000000000001"}
	fundRepo := &fakeFundRepo{findAllResult: []model.InvestmentFund{fund}, findAllTotal: 1}
	ownershipRepo := &fakeAssetOwnershipRepo{findErr: errors.New("db error")}
	svc := newTestFundService(fundRepo, ownershipRepo, &fakeListingRepo{}, &fakeFundBankingClient{}, &fakeFundUserClient{})

	_, err := svc.GetAllFunds(fundSupervisorCtx(), dto.ListFundsQuery{Page: 1, PageSize: 10})

	require.Error(t, err)
}

// ── GetActuaryFunds tests ─────────────────────────────────────────

func TestGetActuaryFunds_Success(t *testing.T) {
	fund := model.InvestmentFund{
		FundID:        1,
		Name:          "Alpha Growth Fund",
		Description:   "IT sector fund",
		ManagerID:     25,
		AccountNumber: "444000000000000001",
	}
	ownership := model.AssetOwnership{AssetID: 5, Amount: 10.0}
	listing := model.Listing{AssetID: 5, Price: 50000.0}

	fundRepo := &fakeFundRepo{findByManagerResult: []model.InvestmentFund{fund}}
	ownershipRepo := &fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{ownership}}
	listingRepo := &fakeListingRepo{byAssetIDs: []model.Listing{listing}}
	bankingClient := &fakeFundBankingClient{
		getAccountResult: &pb.GetAccountByNumberResponse{AvailableBalance: 1500000.0},
	}
	svc := newTestFundService(fundRepo, ownershipRepo, listingRepo, bankingClient, &fakeFundUserClient{})

	resp, err := svc.GetActuaryFunds(fundSupervisorCtx(), 25)

	require.NoError(t, err)
	require.Len(t, resp, 1)
	require.Equal(t, "Alpha Growth Fund", resp[0].Name)
	require.Equal(t, 1500000.0, resp[0].LiquidAssets)
	// securitiesValue = 10 * 50000 = 500000
	// fundValue = 1500000 + 500000 = 2000000
	require.Equal(t, 2000000.0, resp[0].FundValue)
}

func TestGetActuaryFunds_Empty(t *testing.T) {
	fundRepo := &fakeFundRepo{findByManagerResult: []model.InvestmentFund{}}
	svc := newTestFundService(fundRepo, &fakeAssetOwnershipRepo{}, &fakeListingRepo{}, &fakeFundBankingClient{}, &fakeFundUserClient{})

	resp, err := svc.GetActuaryFunds(fundSupervisorCtx(), 99)

	require.NoError(t, err)
	require.Empty(t, resp)
}

func TestGetActuaryFunds_RepoError(t *testing.T) {
	fundRepo := &fakeFundRepo{findByManagerErr: errors.New("db error")}
	svc := newTestFundService(fundRepo, &fakeAssetOwnershipRepo{}, &fakeListingRepo{}, &fakeFundBankingClient{}, &fakeFundUserClient{})

	_, err := svc.GetActuaryFunds(fundSupervisorCtx(), 25)

	require.Error(t, err)
}

func TestGetActuaryFunds_OwnershipRepoError(t *testing.T) {
	fund := model.InvestmentFund{FundID: 1, Name: "Fund", ManagerID: 25}
	fundRepo := &fakeFundRepo{findByManagerResult: []model.InvestmentFund{fund}}
	ownershipRepo := &fakeAssetOwnershipRepo{findErr: errors.New("db error")}
	svc := newTestFundService(fundRepo, ownershipRepo, &fakeListingRepo{}, &fakeFundBankingClient{}, &fakeFundUserClient{})

	_, err := svc.GetActuaryFunds(fundSupervisorCtx(), 25)

	require.Error(t, err)
}

func TestWithdrawFromFund_ClientSuccess(t *testing.T) {
	fund := &model.InvestmentFund{FundID: 1, Name: "Alpha Growth Fund", AccountNumber: "fund-account"}
	positionRepo := &fakePositionRepo{findResult: &model.ClientFundPosition{
		ClientID:            99,
		OwnerType:           model.OwnerTypeClient,
		FundID:              1,
		TotalInvestedAmount: 2000,
	}}
	redemptionRepo := &fakeRedemptionRepo{}
	bankingClient := &fakeFundBankingClient{
		accountsByNumber: map[string]*pb.GetAccountByNumberResponse{
			"fund-account":   {AccountNumber: "fund-account", AccountType: "Fund", CurrencyCode: "RSD", AvailableBalance: 2000},
			"client-account": {AccountNumber: "client-account", ClientId: 99, AccountType: "Current", CurrencyCode: "RSD"},
		},
	}

	exchange := defaultExchange()
	svc := NewInvestmentFundService(&fakeFundRepo{findByIDResult: fund}, positionRepo, &fakeListingRepo{}, &fakeInvestmentRepo{}, redemptionRepo, &fakeAssetOwnershipRepo{}, &fakeExchangeRepo{exchange: exchange}, &fakeStockRepo{}, &fakeOptionRepo{}, &fakeFuturesRepo{}, &fakeForexRepo{}, bankingClient, &fakeFundUserClient{}, nil)

	resp, err := svc.WithdrawFromFund(fundClientCtx(), 1, dto.WithdrawFromFundRequest{
		AccountNumber: "client-account",
		Amount:        1000,
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, model.FundRedemptionCompleted, resp.Status)
	require.Equal(t, 1000.0, resp.WithdrawnAmountRSD)
	require.Equal(t, 1000.0, resp.TotalInvestedRSD)
	require.Len(t, bankingClient.payments, 1)
	require.Equal(t, "fund-account", bankingClient.payments[0].PayerAccountNumber)
	require.Equal(t, "client-account", bankingClient.payments[0].RecipientAccountNumber)
	require.False(t, bankingClient.payments[0].CommissionExempt)
	require.NotNil(t, redemptionRepo.created)
	require.Equal(t, model.FundRedemptionCompleted, redemptionRepo.created.Status)
	require.NotNil(t, positionRepo.upserted)
	require.Equal(t, 1000.0, positionRepo.upserted.TotalInvestedAmount)
}

func TestWithdrawFromFund_SupervisorSuccessCommissionExempt(t *testing.T) {
	fund := &model.InvestmentFund{FundID: 1, Name: "Alpha Growth Fund", AccountNumber: "fund-account"}
	positionRepo := &fakePositionRepo{findResult: &model.ClientFundPosition{
		ClientID:            25,
		OwnerType:           model.OwnerTypeActuary,
		FundID:              1,
		TotalInvestedAmount: 3000,
	}}
	bankingClient := &fakeFundBankingClient{
		accountsByNumber: map[string]*pb.GetAccountByNumberResponse{
			"fund-account": {AccountNumber: "fund-account", AccountType: "Fund", CurrencyCode: "RSD", AvailableBalance: 3000},
			"bank-account": {AccountNumber: "bank-account", AccountType: "Bank", CurrencyCode: "RSD"},
		},
	}

	exchange := defaultExchange()
	svc := NewInvestmentFundService(&fakeFundRepo{findByIDResult: fund}, positionRepo, &fakeListingRepo{}, &fakeInvestmentRepo{}, &fakeRedemptionRepo{}, &fakeAssetOwnershipRepo{}, &fakeExchangeRepo{exchange: exchange}, &fakeStockRepo{}, &fakeOptionRepo{}, &fakeFuturesRepo{}, &fakeForexRepo{}, bankingClient, &fakeFundUserClient{}, nil)

	resp, err := svc.WithdrawFromFund(fundSupervisorCtx(), 1, dto.WithdrawFromFundRequest{
		AccountNumber: "bank-account",
		Amount:        1500,
	})

	require.NoError(t, err)
	require.Equal(t, model.FundRedemptionCompleted, resp.Status)
	require.Len(t, bankingClient.payments, 1)
	require.True(t, bankingClient.payments[0].CommissionExempt)
	require.Equal(t, 1500.0, resp.TotalInvestedRSD)
}

func TestWithdrawFromFund_ExceedsAvailablePosition(t *testing.T) {
	fund := &model.InvestmentFund{FundID: 1, Name: "Alpha Growth Fund", AccountNumber: "fund-account"}
	positionRepo := &fakePositionRepo{findResult: &model.ClientFundPosition{
		ClientID:            99,
		OwnerType:           model.OwnerTypeClient,
		FundID:              1,
		TotalInvestedAmount: 1000,
	}}
	redemptionRepo := &fakeRedemptionRepo{pendingSum: 300}
	bankingClient := &fakeFundBankingClient{
		accountsByNumber: map[string]*pb.GetAccountByNumberResponse{
			"client-account": {AccountNumber: "client-account", ClientId: 99, AccountType: "Current", CurrencyCode: "RSD"},
		},
	}

	exchange := defaultExchange()
	svc := NewInvestmentFundService(&fakeFundRepo{findByIDResult: fund}, positionRepo, &fakeListingRepo{}, &fakeInvestmentRepo{}, redemptionRepo, &fakeAssetOwnershipRepo{}, &fakeExchangeRepo{exchange: exchange}, &fakeStockRepo{}, &fakeOptionRepo{}, &fakeFuturesRepo{}, &fakeForexRepo{}, bankingClient, &fakeFundUserClient{}, nil)

	resp, err := svc.WithdrawFromFund(fundClientCtx(), 1, dto.WithdrawFromFundRequest{
		AccountNumber: "client-account",
		Amount:        800,
	})

	require.Nil(t, resp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds available")
	require.Empty(t, bankingClient.payments)
}

func TestWithdrawFromFund_InsufficientLiquidityWithoutSecurities(t *testing.T) {
	fund := &model.InvestmentFund{FundID: 1, Name: "Alpha Growth Fund", AccountNumber: "fund-account"}
	positionRepo := &fakePositionRepo{findResult: &model.ClientFundPosition{
		ClientID:            99,
		OwnerType:           model.OwnerTypeClient,
		FundID:              1,
		TotalInvestedAmount: 2000,
	}}
	bankingClient := &fakeFundBankingClient{
		accountsByNumber: map[string]*pb.GetAccountByNumberResponse{
			"fund-account":   {AccountNumber: "fund-account", AccountType: "Fund", CurrencyCode: "RSD", AvailableBalance: 100},
			"client-account": {AccountNumber: "client-account", ClientId: 99, AccountType: "Current", CurrencyCode: "RSD"},
		},
	}

	exchange := defaultExchange()
	svc := NewInvestmentFundService(&fakeFundRepo{findByIDResult: fund}, positionRepo, &fakeListingRepo{}, &fakeInvestmentRepo{}, &fakeRedemptionRepo{}, &fakeAssetOwnershipRepo{}, &fakeExchangeRepo{exchange: exchange}, &fakeStockRepo{}, &fakeOptionRepo{}, &fakeFuturesRepo{}, &fakeForexRepo{}, bankingClient, &fakeFundUserClient{}, nil)

	resp, err := svc.WithdrawFromFund(fundClientCtx(), 1, dto.WithdrawFromFundRequest{
		AccountNumber: "client-account",
		Amount:        1000,
	})

	require.Nil(t, resp)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient liquid assets")
	require.Empty(t, bankingClient.payments)
}

func validFundRequest() dto.CreateFundRequest {
	return dto.CreateFundRequest{
		Name:                "Alpha Growth Fund",
		Description:         "Fund focused on the IT sector.",
		MinimumContribution: 1000.00,
	}
}

func fundClientCtx() context.Context {
	clientID := uint(99)
	return auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityID:   1,
		IdentityType: auth.IdentityClient,
		ClientID:     &clientID,
	})
}
