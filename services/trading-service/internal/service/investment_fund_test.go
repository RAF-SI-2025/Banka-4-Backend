package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	commonErrors "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/stretchr/testify/require"
)

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

	updateManagerIDResult int64
	updateManagerIDErr    error

	savedPerformances []*model.FundPerformance

	getAllPerformanceHistoriesResult map[uint][]model.FundPerformance
	getAllPerformanceHistoriesErr    error
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
	f.savedPerformances = append(f.savedPerformances, perf)
	return nil
}

func (f *fakeFundRepo) UpdateManagerID(ctx context.Context, fromManagerID uint, toManagerID uint) (int64, error) {
	return f.updateManagerIDResult, f.updateManagerIDErr
}

func (f *fakeFundRepo) GetAllPerformanceHistories(ctx context.Context, minSnapshots int) (map[uint][]model.FundPerformance, error) {
	if f.getAllPerformanceHistoriesErr != nil {
		return nil, f.getAllPerformanceHistoriesErr
	}
	if f.getAllPerformanceHistoriesResult != nil {
		return f.getAllPerformanceHistoriesResult, nil
	}
	return map[uint][]model.FundPerformance{}, nil
}

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

func (f *fakeFundBankingClient) ReserveOtcFunds(_ context.Context, req *pb.ReserveOtcFundsRequest) (*pb.OtcFundsReservationResponse, error) {
	return &pb.OtcFundsReservationResponse{ExecutionId: req.ExecutionId, Status: pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_RESERVED}, nil
}

func (f *fakeFundBankingClient) ReleaseOtcFunds(_ context.Context, executionID string) (*pb.OtcFundsReservationResponse, error) {
	return &pb.OtcFundsReservationResponse{ExecutionId: executionID, Status: pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_RELEASED}, nil
}

func (f *fakeFundBankingClient) CommitOtcFunds(_ context.Context, executionID string) (*pb.OtcFundsReservationResponse, error) {
	return &pb.OtcFundsReservationResponse{ExecutionId: executionID, Status: pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_COMMITTED}, nil
}

func (f *fakeFundBankingClient) RefundOtcFunds(_ context.Context, executionID string) (*pb.OtcFundsReservationResponse, error) {
	return &pb.OtcFundsReservationResponse{ExecutionId: executionID, Status: pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_REFUNDED}, nil
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

	getClientsByIdsResp *pb.GetClientsByIdsResponse
	getClientsByIdsErr  error

	getEmployeeByIdResp *pb.GetEmployeeByIdResponse
	getEmployeeByIdErr  error

	getAllClientsResp *pb.GetAllClientsResponse
	getAllClientsErr  error

	getAllActuariesResp *pb.GetAllActuariesResponse
	getAllActuariesErr  error

	getIdentityByUserIdResp *pb.GetIdentityByUserIdResponse
	getIdentityByUserIdErr  error

	incrementUsedLimitResp *pb.IncrementUsedLimitResponse
	incrementUsedLimitErr  error
}

func (f *fakeFundUserClient) GetClientById(_ context.Context, _ uint64) (*pb.GetClientByIdResponse, error) {
	return f.getClientByIdResp, f.getClientByIdErr
}

