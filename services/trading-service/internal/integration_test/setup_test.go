//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/audit"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	commonjwt "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/jwt"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/logging"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/permission"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/handler"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/config"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/server"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/service"

	"github.com/gin-gonic/gin"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var sharedDB *gorm.DB
var uniqueCounter atomic.Uint64

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	_ = logging.Init("test")

	ctx := context.Background()
	container, err := tcpostgres.Run(
		ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("trading_service_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		log.Fatalf("start postgres container: %v", err)
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("build postgres connection string: %v", err)
	}

	db, err := gorm.Open(gormpostgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("open gorm db: %v", err)
	}

	if err := db.AutoMigrate(
		&model.Exchange{},
		&model.Asset{},
		&model.Listing{},
		&model.ListingDailyPriceInfo{},
		&model.Stock{},
		&model.FuturesContract{},
		&model.ForexPair{},
		&model.Option{},
		&model.Order{},
		&model.AssetOwnership{},
		&model.OrderTransaction{},
		&model.AccumulatedTax{},
		&model.TaxCollection{},
		&model.OtcOffer{},
		&model.OtcOptionContract{},
		&model.OtcShareReservation{},
		&model.OtcExecutionSaga{},
		&model.InvestmentFund{},
		&model.ClientFundPosition{},
		&model.ClientFundInvestment{},
		&model.ClientFundRedemption{},
		&model.Watchlist{},
		&model.WatchlistItem{},
		&model.FundPerformance{},
		&audit.AuditLog{},
	); err != nil {
		log.Fatalf("auto migrate test schema: %v", err)
	}

	sharedDB = db
	code := m.Run()

	sqlDB, _ := db.DB()
	_ = sqlDB.Close()
	_ = container.Terminate(ctx)

	os.Exit(code)
}

type fakeUserClient struct {
	supervisorIDs map[uint64]bool
	agentIDs      map[uint64]bool
	identityResp  *pb.GetIdentityByUserIdResponse
	identityErr   error
}

func (f *fakeUserClient) GetClientById(_ context.Context, id uint64) (*pb.GetClientByIdResponse, error) {
	return &pb.GetClientByIdResponse{
		Id:         id,
		Email:      fmt.Sprintf("client-%d@example.com", id),
		FullName:   fmt.Sprintf("Client %d", id),
		IdentityId: id,
	}, nil
}

func (f *fakeUserClient) GetClientsByIds(_ context.Context, ids []uint64) (*pb.GetClientsByIdsResponse, error) {
	return &pb.GetClientsByIdsResponse{
		Clients: make([]*pb.GetClientByIdResponse, 0),
	}, nil
}

func (f *fakeUserClient) GetEmployeeById(_ context.Context, id uint64) (*pb.GetEmployeeByIdResponse, error) {
	isSupervisor := f.supervisorIDs[id]
	isAgent := f.agentIDs[id]
	return &pb.GetEmployeeByIdResponse{
		Id:           id,
		Email:        fmt.Sprintf("employee-%d@example.com", id),
		FullName:     fmt.Sprintf("Employee %d", id),
		IsSupervisor: isSupervisor,
		IsAgent:      isAgent,
		IdentityId:   id,
	}, nil
}

func (f *fakeUserClient) GetClientByIdentityId(_ context.Context, id uint64) (*pb.GetClientByIdResponse, error) {
	return &pb.GetClientByIdResponse{
		Id:         id,
		Email:      fmt.Sprintf("client-%d@example.com", id),
		FullName:   fmt.Sprintf("Client %d", id),
		IdentityId: id,
	}, nil
}

func (f *fakeUserClient) GetEmployeeByIdentityId(_ context.Context, id uint64) (*pb.GetEmployeeByIdResponse, error) {
	isSupervisor := f.supervisorIDs[id]
	isAgent := f.agentIDs[id]
	return &pb.GetEmployeeByIdResponse{
		Id:           id,
		Email:        fmt.Sprintf("employee-%d@example.com", id),
		FullName:     fmt.Sprintf("Employee %d", id),
		IsSupervisor: isSupervisor,
		IsAgent:      isAgent,
		IdentityId:   id,
	}, nil
}

func (f *fakeUserClient) GetAllClients(_ context.Context, _, _ int32, _, _ string) (*pb.GetAllClientsResponse, error) {
	return &pb.GetAllClientsResponse{
		Clients: []*pb.ClientResponse{
			{Id: 1, FirstName: "Test", LastName: "Client", Email: "client1@example.com"},
		},
		Total: 1,
	}, nil
}

