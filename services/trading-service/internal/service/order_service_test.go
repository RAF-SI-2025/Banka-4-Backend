package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/permission"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
)

// ── Fake Order Repository ─────────────────────────────────────────

type fakeOrderRepo struct {
	// FindAll
	orders  []model.Order
	total   int64
	findErr error

	// FindByID
	orderByID   *model.Order
	findByIDErr error

	// Create / Save
	createErr error
	saveErr   error

	// FindReadyForExecution
	readyOrders []model.Order
	readyErr    error

	// captured
	capturedOrder  *model.Order
	capturedUserID *uint

	// new fields for FindUserOrders
	userOrders          []model.Order
	userOrdersTotal     int64
	userOrdersErr       error
	capturedUserQuery   dto.UserOrdersQuery
	capturedUserIDOrder uint
	capturedOwnerType   model.OwnerType
}

func (r *fakeOrderRepo) FindAll(_ context.Context, _, _ int, userID *uint, _ *model.OwnerType, _ *model.OrderStatus, _ *model.OrderDirection, _ *bool) ([]model.Order, int64, error) {
	r.capturedUserID = userID
	return r.orders, r.total, r.findErr
}

func (r *fakeOrderRepo) FindByID(_ context.Context, _ uint) (*model.Order, error) {
	return r.orderByID, r.findByIDErr
}

func (r *fakeOrderRepo) Create(_ context.Context, order *model.Order) error {
	r.capturedOrder = order
	return r.createErr
}

func (r *fakeOrderRepo) Save(_ context.Context, order *model.Order) error {
	r.capturedOrder = order
	return r.saveErr
}

func (r *fakeOrderRepo) FindReadyForExecution(_ context.Context, _ time.Time, _ int) ([]model.Order, error) {
	return r.readyOrders, r.readyErr
}

func (r *fakeOrderRepo) FindUserOrders(ctx context.Context, userID uint, ownerType model.OwnerType, query dto.UserOrdersQuery) ([]model.Order, int64, error) {
	r.capturedUserIDOrder = userID
	r.capturedOwnerType = ownerType
	r.capturedUserQuery = query
	return r.userOrders, r.userOrdersTotal, r.userOrdersErr
}

// ── Fake Order Transaction Repository ─────────────────────────────

type fakeOrderTransactionRepo struct {
	createErr error
}

func (r *fakeOrderTransactionRepo) Create(_ context.Context, _ *model.OrderTransaction) error {
	return r.createErr
}

// ── Fake Exchange Repository ──────────────────────────────────────

type fakeExchangeRepo struct {
	exchange    *model.Exchange
	findErr     error
	exchanges   []model.Exchange
	findAllErr  error
	total       int64
	toggledExch *model.Exchange
	toggleErr   error
}

func (r *fakeExchangeRepo) FindByMicCode(_ context.Context, _ string) (*model.Exchange, error) {
	return r.exchange, r.findErr
}

func (r *fakeExchangeRepo) FindAll(_ context.Context, _, _ int) ([]model.Exchange, int64, error) {
	return r.exchanges, r.total, r.findAllErr
}

func (r *fakeExchangeRepo) ToggleTradingEnabled(_ context.Context, _ string) (*model.Exchange, error) {
	return r.toggledExch, r.toggleErr
}

// ── Fake Listing Repository ───────────────────────────────────────

type fakeListingRepo struct {
	listing        *model.Listing
	findByIDErr    error
	dailyPriceInfo *model.ListingDailyPriceInfo
	dailyPriceErr  error

	// stubs for the rest of the interface
	allListings   []model.Listing
	findAllErr    error
	countVal      int64
	countErr      error
	byAssetIDs    []model.Listing
	byAssetIDsErr error
}

func (r *fakeListingRepo) FindByID(_ context.Context, _ uint, daysBack int) (*model.Listing, error) {
	return r.listing, r.findByIDErr
}

func (r *fakeListingRepo) FindLatestDailyPriceInfo(_ context.Context, _ uint) (*model.ListingDailyPriceInfo, error) {
	return r.dailyPriceInfo, r.dailyPriceErr
}

func (r *fakeListingRepo) FindAll(_ context.Context) ([]model.Listing, error) {
	return r.allListings, r.findAllErr
}

func (r *fakeListingRepo) FindStocks(_ context.Context, _ repository.ListingFilter) ([]model.Listing, int64, error) {
	return nil, 0, nil
}

func (r *fakeListingRepo) FindFutures(_ context.Context, _ repository.ListingFilter) ([]model.Listing, int64, error) {
	return nil, 0, nil
}

func (r *fakeListingRepo) FindOptions(_ context.Context, _ repository.ListingFilter) ([]model.Listing, int64, error) {
	return nil, 0, nil
}

func (r *fakeListingRepo) Upsert(_ context.Context, _ *model.Listing) error { return nil }

func (r *fakeListingRepo) UpdatePriceAndAsk(_ context.Context, _ *model.Listing, _, _ float64) error {
	return nil
}

func (r *fakeListingRepo) Count(_ context.Context) (int64, error) {
	return r.countVal, r.countErr
}

func (r *fakeListingRepo) CreateDailyPriceInfo(_ context.Context, _ *model.ListingDailyPriceInfo) error {
	return nil
}

func (r *fakeListingRepo) FindLastDailyPriceInfo(_ context.Context, _ uint, _ time.Time) (*model.ListingDailyPriceInfo, error) {
	return r.dailyPriceInfo, r.dailyPriceErr
}

func (r *fakeListingRepo) FindByAssetType(_ context.Context, _ model.AssetType) ([]model.Listing, error) {
	return nil, nil
}

func (r *fakeListingRepo) FindByAssetIDs(_ context.Context, _ []uint) ([]model.Listing, error) {
	return r.byAssetIDs, r.byAssetIDsErr
}

// ── Fake User Service Client ──────────────────────────────────────

type fakeUserServiceClient struct {
	employeeResp *pb.GetEmployeeByIdResponse
	employeeErr  error
	clientResp   *pb.GetClientByIdResponse
	clientErr    error
	clientsResp  *pb.GetClientsByIdsResponse
	clientsErr   error
	identityResp *pb.GetIdentityByUserIdResponse
	identityErr  error

	incrementUsedLimitResp       *pb.IncrementUsedLimitResponse
	incrementUsedLimitErr        error
	incrementUsedLimitCalled     bool
	incrementUsedLimitEmployeeID uint64
	incrementUsedLimitAmount     float64
}

func (c *fakeUserServiceClient) GetEmployeeById(_ context.Context, _ uint64) (*pb.GetEmployeeByIdResponse, error) {
	return c.employeeResp, c.employeeErr
}

func (c *fakeUserServiceClient) GetClientById(_ context.Context, _ uint64) (*pb.GetClientByIdResponse, error) {
	return c.clientResp, c.clientErr
}

func (c *fakeUserServiceClient) GetClientsByIds(_ context.Context, _ []uint64) (*pb.GetClientsByIdsResponse, error) {
	return c.clientsResp, c.clientsErr
}

func (c *fakeUserServiceClient) GetClientByIdentityId(_ context.Context, _ uint64) (*pb.GetClientByIdResponse, error) {
	return c.clientResp, c.clientErr
}

func (c *fakeUserServiceClient) GetEmployeeByIdentityId(_ context.Context, _ uint64) (*pb.GetEmployeeByIdResponse, error) {
	return c.employeeResp, c.employeeErr
}

func (c *fakeUserServiceClient) GetAllClients(_ context.Context, _, _ int32, _, _ string) (*pb.GetAllClientsResponse, error) {
	return nil, nil
}

func (c *fakeUserServiceClient) GetAllActuaries(_ context.Context, _, _ int32, _, _ string) (*pb.GetAllActuariesResponse, error) {
	return nil, nil
}

func (c *fakeUserServiceClient) GetIdentityByUserId(_ context.Context, _ uint64, _ string) (*pb.GetIdentityByUserIdResponse, error) {
	return c.identityResp, c.identityErr
}

func (c *fakeUserServiceClient) IncrementUsedLimit(_ context.Context, employeeID uint64, amount float64) (*pb.IncrementUsedLimitResponse, error) {
	c.incrementUsedLimitCalled = true
	c.incrementUsedLimitEmployeeID = employeeID
	c.incrementUsedLimitAmount = amount
	if c.incrementUsedLimitErr != nil {
		return nil, c.incrementUsedLimitErr
	}
	if c.incrementUsedLimitResp != nil {
		return c.incrementUsedLimitResp, nil
	}
	return &pb.IncrementUsedLimitResponse{UsedLimit: amount}, nil
}

// ── Fake Banking Client (order-specific) ──────────────────────────

type fakeOrderBankingClient struct {
	accountResp      *pb.GetAccountByNumberResponse
	accountErr       error
	hasActiveLoan    bool
	hasActiveLoanErr error
	settlementResp   *pb.ExecuteTradeSettlementResponse
	settlementErr    error
	accountCurrency  string
}

func (c *fakeOrderBankingClient) GetAccountCurrency(_ context.Context, _ string) (string, error) {
	if c.accountCurrency == "" {
		return "RSD", nil
	}
	return c.accountCurrency, nil
}
func (f *fakeOrderBankingClient) CreateFundAccount(ctx context.Context, fundName string, managerID uint64) (string, error) {
	return "", nil
}
func (c *fakeOrderBankingClient) GetAccountByNumber(_ context.Context, _ string) (*pb.GetAccountByNumberResponse, error) {
	return c.accountResp, c.accountErr
}

func (c *fakeOrderBankingClient) HasActiveLoan(_ context.Context, _ uint64) (*pb.HasActiveLoanResponse, error) {
	if c.hasActiveLoanErr != nil {
		return nil, c.hasActiveLoanErr
	}

	return &pb.HasActiveLoanResponse{HasActiveLoan: c.hasActiveLoan}, nil
}

func (c *fakeOrderBankingClient) CreatePaymentWithoutVerification(_ context.Context, _ *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error) {
	return nil, nil
}

func (c *fakeOrderBankingClient) GetAccountsByClientID(_ context.Context, _ uint64) (*pb.GetAccountsByClientIDResponse, error) {
	return nil, nil
}

func (c *fakeOrderBankingClient) ConvertCurrency(_ context.Context, amount float64, _, _ string) (float64, error) {
	return amount, nil
}

func (c *fakeOrderBankingClient) ExecuteTradeSettlement(_ context.Context, _, _ string, _ pb.TradeSettlementDirection, _ float64) (*pb.ExecuteTradeSettlementResponse, error) {
	return c.settlementResp, c.settlementErr
}

func (c *fakeOrderBankingClient) ReserveOtcFunds(_ context.Context, req *pb.ReserveOtcFundsRequest) (*pb.OtcFundsReservationResponse, error) {
	return &pb.OtcFundsReservationResponse{ExecutionId: req.ExecutionId, Status: pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_RESERVED}, nil
}

func (c *fakeOrderBankingClient) ReleaseOtcFunds(_ context.Context, executionID string) (*pb.OtcFundsReservationResponse, error) {
	return &pb.OtcFundsReservationResponse{ExecutionId: executionID, Status: pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_RELEASED}, nil
}

func (c *fakeOrderBankingClient) CommitOtcFunds(_ context.Context, executionID string) (*pb.OtcFundsReservationResponse, error) {
	return &pb.OtcFundsReservationResponse{ExecutionId: executionID, Status: pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_COMMITTED}, nil
}

func (c *fakeOrderBankingClient) RefundOtcFunds(_ context.Context, executionID string) (*pb.OtcFundsReservationResponse, error) {
	return &pb.OtcFundsReservationResponse{ExecutionId: executionID, Status: pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_REFUNDED}, nil
}

// ── Fake Tax Recorder ─────────────────────────────────────────────

type fakeTaxRecorder struct {
	recordedAccountNumber string
	recordedEmployeeID    *uint
	recordedProfit        float64
	recordedCurrency      string
	recordErr             error
	called                bool
}