func (f *fakeFundUserClient) GetClientsByIds(_ context.Context, _ []uint64) (*pb.GetClientsByIdsResponse, error) {
	return f.getClientsByIdsResp, f.getClientsByIdsErr
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

func (f *fakeFundUserClient) IncrementUsedLimit(ctx context.Context, employeeID uint64, amount float64) (*pb.IncrementUsedLimitResponse, error) {
	return f.incrementUsedLimitResp, f.incrementUsedLimitErr
}

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
func (f *fakeUserClient) GetClientsByIds(_ context.Context, _ []uint64) (*pb.GetClientsByIdsResponse, error) {
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
func (f *fakeUserClient) IncrementUsedLimit(ctx context.Context, employeeID uint64, amount float64) (*pb.IncrementUsedLimitResponse, error) {
	return nil, nil
}

func newTestFundServiceWithListing(fundRepo *fakeFundRepo, listingRepo *fakeListingRepo, bankingClient *fakeFundBankingClient, userClient *fakeUserClient) *InvestmentFundService {
	exchange := defaultExchange()
	svc := NewInvestmentFundService(fundRepo, &fakePositionRepo{}, listingRepo, &fakeInvestmentRepo{}, &fakeRedemptionRepo{}, &fakeAssetOwnershipRepo{}, &fakeExchangeRepo{exchange: exchange}, &fakeStockRepo{},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{}, bankingClient, userClient, nil)
	svc.listingRepo = listingRepo // inject listingRepo
	return svc
}

func TestGetFundDetail_Success(t *testing.T) {
	fund := &model.InvestmentFund{
		FundID:              1,
		Name:                "Test Fund",
		Description:         "A test fund",
		MinimumContribution: 500,
		ManagerID:           10,
		AccountNumber:       "ACC123",
		Positions: []model.ClientFundPosition{
			{TotalInvestedAmount: 2000.0},
		},
	}
	bankingClient := &fakeFundBankingClient{
		getAccountResult: &pb.GetAccountByNumberResponse{AvailableBalance: 3000},
	}
	userClient := &fakeUserClient{}
	fundRepo := &fakeFundRepo{
		findByIDResult: fund,
		findHoldingsResult: []model.AssetOwnership{
			{
				AssetID:   100,
				UserId:    1,
				OwnerType: model.OwnerTypeFund,
				Amount:    10,
				Asset:     model.Asset{Ticker: "AAPL"}, // not pointer
				UpdatedAt: time.Now().Add(-24 * time.Hour),
			},
			{
				AssetID:   101,
				UserId:    1,
				OwnerType: model.OwnerTypeFund,
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
	ownershipRepo := &fakeAssetOwnershipRepo{
		ownerships: []model.AssetOwnership{
			{AssetID: 100, Amount: 10},
			{AssetID: 101, Amount: 5},
		},
	}
	exchange := defaultExchange()
	svc := NewInvestmentFundService(fundRepo, &fakePositionRepo{}, listingRepo, &fakeInvestmentRepo{}, &fakeRedemptionRepo{}, ownershipRepo, &fakeExchangeRepo{exchange: exchange}, &fakeStockRepo{}, &fakeOptionRepo{}, &fakeFuturesRepo{}, &fakeForexRepo{}, bankingClient, userClient, nil)

	resp, err := svc.GetFundDetail(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, "Test Fund", resp.Name)
	require.Equal(t, "A test fund", resp.Description)
	require.Equal(t, 500.0, resp.MinInvestment)
	require.Equal(t, "Manager 10", resp.Manager)
	require.Equal(t, 3000.0, resp.LiquidAssets)

	// fundValue = (10*120)+(5*110) = 1200+550=1750
	// profit = 1750 - 1500 = 250
	require.Equal(t, 1750.0+3000.0, resp.FundValue)
	require.Equal(t, 1750.0+3000.0-2000.0, resp.Profit)

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
		OwnerType:           model.OwnerTypeBank,
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

func TestWithdrawFromFund_LegacyPositionUsesNAVForAvailableValue(t *testing.T) {
	fund := &model.InvestmentFund{FundID: 1, Name: "Alpha Growth Fund", AccountNumber: "fund-account"}
	positionRepo := &fakePositionRepo{
		findResult: &model.ClientFundPosition{
			ClientID:            99,
			OwnerType:           model.OwnerTypeClient,
			FundID:              1,
			TotalInvestedAmount: 1000,
		},
		findByFundRes: []model.ClientFundPosition{
			{ClientID: 99, OwnerType: model.OwnerTypeClient, FundID: 1, TotalInvestedAmount: 1000},
			{ClientID: 100, OwnerType: model.OwnerTypeClient, FundID: 1, TotalInvestedAmount: 1000},
		},
	}
	bankingClient := &fakeFundBankingClient{
		accountsByNumber: map[string]*pb.GetAccountByNumberResponse{
			"fund-account":   {AccountNumber: "fund-account", AccountType: "Fund", CurrencyCode: "RSD", AvailableBalance: 4000},
			"client-account": {AccountNumber: "client-account", ClientId: 99, AccountType: "Current", CurrencyCode: "RSD"},
		},
	}

	exchange := defaultExchange()
	svc := NewInvestmentFundService(&fakeFundRepo{findByIDResult: fund}, positionRepo, &fakeListingRepo{}, &fakeInvestmentRepo{}, &fakeRedemptionRepo{}, &fakeAssetOwnershipRepo{}, &fakeExchangeRepo{exchange: exchange}, &fakeStockRepo{}, &fakeOptionRepo{}, &fakeFuturesRepo{}, &fakeForexRepo{}, bankingClient, &fakeFundUserClient{}, nil)

	resp, err := svc.WithdrawFromFund(fundClientCtx(), 1, dto.WithdrawFromFundRequest{
		AccountNumber: "client-account",
		Amount:        1500,
	})

	require.NoError(t, err)
	require.Equal(t, model.FundRedemptionCompleted, resp.Status)
	require.Equal(t, 1500.0, resp.WithdrawnAmountRSD)
	require.InDelta(t, 250.0, resp.TotalInvestedRSD, 0.0001)
	require.NotNil(t, positionRepo.upserted)
	require.InDelta(t, 250.0, positionRepo.upserted.UnitsOwned, 0.0001)
	require.InDelta(t, 250.0, positionRepo.upserted.TotalInvestedAmount, 0.0001)
	require.Len(t, bankingClient.payments, 1)
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
func TestTransferFunds_Success(t *testing.T) {
	// 1. Priprema: želimo da simuliramo da je menadžer 2 imao 5 fondova
	fundRepo := &fakeFundRepo{
		updateManagerIDResult: 5,
		updateManagerIDErr:    nil,
	}

	// 2. Kreiranje servisa sa tvojim tačnim parametrima
	// Koristimo tvoj NewInvestmentFundService i prosleđujemo fejove gde treba
	svc := NewInvestmentFundService(
		fundRepo, // tvoj mock repo
		&fakePositionRepo{},
		&fakeListingRepo{},
		&fakeInvestmentRepo{},
		&fakeRedemptionRepo{},
		&fakeAssetOwnershipRepo{},
		&fakeExchangeRepo{},
		&fakeStockRepo{},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		&fakeFundBankingClient{},
		&fakeFundUserClient{},
		nil, // scheduler
	)

	// 3. Izvršavanje: pokušavamo da prebacimo sa menadžera 2 na menadžera 1
	count, err := svc.TransferFunds(context.Background(), 2, 1)

	// 4. Provera rezultata
	require.NoError(t, err)
	require.Equal(t, 5, count, "Servis bi trebalo da javi da je prebačeno 5 fondova")
}

func TestCalculateAndSaveDailyHistory_Success(t *testing.T) {
	ctx := context.Background()

	fund1 := model.InvestmentFund{
		FundID:        1,
		AccountNumber: "FUND-123",
		Positions: []model.ClientFundPosition{
			{TotalInvestedAmount: 500},
			{TotalInvestedAmount: 1500},
		},
	}
	fund2 := model.InvestmentFund{
		FundID:        2,
		AccountNumber: "FUND-456",
		Positions: []model.ClientFundPosition{
			{TotalInvestedAmount: 1000},
		},
	}

	fundRepo := &fakeFundRepo{
		findAllResult: []model.InvestmentFund{fund1, fund2},
	}

	bankingClient := &fakeFundBankingClient{
		getAccountResult: &pb.GetAccountByNumberResponse{
			AvailableBalance: 1000.0,
		},
	}

	svc := newTestFundService(fundRepo, &fakeAssetOwnershipRepo{}, &fakeListingRepo{}, bankingClient, &fakeFundUserClient{})

	err := svc.CalculateAndSaveDailyHistory(ctx)
	require.NoError(t, err)

	require.Len(t, fundRepo.savedPerformances, 2)

	rec1 := fundRepo.savedPerformances[0]
	require.Equal(t, uint(1), rec1.FundID)
	require.Equal(t, 1000.0, rec1.FundValue)
	require.Equal(t, 1000.0, rec1.LiquidAssets)
	require.Equal(t, -1000.0, rec1.Profit)

	rec2 := fundRepo.savedPerformances[1]
	require.Equal(t, uint(2), rec2.FundID)
	require.Equal(t, 1000.0, rec2.FundValue)
	require.Equal(t, 1000.0, rec2.LiquidAssets)
	require.Equal(t, 0.0, rec2.Profit)
}

func TestCalculateAndSaveDailyHistory_ErrorHandlingSkip(t *testing.T) {
	ctx := context.Background()

	fund1 := model.InvestmentFund{
		FundID:        1,
		AccountNumber: "FUND-123",
	}
	fund2 := model.InvestmentFund{
		FundID:        2,
		AccountNumber: "FUND-ERROR",
	}

	fundRepo := &fakeFundRepo{
		findAllResult: []model.InvestmentFund{fund1, fund2},
	}

	customBankingClient := &testCustomBankingClient{
		fakeFundBankingClient: fakeFundBankingClient{
			getAccountResult: &pb.GetAccountByNumberResponse{
				AvailableBalance: 1000.0,
			},
		},
		getAccountByNumberFunc: func(ctx context.Context, accNum string) (*pb.GetAccountByNumberResponse, error) {
			if accNum == "FUND-ERROR" {
				return nil, errors.New("banking api error")
			}
			return &pb.GetAccountByNumberResponse{AvailableBalance: 2000.0}, nil
		},
	}

	exchange := defaultExchange()
	svc := NewInvestmentFundService(
		fundRepo,
		&fakePositionRepo{},
		&fakeListingRepo{},
		&fakeInvestmentRepo{},
		nil,
		&fakeAssetOwnershipRepo{},
		&fakeExchangeRepo{exchange: exchange},
		nil,
		nil,
		nil,
		nil,
		customBankingClient,
		&fakeFundUserClient{},
		nil,
	)

	err := svc.CalculateAndSaveDailyHistory(ctx)
	require.NoError(t, err)

	require.Len(t, fundRepo.savedPerformances, 1)
	require.Equal(t, uint(1), fundRepo.savedPerformances[0].FundID)
	require.Equal(t, 2000.0, fundRepo.savedPerformances[0].FundValue)
}

func TestCalculateAndSaveDailyHistory_RepositoryError(t *testing.T) {
	ctx := context.Background()

	fundRepo := &fakeFundRepo{
		findAllErr: errors.New("database connection failed"),
	}

	svc := newTestFundService(fundRepo, &fakeAssetOwnershipRepo{}, &fakeListingRepo{}, &fakeFundBankingClient{}, &fakeFundUserClient{})

	err := svc.CalculateAndSaveDailyHistory(ctx)
	require.Error(t, err)
	require.Equal(t, "database connection failed", err.Error())
}

type testCustomBankingClient struct {
	fakeFundBankingClient
	getAccountByNumberFunc func(ctx context.Context, accountNumber string) (*pb.GetAccountByNumberResponse, error)
}

func (c *testCustomBankingClient) GetAccountByNumber(ctx context.Context, accountNumber string) (*pb.GetAccountByNumberResponse, error) {
	if c.getAccountByNumberFunc != nil {
		return c.getAccountByNumberFunc(ctx, accountNumber)
	}
	return c.fakeFundBankingClient.GetAccountByNumber(ctx, accountNumber)
}
func (c *testCustomBankingClient) HasActiveLoan(_ context.Context, _ uint64) (*pb.HasActiveLoanResponse, error) {
	return nil, nil
}
func (c *testCustomBankingClient) CreatePaymentWithoutVerification(_ context.Context, _ *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error) {
	return nil, nil
}
func (c *testCustomBankingClient) GetAccountsByClientID(_ context.Context, _ uint64) (*pb.GetAccountsByClientIDResponse, error) {
	return nil, nil
}
func (c *testCustomBankingClient) ConvertCurrency(_ context.Context, amount float64, _, _ string) (float64, error) {
	return amount, nil
}
func (c *testCustomBankingClient) ExecuteTradeSettlement(_ context.Context, _, _ string, _ pb.TradeSettlementDirection, _ float64) (*pb.ExecuteTradeSettlementResponse, error) {
	return nil, nil
}
func (c *testCustomBankingClient) GetAccountCurrency(_ context.Context, _ string) (string, error) {
	return "RSD", nil
}
func (c *testCustomBankingClient) CreateFundAccount(_ context.Context, _ string, _ uint64) (string, error) {
	return "", nil
}

func TestMapFundPaymentError_NotFound(t *testing.T) {
	err := status.Error(codes.NotFound, "account not found")
	mapped := mapFundPaymentError(err)
	require.Error(t, mapped)
	require.Contains(t, mapped.Error(), "account not found")
}

func TestMapFundPaymentError_FailedPrecondition(t *testing.T) {
	err := status.Error(codes.FailedPrecondition, "insufficient funds")
	mapped := mapFundPaymentError(err)
	require.Error(t, mapped)
	require.Contains(t, mapped.Error(), "insufficient funds")
}

func TestMapFundPaymentError_Unknown(t *testing.T) {
	err := errors.New("something went wrong")
	mapped := mapFundPaymentError(err)
	require.Error(t, mapped)
	// ServiceUnavailableErr wraps the original error; the Code is 503
	var appErr *commonErrors.AppError
	require.True(t, errors.As(mapped, &appErr))
	require.Equal(t, 503, appErr.Code)
}

func TestProcessPendingRedemptions_Empty(t *testing.T) {
	redemptionRepo := &fakeRedemptionRepo{pending: []model.ClientFundRedemption{}}
	exchange := defaultExchange()
	svc := NewInvestmentFundService(
		&fakeFundRepo{}, &fakePositionRepo{}, &fakeListingRepo{},
		&fakeInvestmentRepo{}, redemptionRepo, &fakeAssetOwnershipRepo{},
		&fakeExchangeRepo{exchange: exchange}, &fakeStockRepo{},
		&fakeOptionRepo{}, &fakeFuturesRepo{}, &fakeForexRepo{},
		&fakeFundBankingClient{}, &fakeFundUserClient{}, nil,
	)

	err := svc.ProcessPendingRedemptions(context.Background())
	require.NoError(t, err)
}

func TestProcessPendingRedemptions_RepoError(t *testing.T) {
	redemptionRepo := &fakeRedemptionRepo{findErr: errors.New("db down")}
	exchange := defaultExchange()
	svc := NewInvestmentFundService(
		&fakeFundRepo{}, &fakePositionRepo{}, &fakeListingRepo{},
		&fakeInvestmentRepo{}, redemptionRepo, &fakeAssetOwnershipRepo{},
		&fakeExchangeRepo{exchange: exchange}, &fakeStockRepo{},
		&fakeOptionRepo{}, &fakeFuturesRepo{}, &fakeForexRepo{},
		&fakeFundBankingClient{}, &fakeFundUserClient{}, nil,
	)

	err := svc.ProcessPendingRedemptions(context.Background())
	require.Error(t, err)
}

func TestProcessPendingRedemptions_OneSuccess(t *testing.T) {
	fund := &model.InvestmentFund{
		FundID:        1,
		Name:          "Test Fund",
		AccountNumber: "fund-acc",
	}
	redemptionRepo := &fakeRedemptionRepo{
		pending: []model.ClientFundRedemption{
			{
				ClientFundRedemptionID: 10,
				ClientID:               99,
				OwnerType:              model.OwnerTypeClient,
				FundID:                 1,
				Fund:                   *fund,
				AccountNumber:          "client-acc",
				Amount:                 500,
				Status:                 model.FundRedemptionPendingLiquidation,
			},
		},
	}
	positionRepo := &fakePositionRepo{
		findResult: &model.ClientFundPosition{
			ClientID:            99,
			OwnerType:           model.OwnerTypeClient,
			FundID:              1,
			TotalInvestedAmount: 2000,
		},
	}
	bankingClient := &fakeFundBankingClient{
		accountsByNumber: map[string]*pb.GetAccountByNumberResponse{
			"fund-acc":   {AccountNumber: "fund-acc", AvailableBalance: 5000, CurrencyCode: "RSD"},
			"client-acc": {AccountNumber: "client-acc", ClientId: 99, CurrencyCode: "RSD"},
		},
	}

	exchange := defaultExchange()
	svc := NewInvestmentFundService(
		&fakeFundRepo{findByIDResult: fund}, positionRepo, &fakeListingRepo{},
		&fakeInvestmentRepo{}, redemptionRepo, &fakeAssetOwnershipRepo{},
		&fakeExchangeRepo{exchange: exchange}, &fakeStockRepo{},
		&fakeOptionRepo{}, &fakeFuturesRepo{}, &fakeForexRepo{},
		bankingClient, &fakeFundUserClient{}, nil,
	)

	err := svc.ProcessPendingRedemptions(context.Background())
	require.NoError(t, err)
	require.Len(t, bankingClient.payments, 1)
}

func TestProcessPendingRedemption_Success(t *testing.T) {
	fund := &model.InvestmentFund{
		FundID:        1,
		Name:          "Test Fund",
		AccountNumber: "fund-acc",
	}
	redemption := &model.ClientFundRedemption{
		ClientFundRedemptionID: 10,
		ClientID:               99,
		OwnerType:              model.OwnerTypeClient,
		FundID:                 1,
		Fund:                   *fund,
		AccountNumber:          "client-acc",
		Amount:                 500,
		Status:                 model.FundRedemptionPendingLiquidation,
	}
	positionRepo := &fakePositionRepo{
		findResult: &model.ClientFundPosition{
			ClientID:            99,
			OwnerType:           model.OwnerTypeClient,
			FundID:              1,
			TotalInvestedAmount: 2000,
		},
	}
	redemptionRepo := &fakeRedemptionRepo{}
	bankingClient := &fakeFundBankingClient{
		accountsByNumber: map[string]*pb.GetAccountByNumberResponse{
			"fund-acc":   {AccountNumber: "fund-acc", AvailableBalance: 5000, CurrencyCode: "RSD"},
			"client-acc": {AccountNumber: "client-acc", ClientId: 99, CurrencyCode: "RSD"},
		},
	}

	exchange := defaultExchange()
	svc := NewInvestmentFundService(
		&fakeFundRepo{findByIDResult: fund}, positionRepo, &fakeListingRepo{},
		&fakeInvestmentRepo{}, redemptionRepo, &fakeAssetOwnershipRepo{},
		&fakeExchangeRepo{exchange: exchange}, &fakeStockRepo{},
		&fakeOptionRepo{}, &fakeFuturesRepo{}, &fakeForexRepo{},
		bankingClient, &fakeFundUserClient{}, nil,
	)

	err := svc.processPendingRedemption(context.Background(), redemption)
	require.NoError(t, err)
	require.Len(t, bankingClient.payments, 1)
	require.Equal(t, "fund-acc", bankingClient.payments[0].PayerAccountNumber)
	require.Equal(t, "client-acc", bankingClient.payments[0].RecipientAccountNumber)
	require.NotNil(t, positionRepo.upserted)
	require.Equal(t, 1500.0, positionRepo.upserted.TotalInvestedAmount)
}

func TestProcessPendingRedemption_LegacyPositionUsesNAV(t *testing.T) {
	fund := &model.InvestmentFund{
		FundID:        1,
		Name:          "Test Fund",
		AccountNumber: "fund-acc",
	}
	redemption := &model.ClientFundRedemption{
		ClientFundRedemptionID: 10,
		ClientID:               99,
		OwnerType:              model.OwnerTypeClient,
		FundID:                 1,
		Fund:                   *fund,
		AccountNumber:          "client-acc",
		Amount:                 1500,
		Status:                 model.FundRedemptionPendingLiquidation,
	}
	positionRepo := &fakePositionRepo{
		findResult: &model.ClientFundPosition{
			ClientID:            99,
			OwnerType:           model.OwnerTypeClient,
			FundID:              1,
			TotalInvestedAmount: 1000,
		},
		findByFundRes: []model.ClientFundPosition{
			{ClientID: 99, OwnerType: model.OwnerTypeClient, FundID: 1, TotalInvestedAmount: 1000},
			{ClientID: 100, OwnerType: model.OwnerTypeClient, FundID: 1, TotalInvestedAmount: 1000},
		},
	}
	redemptionRepo := &fakeRedemptionRepo{}
	bankingClient := &fakeFundBankingClient{
		accountsByNumber: map[string]*pb.GetAccountByNumberResponse{
			"fund-acc":   {AccountNumber: "fund-acc", AvailableBalance: 5000, CurrencyCode: "RSD"},
			"client-acc": {AccountNumber: "client-acc", ClientId: 99, CurrencyCode: "RSD"},
		},
	}

	exchange := defaultExchange()
	svc := NewInvestmentFundService(
		&fakeFundRepo{findByIDResult: fund}, positionRepo, &fakeListingRepo{},
		&fakeInvestmentRepo{}, redemptionRepo, &fakeAssetOwnershipRepo{},
		&fakeExchangeRepo{exchange: exchange}, &fakeStockRepo{},
		&fakeOptionRepo{}, &fakeFuturesRepo{}, &fakeForexRepo{},
		bankingClient, &fakeFundUserClient{}, nil,
	)

	err := svc.processPendingRedemption(context.Background(), redemption)
	require.NoError(t, err)
	require.Len(t, bankingClient.payments, 1)
	require.NotNil(t, positionRepo.upserted)
	require.InDelta(t, 400.0, positionRepo.upserted.UnitsOwned, 0.0001)
	require.InDelta(t, 400.0, positionRepo.upserted.TotalInvestedAmount, 0.0001)
}

func TestProcessPendingRedemption_BankingClientFails(t *testing.T) {
	fund := &model.InvestmentFund{
		FundID:        1,
		Name:          "Test Fund",
		AccountNumber: "fund-acc",
	}
	redemption := &model.ClientFundRedemption{
		ClientFundRedemptionID: 10,
		ClientID:               99,
		OwnerType:              model.OwnerTypeClient,
		FundID:                 1,
		Fund:                   *fund,
		AccountNumber:          "client-acc",
		Amount:                 500,
		Status:                 model.FundRedemptionPendingLiquidation,
	}
	positionRepo := &fakePositionRepo{
		findResult: &model.ClientFundPosition{
			ClientID:            99,
			OwnerType:           model.OwnerTypeClient,
			FundID:              1,
			TotalInvestedAmount: 2000,
		},
	}

	bankingClient := &fakeFundBankingClient{
		accountsByNumber: map[string]*pb.GetAccountByNumberResponse{
			"fund-acc":   {AccountNumber: "fund-acc", AvailableBalance: 5000, CurrencyCode: "RSD"},
			"client-acc": {AccountNumber: "client-acc", ClientId: 99, CurrencyCode: "RSD"},
		},
		paymentErr: errors.New("payment service down"),
	}

	exchange := defaultExchange()
	svc := NewInvestmentFundService(
		&fakeFundRepo{findByIDResult: fund}, positionRepo, &fakeListingRepo{},
		&fakeInvestmentRepo{}, &fakeRedemptionRepo{}, &fakeAssetOwnershipRepo{},
		&fakeExchangeRepo{exchange: exchange}, &fakeStockRepo{},
		&fakeOptionRepo{}, &fakeFuturesRepo{}, &fakeForexRepo{},
		bankingClient, &fakeFundUserClient{}, nil,
	)

	err := svc.processPendingRedemption(context.Background(), redemption)
	require.Error(t, err)
	var appErr *commonErrors.AppError
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, 503, appErr.Code)
}

func TestLiquidateFundAssets_EmptyOwnerships(t *testing.T) {
	ownershipRepo := &fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{}}
	svc := newTestFundService(&fakeFundRepo{}, ownershipRepo, &fakeListingRepo{}, &fakeFundBankingClient{}, &fakeFundUserClient{})

	fund := &model.InvestmentFund{FundID: 1, AccountNumber: "fund-acc"}
	count, err := svc.liquidateFundAssets(context.Background(), fund, 1000)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestLiquidateFundAssets_OwnershipRepoError(t *testing.T) {
	ownershipRepo := &fakeAssetOwnershipRepo{findErr: errors.New("db error")}
	svc := newTestFundService(&fakeFundRepo{}, ownershipRepo, &fakeListingRepo{}, &fakeFundBankingClient{}, &fakeFundUserClient{})

	fund := &model.InvestmentFund{FundID: 1, AccountNumber: "fund-acc"}
	_, err := svc.liquidateFundAssets(context.Background(), fund, 1000)
	require.Error(t, err)
}

func TestLiquidateFundAssets_AllZeroAmountOwnerships(t *testing.T) {
	ownershipRepo := &fakeAssetOwnershipRepo{
		ownerships: []model.AssetOwnership{
			{AssetID: 1, Amount: 0},
			{AssetID: 2, Amount: -1},
		},
	}
	svc := newTestFundService(&fakeFundRepo{}, ownershipRepo, &fakeListingRepo{}, &fakeFundBankingClient{}, &fakeFundUserClient{})

	fund := &model.InvestmentFund{FundID: 1, AccountNumber: "fund-acc"}
	count, err := svc.liquidateFundAssets(context.Background(), fund, 1000)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestLiquidateFundAssets_ListingRepoError(t *testing.T) {
	ownershipRepo := &fakeAssetOwnershipRepo{
		ownerships: []model.AssetOwnership{
			{AssetID: 10, Amount: 5},
		},
	}
	listingRepo := &fakeListingRepo{byAssetIDsErr: errors.New("listing db error")}
	svc := newTestFundService(&fakeFundRepo{}, ownershipRepo, listingRepo, &fakeFundBankingClient{}, &fakeFundUserClient{})

	fund := &model.InvestmentFund{FundID: 1, AccountNumber: "fund-acc"}
	_, err := svc.liquidateFundAssets(context.Background(), fund, 1000)
	require.Error(t, err)
}

func TestLiquidateFundAssets_CurrencyConversionError(t *testing.T) {
	ownershipRepo := &fakeAssetOwnershipRepo{
		ownerships: []model.AssetOwnership{
			{AssetID: 10, Amount: 5},
		},
	}
	exchange := &model.Exchange{Currency: "USD"}
	listingRepo := &fakeListingRepo{
		byAssetIDs: []model.Listing{
			{ListingID: 100, AssetID: 10, Price: 50, Exchange: exchange},
		},
	}
	bankingClient := &fakeFundBankingClient{
		convertCurrencyFunc: func(amount float64, from, to string) (float64, error) {
			return 0, errors.New("conversion service down")
		},
	}
	svc := newTestFundService(&fakeFundRepo{}, ownershipRepo, listingRepo, bankingClient, &fakeFundUserClient{})

	fund := &model.InvestmentFund{FundID: 1, AccountNumber: "fund-acc"}
	_, err := svc.liquidateFundAssets(context.Background(), fund, 1000)
	require.Error(t, err)
	var appErr *commonErrors.AppError
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, 503, appErr.Code)
}

func TestLiquidateFundAssets_NoCandidatesAfterFiltering(t *testing.T) {
	ownershipRepo := &fakeAssetOwnershipRepo{
		ownerships: []model.AssetOwnership{
			{AssetID: 10, Amount: 5},
		},
	}
	// listing has Price <= 0
	listingRepo := &fakeListingRepo{
		byAssetIDs: []model.Listing{
			{ListingID: 100, AssetID: 10, Price: 0},
		},
	}
	svc := newTestFundService(&fakeFundRepo{}, ownershipRepo, listingRepo, &fakeFundBankingClient{}, &fakeFundUserClient{})

	fund := &model.InvestmentFund{FundID: 1, AccountNumber: "fund-acc"}
	count, err := svc.liquidateFundAssets(context.Background(), fund, 1000)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestLiquidateFundAssets_ConvertedPriceZero(t *testing.T) {
	ownershipRepo := &fakeAssetOwnershipRepo{
		ownerships: []model.AssetOwnership{
			{AssetID: 10, Amount: 5},
		},
	}
	listingRepo := &fakeListingRepo{
		byAssetIDs: []model.Listing{
			{ListingID: 100, AssetID: 10, Price: 50},
		},
	}
	bankingClient := &fakeFundBankingClient{
		convertCurrencyFunc: func(amount float64, from, to string) (float64, error) {
			return 0, nil // price converts to zero
		},
	}
	svc := newTestFundService(&fakeFundRepo{}, ownershipRepo, listingRepo, bankingClient, &fakeFundUserClient{})

	fund := &model.InvestmentFund{FundID: 1, AccountNumber: "fund-acc"}
	count, err := svc.liquidateFundAssets(context.Background(), fund, 1000)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestLiquidateFundAssets_NilOrderService(t *testing.T) {
	ownershipRepo := &fakeAssetOwnershipRepo{
		ownerships: []model.AssetOwnership{
			{AssetID: 10, Amount: 5},
		},
	}
	listingRepo := &fakeListingRepo{
		byAssetIDs: []model.Listing{
			{ListingID: 100, AssetID: 10, Price: 50},
		},
	}
	// newTestFundService passes nil for orderService
	svc := newTestFundService(&fakeFundRepo{}, ownershipRepo, listingRepo, &fakeFundBankingClient{}, &fakeFundUserClient{})

	fund := &model.InvestmentFund{FundID: 1, AccountNumber: "fund-acc"}
	_, err := svc.liquidateFundAssets(context.Background(), fund, 1000)
	require.Error(t, err)
	var appErr *commonErrors.AppError
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, 503, appErr.Code)
}

// newTestFundServiceWithOrderService creates an InvestmentFundService with a real
// OrderService wired from fake repositories, suitable for testing liquidateFundAssets.
func newTestFundServiceWithOrderService(
	ownershipRepo *fakeAssetOwnershipRepo,
	fundListingRepo *fakeListingRepo,
	bankingClient *fakeFundBankingClient,
	orderBankingClient *fakeOrderBankingClient,
	orderListingRepo *fakeListingRepo,
	orderRepo *fakeOrderRepo,
	userClient *fakeUserServiceClient,
) *InvestmentFundService {
	exchange := defaultExchange()
	orderSvc := NewOrderService(
		orderRepo,
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{exchange: exchange},
		orderListingRepo,
		ownershipRepo,
		&fakeFuturesRepo{},
		&fakeOptionRepo{},
		&fakeFundRepo{},
		userClient,
		orderBankingClient,
		&fakeTaxRecorder{},
		fakeAuditService(nil),
	)
	return NewInvestmentFundService(
		&fakeFundRepo{},
		&fakePositionRepo{},
		fundListingRepo,
		&fakeInvestmentRepo{},
		&fakeRedemptionRepo{},
		ownershipRepo,
		&fakeExchangeRepo{exchange: exchange},
		&fakeStockRepo{},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		bankingClient,
		&fakeFundUserClient{},
		orderSvc,
	)
}

func TestLiquidateFundAssets_SuccessfulLiquidation(t *testing.T) {
	ownershipRepo := &fakeAssetOwnershipRepo{
		ownerships: []model.AssetOwnership{
			{AssetID: 10, Amount: 5, UserId: 1, OwnerType: model.OwnerTypeFund},
		},
	}
	fundListingRepo := &fakeListingRepo{
		byAssetIDs: []model.Listing{
			{ListingID: 100, AssetID: 10, Price: 200, ExchangeMIC: "XTST"},
		},
	}
	// Used by OrderService.CreateFundLiquidationOrder -> placeOrder -> listingRepo.FindByID
	orderListingRepo := &fakeListingRepo{
		listing: &model.Listing{
			ListingID:   100,
			AssetID:     10,
			Price:       200,
			Ask:         201,
			ExchangeMIC: "XTST",
			// nil Asset skips settlement date validation and sell ownership check
		},
	}
	orderBankingClient := &fakeOrderBankingClient{
		accountResp: &pb.GetAccountByNumberResponse{
			AccountNumber:    "fund-acc",
			AccountType:      "Fund",
			AvailableBalance: 50000,
		},
	}
	userClient := &fakeUserServiceClient{
		employeeResp: &pb.GetEmployeeByIdResponse{
			Id:           25,
			IsSupervisor: true,
			IsAgent:      true,
		},
	}
	orderRepo := &fakeOrderRepo{}

	svc := newTestFundServiceWithOrderService(
		ownershipRepo, fundListingRepo, &fakeFundBankingClient{},
		orderBankingClient, orderListingRepo, orderRepo, userClient,
	)

	fund := &model.InvestmentFund{FundID: 1, ManagerID: 25, AccountNumber: "fund-acc"}
	count, err := svc.liquidateFundAssets(context.Background(), fund, 500)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.NotNil(t, orderRepo.capturedOrder)
	require.Equal(t, model.OrderDirectionSell, orderRepo.capturedOrder.Direction)
}

func TestLiquidateFundAssets_OrderCreationError(t *testing.T) {
	ownershipRepo := &fakeAssetOwnershipRepo{
		ownerships: []model.AssetOwnership{
			{AssetID: 10, Amount: 5, UserId: 1, OwnerType: model.OwnerTypeFund},
		},
	}
	fundListingRepo := &fakeListingRepo{
		byAssetIDs: []model.Listing{
			{ListingID: 100, AssetID: 10, Price: 200, ExchangeMIC: "XTST"},
		},
	}
	// Make the order listing repo return nil to trigger a "listing not found" error in placeOrder
	orderListingRepo := &fakeListingRepo{listing: nil}
	orderBankingClient := &fakeOrderBankingClient{
		accountResp: &pb.GetAccountByNumberResponse{
			AccountNumber: "fund-acc",
			AccountType:   "Fund",
		},
	}
	orderRepo := &fakeOrderRepo{}
	userClient := &fakeUserServiceClient{}

	svc := newTestFundServiceWithOrderService(
		ownershipRepo, fundListingRepo, &fakeFundBankingClient{},
		orderBankingClient, orderListingRepo, orderRepo, userClient,
	)

	fund := &model.InvestmentFund{FundID: 1, ManagerID: 25, AccountNumber: "fund-acc"}
	count, err := svc.liquidateFundAssets(context.Background(), fund, 500)
	require.Error(t, err)
	require.Equal(t, 0, count)
}

func TestLiquidateFundAssets_MultipleCandidatesWithBudget(t *testing.T) {
	ownershipRepo := &fakeAssetOwnershipRepo{
		ownerships: []model.AssetOwnership{
			{AssetID: 10, Amount: 3, UserId: 1, OwnerType: model.OwnerTypeFund},
			{AssetID: 20, Amount: 10, UserId: 1, OwnerType: model.OwnerTypeFund},
		},
	}
	fundListingRepo := &fakeListingRepo{
		byAssetIDs: []model.Listing{
			{ListingID: 100, AssetID: 10, Price: 500, ExchangeMIC: "XTST"},
			{ListingID: 200, AssetID: 20, Price: 100, ExchangeMIC: "XTST"},
		},
	}
	// Both listings returned by FindByID in order
	callCount := 0
	orderListingRepo := &fakeListingRepo{}
	// We need FindByID to return different listings for different IDs.
	// fakeListingRepo only has one listing field, so the same listing is returned
	// for both calls. We'll set it to a generic one that works for both.
	orderListingRepo.listing = &model.Listing{
		ListingID:   100,
		AssetID:     10,
		Price:       500,
		Ask:         501,
		ExchangeMIC: "XTST",
	}
	_ = callCount // not needed with single listing stub

	orderBankingClient := &fakeOrderBankingClient{
		accountResp: &pb.GetAccountByNumberResponse{
			AccountNumber:    "fund-acc",
			AccountType:      "Fund",
			AvailableBalance: 50000,
		},
	}
	userClient := &fakeUserServiceClient{
		employeeResp: &pb.GetEmployeeByIdResponse{
			Id:           25,
			IsSupervisor: true,
			IsAgent:      true,
		},
	}
	orderRepo := &fakeOrderRepo{}

	svc := newTestFundServiceWithOrderService(
		ownershipRepo, fundListingRepo, &fakeFundBankingClient{},
		orderBankingClient, orderListingRepo, orderRepo, userClient,
	)

	// Target = 800 RSD, candidate 1 value = 3*500=1500 (highest), candidate 2 = 10*100=1000
	// Sorted by value: candidate 1 first. Need ceil(800/500)=2 units, available 3. Creates order for 2.
	// remaining = 800 - 2*500 = -200, so loop breaks after first candidate.
	fund := &model.InvestmentFund{FundID: 1, ManagerID: 25, AccountNumber: "fund-acc"}
	count, err := svc.liquidateFundAssets(context.Background(), fund, 800)
	require.NoError(t, err)
	require.Equal(t, 1, count) // only first candidate needed
}

func TestLiquidateFundAssets_ListingNoMatchingOwnership(t *testing.T) {
	ownershipRepo := &fakeAssetOwnershipRepo{
		ownerships: []model.AssetOwnership{
			{AssetID: 10, Amount: 5},
		},
	}
	// Listing for a different assetID that has no ownership
	listingRepo := &fakeListingRepo{
		byAssetIDs: []model.Listing{
			{ListingID: 100, AssetID: 99, Price: 50},
		},
	}
	svc := newTestFundService(&fakeFundRepo{}, ownershipRepo, listingRepo, &fakeFundBankingClient{}, &fakeFundUserClient{})

	fund := &model.InvestmentFund{FundID: 1, AccountNumber: "fund-acc"}
	count, err := svc.liquidateFundAssets(context.Background(), fund, 1000)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestLiquidateFundAssets_FractionalAmount(t *testing.T) {
	// Ownership amount is 0.5 -> math.Floor(0.5) = 0, should skip candidate
	ownershipRepo := &fakeAssetOwnershipRepo{
		ownerships: []model.AssetOwnership{
			{AssetID: 10, Amount: 0.5},
		},
	}
	listingRepo := &fakeListingRepo{
		byAssetIDs: []model.Listing{
			{ListingID: 100, AssetID: 10, Price: 100},
		},
	}

	orderBankingClient := &fakeOrderBankingClient{
		accountResp: &pb.GetAccountByNumberResponse{
			AccountNumber: "fund-acc",
			AccountType:   "Fund",
		},
	}
	userClient := &fakeUserServiceClient{
		employeeResp: &pb.GetEmployeeByIdResponse{
			Id:           25,
			IsSupervisor: true,
		},
	}
	orderRepo := &fakeOrderRepo{}
	orderListingRepo := &fakeListingRepo{
		listing: &model.Listing{ListingID: 100, AssetID: 10, Price: 100, ExchangeMIC: "XTST"},
	}

	svc := newTestFundServiceWithOrderService(
		ownershipRepo, listingRepo, &fakeFundBankingClient{},
		orderBankingClient, orderListingRepo, orderRepo, userClient,
	)

	fund := &model.InvestmentFund{FundID: 1, ManagerID: 25, AccountNumber: "fund-acc"}
	count, err := svc.liquidateFundAssets(context.Background(), fund, 1000)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func TestLiquidateFundAssets_ExchangeCurrencyUsed(t *testing.T) {
	ownershipRepo := &fakeAssetOwnershipRepo{
		ownerships: []model.AssetOwnership{
			{AssetID: 10, Amount: 5},
		},
	}
	exchange := &model.Exchange{Currency: "EUR"}
	listingRepo := &fakeListingRepo{
		byAssetIDs: []model.Listing{
			{ListingID: 100, AssetID: 10, Price: 50, Exchange: exchange},
		},
	}
	var capturedFrom string
	bankingClient := &fakeFundBankingClient{
		convertCurrencyFunc: func(amount float64, from, to string) (float64, error) {
			capturedFrom = from
			return amount * 117.0, nil // EUR -> RSD
		},
	}
	// nil orderService so we hit that error, but at least verify currency was EUR
	svc := newTestFundService(&fakeFundRepo{}, ownershipRepo, listingRepo, bankingClient, &fakeFundUserClient{})

	fund := &model.InvestmentFund{FundID: 1, AccountNumber: "fund-acc"}
	_, _ = svc.liquidateFundAssets(context.Background(), fund, 1000)
	require.Equal(t, "EUR", capturedFrom)
}

func TestValidateFundAccount_GRPCNotFoundError(t *testing.T) {
	bankingClient := &testCustomBankingClient{
		getAccountByNumberFunc: func(_ context.Context, _ string) (*pb.GetAccountByNumberResponse, error) {
			return nil, status.Error(codes.NotFound, "account not found")
		},
	}
	exchange := defaultExchange()
	svc := NewInvestmentFundService(
		&fakeFundRepo{}, &fakePositionRepo{}, &fakeListingRepo{},
		&fakeInvestmentRepo{}, &fakeRedemptionRepo{}, &fakeAssetOwnershipRepo{},
		&fakeExchangeRepo{exchange: exchange}, &fakeStockRepo{},
		&fakeOptionRepo{}, &fakeFuturesRepo{}, &fakeForexRepo{},
		bankingClient, &fakeFundUserClient{}, nil,
	)

	clientID := uint(99)
	authCtx := &auth.AuthContext{
		IdentityID:   1,
		IdentityType: auth.IdentityClient,
		ClientID:     &clientID,
	}
	_, err := svc.validateFundAccount(context.Background(), "ACC-123", authCtx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestValidateFundAccount_GRPCOtherError(t *testing.T) {
	bankingClient := &testCustomBankingClient{
		getAccountByNumberFunc: func(_ context.Context, _ string) (*pb.GetAccountByNumberResponse, error) {
			return nil, status.Error(codes.Internal, "internal error")
		},
	}
	exchange := defaultExchange()
	svc := NewInvestmentFundService(
		&fakeFundRepo{}, &fakePositionRepo{}, &fakeListingRepo{},
		&fakeInvestmentRepo{}, &fakeRedemptionRepo{}, &fakeAssetOwnershipRepo{},
		&fakeExchangeRepo{exchange: exchange}, &fakeStockRepo{},
		&fakeOptionRepo{}, &fakeFuturesRepo{}, &fakeForexRepo{},
		bankingClient, &fakeFundUserClient{}, nil,
	)

	clientID := uint(99)
	authCtx := &auth.AuthContext{
		IdentityID:   1,
		IdentityType: auth.IdentityClient,
		ClientID:     &clientID,
	}
	_, err := svc.validateFundAccount(context.Background(), "ACC-123", authCtx)
	require.Error(t, err)
	var appErr *commonErrors.AppError
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, 503, appErr.Code)
}

func TestValidateFundAccount_NilAccount(t *testing.T) {
	bankingClient := &testCustomBankingClient{
		getAccountByNumberFunc: func(_ context.Context, _ string) (*pb.GetAccountByNumberResponse, error) {
			return nil, nil
		},
	}
	exchange := defaultExchange()
	svc := NewInvestmentFundService(
		&fakeFundRepo{}, &fakePositionRepo{}, &fakeListingRepo{},
		&fakeInvestmentRepo{}, &fakeRedemptionRepo{}, &fakeAssetOwnershipRepo{},
		&fakeExchangeRepo{exchange: exchange}, &fakeStockRepo{},
		&fakeOptionRepo{}, &fakeFuturesRepo{}, &fakeForexRepo{},
		bankingClient, &fakeFundUserClient{}, nil,
	)

	clientID := uint(99)
	authCtx := &auth.AuthContext{
		IdentityID:   1,
		IdentityType: auth.IdentityClient,
		ClientID:     &clientID,
	}
	_, err := svc.validateFundAccount(context.Background(), "ACC-123", authCtx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestValidateFundAccount_ClientDoesNotOwnAccount(t *testing.T) {
	bankingClient := &testCustomBankingClient{
		getAccountByNumberFunc: func(_ context.Context, _ string) (*pb.GetAccountByNumberResponse, error) {
			return &pb.GetAccountByNumberResponse{
				AccountNumber: "ACC-123",
				ClientId:      999, // different client
				AccountType:   "Current",
			}, nil
		},
	}
	exchange := defaultExchange()
	svc := NewInvestmentFundService(
		&fakeFundRepo{}, &fakePositionRepo{}, &fakeListingRepo{},
		&fakeInvestmentRepo{}, &fakeRedemptionRepo{}, &fakeAssetOwnershipRepo{},
		&fakeExchangeRepo{exchange: exchange}, &fakeStockRepo{},
		&fakeOptionRepo{}, &fakeFuturesRepo{}, &fakeForexRepo{},
		bankingClient, &fakeFundUserClient{}, nil,
	)

	clientID := uint(99)
	authCtx := &auth.AuthContext{
		IdentityID:   1,
		IdentityType: auth.IdentityClient,
		ClientID:     &clientID,
	}
	_, err := svc.validateFundAccount(context.Background(), "ACC-123", authCtx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not belong")
}

func TestValidateFundAccount_ClientNilClientID(t *testing.T) {
	bankingClient := &testCustomBankingClient{
		getAccountByNumberFunc: func(_ context.Context, _ string) (*pb.GetAccountByNumberResponse, error) {
			return &pb.GetAccountByNumberResponse{
				AccountNumber: "ACC-123",
				ClientId:      99,
				AccountType:   "Current",
			}, nil
		},
	}
	exchange := defaultExchange()
	svc := NewInvestmentFundService(
		&fakeFundRepo{}, &fakePositionRepo{}, &fakeListingRepo{},
		&fakeInvestmentRepo{}, &fakeRedemptionRepo{}, &fakeAssetOwnershipRepo{},
		&fakeExchangeRepo{exchange: exchange}, &fakeStockRepo{},
		&fakeOptionRepo{}, &fakeFuturesRepo{}, &fakeForexRepo{},
		bankingClient, &fakeFundUserClient{}, nil,
	)

	authCtx := &auth.AuthContext{
		IdentityID:   1,
		IdentityType: auth.IdentityClient,
		ClientID:     nil, // nil client ID
	}
	_, err := svc.validateFundAccount(context.Background(), "ACC-123", authCtx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not belong")
}

func TestValidateFundAccount_EmployeeNonBankAccount(t *testing.T) {
	bankingClient := &testCustomBankingClient{
		getAccountByNumberFunc: func(_ context.Context, _ string) (*pb.GetAccountByNumberResponse, error) {
			return &pb.GetAccountByNumberResponse{
				AccountNumber: "ACC-123",
				AccountType:   "Current", // not Bank
			}, nil
		},
	}
	exchange := defaultExchange()
	svc := NewInvestmentFundService(
		&fakeFundRepo{}, &fakePositionRepo{}, &fakeListingRepo{},
		&fakeInvestmentRepo{}, &fakeRedemptionRepo{}, &fakeAssetOwnershipRepo{},
		&fakeExchangeRepo{exchange: exchange}, &fakeStockRepo{},
		&fakeOptionRepo{}, &fakeFuturesRepo{}, &fakeForexRepo{},
		bankingClient, &fakeFundUserClient{}, nil,
	)

	employeeID := uint(25)
	authCtx := &auth.AuthContext{
		IdentityID:   200,
		IdentityType: auth.IdentityEmployee,
		EmployeeID:   &employeeID,
	}
	_, err := svc.validateFundAccount(context.Background(), "ACC-123", authCtx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "bank account")
}

func TestValidateFundAccount_ClientSuccess(t *testing.T) {
	bankingClient := &testCustomBankingClient{
		getAccountByNumberFunc: func(_ context.Context, _ string) (*pb.GetAccountByNumberResponse, error) {
			return &pb.GetAccountByNumberResponse{
				AccountNumber: "ACC-123",
				ClientId:      99,
				AccountType:   "Current",
			}, nil
		},
	}
	exchange := defaultExchange()
	svc := NewInvestmentFundService(
		&fakeFundRepo{}, &fakePositionRepo{}, &fakeListingRepo{},
		&fakeInvestmentRepo{}, &fakeRedemptionRepo{}, &fakeAssetOwnershipRepo{},
		&fakeExchangeRepo{exchange: exchange}, &fakeStockRepo{},
		&fakeOptionRepo{}, &fakeFuturesRepo{}, &fakeForexRepo{},
		bankingClient, &fakeFundUserClient{}, nil,
	)

	clientID := uint(99)
	authCtx := &auth.AuthContext{
		IdentityID:   1,
		IdentityType: auth.IdentityClient,
		ClientID:     &clientID,
	}
	account, err := svc.validateFundAccount(context.Background(), "ACC-123", authCtx)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, "ACC-123", account.GetAccountNumber())
}

func TestValidateFundAccount_EmployeeBankAccountSuccess(t *testing.T) {
	bankingClient := &testCustomBankingClient{
		getAccountByNumberFunc: func(_ context.Context, _ string) (*pb.GetAccountByNumberResponse, error) {
			return &pb.GetAccountByNumberResponse{
				AccountNumber: "BANK-ACC",
				AccountType:   "Bank",
			}, nil
		},
	}
	exchange := defaultExchange()
	svc := NewInvestmentFundService(
		&fakeFundRepo{}, &fakePositionRepo{}, &fakeListingRepo{},
		&fakeInvestmentRepo{}, &fakeRedemptionRepo{}, &fakeAssetOwnershipRepo{},
		&fakeExchangeRepo{exchange: exchange}, &fakeStockRepo{},
		&fakeOptionRepo{}, &fakeFuturesRepo{}, &fakeForexRepo{},
		bankingClient, &fakeFundUserClient{}, nil,
	)

	employeeID := uint(25)
	authCtx := &auth.AuthContext{
		IdentityID:   200,
		IdentityType: auth.IdentityEmployee,
		EmployeeID:   &employeeID,
	}
	account, err := svc.validateFundAccount(context.Background(), "BANK-ACC", authCtx)
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, "BANK-ACC", account.GetAccountNumber())
}

func TestGetClientFundPositions_UsesUnitsForShareAndProfit(t *testing.T) {
	fund := model.InvestmentFund{
		FundID:        1,
		Name:          "Growth Fund",
		Description:   "Growth strategy",
		AccountNumber: "fund-account",
	}

	positionRepo := &fakePositionRepo{
		findByClientRes: []model.ClientFundPosition{
			{
				ClientID:            1,
				OwnerType:           model.OwnerTypeClient,
				FundID:              fund.FundID,
				Fund:                &fund,
				UnitsOwned:          1000,
				TotalInvestedAmount: 1000,
			},
		},
		findByFundRes: []model.ClientFundPosition{
			{ClientID: 1, OwnerType: model.OwnerTypeClient, FundID: fund.FundID, UnitsOwned: 1000, TotalInvestedAmount: 1000},
			{ClientID: 2, OwnerType: model.OwnerTypeClient, FundID: fund.FundID, UnitsOwned: 1000, TotalInvestedAmount: 1000},
			{ClientID: 3, OwnerType: model.OwnerTypeClient, FundID: fund.FundID, UnitsOwned: 500, TotalInvestedAmount: 1000},
		},
	}

	bankingClient := &fakeFundBankingClient{
		accountsByNumber: map[string]*pb.GetAccountByNumberResponse{
			"fund-account": {AccountNumber: "fund-account", AvailableBalance: 4000, CurrencyCode: "RSD"},
		},
	}

	exchange := defaultExchange()
	svc := NewInvestmentFundService(
		&fakeFundRepo{findByIDResult: &fund},
		positionRepo,
		&fakeListingRepo{},
		&fakeInvestmentRepo{},
		&fakeRedemptionRepo{},
		&fakeAssetOwnershipRepo{},
		&fakeExchangeRepo{exchange: exchange},
		&fakeStockRepo{},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		bankingClient,
		&fakeFundUserClient{},
		nil,
	)

	resp, err := svc.GetClientFundPositions(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, resp, 1)
	require.InDelta(t, 40.0, resp[0].ClientsSharePercent, 0.0001)
	require.InDelta(t, 1600.0, resp[0].ClientsShareValueRSD, 0.0001)
	require.InDelta(t, 600.0, resp[0].TotalProfit, 0.0001)
}

func TestGetClientFundPositions_MixedLegacyAndUnitsShareProfit(t *testing.T) {
	fund := model.InvestmentFund{
		FundID:        1,
		Name:          "Mixed Fund",
		Description:   "legacy + units",
		AccountNumber: "fund-account",
	}

	positionRepo := &fakePositionRepo{
		findByClientRes: []model.ClientFundPosition{
			{
				ClientID:            1,
				OwnerType:           model.OwnerTypeClient,
				FundID:              fund.FundID,
				Fund:                &fund,
				TotalInvestedAmount: 1000,
			},
		},
		findByFundRes: []model.ClientFundPosition{
			{ClientID: 1, OwnerType: model.OwnerTypeClient, FundID: fund.FundID, TotalInvestedAmount: 1000},
			{ClientID: 2, OwnerType: model.OwnerTypeClient, FundID: fund.FundID, UnitsOwned: 500, TotalInvestedAmount: 500},
		},
	}

	bankingClient := &fakeFundBankingClient{
		accountsByNumber: map[string]*pb.GetAccountByNumberResponse{
			"fund-account": {AccountNumber: "fund-account", AvailableBalance: 3000, CurrencyCode: "RSD"},
		},
	}

	exchange := defaultExchange()
	svc := NewInvestmentFundService(
		&fakeFundRepo{findByIDResult: &fund},
		positionRepo,
		&fakeListingRepo{},
		&fakeInvestmentRepo{},
		&fakeRedemptionRepo{},
		&fakeAssetOwnershipRepo{},
		&fakeExchangeRepo{exchange: exchange},
		&fakeStockRepo{},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		bankingClient,
		&fakeFundUserClient{},
		nil,
	)

	resp, err := svc.GetClientFundPositions(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, resp, 1)
	// total units = 1000 (legacy fallback) + 500 = 1500
	// client 1 share = 1000/1500 = 66.6667%
	// fund value = 3000, share value = 2000, profit = 2000 - 1000 = 1000
	require.InDelta(t, 66.6667, resp[0].ClientsSharePercent, 0.0001)
	require.InDelta(t, 2000.0, resp[0].ClientsShareValueRSD, 0.0001)
	require.InDelta(t, 1000.0, resp[0].TotalProfit, 0.0001)
}

func TestInvestInFund_UsesNAVToCalculateUnits(t *testing.T) {
	fund := &model.InvestmentFund{
		FundID:              1,
		Name:                "Growth Fund",
		MinimumContribution: 10,
		AccountNumber:       "fund-account",
	}

	positionRepo := &fakePositionRepo{
		findByFundRes: []model.ClientFundPosition{
			{ClientID: 1, OwnerType: model.OwnerTypeClient, FundID: 1, UnitsOwned: 1000, TotalInvestedAmount: 1000},
			{ClientID: 2, OwnerType: model.OwnerTypeClient, FundID: 1, UnitsOwned: 1000, TotalInvestedAmount: 1000},
		},
	}

	bankingClient := &fakeFundBankingClient{
		accountsByNumber: map[string]*pb.GetAccountByNumberResponse{
			"client-account": {AccountNumber: "client-account", ClientId: 99, AccountType: "Current", CurrencyCode: "RSD"},
			"fund-account":   {AccountNumber: "fund-account", AccountType: "Fund", CurrencyCode: "RSD", AvailableBalance: 4000},
		},
	}

	exchange := defaultExchange()
	svc := NewInvestmentFundService(
		&fakeFundRepo{findByIDResult: fund},
		positionRepo,
		&fakeListingRepo{},
		&fakeInvestmentRepo{},
		&fakeRedemptionRepo{},
		&fakeAssetOwnershipRepo{},
		&fakeExchangeRepo{exchange: exchange},
		&fakeStockRepo{},
		&fakeOptionRepo{},
		&fakeFuturesRepo{},
		&fakeForexRepo{},
		bankingClient,
		&fakeFundUserClient{},
		nil,
	)

	resp, err := svc.InvestInFund(fundClientCtx(), 1, dto.InvestInFundRequest{
		AccountNumber: "client-account",
		Amount:        1000,
	})
	require.NoError(t, err)
	require.NotNil(t, positionRepo.upserted)
	require.InDelta(t, 500.0, positionRepo.upserted.UnitsOwned, 0.0001)
	require.InDelta(t, 1000.0, positionRepo.upserted.TotalInvestedAmount, 0.0001)
	require.Equal(t, 1000.0, resp.TotalInvestedRSD)
}

func TestApplyRedemptionToPosition_MultiplePartialRedemptionsPreserveProportionalCostBasis(t *testing.T) {
	position := &model.ClientFundPosition{
		ClientID:            99,
		OwnerType:           model.OwnerTypeClient,
		FundID:              1,
		UnitsOwned:          1000,
		TotalInvestedAmount: 1000,
	}

	nav := 2.0
	applyRedemptionToPosition(position, 500, nav)
	require.InDelta(t, 750.0, position.UnitsOwned, 0.0001)
	require.InDelta(t, 750.0, position.TotalInvestedAmount, 0.0001)

	applyRedemptionToPosition(position, 500, nav)
	require.InDelta(t, 500.0, position.UnitsOwned, 0.0001)
	require.InDelta(t, 500.0, position.TotalInvestedAmount, 0.0001)

	avgCostPerUnit := position.TotalInvestedAmount / position.UnitsOwned
	require.InDelta(t, 1.0, avgCostPerUnit, 0.0001)
}