func (f *fakeUserClient) GetAllActuaries(_ context.Context, _, _ int32, _, _ string) (*pb.GetAllActuariesResponse, error) {
	return &pb.GetAllActuariesResponse{
		Actuaries: []*pb.ActuaryResponse{
			{Id: 10, FirstName: "Test", LastName: "Actuary", Email: "actuary@example.com"},
		},
		Total: 1,
	}, nil
}

func (c *fakeUserClient) GetIdentityByUserId(_ context.Context, _ uint64, _ string) (*pb.GetIdentityByUserIdResponse, error) {
	return c.identityResp, c.identityErr
}

func (c *fakeUserClient) IncrementUsedLimit(_ context.Context, _ uint64, _ float64) (*pb.IncrementUsedLimitResponse, error) {
	return nil, nil
}

type fakeTaxRecorder struct{}

func (f *fakeTaxRecorder) RecordTax(_ context.Context, _ string, _ *uint, _ float64, _ string) error {
	return nil
}

type fakeBankingClient struct {
	accountByNumber map[string]uint64
}

func (f *fakeBankingClient) GetAccountByNumber(_ context.Context, accountNumber string) (*pb.GetAccountByNumberResponse, error) {
	clientID, ok := f.accountByNumber[accountNumber]
	if !ok {
		return &pb.GetAccountByNumberResponse{
			AccountNumber:    accountNumber,
			ClientId:         1,
			AccountType:      "Bank",
			CurrencyCode:     "RSD",
			AvailableBalance: 1_000_000,
		}, nil
	}
	return &pb.GetAccountByNumberResponse{
		ClientId:         clientID,
		AccountNumber:    accountNumber,
		CurrencyCode:     "RSD",
		AvailableBalance: 1_000_000,
	}, nil
}
func (f *fakeBankingClient) GetAccountCurrency(_ context.Context, _ string) (string, error) {
	return "RSD", nil
}

func (f *fakeBankingClient) CreateFundAccount(_ context.Context, fundName string, _ uint64) (string, error) {
	return fmt.Sprintf("444000199999%06d", uniqueCounter.Add(1)), nil
}

func (f *fakeBankingClient) HasActiveLoan(_ context.Context, _ uint64) (*pb.HasActiveLoanResponse, error) {
	return &pb.HasActiveLoanResponse{HasActiveLoan: true}, nil
}

func (f *fakeBankingClient) CreatePaymentWithoutVerification(_ context.Context, _ *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error) {
	return &pb.CreatePaymentResponse{PaymentId: 1}, nil
}

func (f *fakeBankingClient) GetAccountsByClientID(_ context.Context, _ uint64) (*pb.GetAccountsByClientIDResponse, error) {
	return &pb.GetAccountsByClientIDResponse{
		Accounts: []*pb.AccountInfo{
			{AccountNumber: "444000100000000001", CurrencyCode: "RSD"},
		},
	}, nil
}

func (f *fakeBankingClient) ConvertCurrency(_ context.Context, amount float64, _, _ string) (float64, error) {
	return amount, nil
}

func (f *fakeBankingClient) ExecuteTradeSettlement(_ context.Context, _, _ string, _ pb.TradeSettlementDirection, _ float64) (*pb.ExecuteTradeSettlementResponse, error) {
	return &pb.ExecuteTradeSettlementResponse{TransactionId: 1}, nil
}

func (f *fakeBankingClient) ReserveOtcFunds(_ context.Context, req *pb.ReserveOtcFundsRequest) (*pb.OtcFundsReservationResponse, error) {
	return &pb.OtcFundsReservationResponse{
		ExecutionId:         req.ExecutionId,
		Status:              pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_RESERVED,
		TradeAmount:         req.Amount,
		TradeCurrencyCode:   req.CurrencyCode,
		BuyerAccountNumber:  req.BuyerAccountNumber,
		SellerAccountNumber: req.SellerAccountNumber,
	}, nil
}

func (f *fakeBankingClient) ReleaseOtcFunds(_ context.Context, executionID string) (*pb.OtcFundsReservationResponse, error) {
	return &pb.OtcFundsReservationResponse{ExecutionId: executionID, Status: pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_RELEASED}, nil
}

func (f *fakeBankingClient) CommitOtcFunds(_ context.Context, executionID string) (*pb.OtcFundsReservationResponse, error) {
	return &pb.OtcFundsReservationResponse{ExecutionId: executionID, Status: pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_COMMITTED}, nil
}