func (f *fakeTaxRecorder) RecordTax(_ context.Context, accountNumber string, employeeID *uint, profit float64, currencyCode string) error {
	f.called = true
	f.recordedAccountNumber = accountNumber
	f.recordedEmployeeID = employeeID
	f.recordedProfit = profit
	f.recordedCurrency = currencyCode
	return f.recordErr
}

// ── Helpers ───────────────────────────────────────────────────────

func clientAuthCtx() context.Context {
	clientID := uint(10)
	return auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityID:   1,
		IdentityType: auth.IdentityClient,
		ClientID:     &clientID,
		Permissions:  []permission.Permission{permission.Trading, permission.TradingMargin},
	})
}

func employeeAuthCtx(employeeID uint) context.Context {
	return auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityID:   100,
		IdentityType: auth.IdentityEmployee,
		EmployeeID:   &employeeID,
		Permissions:  []permission.Permission{permission.Trading, permission.TradingMargin},
	})
}

func supervisorAuthCtx(employeeID uint) context.Context {
	return auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityID:   200,
		IdentityType: auth.IdentityEmployee,
		EmployeeID:   &employeeID,
		Permissions:  []permission.Permission{permission.Trading, permission.TradingMargin},
	})
}

func newTestOrderService(
	orderRepo *fakeOrderRepo,
	orderTxRepo *fakeOrderTransactionRepo,
	exchangeRepo *fakeExchangeRepo,
	listingRepo *fakeListingRepo,
	userClient *fakeUserServiceClient,
	bankingClient *fakeOrderBankingClient,
	taxRecorder *fakeTaxRecorder,
) *OrderService {
	svc := NewOrderService(orderRepo, orderTxRepo, exchangeRepo, listingRepo, &fakeAssetOwnershipRepo{}, &fakeFuturesRepo{}, &fakeOptionRepo{}, &fakeFundRepo{}, userClient, bankingClient, taxRecorder, fakeAuditService(nil))
	svc.now = func() time.Time {
		return time.Date(2025, 6, 4, 10, 0, 0, 0, time.UTC)
	}
	return svc
}

func defaultExchange() *model.Exchange {
	return &model.Exchange{
		ExchangeID:     1,
		Name:           "Test Exchange",
		Acronym:        "TST",
		MicCode:        "XTST",
		Polity:         "USA",
		Currency:       "USD",
		TimeZone:       0,
		OpenTime:       "09:00",
		CloseTime:      "16:00",
		TradingEnabled: true,
	}
}

func defaultListing() *model.Listing {
	return &model.Listing{
		ListingID:         1,
		ExchangeMIC:       "XTST",
		Price:             150.0,
		Ask:               151.0,
		MaintenanceMargin: 10.0,
		Asset: &model.Asset{
			Ticker:    "AAPL",
			Name:      "Apple Inc",
			AssetType: model.AssetTypeStock,
		},
	}
}

func defaultAccountResp(clientID uint64) *pb.GetAccountByNumberResponse {
	return &pb.GetAccountByNumberResponse{
		ClientId:         clientID,
		AccountType:      "Current",
		CurrencyCode:     "USD",
		AvailableBalance: 1000,
	}
}

func defaultFundAccountResp(managerID uint64) *pb.GetAccountByNumberResponse {
	return &pb.GetAccountByNumberResponse{
		ClientId:         managerID,
		AccountType:      "Bank",
		CurrencyCode:     "RSD",
		AvailableBalance: 100000,
	}
}

// ── GetOrders Tests ───────────────────────────────────────────────