func (f *fakeBankingClient) RefundOtcFunds(_ context.Context, executionID string) (*pb.OtcFundsReservationResponse, error) {
	return &pb.OtcFundsReservationResponse{ExecutionId: executionID, Status: pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_REFUNDED}, nil
}

type fakePermissionProvider struct {
	perms []permission.Permission
}

func (f *fakePermissionProvider) GetPermissions(_ context.Context, _ *commonjwt.Claims) ([]permission.Permission, error) {
	if f.perms != nil {
		return f.perms, nil
	}

	return []permission.Permission{permission.Trading, permission.TradingMargin}, nil
}

func testConfig() *config.Configuration {
	return &config.Configuration{
		Env:              "test",
		JWTSecret:        "test-secret",
		TaxAccountNumber: "444000000000000099",
		URLs: config.URLConfig{
			FrontendBaseURL: "http://localhost:5173",
		},
	}
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	tx := sharedDB.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}

	t.Cleanup(func() {
		tx.Rollback()
	})

	return tx
}

func setupTestRouter(t *testing.T, db *gorm.DB) (*gin.Engine, *fakeUserClient) {
	return setupTestRouterWithPermissions(t, db, nil)
}

func setupTestRouterWithPermissions(t *testing.T, db *gorm.DB, perms []permission.Permission) (*gin.Engine, *fakeUserClient) {
	t.Helper()

	cfg := testConfig()

	userClient := &fakeUserClient{
		supervisorIDs: map[uint64]bool{10: true},
		agentIDs:      map[uint64]bool{20: true},
	}
	var bankingClient client.BankingClient = &fakeBankingClient{
		accountByNumber: map[string]uint64{
			"seller-acc": 20,
			"buyer-acc":  10,
		},
	}

	var permProvider auth.PermissionProvider = &fakePermissionProvider{perms: perms}

	exchangeRepo := repository.NewExchangeRepository(db)
	listingRepo := repository.NewListingRepository(db)
	orderRepo := repository.NewOrderRepository(db)
	orderTxRepo := repository.NewOrderTransactionRepository(db)
	assetOwnershipRepo := repository.NewAssetOwnershipRepository(db)
	stockRepo := repository.NewStockRepository(db)
	futuresRepo := repository.NewFuturesContractRepository(db)
	forexRepo := repository.NewForexRepository(db)
	optionRepo := repository.NewOptionRepository(db)
	taxRepo := repository.NewTaxRepository(db)
	otcOfferRepo := repository.NewOtcOfferRepository(db)
	otcContractRepo := repository.NewOtcOptionContractRepository(db)
	otcShareReservationRepo := repository.NewOtcShareReservationRepository(db)
	otcExecutionRepo := repository.NewOtcExecutionSagaRepository(db)
	txManager := repository.NewGormTransactionManager(db)

	auditSvc := audit.NewService(audit.NewRepository(db))

	exchangeSvc := service.NewExchangeService(exchangeRepo)
	listingSvc := service.NewListingService(listingRepo, futuresRepo, forexRepo, optionRepo)
	fundRepo := repository.NewInvestmentFundRepository(db)
	var taxRecorder service.TaxRecorder = &fakeTaxRecorder{}
	orderSvc := service.NewOrderService(orderRepo, orderTxRepo, exchangeRepo, listingRepo, assetOwnershipRepo, futuresRepo, optionRepo, fundRepo, userClient, bankingClient, taxRecorder, auditSvc)
	positionRepo := repository.NewClientFundPositionRepository(db)
	investmentRepo := repository.NewClientFundInvestmentRepository(db)
	redemptionRepo := repository.NewClientFundRedemptionRepository(db)
	fundSvc := service.NewInvestmentFundService(fundRepo, positionRepo, listingRepo, investmentRepo, redemptionRepo, assetOwnershipRepo, exchangeRepo, stockRepo, optionRepo, futuresRepo, forexRepo, bankingClient, userClient, nil)
	fundHandler := handler.NewInvestmentFundHandler(fundSvc)
	portfolioSvc := service.NewPortfolioService(assetOwnershipRepo, stockRepo, optionRepo, futuresRepo, forexRepo, bankingClient, userClient)

	taxSvc := service.NewTaxService(taxRepo, bankingClient, cfg, auditSvc)
	otcSvc := service.NewOTCService(assetOwnershipRepo, listingRepo, userClient)
	otcProcessingSvc := service.NewOtcDealProcessingService(otcOfferRepo, otcContractRepo, otcShareReservationRepo, otcExecutionRepo, assetOwnershipRepo, txManager, bankingClient)
	otcOfferSvc := service.NewOtcOfferService(otcOfferRepo, otcContractRepo, assetOwnershipRepo, stockRepo, bankingClient,
		userClient, otcProcessingSvc)

	healthHandler := handler.NewHealthHandler()
	exchangeHandler := handler.NewExchangeHandler(exchangeSvc)
	listingHandler := handler.NewListingHandler(listingSvc)
	orderHandler := handler.NewOrderHandler(orderSvc)
	portfolioHandler := handler.NewPortfolioHandler(portfolioSvc)
	taxHandler := handler.NewTaxHandler(taxSvc, userClient)
	otcHandler := handler.NewOTCHandler(otcSvc)
	otcOfferHandler := handler.NewOtcOfferHandler(otcOfferSvc)
	watchlistHandler := handler.NewWatchlistHandler(service.NewWatchlistService(repository.NewWatchlistRepository(db), listingRepo))

	verifier := auth.TokenVerifier(commonjwt.NewJWTVerifier(cfg.JWTSecret))

	r := gin.New()
	server.InitRouter(r, cfg)
	server.SetupRoutes(r, healthHandler, taxHandler, exchangeHandler, orderHandler, portfolioHandler, listingHandler, otcHandler, otcOfferHandler, fundHandler, watchlistHandler, verifier, permProvider, userClient)

	return r, userClient
}

func seedExchange(t *testing.T, db *gorm.DB, micCode string) *model.Exchange {
	t.Helper()

	exchange := &model.Exchange{
		Name:           fmt.Sprintf("Exchange %s", micCode),
		Acronym:        micCode,
		MicCode:        micCode,
		Polity:         "United States",
		Currency:       "USD",
		TimeZone:       -5,
		OpenTime:       "09:30",
		CloseTime:      "16:00",
		TradingEnabled: true,
	}

	if err := db.Create(exchange).Error; err != nil {
		t.Fatalf("seed exchange: %v", err)
	}

	return exchange
}

func seedListing(t *testing.T, db *gorm.DB, ticker, exchangeMIC string, assetType model.AssetType, price float64) *model.Listing {
	t.Helper()

	asset := &model.Asset{
		Ticker:    ticker,
		Name:      fmt.Sprintf("Listing %s", ticker),
		AssetType: assetType,
	}
	if err := db.Create(asset).Error; err != nil {
		t.Fatalf("seed asset: %v", err)
	}

	listing := &model.Listing{
		AssetID:     asset.AssetID,
		ExchangeMIC: exchangeMIC,
		LastRefresh: time.Now(),
		Price:       price,
		Ask:         price * 1.01,
	}

	if err := db.Create(listing).Error; err != nil {
		t.Fatalf("seed listing: %v", err)
	}

	return listing
}

func seedStock(t *testing.T, db *gorm.DB, listingID uint) *model.Stock {
	t.Helper()

	var listing model.Listing
	if err := db.First(&listing, listingID).Error; err != nil {
		t.Fatalf("seed stock: lookup listing: %v", err)
	}

	stock := &model.Stock{
		AssetID:           listing.AssetID,
		OutstandingShares: 1_000_000,
		DividendYield:     2.5,
	}

	if err := db.Create(stock).Error; err != nil {
		t.Fatalf("seed stock: %v", err)
	}

	return stock
}

func seedFuture(t *testing.T, db *gorm.DB, listingID uint) *model.FuturesContract {
	t.Helper()

	var listing model.Listing
	if err := db.First(&listing, listingID).Error; err != nil {
		t.Fatalf("seed future: lookup listing: %v", err)
	}

	fc := &model.FuturesContract{
		AssetID:        listing.AssetID,
		ContractSize:   100,
		ContractUnit:   "barrels",
		SettlementDate: time.Now().AddDate(0, 3, 0),
	}

	if err := db.Create(fc).Error; err != nil {
		t.Fatalf("seed future: %v", err)
	}

	return fc
}

func seedForex(t *testing.T, db *gorm.DB, listingID uint) *model.ForexPair {
	t.Helper()

	var listing model.Listing
	if err := db.First(&listing, listingID).Error; err != nil {
		t.Fatalf("seed forex: lookup listing: %v", err)
	}

	pair := &model.ForexPair{
		AssetID: listing.AssetID,
		Base:    "EUR",
		Quote:   "USD",
		Rate:    1.08,
	}

	if err := db.Create(pair).Error; err != nil {
		t.Fatalf("seed forex: %v", err)
	}

	return pair
}