func TestGetOrders_Success(t *testing.T) {
	orders := []model.Order{
		{OrderID: 1, OrderOwnerUserID: 1, Status: model.OrderStatusApproved},
		{OrderID: 2, OrderOwnerUserID: 2, Status: model.OrderStatusPending},
	}
	repo := &fakeOrderRepo{orders: orders, total: 2}
	svc := newTestOrderService(repo, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, &fakeListingRepo{}, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	result, total, err := svc.GetOrders(context.Background(), dto.ListOrdersQuery{Page: 0, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.Equal(t, int64(2), total)
}

func TestGetOrders_Empty(t *testing.T) {
	repo := &fakeOrderRepo{orders: []model.Order{}, total: 0}
	svc := newTestOrderService(repo, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, &fakeListingRepo{}, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	result, total, err := svc.GetOrders(context.Background(), dto.ListOrdersQuery{Page: 0, PageSize: 10})
	require.NoError(t, err)
	require.Empty(t, result)
	require.Equal(t, int64(0), total)
}

func TestGetOrders_RepoError(t *testing.T) {
	repo := &fakeOrderRepo{findErr: errors.New("db error")}
	svc := newTestOrderService(repo, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, &fakeListingRepo{}, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	result, total, err := svc.GetOrders(context.Background(), dto.ListOrdersQuery{})
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, int64(0), total)
}

// ── CreateOrder Tests ─────────────────────────────────────────────

func TestCreateOrder_MarketBuy_ClientAutoApproved(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()

	svc := newTestOrderService(
		&fakeOrderRepo{},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{exchange: exchange},
		&fakeListingRepo{listing: listing},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{accountResp: defaultAccountResp(10)},
		&fakeTaxRecorder{},
	)

	ctx := clientAuthCtx()
	req := dto.CreateOrderRequest{
		ListingID:     1,
		AccountNumber: "444000100000000110",
		OrderType:     model.OrderTypeMarket,
		Direction:     model.OrderDirectionBuy,
		Quantity:      10,
	}

	order, err := svc.CreateOrder(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, model.OrderStatusApproved, order.Status)
	require.Equal(t, model.OrderTypeMarket, order.OrderType)
	require.Equal(t, model.OrderDirectionBuy, order.Direction)
	require.Equal(t, uint(10), order.Quantity)
	require.True(t, order.Triggered)
	require.True(t, order.CommissionExempt == false)
}

func TestCreateOrder_LimitSell_Success(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()
	limitVal := 155.0

	ownershipRepo := &fakeAssetOwnershipRepo{
		ownerships: []model.AssetOwnership{
			{AssetID: listing.AssetID, Amount: 10},
		},
	}
	svc := NewOrderService(
		&fakeOrderRepo{},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{exchange: exchange},
		&fakeListingRepo{listing: listing},
		ownershipRepo,
		&fakeFuturesRepo{},
		&fakeOptionRepo{},
		&fakeFundRepo{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{accountResp: defaultAccountResp(10)},
		&fakeTaxRecorder{},
		fakeAuditService(nil),
	)
	svc.now = func() time.Time {
		return time.Date(2025, 6, 4, 10, 0, 0, 0, time.UTC)
	}

	svc.assetOwnershipRepo = &fakeAssetOwnershipRepo{
		ownerships: []model.AssetOwnership{
			{AssetID: listing.AssetID, UserId: 1, OwnerType: model.OwnerTypeClient, Amount: 10},
		},
	}
	ctx := clientAuthCtx()
	req := dto.CreateOrderRequest{
		ListingID:     1,
		AccountNumber: "444000100000000110",
		OrderType:     model.OrderTypeLimit,
		Direction:     model.OrderDirectionSell,
		Quantity:      5,
		LimitValue:    &limitVal,
	}

	order, err := svc.CreateOrder(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, model.OrderTypeLimit, order.OrderType)
	require.Equal(t, model.OrderDirectionSell, order.Direction)
	require.NotNil(t, order.LimitValue)
	require.Equal(t, 155.0, *order.LimitValue)
}

func TestCreateOrder_ClientMargin_WithActiveLoanAndFunds_Success(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()

	svc := newTestOrderService(
		&fakeOrderRepo{},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{exchange: exchange},
		&fakeListingRepo{listing: listing},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{accountResp: defaultAccountResp(10), hasActiveLoan: true},
		&fakeTaxRecorder{},
	)

	ctx := clientAuthCtx()

	req := dto.CreateOrderRequest{
		ListingID:     1,
		AccountNumber: "444000100000000110",
		OrderType:     model.OrderTypeMarket,
		Direction:     model.OrderDirectionBuy,
		Quantity:      10,
		Margin:        true,
	}

	order, err := svc.CreateOrder(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, order)
	require.True(t, order.Margin)
}

func TestCreateOrder_ClientMargin_WithoutActiveLoan_Forbidden(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()

	svc := newTestOrderService(
		&fakeOrderRepo{},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{exchange: exchange},
		&fakeListingRepo{listing: listing},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{accountResp: defaultAccountResp(10), hasActiveLoan: false},
		&fakeTaxRecorder{},
	)

	ctx := clientAuthCtx()
	req := dto.CreateOrderRequest{
		ListingID:     1,
		AccountNumber: "444000100000000110",
		OrderType:     model.OrderTypeMarket,
		Direction:     model.OrderDirectionBuy,
		Quantity:      10,
		Margin:        true,
	}

	order, err := svc.CreateOrder(ctx, req)
	require.Error(t, err)
	require.Nil(t, order)
	require.Contains(t, err.Error(), "active loan required for margin trading")
}

func TestCreateOrder_EmployeeMargin_WithoutPermission_Forbidden(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()

	svc := newTestOrderService(
		&fakeOrderRepo{},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{exchange: exchange},
		&fakeListingRepo{listing: listing},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{accountResp: &pb.GetAccountByNumberResponse{
			AccountType:      "Bank",
			CurrencyCode:     "USD",
			AvailableBalance: 1000,
		}},
		&fakeTaxRecorder{},
	)

	employeeID := uint(5)
	ctx := auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityID:   100,
		IdentityType: auth.IdentityEmployee,
		EmployeeID:   &employeeID,
		Permissions:  []permission.Permission{permission.Trading},
	})
	req := dto.CreateOrderRequest{
		ListingID:     1,
		AccountNumber: "444000100000000110",
		OrderType:     model.OrderTypeMarket,
		Direction:     model.OrderDirectionBuy,
		Quantity:      10,
		Margin:        true,
	}

	order, err := svc.CreateOrder(ctx, req)
	require.Error(t, err)
	require.Nil(t, order)
	require.Contains(t, err.Error(), "margin trading permission required")
}

func TestCreateOrder_Margin_WithInsufficientFunds_Forbidden(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()

	accountResp := defaultAccountResp(10)
	accountResp.AvailableBalance = 10
	svc := newTestOrderService(
		&fakeOrderRepo{},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{exchange: exchange},
		&fakeListingRepo{listing: listing},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{accountResp: accountResp, hasActiveLoan: true},
		&fakeTaxRecorder{},
	)

	ctx := clientAuthCtx()
	req := dto.CreateOrderRequest{
		ListingID:     1,
		AccountNumber: "444000100000000110",
		OrderType:     model.OrderTypeMarket,
		Direction:     model.OrderDirectionBuy,
		Quantity:      10,
		Margin:        true,
	}

	order, err := svc.CreateOrder(ctx, req)
	require.Error(t, err)
	require.Nil(t, order)
	require.Contains(t, err.Error(), "insufficient account funds")
}

func TestCreateOrder_MissingAuthContext(t *testing.T) {
	svc := newTestOrderService(
		&fakeOrderRepo{},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{},
		&fakeListingRepo{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{},
		&fakeTaxRecorder{},
	)

	req := dto.CreateOrderRequest{
		ListingID:     1,
		AccountNumber: "444000100000000110",
		OrderType:     model.OrderTypeMarket,
		Direction:     model.OrderDirectionBuy,
		Quantity:      10,
	}

	order, err := svc.CreateOrder(context.Background(), req)
	require.Error(t, err)
	require.Nil(t, order)
	require.Contains(t, err.Error(), "not authenticated")
}

func TestCreateOrder_ListingNotFound(t *testing.T) {
	svc := newTestOrderService(
		&fakeOrderRepo{},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{},
		&fakeListingRepo{listing: nil},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{accountResp: defaultAccountResp(10)},
		&fakeTaxRecorder{},
	)

	ctx := clientAuthCtx()
	req := dto.CreateOrderRequest{
		ListingID:     999,
		AccountNumber: "444000100000000110",
		OrderType:     model.OrderTypeMarket,
		Direction:     model.OrderDirectionBuy,
		Quantity:      10,
	}

	order, err := svc.CreateOrder(ctx, req)
	require.Error(t, err)
	require.Nil(t, order)
	require.Contains(t, err.Error(), "listing not found")
}

func TestCreateOrder_AccountValidationFailure_NotFound(t *testing.T) {
	svc := newTestOrderService(
		&fakeOrderRepo{},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{},
		&fakeListingRepo{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{accountErr: status.Error(codes.NotFound, "account not found")},
		&fakeTaxRecorder{},
	)

	ctx := clientAuthCtx()
	req := dto.CreateOrderRequest{
		ListingID:     1,
		AccountNumber: "444000100000000999",
		OrderType:     model.OrderTypeMarket,
		Direction:     model.OrderDirectionBuy,
		Quantity:      10,
	}

	order, err := svc.CreateOrder(ctx, req)
	require.Error(t, err)
	require.Nil(t, order)
	require.Contains(t, err.Error(), "account not found")
}

func TestCreateOrder_AccountDoesNotBelongToClient(t *testing.T) {
	svc := newTestOrderService(
		&fakeOrderRepo{},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{},
		&fakeListingRepo{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{accountResp: &pb.GetAccountByNumberResponse{ClientId: 999, AccountType: "Current"}},
		&fakeTaxRecorder{},
	)

	ctx := clientAuthCtx()
	req := dto.CreateOrderRequest{
		ListingID:     1,
		AccountNumber: "444000100000000110",
		OrderType:     model.OrderTypeMarket,
		Direction:     model.OrderDirectionBuy,
		Quantity:      10,
	}

	order, err := svc.CreateOrder(ctx, req)
	require.Error(t, err)
	require.Nil(t, order)
	require.Contains(t, err.Error(), "account does not belong to you")
}

func TestCreateOrder_EmployeeAgent_SufficientLimit_Approved(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()

	svc := newTestOrderService(
		&fakeOrderRepo{},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{exchange: exchange},
		&fakeListingRepo{listing: listing},
		&fakeUserServiceClient{
			employeeResp: &pb.GetEmployeeByIdResponse{
				Id:           5,
				IsAgent:      true,
				NeedApproval: false,
				OrderLimit:   1000000,
				UsedLimit:    0,
			},
			identityResp: &pb.GetIdentityByUserIdResponse{
				IdentityId: 5,
			},
		},
		&fakeOrderBankingClient{accountResp: &pb.GetAccountByNumberResponse{AccountType: "Bank"}},
		&fakeTaxRecorder{},
	)

	ctx := employeeAuthCtx(5)
	req := dto.CreateOrderRequest{
		ListingID:     1,
		AccountNumber: "444000100000000110",
		OrderType:     model.OrderTypeMarket,
		Direction:     model.OrderDirectionBuy,
		Quantity:      10,
	}

	order, err := svc.CreateOrder(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, model.OrderStatusApproved, order.Status)
	require.True(t, order.CommissionExempt)
}

func TestCreateOrder_EmployeeAgent_ExceedsLimit_Pending(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()

	svc := newTestOrderService(
		&fakeOrderRepo{},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{exchange: exchange},
		&fakeListingRepo{listing: listing},
		&fakeUserServiceClient{
			employeeResp: &pb.GetEmployeeByIdResponse{
				Id:           5,
				IsAgent:      true,
				NeedApproval: false,
				OrderLimit:   100,
				UsedLimit:    99,
			},
		},
		&fakeOrderBankingClient{accountResp: &pb.GetAccountByNumberResponse{AccountType: "Bank"}},
		&fakeTaxRecorder{},
	)

	ctx := employeeAuthCtx(5)
	req := dto.CreateOrderRequest{
		ListingID:     1,
		AccountNumber: "444000100000000110",
		OrderType:     model.OrderTypeMarket,
		Direction:     model.OrderDirectionBuy,
		Quantity:      10,
	}

	order, err := svc.CreateOrder(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, model.OrderStatusPending, order.Status)
}

func TestCreateOrder_EmployeeNeedsApproval_Pending(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()

	svc := newTestOrderService(
		&fakeOrderRepo{},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{exchange: exchange},
		&fakeListingRepo{listing: listing},
		&fakeUserServiceClient{
			employeeResp: &pb.GetEmployeeByIdResponse{
				Id:           5,
				IsAgent:      true,
				NeedApproval: true,
				OrderLimit:   1000000,
				UsedLimit:    0,
			},
			identityResp: &pb.GetIdentityByUserIdResponse{
				IdentityId: 5,
			},
		},
		&fakeOrderBankingClient{accountResp: &pb.GetAccountByNumberResponse{AccountType: "Bank"}},
		&fakeTaxRecorder{},
	)

	ctx := employeeAuthCtx(5)
	req := dto.CreateOrderRequest{
		ListingID:     1,
		AccountNumber: "444000100000000110",
		OrderType:     model.OrderTypeMarket,
		Direction:     model.OrderDirectionBuy,
		Quantity:      10,
	}

	order, err := svc.CreateOrder(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, model.OrderStatusPending, order.Status)
}

func TestCreateOrder_EmployeeNotAgent_Pending(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()

	svc := newTestOrderService(
		&fakeOrderRepo{},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{exchange: exchange},
		&fakeListingRepo{listing: listing},
		&fakeUserServiceClient{
			employeeResp: &pb.GetEmployeeByIdResponse{
				Id:      5,
				IsAgent: false,
			},
			identityResp: &pb.GetIdentityByUserIdResponse{
				IdentityId: 5,
			},
		},
		&fakeOrderBankingClient{accountResp: &pb.GetAccountByNumberResponse{AccountType: "Bank"}},
		&fakeTaxRecorder{},
	)

	ctx := employeeAuthCtx(5)
	req := dto.CreateOrderRequest{
		ListingID:     1,
		AccountNumber: "444000100000000110",
		OrderType:     model.OrderTypeMarket,
		Direction:     model.OrderDirectionBuy,
		Quantity:      10,
	}

	order, err := svc.CreateOrder(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, model.OrderStatusPending, order.Status)
}

func TestCreateOrder_LimitOrder_MissingLimitValue(t *testing.T) {
	svc := newTestOrderService(
		&fakeOrderRepo{},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{},
		&fakeListingRepo{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{},
		&fakeTaxRecorder{},
	)

	ctx := clientAuthCtx()
	req := dto.CreateOrderRequest{
		ListingID:     1,
		AccountNumber: "444000100000000110",
		OrderType:     model.OrderTypeLimit,
		Direction:     model.OrderDirectionBuy,
		Quantity:      10,
		LimitValue:    nil,
	}

	order, err := svc.CreateOrder(ctx, req)
	require.Error(t, err)
	require.Nil(t, order)
	require.Contains(t, err.Error(), "limitValue is required")
}

func TestCreateOrder_StopOrder_MissingStopValue(t *testing.T) {
	svc := newTestOrderService(
		&fakeOrderRepo{},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{},
		&fakeListingRepo{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{},
		&fakeTaxRecorder{},
	)

	ctx := clientAuthCtx()
	req := dto.CreateOrderRequest{
		ListingID:     1,
		AccountNumber: "444000100000000110",
		OrderType:     model.OrderTypeStop,
		Direction:     model.OrderDirectionBuy,
		Quantity:      10,
		StopValue:     nil,
	}

	order, err := svc.CreateOrder(ctx, req)
	require.Error(t, err)
	require.Nil(t, order)
	require.Contains(t, err.Error(), "stopValue is required")
}

func TestCreateOrder_EmployeeMustUseBankAccount(t *testing.T) {
	svc := newTestOrderService(
		&fakeOrderRepo{},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{},
		&fakeListingRepo{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{accountResp: &pb.GetAccountByNumberResponse{AccountType: "Current"}},
		&fakeTaxRecorder{},
	)

	ctx := employeeAuthCtx(5)
	req := dto.CreateOrderRequest{
		ListingID:     1,
		AccountNumber: "444000100000000110",
		OrderType:     model.OrderTypeMarket,
		Direction:     model.OrderDirectionBuy,
		Quantity:      10,
	}

	order, err := svc.CreateOrder(ctx, req)
	require.Error(t, err)
	require.Nil(t, order)
	require.Contains(t, err.Error(), "employees must use a bank account")
}

func TestCreateOrder_RepoCreateError(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()

	svc := newTestOrderService(
		&fakeOrderRepo{createErr: errors.New("db insert error")},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{exchange: exchange},
		&fakeListingRepo{listing: listing},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{accountResp: defaultAccountResp(10)},
		&fakeTaxRecorder{},
	)

	ctx := clientAuthCtx()
	req := dto.CreateOrderRequest{
		ListingID:     1,
		AccountNumber: "444000100000000110",
		OrderType:     model.OrderTypeMarket,
		Direction:     model.OrderDirectionBuy,
		Quantity:      10,
	}

	order, err := svc.CreateOrder(ctx, req)
	require.Error(t, err)
	require.Nil(t, order)
}

// ── ApproveOrder Tests ────────────────────────────────────────────

func TestApproveOrder_Success(t *testing.T) {
	ppu := 5.0
	pendingOrder := &model.Order{
		OrderID:      1,
		Status:       model.OrderStatusPending,
		Listing:      model.Listing{ExchangeMIC: "XTST"},
		PricePerUnit: &ppu,
	}

	svc := newTestOrderService(
		&fakeOrderRepo{orderByID: pendingOrder},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{exchange: defaultExchange()},
		&fakeListingRepo{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{},
		&fakeTaxRecorder{},
	)

	ctx := employeeAuthCtx(5)
	order, err := svc.ApproveOrder(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, model.OrderStatusApproved, order.Status)
	require.NotNil(t, order.ApprovedBy)
	require.Equal(t, uint(100), *order.ApprovedBy)
	require.NotNil(t, order.NextExecutionAt)
}

func TestApproveOrder_NotFound(t *testing.T) {
	svc := newTestOrderService(
		&fakeOrderRepo{orderByID: nil},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{},
		&fakeListingRepo{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{},
		&fakeTaxRecorder{},
	)

	ctx := employeeAuthCtx(5)
	order, err := svc.ApproveOrder(ctx, 999)
	require.Error(t, err)
	require.Nil(t, order)
	require.Contains(t, err.Error(), "order not found")
}

func TestApproveOrder_NotPending(t *testing.T) {
	approvedOrder := &model.Order{
		OrderID: 1,
		Status:  model.OrderStatusApproved,
	}

	svc := newTestOrderService(
		&fakeOrderRepo{orderByID: approvedOrder},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{},
		&fakeListingRepo{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{},
		&fakeTaxRecorder{},
	)

	ctx := employeeAuthCtx(5)
	order, err := svc.ApproveOrder(ctx, 1)
	require.Error(t, err)
	require.Nil(t, order)
	require.Contains(t, err.Error(), "only pending orders can be approved")
}

func TestApproveOrder_MissingAuth(t *testing.T) {
	pendingOrder := &model.Order{
		OrderID: 1,
		Status:  model.OrderStatusPending,
	}

	svc := newTestOrderService(
		&fakeOrderRepo{orderByID: pendingOrder},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{},
		&fakeListingRepo{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{},
		&fakeTaxRecorder{},
	)

	order, err := svc.ApproveOrder(context.Background(), 1)
	require.Error(t, err)
	require.Nil(t, order)
	require.Contains(t, err.Error(), "not authenticated")
}

// ── DeclineOrder Tests ────────────────────────────────────────────

func TestDeclineOrder_Success(t *testing.T) {
	pendingOrder := &model.Order{
		OrderID: 1,
		Status:  model.OrderStatusPending,
	}

	svc := newTestOrderService(
		&fakeOrderRepo{orderByID: pendingOrder},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{},
		&fakeListingRepo{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{},
		&fakeTaxRecorder{},
	)

	ctx := employeeAuthCtx(5)
	order, err := svc.DeclineOrder(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, model.OrderStatusDeclined, order.Status)
	require.True(t, order.IsDone)
	require.Nil(t, order.NextExecutionAt)
	require.NotNil(t, order.ApprovedBy)
}

func TestDeclineOrder_NotFound(t *testing.T) {
	svc := newTestOrderService(
		&fakeOrderRepo{orderByID: nil},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{},
		&fakeListingRepo{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{},
		&fakeTaxRecorder{},
	)

	ctx := employeeAuthCtx(5)
	order, err := svc.DeclineOrder(ctx, 999)
	require.Error(t, err)
	require.Nil(t, order)
	require.Contains(t, err.Error(), "order not found")
}

func TestDeclineOrder_NotPending(t *testing.T) {
	approvedOrder := &model.Order{
		OrderID: 1,
		Status:  model.OrderStatusApproved,
	}

	svc := newTestOrderService(
		&fakeOrderRepo{orderByID: approvedOrder},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{},
		&fakeListingRepo{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{},
		&fakeTaxRecorder{},
	)

	ctx := employeeAuthCtx(5)
	order, err := svc.DeclineOrder(ctx, 1)
	require.Error(t, err)
	require.Nil(t, order)
	require.Contains(t, err.Error(), "only pending orders can be declined")
}

func TestDeclineOrder_MissingAuth(t *testing.T) {
	pendingOrder := &model.Order{
		OrderID: 1,
		Status:  model.OrderStatusPending,
	}

	svc := newTestOrderService(
		&fakeOrderRepo{orderByID: pendingOrder},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{},
		&fakeListingRepo{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{},
		&fakeTaxRecorder{},
	)

	order, err := svc.DeclineOrder(context.Background(), 1)
	require.Error(t, err)
	require.Nil(t, order)
	require.Contains(t, err.Error(), "not authenticated")
}

// ── CancelOrder Tests ─────────────────────────────────────────────

func TestCancelOrder_OwnerCancelsOwn(t *testing.T) {
	pendingOrder := &model.Order{
		OrderID:          1,
		OrderOwnerUserID: 10,
		Status:           model.OrderStatusPending,
		IsDone:           false,
	}

	svc := newTestOrderService(
		&fakeOrderRepo{orderByID: pendingOrder},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{},
		&fakeListingRepo{},
		&fakeUserServiceClient{
			identityResp: &pb.GetIdentityByUserIdResponse{
				IdentityId: 1,
			},
		},
		&fakeOrderBankingClient{},
		&fakeTaxRecorder{},
	)

	ctx := clientAuthCtx()
	order, err := svc.CancelOrder(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, model.OrderStatusDeclined, order.Status)
	require.True(t, order.IsDone)
	require.Nil(t, order.NextExecutionAt)
}

func TestCancelOrder_SupervisorCancelsOther(t *testing.T) {
	pendingOrder := &model.Order{
		OrderID:          1,
		OrderOwnerUserID: 999,
		Status:           model.OrderStatusApproved,
		IsDone:           false,
	}

	svc := newTestOrderService(
		&fakeOrderRepo{orderByID: pendingOrder},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{},
		&fakeListingRepo{},
		&fakeUserServiceClient{
			employeeResp: &pb.GetEmployeeByIdResponse{
				Id:           7,
				IsSupervisor: true,
			},
			identityResp: &pb.GetIdentityByUserIdResponse{
				IdentityId: 5,
			},
		},
		&fakeOrderBankingClient{},
		&fakeTaxRecorder{},
	)

	ctx := supervisorAuthCtx(7)
	order, err := svc.CancelOrder(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, model.OrderStatusDeclined, order.Status)
	require.True(t, order.IsDone)
}

func TestCancelOrder_NonOwnerNonSupervisor_Forbidden(t *testing.T) {
	pendingOrder := &model.Order{
		OrderID:          1,
		OrderOwnerUserID: 999,
		Status:           model.OrderStatusPending,
		IsDone:           false,
	}

	svc := newTestOrderService(
		&fakeOrderRepo{orderByID: pendingOrder},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{},
		&fakeListingRepo{},
		&fakeUserServiceClient{
			employeeResp: &pb.GetEmployeeByIdResponse{
				Id:           5,
				IsSupervisor: false,
			},
			identityResp: &pb.GetIdentityByUserIdResponse{
				IdentityId: 5,
			},
		},
		&fakeOrderBankingClient{},
		&fakeTaxRecorder{},
	)

	ctx := employeeAuthCtx(5)
	order, err := svc.CancelOrder(ctx, 1)
	require.Error(t, err)
	require.Nil(t, order)
	require.Contains(t, err.Error(), "only the order owner or a supervisor")
}

func TestCancelOrder_AlreadyDone(t *testing.T) {
	doneOrder := &model.Order{
		OrderID:          1,
		OrderOwnerUserID: 10,
		Status:           model.OrderStatusApproved,
		IsDone:           true,
	}

	svc := newTestOrderService(
		&fakeOrderRepo{orderByID: doneOrder},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{},
		&fakeListingRepo{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 1}},
		&fakeOrderBankingClient{},
		&fakeTaxRecorder{},
	)

	ctx := clientAuthCtx()
	order, err := svc.CancelOrder(ctx, 1)
	require.Error(t, err)
	require.Nil(t, order)
	require.Contains(t, err.Error(), "cannot cancel a completed order")
}

func TestCancelOrder_NotFound(t *testing.T) {
	svc := newTestOrderService(
		&fakeOrderRepo{orderByID: nil},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{},
		&fakeListingRepo{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{},
		&fakeTaxRecorder{},
	)

	ctx := clientAuthCtx()
	order, err := svc.CancelOrder(ctx, 999)
	require.Error(t, err)
	require.Nil(t, order)
	require.Contains(t, err.Error(), "order not found")
}

func TestCancelOrder_DeclinedStatus_BadRequest(t *testing.T) {
	declinedOrder := &model.Order{
		OrderID:          1,
		OrderOwnerUserID: 10,
		Status:           model.OrderStatusDeclined,
		IsDone:           false,
	}

	svc := newTestOrderService(
		&fakeOrderRepo{orderByID: declinedOrder},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{},
		&fakeListingRepo{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 1}},
		&fakeOrderBankingClient{},
		&fakeTaxRecorder{},
	)

	ctx := clientAuthCtx()
	order, err := svc.CancelOrder(ctx, 1)
	require.Error(t, err)
	require.Nil(t, order)
	require.Contains(t, err.Error(), "only pending or approved orders can be cancelled")
}

func TestCancelOrder_MissingAuth(t *testing.T) {
	pendingOrder := &model.Order{
		OrderID:          1,
		OrderOwnerUserID: 1,
		Status:           model.OrderStatusPending,
		IsDone:           false,
	}

	svc := newTestOrderService(
		&fakeOrderRepo{orderByID: pendingOrder},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{},
		&fakeListingRepo{},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{},
		&fakeTaxRecorder{},
	)

	order, err := svc.CancelOrder(context.Background(), 1)
	require.Error(t, err)
	require.Nil(t, order)
	require.Contains(t, err.Error(), "not authenticated")
}

// ── Pure function tests ───────────────────────────────────────────

func TestValidateOrderTypeFields(t *testing.T) {
	require.NoError(t, validateOrderTypeFields(placeOrderParams{OrderType: model.OrderTypeMarket}))

	require.Error(t, validateOrderTypeFields(placeOrderParams{OrderType: model.OrderTypeLimit}))
	lv := 100.0
	require.NoError(t, validateOrderTypeFields(placeOrderParams{OrderType: model.OrderTypeLimit, LimitValue: &lv}))

	require.Error(t, validateOrderTypeFields(placeOrderParams{OrderType: model.OrderTypeStop}))
	sv := 90.0
	require.NoError(t, validateOrderTypeFields(placeOrderParams{OrderType: model.OrderTypeStop, StopValue: &sv}))

	require.Error(t, validateOrderTypeFields(placeOrderParams{OrderType: model.OrderTypeStopLimit, LimitValue: &lv}))
	require.Error(t, validateOrderTypeFields(placeOrderParams{OrderType: model.OrderTypeStopLimit, StopValue: &sv}))
	require.NoError(t, validateOrderTypeFields(placeOrderParams{OrderType: model.OrderTypeStopLimit, LimitValue: &lv, StopValue: &sv}))
}

func TestCalculateInitialPricePerUnit(t *testing.T) {
	listing := &model.Listing{Price: 150.0, Ask: 151.0}
	lv := 155.0
	sv := 145.0

	p := calculateInitialPricePerUnit(placeOrderParams{OrderType: model.OrderTypeMarket, Direction: model.OrderDirectionBuy}, listing)
	require.NotNil(t, p)
	require.Equal(t, 151.0, *p)

	p = calculateInitialPricePerUnit(placeOrderParams{OrderType: model.OrderTypeMarket, Direction: model.OrderDirectionSell}, listing)
	require.NotNil(t, p)
	require.Equal(t, 150.0, *p)

	p = calculateInitialPricePerUnit(placeOrderParams{OrderType: model.OrderTypeLimit, LimitValue: &lv}, listing)
	require.NotNil(t, p)
	require.Equal(t, 155.0, *p)

	p = calculateInitialPricePerUnit(placeOrderParams{OrderType: model.OrderTypeStop, StopValue: &sv}, listing)
	require.NotNil(t, p)
	require.Equal(t, 145.0, *p)
}

func TestCalculateCommission(t *testing.T) {
	require.Equal(t, 0.0, calculateCommission(model.OrderTypeMarket, 0))
	require.Equal(t, 0.0, calculateCommission(model.OrderTypeMarket, -10))
	require.InDelta(t, 0.14*10, calculateCommission(model.OrderTypeMarket, 10), 0.001)
	require.Equal(t, 7.0, calculateCommission(model.OrderTypeMarket, 100))

	require.InDelta(t, 0.24*10, calculateCommission(model.OrderTypeLimit, 10), 0.001)
	require.Equal(t, 12.0, calculateCommission(model.OrderTypeLimit, 100))
}

func TestNormalizeCurrencyCode(t *testing.T) {
	require.Equal(t, "USD", normalizeCurrencyCode("usd"))
	require.Equal(t, "EUR", normalizeCurrencyCode(" eur "))
	require.Equal(t, "RSD", normalizeCurrencyCode("RSD"))
}

func TestDereferencePrice(t *testing.T) {
	require.Equal(t, 0.0, dereferencePrice(nil))
	v := 42.5
	require.Equal(t, 42.5, dereferencePrice(&v))
}

func TestIsStopConditionMet(t *testing.T) {
	listing := &model.Listing{Price: 150.0, Ask: 151.0}

	order := &model.Order{StopValue: nil}
	require.True(t, isStopConditionMet(order, listing))

	sv := 150.0
	order = &model.Order{Direction: model.OrderDirectionBuy, StopValue: &sv}
	require.True(t, isStopConditionMet(order, listing))

	sv = 200.0
	order = &model.Order{Direction: model.OrderDirectionBuy, StopValue: &sv}
	require.False(t, isStopConditionMet(order, listing))

	sv = 150.0
	order = &model.Order{Direction: model.OrderDirectionSell, StopValue: &sv}
	require.True(t, isStopConditionMet(order, listing))

	sv = 100.0
	order = &model.Order{Direction: model.OrderDirectionSell, StopValue: &sv}
	require.False(t, isStopConditionMet(order, listing))
}

func TestResolveExecutionPrice(t *testing.T) {
	listing := &model.Listing{Price: 150.0, Ask: 151.0}

	p, ok := resolveExecutionPrice(&model.Order{OrderType: model.OrderTypeMarket, Direction: model.OrderDirectionBuy}, listing)
	require.True(t, ok)
	require.Equal(t, 151.0, p)

	p, ok = resolveExecutionPrice(&model.Order{OrderType: model.OrderTypeMarket, Direction: model.OrderDirectionSell}, listing)
	require.True(t, ok)
	require.Equal(t, 150.0, p)

	lv := 155.0
	p, ok = resolveExecutionPrice(&model.Order{OrderType: model.OrderTypeLimit, Direction: model.OrderDirectionBuy, LimitValue: &lv}, listing)
	require.True(t, ok)
	require.Equal(t, 151.0, p)

	lv = 140.0
	_, ok = resolveExecutionPrice(&model.Order{OrderType: model.OrderTypeLimit, Direction: model.OrderDirectionBuy, LimitValue: &lv}, listing)
	require.False(t, ok)

	lv = 140.0
	p, ok = resolveExecutionPrice(&model.Order{OrderType: model.OrderTypeLimit, Direction: model.OrderDirectionSell, LimitValue: &lv}, listing)
	require.True(t, ok)
	require.Equal(t, 150.0, p)

	lv = 160.0
	_, ok = resolveExecutionPrice(&model.Order{OrderType: model.OrderTypeLimit, Direction: model.OrderDirectionSell, LimitValue: &lv}, listing)
	require.False(t, ok)
}

func TestIsWeekend(t *testing.T) {
	sat := time.Date(2025, 6, 7, 12, 0, 0, 0, time.UTC)
	sun := time.Date(2025, 6, 8, 12, 0, 0, 0, time.UTC)
	mon := time.Date(2025, 6, 9, 12, 0, 0, 0, time.UTC)

	require.True(t, isWeekend(sat))
	require.True(t, isWeekend(sun))
	require.False(t, isWeekend(mon))
}

// ── failOrder Tests ──────────────────────────────────────────────

func TestFailOrder_SetsFieldsAndSaves(t *testing.T) {
	repo := &fakeOrderRepo{}
	svc := newTestOrderService(repo, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, &fakeListingRepo{}, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	nextExec := time.Date(2025, 6, 5, 12, 0, 0, 0, time.UTC)
	order := &model.Order{
		OrderID:         1,
		Status:          model.OrderStatusApproved,
		IsDone:          false,
		NextExecutionAt: &nextExec,
	}

	err := svc.failOrder(context.Background(), order, model.OrderStatusDeclined)
	require.NoError(t, err)
	require.Equal(t, model.OrderStatusDeclined, order.Status)
	require.True(t, order.IsDone)
	require.Nil(t, order.NextExecutionAt)
	require.NotNil(t, repo.capturedOrder)
}

func TestFailOrder_RepoSaveError(t *testing.T) {
	repo := &fakeOrderRepo{saveErr: errors.New("save failed")}
	svc := newTestOrderService(repo, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, &fakeListingRepo{}, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	order := &model.Order{OrderID: 1, Status: model.OrderStatusApproved}
	err := svc.failOrder(context.Background(), order, model.OrderStatusDeclined)
	require.Error(t, err)
}

// ── resolveFillQuantity Tests ────────────────────────────────────

func TestResolveFillQuantity_RemainingZero(t *testing.T) {
	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, &fakeListingRepo{}, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	order := &model.Order{Quantity: 10, FilledQty: 10}
	require.Equal(t, uint(0), svc.resolveFillQuantity(order))
}

func TestResolveFillQuantity_AllOrNone(t *testing.T) {
	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, &fakeListingRepo{}, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	order := &model.Order{Quantity: 10, FilledQty: 3, AllOrNone: true}
	require.Equal(t, uint(7), svc.resolveFillQuantity(order))
}

func TestResolveFillQuantity_RemainingOne(t *testing.T) {
	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, &fakeListingRepo{}, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	order := &model.Order{Quantity: 5, FilledQty: 4, AllOrNone: false}
	require.Equal(t, uint(1), svc.resolveFillQuantity(order))
}

func TestResolveFillQuantity_RandomInRange(t *testing.T) {
	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, &fakeListingRepo{}, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	order := &model.Order{Quantity: 100, FilledQty: 0, AllOrNone: false}

	for i := 0; i < 50; i++ {
		qty := svc.resolveFillQuantity(order)
		require.GreaterOrEqual(t, qty, uint(1))
		require.LessOrEqual(t, qty, uint(100))
	}
}

// ── resolveExchangeSession Tests ─────────────────────────────────

func TestResolveExchangeSession_NilExchange(t *testing.T) {
	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, &fakeListingRepo{}, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	session := svc.resolveExchangeSession(nil)
	require.True(t, session.IsOpen)
	require.False(t, session.IsClosed)
}

func TestResolveExchangeSession_TradingDisabled(t *testing.T) {
	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, &fakeListingRepo{}, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	exchange := &model.Exchange{
		TradingEnabled: false,
		OpenTime:       "09:00",
		CloseTime:      "16:00",
		TimeZone:       0,
	}
	session := svc.resolveExchangeSession(exchange)
	require.True(t, session.IsOpen)
	require.False(t, session.IsClosed)
}

func TestResolveExchangeSession_DuringOpenHours(t *testing.T) {
	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, &fakeListingRepo{}, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	exchange := &model.Exchange{
		TradingEnabled: true,
		OpenTime:       "09:00",
		CloseTime:      "16:00",
		TimeZone:       0,
	}
	session := svc.resolveExchangeSession(exchange)
	require.True(t, session.IsOpen)
	require.False(t, session.IsClosed)
}

func TestResolveExchangeSession_DuringClosedHours_BeforeOpen(t *testing.T) {
	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, &fakeListingRepo{}, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	exchange := &model.Exchange{
		TradingEnabled: true,
		OpenTime:       "16:00",
		CloseTime:      "23:00",
		TimeZone:       5,
	}
	session := svc.resolveExchangeSession(exchange)
	require.True(t, session.IsClosed)
	require.False(t, session.IsOpen)
	require.False(t, session.NextOpen.IsZero())
}

func TestResolveExchangeSession_DuringClosedHours_AfterClose(t *testing.T) {
	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, &fakeListingRepo{}, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	exchange := &model.Exchange{
		TradingEnabled: true,
		OpenTime:       "06:00",
		CloseTime:      "09:00",
		TimeZone:       0,
	}
	session := svc.resolveExchangeSession(exchange)
	require.True(t, session.IsClosed)
	require.False(t, session.IsOpen)
	require.False(t, session.NextOpen.IsZero())
}

// ── processOrder basic paths ─────────────────────────────────────

func TestProcessOrder_ListingNotFound_FailsOrder(t *testing.T) {
	orderRepo := &fakeOrderRepo{}
	listingRepo := &fakeListingRepo{listing: nil}
	svc := newTestOrderService(orderRepo, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, listingRepo, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	order := &model.Order{
		OrderID:   1,
		ListingID: 999,
		Status:    model.OrderStatusApproved,
	}

	err := svc.processOrder(context.Background(), order)
	require.NoError(t, err)
	require.True(t, order.IsDone)
	require.Equal(t, model.OrderStatusDeclined, order.Status)
	require.Nil(t, order.NextExecutionAt)
}

func TestProcessOrder_ExchangeNotFound_FailsOrder(t *testing.T) {
	listing := defaultListing()
	orderRepo := &fakeOrderRepo{}
	listingRepo := &fakeListingRepo{listing: listing}
	exchangeRepo := &fakeExchangeRepo{exchange: nil}
	svc := newTestOrderService(orderRepo, &fakeOrderTransactionRepo{}, exchangeRepo, listingRepo, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	order := &model.Order{
		OrderID:   1,
		ListingID: 1,
		Status:    model.OrderStatusApproved,
	}

	err := svc.processOrder(context.Background(), order)
	require.NoError(t, err)
	require.True(t, order.IsDone)
	require.Equal(t, model.OrderStatusDeclined, order.Status)
}

func TestProcessOrder_ListingRepoError(t *testing.T) {
	listingRepo := &fakeListingRepo{findByIDErr: errors.New("db error")}
	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, listingRepo, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	order := &model.Order{OrderID: 1, ListingID: 1}
	err := svc.processOrder(context.Background(), order)
	require.Error(t, err)
}

func TestProcessOrder_ExchangeRepoError(t *testing.T) {
	listing := defaultListing()
	listingRepo := &fakeListingRepo{listing: listing}
	exchangeRepo := &fakeExchangeRepo{findErr: errors.New("db error")}
	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, exchangeRepo, listingRepo, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	order := &model.Order{OrderID: 1, ListingID: 1}
	err := svc.processOrder(context.Background(), order)
	require.Error(t, err)
}

func TestProcessOrder_MarketOrder_FullFill(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()
	orderRepo := &fakeOrderRepo{}
	listingRepo := &fakeListingRepo{listing: listing}
	exchangeRepo := &fakeExchangeRepo{exchange: exchange}
	txRepo := &fakeOrderTransactionRepo{}
	bankingClient := &fakeOrderBankingClient{
		settlementResp: &pb.ExecuteTradeSettlementResponse{
			SourceAmount:            151.0,
			SourceCurrencyCode:      "USD",
			DestinationAmount:       151.0,
			DestinationCurrencyCode: "USD",
		},
	}
	svc := newTestOrderService(orderRepo, txRepo, exchangeRepo, listingRepo, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, bankingClient, &fakeTaxRecorder{})

	order := &model.Order{
		OrderID:          1,
		ListingID:        1,
		OrderType:        model.OrderTypeMarket,
		Direction:        model.OrderDirectionBuy,
		Quantity:         1,
		FilledQty:        0,
		ContractSize:     1,
		Triggered:        true,
		AllOrNone:        true,
		Status:           model.OrderStatusApproved,
		AccountNumber:    "444000100000000110",
		CommissionExempt: true,
	}

	err := svc.processOrder(context.Background(), order)
	require.NoError(t, err)
	require.Equal(t, uint(1), order.FilledQty)
	require.True(t, order.IsDone)
	require.Nil(t, order.NextExecutionAt)
}

// ── processDueOrders Tests ───────────────────────────────────────

func TestApproveOrder_ActuaryBuy_IncrementsUsedLimit(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()
	price := 151.0
	pendingOrder := &model.Order{
		OrderID:          1,
		OrderOwnerUserID: 42,
		ListingID:        1,
		Listing:          *listing,
		OrderType:        model.OrderTypeMarket,
		Direction:        model.OrderDirectionBuy,
		Quantity:         2,
		FilledQty:        0,
		ContractSize:     1,
		Triggered:        true,
		AllOrNone:        true,
		Status:           model.OrderStatusPending,
		AccountNumber:    "444000100000000110",
		CommissionExempt: true,
		OwnerType:        model.OwnerTypeActuary,
		PricePerUnit:     &price,
	}
	userClient := &fakeUserServiceClient{
		employeeResp: &pb.GetEmployeeByIdResponse{Id: 42, IsAgent: true},
		identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5},
	}
	svc := newTestOrderService(
		&fakeOrderRepo{orderByID: pendingOrder},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{exchange: exchange},
		&fakeListingRepo{listing: listing},
		userClient,
		&fakeOrderBankingClient{},
		&fakeTaxRecorder{},
	)

	ctx := employeeAuthCtx(42)

	_, err := svc.ApproveOrder(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, uint64(42), userClient.incrementUsedLimitEmployeeID)
	// 2 qty * 151.0 price = 302.0
	require.InDelta(t, 302.0, userClient.incrementUsedLimitAmount, 0.01)
}

func TestApproveOrder_SupervisorBuy_DoesNotIncrementUsedLimit(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()
	price := 151.0
	pendingOrder := &model.Order{
		OrderID:          1,
		OrderOwnerUserID: 42,
		ListingID:        1,
		Listing:          *listing,
		OrderType:        model.OrderTypeMarket,
		Direction:        model.OrderDirectionBuy,
		Quantity:         1,
		FilledQty:        0,
		ContractSize:     1,
		Triggered:        true,
		AllOrNone:        true,
		Status:           model.OrderStatusPending,
		AccountNumber:    "444000100000000110",
		CommissionExempt: true,
		OwnerType:        model.OwnerTypeActuary,
		PricePerUnit:     &price,
	}
	userClient := &fakeUserServiceClient{
		employeeResp: &pb.GetEmployeeByIdResponse{Id: 42, IsAgent: false, IsSupervisor: true},
		identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5},
	}
	svc := newTestOrderService(
		&fakeOrderRepo{orderByID: pendingOrder},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{exchange: exchange},
		&fakeListingRepo{listing: listing},
		userClient,
		&fakeOrderBankingClient{},
		&fakeTaxRecorder{},
	)

	ctx := employeeAuthCtx(42)

	_, err := svc.ApproveOrder(ctx, 1)
	require.NoError(t, err)
	require.False(t, userClient.incrementUsedLimitCalled)
}

func TestApproveOrder_ActuarySell_DoesNotIncrementUsedLimit(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()
	price := 151.0
	pendingOrder := &model.Order{
		OrderID:          1,
		OrderOwnerUserID: 42,
		ListingID:        1,
		Listing:          *listing,
		OrderType:        model.OrderTypeMarket,
		Direction:        model.OrderDirectionSell,
		Quantity:         1,
		FilledQty:        0,
		ContractSize:     1,
		Triggered:        true,
		AllOrNone:        true,
		Status:           model.OrderStatusPending,
		AccountNumber:    "444000100000000110",
		CommissionExempt: true,
		OwnerType:        model.OwnerTypeActuary,
		PricePerUnit:     &price,
	}
	userClient := &fakeUserServiceClient{
		identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5},
	}
	svc := newTestOrderService(
		&fakeOrderRepo{orderByID: pendingOrder},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{exchange: exchange},
		&fakeListingRepo{listing: listing},
		userClient,
		&fakeOrderBankingClient{},
		&fakeTaxRecorder{},
	)
	svc.assetOwnershipRepo = &fakeAssetOwnershipRepo{
		ownerships: []model.AssetOwnership{
			{AssetID: listing.AssetID, UserId: 5, OwnerType: model.OwnerTypeActuary, Amount: 1, AvgBuyPriceRSD: 200},
		},
	}

	ctx := employeeAuthCtx(42)

	_, err := svc.ApproveOrder(ctx, 1)
	require.NoError(t, err)
	require.False(t, userClient.incrementUsedLimitCalled)
}

// processOrder tests — confirm it no longer touches the used limit at all

func TestProcessOrder_ActuaryBuy_DoesNotIncrementUsedLimit(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()
	userClient := &fakeUserServiceClient{
		employeeResp: &pb.GetEmployeeByIdResponse{Id: 42, IsAgent: true},
		identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5},
	}
	bankingClient := &fakeOrderBankingClient{
		settlementResp: &pb.ExecuteTradeSettlementResponse{
			SourceAmount:            302.0,
			SourceCurrencyCode:      "USD",
			DestinationAmount:       302.0,
			DestinationCurrencyCode: "USD",
		},
	}
	svc := newTestOrderService(
		&fakeOrderRepo{},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{exchange: exchange},
		&fakeListingRepo{listing: listing},
		userClient,
		bankingClient,
		&fakeTaxRecorder{},
	)

	order := &model.Order{
		OrderID:          1,
		OrderOwnerUserID: 42,
		ListingID:        1,
		OrderType:        model.OrderTypeMarket,
		Direction:        model.OrderDirectionBuy,
		Quantity:         2,
		FilledQty:        0,
		ContractSize:     1,
		Triggered:        true,
		AllOrNone:        true,
		Status:           model.OrderStatusApproved,
		AccountNumber:    "444000100000000110",
		CommissionExempt: true,
		OwnerType:        model.OwnerTypeActuary,
	}

	err := svc.processOrder(context.Background(), order)
	require.NoError(t, err)
	require.False(t, userClient.incrementUsedLimitCalled)
}

func TestProcessOrder_SupervisorBuy_DoesNotIncrementUsedLimit(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()
	userClient := &fakeUserServiceClient{
		employeeResp: &pb.GetEmployeeByIdResponse{Id: 42, IsAgent: false, IsSupervisor: true},
		identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5},
	}
	bankingClient := &fakeOrderBankingClient{
		settlementResp: &pb.ExecuteTradeSettlementResponse{
			SourceAmount:            151.0,
			SourceCurrencyCode:      "USD",
			DestinationAmount:       151.0,
			DestinationCurrencyCode: "USD",
		},
	}
	svc := newTestOrderService(
		&fakeOrderRepo{},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{exchange: exchange},
		&fakeListingRepo{listing: listing},
		userClient,
		bankingClient,
		&fakeTaxRecorder{},
	)

	order := &model.Order{
		OrderID:          1,
		OrderOwnerUserID: 42,
		ListingID:        1,
		OrderType:        model.OrderTypeMarket,
		Direction:        model.OrderDirectionBuy,
		Quantity:         1,
		FilledQty:        0,
		ContractSize:     1,
		Triggered:        true,
		AllOrNone:        true,
		Status:           model.OrderStatusApproved,
		AccountNumber:    "444000100000000110",
		CommissionExempt: true,
		OwnerType:        model.OwnerTypeActuary,
	}

	err := svc.processOrder(context.Background(), order)
	require.NoError(t, err)
	require.False(t, userClient.incrementUsedLimitCalled)
}

func TestProcessOrder_ActuarySell_DoesNotIncrementUsedLimit(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()
	userClient := &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}
	bankingClient := &fakeOrderBankingClient{
		settlementResp: &pb.ExecuteTradeSettlementResponse{
			SourceAmount:            151.0,
			SourceCurrencyCode:      "USD",
			DestinationAmount:       151.0,
			DestinationCurrencyCode: "USD",
		},
	}
	svc := newTestOrderService(
		&fakeOrderRepo{},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{exchange: exchange},
		&fakeListingRepo{listing: listing},
		userClient,
		bankingClient,
		&fakeTaxRecorder{},
	)
	svc.assetOwnershipRepo = &fakeAssetOwnershipRepo{
		ownerships: []model.AssetOwnership{
			{AssetID: listing.AssetID, UserId: 5, OwnerType: model.OwnerTypeActuary, Amount: 1, AvgBuyPriceRSD: 200},
		},
	}

	order := &model.Order{
		OrderID:          1,
		OrderOwnerUserID: 42,
		ListingID:        1,
		OrderType:        model.OrderTypeMarket,
		Direction:        model.OrderDirectionSell,
		Quantity:         1,
		FilledQty:        0,
		ContractSize:     1,
		Triggered:        true,
		AllOrNone:        true,
		Status:           model.OrderStatusApproved,
		AccountNumber:    "444000100000000110",
		CommissionExempt: true,
		OwnerType:        model.OwnerTypeActuary,
	}

	err := svc.processOrder(context.Background(), order)
	require.NoError(t, err)
	require.False(t, userClient.incrementUsedLimitCalled)
}

func TestProcessDueOrders_NoReadyOrders(t *testing.T) {
	orderRepo := &fakeOrderRepo{readyOrders: []model.Order{}}
	svc := newTestOrderService(orderRepo, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, &fakeListingRepo{}, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	err := svc.processDueOrders(context.Background())
	require.NoError(t, err)
}

func TestProcessDueOrders_RepoError(t *testing.T) {
	orderRepo := &fakeOrderRepo{readyErr: errors.New("db error")}
	svc := newTestOrderService(orderRepo, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, &fakeListingRepo{}, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	err := svc.processDueOrders(context.Background())
	require.Error(t, err)
}

func TestProcessDueOrders_WithReadyOrders_ProcessesThem(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()

	readyOrders := []model.Order{
		{
			OrderID:          1,
			ListingID:        1,
			OrderType:        model.OrderTypeMarket,
			Direction:        model.OrderDirectionBuy,
			Quantity:         1,
			ContractSize:     1,
			Triggered:        true,
			AllOrNone:        true,
			Status:           model.OrderStatusApproved,
			AccountNumber:    "444000100000000110",
			CommissionExempt: true,
		},
	}

	orderRepo := &fakeOrderRepo{readyOrders: readyOrders}
	listingRepo := &fakeListingRepo{listing: listing}
	exchangeRepo := &fakeExchangeRepo{exchange: exchange}
	txRepo := &fakeOrderTransactionRepo{}
	bankingClient := &fakeOrderBankingClient{
		settlementResp: &pb.ExecuteTradeSettlementResponse{
			SourceAmount:            151.0,
			SourceCurrencyCode:      "USD",
			DestinationAmount:       151.0,
			DestinationCurrencyCode: "USD",
		},
	}

	svc := newTestOrderService(orderRepo, txRepo, exchangeRepo, listingRepo, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, bankingClient, &fakeTaxRecorder{})

	err := svc.processDueOrders(context.Background())
	require.NoError(t, err)
}

// ── Start / Stop Tests ───────────────────────────────────────────

func TestStartStop_DoesNotPanic(t *testing.T) {
	svc := newTestOrderService(&fakeOrderRepo{readyOrders: []model.Order{}}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, &fakeListingRepo{}, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	svc.Start()
	svc.Start()
	svc.Stop()
	svc.Stop()
}

// ── initialExecutionTime Tests ───────────────────────────────────

func TestInitialExecutionTime_AfterHours(t *testing.T) {
	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, &fakeListingRepo{}, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	session := exchangeSession{IsOpen: true}
	result := svc.initialExecutionTime(session, true)
	expected := svc.now().Add(afterHoursExecutionDelay)
	require.Equal(t, expected, result)
}

func TestInitialExecutionTime_OpenSession(t *testing.T) {
	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, &fakeListingRepo{}, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	session := exchangeSession{IsOpen: true}
	result := svc.initialExecutionTime(session, false)
	require.Equal(t, svc.now(), result)
}

func TestInitialExecutionTime_ClosedSession(t *testing.T) {
	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, &fakeListingRepo{}, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	nextOpen := time.Date(2025, 6, 5, 9, 0, 0, 0, time.UTC)
	session := exchangeSession{IsOpen: false, NextOpen: nextOpen}
	result := svc.initialExecutionTime(session, false)
	require.Equal(t, nextOpen, result)
}

// ── approximateOrderValue Tests ──────────────────────────────────

func TestApproximateOrderValue(t *testing.T) {
	price := 100.0
	order := &model.Order{Quantity: 10, ContractSize: 1, PricePerUnit: &price}
	require.Equal(t, 1000.0, approximateOrderValue(order, 0))

	order2 := &model.Order{Quantity: 5, ContractSize: 2, PricePerUnit: nil}
	require.Equal(t, 500.0, approximateOrderValue(order2, 50.0))

	zero := 0.0
	order3 := &model.Order{Quantity: 5, ContractSize: 2, PricePerUnit: &zero}
	require.Equal(t, 500.0, approximateOrderValue(order3, 50.0))
}

// ── processOrder: closed exchange reschedules ────────────────────

func TestProcessOrder_ClosedExchange_Reschedules(t *testing.T) {
	listing := defaultListing()
	exchange := &model.Exchange{
		ExchangeID:     1,
		Name:           "Late Exchange",
		MicCode:        "XTST",
		Currency:       "USD",
		TimeZone:       10,
		OpenTime:       "09:00",
		CloseTime:      "16:00",
		TradingEnabled: true,
	}

	orderRepo := &fakeOrderRepo{}
	listingRepo := &fakeListingRepo{listing: listing}
	exchangeRepo := &fakeExchangeRepo{exchange: exchange}
	svc := newTestOrderService(orderRepo, &fakeOrderTransactionRepo{}, exchangeRepo, listingRepo, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	order := &model.Order{
		OrderID:      1,
		ListingID:    1,
		OrderType:    model.OrderTypeMarket,
		Direction:    model.OrderDirectionBuy,
		Quantity:     5,
		ContractSize: 1,
		Triggered:    true,
		Status:       model.OrderStatusApproved,
		AfterHours:   false,
	}

	err := svc.processOrder(context.Background(), order)
	require.NoError(t, err)
	require.False(t, order.IsDone)
	require.NotNil(t, order.NextExecutionAt)
}

// ── processOrder: settlement gRPC FailedPrecondition fails order ─

func TestProcessOrder_SettlementFailedPrecondition_FailsOrder(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()

	orderRepo := &fakeOrderRepo{}
	listingRepo := &fakeListingRepo{listing: listing}
	exchangeRepo := &fakeExchangeRepo{exchange: exchange}
	txRepo := &fakeOrderTransactionRepo{}
	bankingClient := &fakeOrderBankingClient{
		settlementErr: status.Error(codes.FailedPrecondition, "insufficient funds"),
	}
	svc := newTestOrderService(orderRepo, txRepo, exchangeRepo, listingRepo, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}}, bankingClient, &fakeTaxRecorder{})

	order := &model.Order{
		OrderID:          1,
		ListingID:        1,
		OrderType:        model.OrderTypeMarket,
		Direction:        model.OrderDirectionBuy,
		Quantity:         1,
		FilledQty:        0,
		ContractSize:     1,
		Triggered:        true,
		AllOrNone:        true,
		Status:           model.OrderStatusApproved,
		AccountNumber:    "444000100000000110",
		CommissionExempt: true,
	}

	err := svc.processOrder(context.Background(), order)
	require.NoError(t, err)
	require.True(t, order.IsDone)
	require.Equal(t, model.OrderStatusDeclined, order.Status)
}

// ── recordProfitTax Tests ────────────────────────────────────────

func TestRecordProfitTax_ClientSell_RecordsTax(t *testing.T) {
	listing := defaultListing()
	listing.Asset = &model.Asset{AssetID: 1, Ticker: "AAPL", AssetType: model.AssetTypeStock}
	listing.AssetID = 1

	ownership := model.AssetOwnership{
		AssetID:        1,
		AvgBuyPriceRSD: 100.0,
		Amount:         1,
	}

	taxRecorder := &fakeTaxRecorder{}
	bankingClient := &fakeOrderBankingClient{
		settlementResp:  &pb.ExecuteTradeSettlementResponse{},
		accountCurrency: "RSD",
	}

	svc := newTestOrderService(
		&fakeOrderRepo{},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{},
		&fakeListingRepo{listing: listing},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		bankingClient,
		taxRecorder,
	)
	svc.assetOwnershipRepo = &fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{ownership}}

	order := &model.Order{
		OrderID:          1,
		OrderOwnerUserID: 10,
		AccountNumber:    "444000100000000110",
		ListingID:        1,
		Listing:          *listing,
		Direction:        model.OrderDirectionSell,
		Quantity:         1,
		FilledQty:        1,
		ContractSize:     1,
		OrderOwnerType:   model.OwnerTypeClient,
	}

	err := svc.recordProfitTax(context.Background(), order, 1, 200.0, "RSD")
	require.NoError(t, err)
	require.True(t, taxRecorder.called)
	require.Equal(t, "444000100000000110", taxRecorder.recordedAccountNumber)
	require.Nil(t, taxRecorder.recordedEmployeeID)
	require.InDelta(t, 100.0, taxRecorder.recordedProfit, 0.01)
}

func TestRecordProfitTax_ActuarySell_PassesEmployeeID(t *testing.T) {
	listing := defaultListing()
	listing.Asset = &model.Asset{AssetID: 1, Ticker: "AAPL", AssetType: model.AssetTypeStock}
	listing.AssetID = 1

	ownership := model.AssetOwnership{
		AssetID:        1,
		AvgBuyPriceRSD: 100.0,
		Amount:         1,
	}

	taxRecorder := &fakeTaxRecorder{}
	bankingClient := &fakeOrderBankingClient{accountCurrency: "RSD"}

	svc := newTestOrderService(
		&fakeOrderRepo{},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{},
		&fakeListingRepo{listing: listing},
		&fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		bankingClient,
		taxRecorder,
	)
	svc.assetOwnershipRepo = &fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{ownership}}

	userID := uint(42)
	order := &model.Order{
		OrderOwnerUserID: userID,
		AccountNumber:    "444000100000000110",
		Listing:          *listing,
		Direction:        model.OrderDirectionSell,
		Quantity:         1,
		FilledQty:        1,
		ContractSize:     1,
		OrderOwnerType:   model.OwnerTypeActuary,
	}

	err := svc.recordProfitTax(context.Background(), order, 1, 200.0, "RSD")
	require.NoError(t, err)
	require.True(t, taxRecorder.called)
	require.NotNil(t, taxRecorder.recordedEmployeeID)
	require.Equal(t, userID, *taxRecorder.recordedEmployeeID)
}

func TestRecordProfitTax_NoProfit_SkipsTax(t *testing.T) {
	listing := defaultListing()
	listing.Asset = &model.Asset{AssetID: 1}
	listing.AssetID = 1

	ownership := model.AssetOwnership{
		AssetID:        1,
		AvgBuyPriceRSD: 300.0,
		Amount:         1,
	}

	taxRecorder := &fakeTaxRecorder{}
	svc := newTestOrderService(
		&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{},
		&fakeListingRepo{listing: listing}, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{accountCurrency: "RSD"}, taxRecorder,
	)
	svc.assetOwnershipRepo = &fakeAssetOwnershipRepo{ownerships: []model.AssetOwnership{ownership}}

	order := &model.Order{
		Listing:        *listing,
		Direction:      model.OrderDirectionSell,
		Quantity:       1,
		FilledQty:      1,
		ContractSize:   1,
		OrderOwnerType: model.OwnerTypeClient,
	}

	err := svc.recordProfitTax(context.Background(), order, 1, 200.0, "RSD")
	require.NoError(t, err)
	require.False(t, taxRecorder.called)
}

func TestRecordProfitTax_BuyOrder_Skipped(t *testing.T) {
	taxRecorder := &fakeTaxRecorder{}
	svc := newTestOrderService(
		&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{},
		&fakeListingRepo{}, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{}, taxRecorder,
	)

	order := &model.Order{
		Direction:    model.OrderDirectionBuy,
		Quantity:     1,
		FilledQty:    1,
		ContractSize: 1,
	}

	err := svc.recordProfitTax(context.Background(), order, 1, 200.0, "RSD")
	require.NoError(t, err)
	require.False(t, taxRecorder.called)
}

func TestRecordProfitTax_PartialFill_Skipped(t *testing.T) {
	taxRecorder := &fakeTaxRecorder{}
	svc := newTestOrderService(
		&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{},
		&fakeListingRepo{}, &fakeUserServiceClient{identityResp: &pb.GetIdentityByUserIdResponse{IdentityId: 5}},
		&fakeOrderBankingClient{}, taxRecorder,
	)

	order := &model.Order{
		Direction:    model.OrderDirectionSell,
		Quantity:     5,
		FilledQty:    2,
		ContractSize: 1,
	}

	err := svc.recordProfitTax(context.Background(), order, 1, 200.0, "RSD")
	require.NoError(t, err)
	require.False(t, taxRecorder.called)
}

// ── CreateFundOrder unit tests ────────────────────────────────────

func newFundOrderService(fundRepo *fakeFundRepo, ownershipRepo *fakeAssetOwnershipRepo) *OrderService {
	// Supervisor: IsAgent=false so resolveOrderStatus returns PENDING without limit checks.
	userClient := &fakeUserServiceClient{
		employeeResp: &pb.GetEmployeeByIdResponse{IsSupervisor: true, IsAgent: false},
	}
	svc := NewOrderService(
		&fakeOrderRepo{},
		&fakeOrderTransactionRepo{},
		&fakeExchangeRepo{exchange: defaultExchange()},
		&fakeListingRepo{listing: defaultListing()},
		ownershipRepo,
		&fakeFuturesRepo{},
		&fakeOptionRepo{},
		fundRepo,
		userClient,
		&fakeOrderBankingClient{accountResp: defaultFundAccountResp(1_000_000)},
		&fakeTaxRecorder{},
		fakeAuditService(nil),
	)
	svc.now = func() time.Time { return time.Date(2025, 6, 4, 10, 0, 0, 0, time.UTC) }
	return svc
}

func defaultFund(managerID uint) *model.InvestmentFund {
	return &model.InvestmentFund{
		FundID:        42,
		Name:          "Test Fund",
		ManagerID:     managerID,
		AccountNumber: "444000000000000000",
	}
}

func TestCreateFundOrder_BuyMarket_Success(t *testing.T) {
	const managerID uint = 5
	svc := newFundOrderService(
		&fakeFundRepo{findByIDResult: defaultFund(managerID)},
		&fakeAssetOwnershipRepo{},
	)

	order, err := svc.CreateFundOrder(supervisorAuthCtx(managerID), dto.CreateFundOrderRequest{
		FundID:    42,
		ListingID: 1,
		OrderType: model.OrderTypeMarket,
		Direction: model.OrderDirectionBuy,
		Quantity:  10,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, managerID, order.OrderOwnerUserID)
	require.Equal(t, model.OwnerTypeActuary, order.OrderOwnerType)
	require.Equal(t, uint(42), order.AssetOwnerUserID)
	require.Equal(t, model.OwnerTypeFund, order.AssetOwnerType)
	require.Equal(t, "444000000000000000", order.AccountNumber)
	require.True(t, order.CommissionExempt)
}

func TestCreateFundOrder_NotAuthenticated(t *testing.T) {
	svc := newFundOrderService(&fakeFundRepo{}, &fakeAssetOwnershipRepo{})

	_, err := svc.CreateFundOrder(context.Background(), dto.CreateFundOrderRequest{
		FundID:    1,
		ListingID: 1,
		OrderType: model.OrderTypeMarket,
		Direction: model.OrderDirectionBuy,
		Quantity:  1,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "only employees")
}

func TestCreateFundOrder_ClientCannotPlaceFundOrder(t *testing.T) {
	svc := newFundOrderService(&fakeFundRepo{}, &fakeAssetOwnershipRepo{})

	_, err := svc.CreateFundOrder(clientAuthCtx(), dto.CreateFundOrderRequest{
		FundID:    1,
		ListingID: 1,
		OrderType: model.OrderTypeMarket,
		Direction: model.OrderDirectionBuy,
		Quantity:  1,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "only employees")
}

func TestCreateFundOrder_FundNotFound(t *testing.T) {
	svc := newFundOrderService(
		&fakeFundRepo{findByIDResult: nil},
		&fakeAssetOwnershipRepo{},
	)

	_, err := svc.CreateFundOrder(supervisorAuthCtx(5), dto.CreateFundOrderRequest{
		FundID:    99,
		ListingID: 1,
		OrderType: model.OrderTypeMarket,
		Direction: model.OrderDirectionBuy,
		Quantity:  1,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "investment fund not found")
}

func TestCreateFundOrder_NotFundManager(t *testing.T) {
	svc := newFundOrderService(
		&fakeFundRepo{findByIDResult: defaultFund(99)},
		&fakeAssetOwnershipRepo{},
	)

	_, err := svc.CreateFundOrder(supervisorAuthCtx(5), dto.CreateFundOrderRequest{
		FundID:    42,
		ListingID: 1,
		OrderType: model.OrderTypeMarket,
		Direction: model.OrderDirectionBuy,
		Quantity:  1,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "not the manager")
}

func TestCreateFundOrder_LimitOrder_MissingLimitValue(t *testing.T) {
	const managerID uint = 5
	svc := newFundOrderService(
		&fakeFundRepo{findByIDResult: defaultFund(managerID)},
		&fakeAssetOwnershipRepo{},
	)

	_, err := svc.CreateFundOrder(supervisorAuthCtx(managerID), dto.CreateFundOrderRequest{
		FundID:    42,
		ListingID: 1,
		OrderType: model.OrderTypeLimit,
		Direction: model.OrderDirectionBuy,
		Quantity:  1,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "limitValue is required")
}

func TestCreateFundOrder_Sell_InsufficientOwnership(t *testing.T) {
	const managerID uint = 5
	listing := defaultListing()
	svc := newFundOrderService(
		&fakeFundRepo{findByIDResult: defaultFund(managerID)},
		&fakeAssetOwnershipRepo{
			ownerships: []model.AssetOwnership{
				{AssetID: listing.AssetID, UserId: 42, OwnerType: model.OwnerTypeFund, Amount: 2},
			},
		},
	)

	_, err := svc.CreateFundOrder(supervisorAuthCtx(managerID), dto.CreateFundOrderRequest{
		FundID:    42,
		ListingID: 1,
		OrderType: model.OrderTypeMarket,
		Direction: model.OrderDirectionSell,
		Quantity:  10,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient")
}

func TestCreateFundOrder_Sell_Success(t *testing.T) {
	const managerID uint = 5
	listing := defaultListing()
	svc := newFundOrderService(
		&fakeFundRepo{findByIDResult: defaultFund(managerID)},
		&fakeAssetOwnershipRepo{
			ownerships: []model.AssetOwnership{
				{AssetID: listing.AssetID, UserId: 42, OwnerType: model.OwnerTypeFund, Amount: 20},
			},
		},
	)

	order, err := svc.CreateFundOrder(supervisorAuthCtx(managerID), dto.CreateFundOrderRequest{
		FundID:    42,
		ListingID: 1,
		OrderType: model.OrderTypeMarket,
		Direction: model.OrderDirectionSell,
		Quantity:  5,
	})

	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, model.OrderDirectionSell, order.Direction)
	require.Equal(t, model.OwnerTypeFund, order.AssetOwnerType)
}

func TestCreateFundLiquidationOrder_UsesFundOrderSemantics(t *testing.T) {
	const managerID uint = 5
	fund := defaultFund(managerID)
	listing := defaultListing()
	svc := newFundOrderService(
		&fakeFundRepo{findByIDResult: fund},
		&fakeAssetOwnershipRepo{
			ownerships: []model.AssetOwnership{
				{AssetID: listing.AssetID, UserId: fund.FundID, OwnerType: model.OwnerTypeFund, Amount: 20},
			},
		},
	)

	order, err := svc.CreateFundLiquidationOrder(clientAuthCtx(), fund, listing.ListingID, 5)

	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, managerID, order.OrderOwnerUserID)
	require.Equal(t, model.OwnerTypeActuary, order.OrderOwnerType)
	require.Equal(t, fund.FundID, order.AssetOwnerUserID)
	require.Equal(t, model.OwnerTypeFund, order.AssetOwnerType)
	require.Equal(t, model.OrderDirectionSell, order.Direction)
	require.True(t, order.CommissionExempt)
	require.Equal(t, model.OrderStatusApproved, order.Status)
}

// newTestOrderServiceForMyOrders creates an OrderService with fake dependencies,
// allowing us to pass a custom orderRepo and other mocks only when needed.
func newTestOrderServiceForMyOrders(
	orderRepo *fakeOrderRepo,
	userClient *fakeUserServiceClient,
	bankingClient *fakeOrderBankingClient,
) *OrderService {
	// Minimal fakes for required dependencies
	orderTxRepo := &fakeOrderTransactionRepo{}
	exchangeRepo := &fakeExchangeRepo{}
	listingRepo := &fakeListingRepo{}
	taxRecorder := &fakeTaxRecorder{}
	assetOwnershipRepo := &fakeAssetOwnershipRepo{}
	futuresRepo := &fakeFuturesRepo{}
	optionRepo := &fakeOptionRepo{}
	fundRepo := &fakeFundRepo{}

	return NewOrderService(
		orderRepo, orderTxRepo, exchangeRepo, listingRepo,
		assetOwnershipRepo, futuresRepo, optionRepo, fundRepo,
		userClient, bankingClient, taxRecorder,
	)
}

// TestGetMyOrders_Success_Client tests that a client can retrieve their own orders.
func TestGetMyOrders_Success_Client(t *testing.T) {
	userID := uint(10)
	ownerType := model.OwnerTypeClient
	expectedOrders := []model.Order{
		{OrderID: 1, OrderOwnerUserID: userID, OrderOwnerType: ownerType, Status: model.OrderStatusApproved},
		{OrderID: 2, OrderOwnerUserID: userID, OrderOwnerType: ownerType, Status: model.OrderStatusPending},
	}
	fakeRepo := &fakeOrderRepo{
		userOrders:      expectedOrders,
		userOrdersTotal: 2,
	}
	svc := newTestOrderServiceForMyOrders(fakeRepo, &fakeUserServiceClient{}, &fakeOrderBankingClient{})

	ctx := clientAuthCtx() // provides client ID 10
	query := dto.UserOrdersQuery{Page: 1, PageSize: 10}

	orders, total, err := svc.GetMyOrders(ctx, query, userID, ownerType)
	require.NoError(t, err)
	require.Len(t, orders, 2)
	require.Equal(t, int64(2), total)

	// Verify repo received correct parameters
	require.Equal(t, userID, fakeRepo.capturedUserIDOrder)
	require.Equal(t, ownerType, fakeRepo.capturedOwnerType)
	require.Equal(t, query.Page, fakeRepo.capturedUserQuery.Page)
}

// TestGetMyOrders_Success_Employee tests that an employee (actuary) can retrieve their own orders.
func TestGetMyOrders_Success_Employee(t *testing.T) {
	userID := uint(20)
	ownerType := model.OwnerTypeActuary
	expectedOrders := []model.Order{
		{OrderID: 3, OrderOwnerUserID: userID, OrderOwnerType: ownerType, Status: model.OrderStatusApproved},
	}
	fakeRepo := &fakeOrderRepo{
		userOrders:      expectedOrders,
		userOrdersTotal: 1,
	}
	svc := newTestOrderServiceForMyOrders(fakeRepo, &fakeUserServiceClient{}, &fakeOrderBankingClient{})

	ctx := employeeAuthCtx(userID) // employee with ID 20
	query := dto.UserOrdersQuery{Page: 1, PageSize: 10}

	orders, total, err := svc.GetMyOrders(ctx, query, userID, ownerType)
	require.NoError(t, err)
	require.Len(t, orders, 1)
	require.Equal(t, int64(1), total)
}

// TestGetMyOrders_FilterByStatus tests filtering by order status.
func TestGetMyOrders_FilterByStatus(t *testing.T) {
	userID := uint(10)
	ownerType := model.OwnerTypeClient
	fakeRepo := &fakeOrderRepo{userOrdersTotal: 0}
	svc := newTestOrderServiceForMyOrders(fakeRepo, &fakeUserServiceClient{}, &fakeOrderBankingClient{})

	ctx := clientAuthCtx()
	status := model.OrderStatusPending
	query := dto.UserOrdersQuery{Status: &status, Page: 1, PageSize: 10}

	_, _, err := svc.GetMyOrders(ctx, query, userID, ownerType)
	require.NoError(t, err)
	require.Equal(t, status, *fakeRepo.capturedUserQuery.Status)
}

// TestGetMyOrders_FilterByOrderType tests filtering by order type.
func TestGetMyOrders_FilterByOrderType(t *testing.T) {
	userID := uint(10)
	ownerType := model.OwnerTypeClient
	fakeRepo := &fakeOrderRepo{userOrdersTotal: 0}
	svc := newTestOrderServiceForMyOrders(fakeRepo, &fakeUserServiceClient{}, &fakeOrderBankingClient{})

	ctx := clientAuthCtx()
	orderType := model.OrderTypeLimit
	query := dto.UserOrdersQuery{OrderType: &orderType, Page: 1, PageSize: 10}

	_, _, err := svc.GetMyOrders(ctx, query, userID, ownerType)
	require.NoError(t, err)
	require.Equal(t, orderType, *fakeRepo.capturedUserQuery.OrderType)
}

// TestGetMyOrders_FilterByAssetType tests filtering by asset type.
func TestGetMyOrders_FilterByAssetType(t *testing.T) {
	userID := uint(10)
	ownerType := model.OwnerTypeClient
	fakeRepo := &fakeOrderRepo{userOrdersTotal: 0}
	svc := newTestOrderServiceForMyOrders(fakeRepo, &fakeUserServiceClient{}, &fakeOrderBankingClient{})

	ctx := clientAuthCtx()
	assetType := model.AssetTypeStock
	query := dto.UserOrdersQuery{AssetType: &assetType, Page: 1, PageSize: 10}

	_, _, err := svc.GetMyOrders(ctx, query, userID, ownerType)
	require.NoError(t, err)
	require.Equal(t, assetType, *fakeRepo.capturedUserQuery.AssetType)
}

// TestGetMyOrders_FilterByDateRange tests date range filtering.
func TestGetMyOrders_FilterByDateRange(t *testing.T) {
	userID := uint(10)
	ownerType := model.OwnerTypeClient
	fakeRepo := &fakeOrderRepo{userOrdersTotal: 0}
	svc := newTestOrderServiceForMyOrders(fakeRepo, &fakeUserServiceClient{}, &fakeOrderBankingClient{})

	ctx := clientAuthCtx()
	from := time.Now().AddDate(0, 0, -7)
	to := time.Now()
	query := dto.UserOrdersQuery{FromDate: &from, ToDate: &to, Page: 1, PageSize: 10}

	_, _, err := svc.GetMyOrders(ctx, query, userID, ownerType)
	require.NoError(t, err)
	require.Equal(t, from, *fakeRepo.capturedUserQuery.FromDate)
	require.Equal(t, to, *fakeRepo.capturedUserQuery.ToDate)
}

// TestGetMyOrders_Pagination tests that page and page_size are passed correctly.
func TestGetMyOrders_Pagination(t *testing.T) {
	userID := uint(10)
	ownerType := model.OwnerTypeClient
	fakeRepo := &fakeOrderRepo{userOrdersTotal: 0}
	svc := newTestOrderServiceForMyOrders(fakeRepo, &fakeUserServiceClient{}, &fakeOrderBankingClient{})

	ctx := clientAuthCtx()
	query := dto.UserOrdersQuery{Page: 2, PageSize: 5}

	_, _, err := svc.GetMyOrders(ctx, query, userID, ownerType)
	require.NoError(t, err)
	require.Equal(t, 2, fakeRepo.capturedUserQuery.Page)
	require.Equal(t, 5, fakeRepo.capturedUserQuery.PageSize)
}

// TestGetMyOrders_NoOrders tests empty result.
func TestGetMyOrders_NoOrders(t *testing.T) {
	userID := uint(10)
	ownerType := model.OwnerTypeClient
	fakeRepo := &fakeOrderRepo{
		userOrders:      []model.Order{},
		userOrdersTotal: 0,
	}
	svc := newTestOrderServiceForMyOrders(fakeRepo, &fakeUserServiceClient{}, &fakeOrderBankingClient{})

	ctx := clientAuthCtx()
	query := dto.UserOrdersQuery{Page: 1, PageSize: 10}

	orders, total, err := svc.GetMyOrders(ctx, query, userID, ownerType)
	require.NoError(t, err)
	require.Empty(t, orders)
	require.Equal(t, int64(0), total)
}

// TestGetMyOrders_RepoError tests repository error handling.
func TestGetMyOrders_RepoError(t *testing.T) {
	userID := uint(10)
	ownerType := model.OwnerTypeClient
	fakeRepo := &fakeOrderRepo{
		userOrdersErr: errors.New("database error"),
	}
	svc := newTestOrderServiceForMyOrders(fakeRepo, &fakeUserServiceClient{}, &fakeOrderBankingClient{})

	ctx := clientAuthCtx()
	query := dto.UserOrdersQuery{Page: 1, PageSize: 10}

	_, _, err := svc.GetMyOrders(ctx, query, userID, ownerType)
	require.Error(t, err)
}