func seedOption(t *testing.T, db *gorm.DB, listingID, stockID uint) *model.Option {
	t.Helper()

	var listing model.Listing
	if err := db.First(&listing, listingID).Error; err != nil {
		t.Fatalf("seed option: lookup listing: %v", err)
	}

	opt := &model.Option{
		AssetID:           listing.AssetID,
		StockID:           stockID,
		OptionType:        model.OptionTypeCall,
		StrikePrice:       150,
		ContractSize:      100,
		SettlementDate:    time.Now().AddDate(0, 1, 0),
		ImpliedVolatility: 0.3,
		OpenInterest:      500,
	}

	if err := db.Create(opt).Error; err != nil {
		t.Fatalf("seed option: %v", err)
	}

	return opt
}

func seedOrder(t *testing.T, db *gorm.DB, userID, listingID uint, direction model.OrderDirection, status model.OrderStatus) *model.Order {
	t.Helper()

	ppu := 5.0
	order := &model.Order{
		OrderOwnerUserID: userID,
		AccountNumber:    "444000100000000001",
		ListingID:        listingID,
		OrderType:        model.OrderTypeMarket,
		Direction:        direction,
		Quantity:         10,
		ContractSize:     1,
		PricePerUnit:     &ppu,
		Status:           status,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := db.Create(order).Error; err != nil {
		t.Fatalf("seed order: %v", err)
	}

	return order
}

func seedDailyPriceInfo(t *testing.T, db *gorm.DB, listingID uint) {
	t.Helper()

	info := &model.ListingDailyPriceInfo{
		ListingID: listingID,
		Date:      time.Now().AddDate(0, 0, -1),
		Price:     100,
		Ask:       101,
		Bid:       99,
		Change:    1.5,
		Volume:    50000,
	}

	if err := db.Create(info).Error; err != nil {
		t.Fatalf("seed daily price info: %v", err)
	}
}

func seedInvestmentFund(t *testing.T, db *gorm.DB, name string, managerID uint) *model.InvestmentFund {
	t.Helper()

	fund := &model.InvestmentFund{
		Name:                name,
		Description:         fmt.Sprintf("Description for %s", name),
		MinimumContribution: 1000.0,
		ManagerID:           managerID,
		AccountNumber:       fmt.Sprintf("444000199999%06d", uniqueCounter.Add(1)),
		CreatedAt:           time.Now(),
	}

	if err := db.Create(fund).Error; err != nil {
		t.Fatalf("seed investment fund: %v", err)
	}

	return fund
}
func authHeaderForSupervisor(t *testing.T) string {
	t.Helper()

	eid := uint(10)
	token, err := commonjwt.GenerateToken(&commonjwt.Claims{
		IdentityID:   100,
		IdentityType: string(auth.IdentityEmployee),
		EmployeeID:   &eid,
	}, testConfig().JWTSecret, 15)
	if err != nil {
		t.Fatalf("generate supervisor token: %v", err)
	}

	return "Bearer " + token
}

func authHeaderForAgent(t *testing.T) string {
	t.Helper()

	eid := uint(20)
	token, err := commonjwt.GenerateToken(&commonjwt.Claims{
		IdentityID:   200,
		IdentityType: string(auth.IdentityEmployee),
		EmployeeID:   &eid,
	}, testConfig().JWTSecret, 15)
	if err != nil {
		t.Fatalf("generate agent token: %v", err)
	}

	return "Bearer " + token
}

func authHeaderForClient(t *testing.T, identityID, clientID uint) string {
	t.Helper()

	cid := clientID
	token, err := commonjwt.GenerateToken(&commonjwt.Claims{
		IdentityID:   identityID,
		IdentityType: string(auth.IdentityClient),
		ClientID:     &cid,
	}, testConfig().JWTSecret, 15)
	if err != nil {
		t.Fatalf("generate client token: %v", err)
	}

	return "Bearer " + token
}

func uniqueValue(t *testing.T, prefix string) string {
	t.Helper()
	return fmt.Sprintf("%s%d", prefix, uniqueCounter.Add(1))
}

func performRequest(t *testing.T, router *gin.Engine, method, path string, body any, authorization string) *httptest.ResponseRecorder {
	t.Helper()

	var bodyReader *bytes.Reader
	if body == nil {
		bodyReader = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func decodeResponse[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()

	var response T
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response body: %v; body=%s", err, recorder.Body.String())
	}
	return response
}

func requireStatus(t *testing.T, recorder *httptest.ResponseRecorder, expected int) {
	t.Helper()

	if recorder.Code != expected {
		t.Fatalf("expected status %d, got %d, body=%s", expected, recorder.Code, recorder.Body.String())
	}
}
