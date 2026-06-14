package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/config"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/middleware"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/repository"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/service"
)

const handlerOurRouting = 444

type fakeHandlerBankingClient struct {
	prepareCalls []*pb.PrepareInterbankCashPostingRequest
	commitCalls  []string
}

func (f *fakeHandlerBankingClient) ReserveOtcFunds(context.Context, *pb.ReserveOtcFundsRequest) (*pb.OtcFundsReservationResponse, error) {
	return &pb.OtcFundsReservationResponse{}, nil
}
func (f *fakeHandlerBankingClient) ReleaseOtcFunds(context.Context, string) (*pb.OtcFundsReservationResponse, error) {
	return &pb.OtcFundsReservationResponse{}, nil
}
func (f *fakeHandlerBankingClient) CommitOtcFunds(context.Context, string) (*pb.OtcFundsReservationResponse, error) {
	return &pb.OtcFundsReservationResponse{}, nil
}
func (f *fakeHandlerBankingClient) RefundOtcFunds(context.Context, string) (*pb.OtcFundsReservationResponse, error) {
	return &pb.OtcFundsReservationResponse{}, nil
}
func (f *fakeHandlerBankingClient) PrepareInterbankCashPosting(_ context.Context, req *pb.PrepareInterbankCashPostingRequest) (*pb.InterbankCashPostingResponse, error) {
	f.prepareCalls = append(f.prepareCalls, req)
	return &pb.InterbankCashPostingResponse{}, nil
}
func (f *fakeHandlerBankingClient) CommitInterbankCashPosting(_ context.Context, postingID string) (*pb.InterbankCashPostingResponse, error) {
	f.commitCalls = append(f.commitCalls, postingID)
	return &pb.InterbankCashPostingResponse{}, nil
}
func (f *fakeHandlerBankingClient) RollbackInterbankCashPosting(context.Context, string) (*pb.InterbankCashPostingResponse, error) {
	return &pb.InterbankCashPostingResponse{}, nil
}
func (f *fakeHandlerBankingClient) FinalizeInterbankPayment(context.Context, uint64, bool) error {
	return nil
}

type fakeHandlerTradingClient struct {
	publicStocks *pb.ListPublicStocksResponse
}

func (f *fakeHandlerTradingClient) ListPublicStocks(context.Context) (*pb.ListPublicStocksResponse, error) {
	if f.publicStocks != nil {
		return f.publicStocks, nil
	}
	return &pb.ListPublicStocksResponse{}, nil
}
func (f *fakeHandlerTradingClient) ReservePeerOtcShares(context.Context, *pb.ReservePeerOtcSharesRequest) (*pb.PeerOtcSharesResponse, error) {
	return &pb.PeerOtcSharesResponse{}, nil
}
func (f *fakeHandlerTradingClient) ReleasePeerOtcShares(context.Context, string) (*pb.PeerOtcSharesResponse, error) {
	return &pb.PeerOtcSharesResponse{}, nil
}
func (f *fakeHandlerTradingClient) ConsumePeerOtcShares(context.Context, string) (*pb.PeerOtcSharesResponse, error) {
	return &pb.PeerOtcSharesResponse{}, nil
}
func (f *fakeHandlerTradingClient) CreditPeerOtcShares(context.Context, *pb.CreditPeerOtcSharesRequest) (*pb.PeerOtcSharesResponse, error) {
	return &pb.PeerOtcSharesResponse{}, nil
}

type fakeHandlerUserClient struct{}

func (fakeHandlerUserClient) GetClientByID(context.Context, uint64) (*pb.GetClientByIdResponse, error) {
	return &pb.GetClientByIdResponse{}, nil
}
func (fakeHandlerUserClient) GetUserByIdentityID(_ context.Context, identityID uint64) (*pb.GetUserByIdentityIdResponse, error) {
	return &pb.GetUserByIdentityIdResponse{UserId: identityID, UserType: "CLIENT", FullName: "User Name"}, nil
}

type protocolHandlerSetup struct {
	db       *gorm.DB
	router   *gin.Engine
	banking  *fakeHandlerBankingClient
	trading  *fakeHandlerTradingClient
	otc      *service.PeerOtcService
	otcRepos struct {
		contracts    repository.PeerContractRepository
		negotiations repository.PeerNegotiationRepository
	}
}

func newProtocolHandlerSetup(t *testing.T) *protocolHandlerSetup {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	database, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(
		&model.InboundMessage{},
		&model.PreparedTransaction{},
		&model.OutboundMessage{},
		&model.PeerNegotiation{},
		&model.PeerContract{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	peers := service.NewPeerResolver(
		config.NewPeerRegistry([]config.Peer{{RoutingNumber: 111, BaseURL: "http://127.0.0.1:1", OurAPIKey: "to-peer", TheirAPIKey: "from-peer"}}),
		&config.Configuration{OurRoutingNumber: handlerOurRouting, OurBankDisplayName: "Banka 4"},
	)
	inbound := repository.NewInboundMessageRepository(database)
	prepared := repository.NewPreparedTransactionRepository(database)
	outbound := repository.NewOutboundMessageRepository(database)
	negotiations := repository.NewPeerNegotiationRepository(database)
	contracts := repository.NewPeerContractRepository(database)
	txManager := repository.NewGormTransactionManager(database)
	banking := &fakeHandlerBankingClient{}
	trading := &fakeHandlerTradingClient{publicStocks: &pb.ListPublicStocksResponse{
		Stocks: []*pb.PublicStockEntry{{Ticker: "AAPL", Sellers: []*pb.PublicStockSeller{{SellerId: 7, Amount: 11}}}},
	}}
	userClient := fakeHandlerUserClient{}
	processor := service.NewMessageProcessor(inbound, prepared, outbound, txManager, peers, banking, trading, contracts, negotiations, userClient)
	otc := service.NewPeerOtcService(negotiations, contracts, peers, service.NewPeerOtcClient(peers), trading, userClient, banking, processor, outbound, txManager)

	router := gin.New()
	router.Use(errors.ErrorHandler())
	router.Use(func(c *gin.Context) {
		c.Set(middleware.PeerContextKey, 111)
		c.Next()
	})
	interbankHandler := NewInterbankHandler(processor)
	peerOtcHandler := NewPeerOtcHandler(otc)
	router.POST("/interbank", interbankHandler.Receive)
	router.POST("/interbank/negotiations", peerOtcHandler.CreateNegotiation)
	router.GET("/interbank/negotiations/:rn/:id", peerOtcHandler.GetNegotiation)
	router.PUT("/interbank/negotiations/:rn/:id", peerOtcHandler.UpdateNegotiation)
	router.DELETE("/interbank/negotiations/:rn/:id", peerOtcHandler.DeleteNegotiation)
	router.GET("/interbank/public-stock", peerOtcHandler.PublicStock)
	router.GET("/interbank/user/:rn/:id", peerOtcHandler.UserLookup)

	return &protocolHandlerSetup{
		db:      database,
		router:  router,
		banking: banking,
		trading: trading,
		otc:     otc,
		otcRepos: struct {
			contracts    repository.PeerContractRepository
			negotiations repository.PeerNegotiationRepository
		}{contracts: contracts, negotiations: negotiations},
	}
}

func performJSON(router *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func handlerMonas(currency string) dto.Asset {
	return dto.Asset{Type: dto.AssetMonas, Body: map[string]any{"currency": currency}}
}

func handlerAcctPosting(num string, amount float64, asset dto.Asset) dto.Posting {
	n := num
	return dto.Posting{Account: dto.TxAccount{Type: dto.TxAccountAccount, Num: &n}, Amount: amount, Asset: asset}
}

func TestInterbankHandlerReceiveNewTxAndCommit(t *testing.T) {
	t.Parallel()

	setup := newProtocolHandlerSetup(t)
	txID := dto.ForeignBankId{RoutingNumber: 111, ID: "handler-tx"}
	tx := dto.Transaction{
		TransactionID: txID,
		Postings: []dto.Posting{
			handlerAcctPosting("444000000000000011", -100, handlerMonas("RSD")),
			handlerAcctPosting("111000000000000011", 100, handlerMonas("RSD")),
		},
	}

	newTx := dto.NewTxMessage{
		IdempotenceKey: dto.IdempotenceKey{RoutingNumber: 111, LocallyGeneratedKey: "new-key"},
		MessageType:    dto.MessageTypeNewTx,
		Message:        tx,
	}
	rec := performJSON(setup.router, http.MethodPost, "/interbank", newTx)
	if rec.Code != http.StatusOK {
		t.Fatalf("new tx status = %d body=%s", rec.Code, rec.Body.String())
	}
	var vote dto.TransactionVote
	if err := json.Unmarshal(rec.Body.Bytes(), &vote); err != nil {
		t.Fatalf("decode vote: %v", err)
	}
	if vote.Vote != dto.VoteYes || len(setup.banking.prepareCalls) != 1 {
		t.Fatalf("unexpected vote/calls vote=%#v calls=%d", vote, len(setup.banking.prepareCalls))
	}

	commit := dto.CommitTxMessage{
		IdempotenceKey: dto.IdempotenceKey{RoutingNumber: 111, LocallyGeneratedKey: "commit-key"},
		MessageType:    dto.MessageTypeCommitTx,
		Message:        dto.CommitTransaction{TransactionID: txID},
	}
	rec = performJSON(setup.router, http.MethodPost, "/interbank", commit)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("commit status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(setup.banking.commitCalls) != 1 {
		t.Fatalf("commit calls = %d, want 1", len(setup.banking.commitCalls))
	}
}

func TestInterbankHandlerRejectsMismatchedIdempotenceSender(t *testing.T) {
	t.Parallel()

	setup := newProtocolHandlerSetup(t)
	msg := dto.NewTxMessage{
		IdempotenceKey: dto.IdempotenceKey{RoutingNumber: 222, LocallyGeneratedKey: "bad-key"},
		MessageType:    dto.MessageTypeNewTx,
		Message:        dto.Transaction{TransactionID: dto.ForeignBankId{RoutingNumber: 222, ID: "tx"}},
	}
	rec := performJSON(setup.router, http.MethodPost, "/interbank", msg)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPeerOtcHandlerNegotiationLookupAndClose(t *testing.T) {
	t.Parallel()

	setup := newProtocolHandlerSetup(t)
	offer := dto.OtcOffer{
		Stock:              dto.StockDescription{Ticker: "AAPL"},
		SettlementDate:     "2030-01-01",
		PricePerUnit:       dto.MonetaryValue{Currency: "RSD", Amount: 100},
		Premium:            dto.MonetaryValue{Currency: "RSD", Amount: 5},
		BuyerID:            dto.ForeignBankId{RoutingNumber: 111, ID: "buyer-1"},
		SellerID:           dto.ForeignBankId{RoutingNumber: handlerOurRouting, ID: "7"},
		Amount:             10,
		LastModifiedBy:     dto.ForeignBankId{RoutingNumber: 111, ID: "buyer-1"},
		BuyerAccountNumber: "111000000000000011",
	}
	rec := performJSON(setup.router, http.MethodPost, "/interbank/negotiations", offer)
	if rec.Code != http.StatusOK {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body.String())
	}
	var id dto.ForeignBankId
	if err := json.Unmarshal(rec.Body.Bytes(), &id); err != nil {
		t.Fatalf("decode id: %v", err)
	}
	if id.RoutingNumber != handlerOurRouting || id.ID == "" {
		t.Fatalf("unexpected id %#v", id)
	}

	rec = performJSON(setup.router, http.MethodGet, "/interbank/negotiations/444/"+id.ID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%s", rec.Code, rec.Body.String())
	}
	var negotiation dto.OtcNegotiation
	if err := json.Unmarshal(rec.Body.Bytes(), &negotiation); err != nil {
		t.Fatalf("decode negotiation: %v", err)
	}
	if !negotiation.IsOngoing || negotiation.Stock.Ticker != "AAPL" {
		t.Fatalf("unexpected negotiation %#v", negotiation)
	}

	rec = performJSON(setup.router, http.MethodPut, "/interbank/negotiations/444/"+id.ID, offer)
	if rec.Code != http.StatusConflict {
		t.Fatalf("same party counter status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performJSON(setup.router, http.MethodDelete, "/interbank/negotiations/444/"+id.ID, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d body=%s", rec.Code, rec.Body.String())
	}
	stored, err := setup.otcRepos.negotiations.FindByID(context.Background(), id.RoutingNumber, id.ID)
	if err != nil {
		t.Fatalf("find stored negotiation: %v", err)
	}
	if stored.Status != model.PeerNegotiationCancelled {
		t.Fatalf("status = %q, want cancelled", stored.Status)
	}
}

func TestPeerOtcHandlerPublicStockAndUserLookup(t *testing.T) {
	t.Parallel()

	setup := newProtocolHandlerSetup(t)
	rec := performJSON(setup.router, http.MethodGet, "/interbank/public-stock", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("public stock status = %d body=%s", rec.Code, rec.Body.String())
	}
	var stocks []dto.PublicStock
	if err := json.Unmarshal(rec.Body.Bytes(), &stocks); err != nil {
		t.Fatalf("decode stocks: %v", err)
	}
	if len(stocks) != 1 || stocks[0].Stock.Ticker != "AAPL" || stocks[0].Sellers[0].Seller.RoutingNumber != handlerOurRouting {
		t.Fatalf("unexpected stocks %#v", stocks)
	}

	rec = performJSON(setup.router, http.MethodGet, "/interbank/user/444/7", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("user lookup status = %d body=%s", rec.Code, rec.Body.String())
	}
	var info dto.UserInformation
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
		t.Fatalf("decode user: %v", err)
	}
	if info.BankDisplayName != "Banka 4" || info.DisplayName != "User Name" {
		t.Fatalf("unexpected user info %#v", info)
	}
}

func TestPeerOtcFrontendHandlerListsLocalState(t *testing.T) {
	t.Parallel()

	setup := newProtocolHandlerSetup(t)
	authRouter := gin.New()
	authRouter.Use(errors.ErrorHandler())
	authRouter.Use(func(c *gin.Context) {
		auth.SetAuth(c, &auth.AuthContext{IdentityID: 9})
		c.Next()
	})
	frontend := NewPeerOtcFrontendHandler(setup.otc)
	authRouter.GET("/api/peer-otc/public-stocks", frontend.ListPublicStocks)
	authRouter.POST("/api/peer-otc/negotiations", frontend.CreateNegotiation)
	authRouter.GET("/api/peer-otc/negotiations", frontend.ListMyNegotiations)
	authRouter.GET("/api/peer-otc/contracts", frontend.ListMyContracts)

	neg := &model.PeerNegotiation{
		ID:                    "local-neg",
		BuyerRoutingNumber:    handlerOurRouting,
		BuyerID:               "9",
		SellerRoutingNumber:   111,
		SellerID:              "seller",
		Ticker:                "AAPL",
		Amount:                2,
		PricePerStock:         100,
		PriceCurrency:         "RSD",
		Premium:               5,
		PremiumCurrency:       "RSD",
		SettlementDate:        "2030-01-01",
		BuyerAccountNumber:    "444000000000000011",
		LastModifiedByRouting: handlerOurRouting,
		LastModifiedByID:      "9",
		Status:                model.PeerNegotiationOngoing,
	}
	if err := setup.otcRepos.negotiations.Create(context.Background(), neg); err != nil {
		t.Fatalf("seed negotiation: %v", err)
	}
	if err := setup.otcRepos.contracts.Create(context.Background(), &model.PeerContract{
		AuthorityRoutingNumber: 111,
		ID:                     "local-contract",
		NegotiationID:          "local-neg",
		BuyerRoutingNumber:     handlerOurRouting,
		BuyerID:                "9",
		SellerRoutingNumber:    111,
		SellerID:               "seller",
		Ticker:                 "AAPL",
		Amount:                 2,
		StrikePrice:            100,
		StrikeCurrency:         "RSD",
		Premium:                5,
		PremiumCurrency:        "RSD",
		SettlementDate:         "2030-01-01",
		Status:                 model.PeerContractActive,
	}); err != nil {
		t.Fatalf("seed contract: %v", err)
	}

	rec := performJSON(authRouter, http.MethodGet, "/api/peer-otc/public-stocks", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list peer public stocks status = %d body=%s", rec.Code, rec.Body.String())
	}
	var peerStocks []dto.PublicStock
	if err := json.Unmarshal(rec.Body.Bytes(), &peerStocks); err != nil {
		t.Fatalf("decode peer public stocks: %v", err)
	}
	if len(peerStocks) != 0 {
		t.Fatalf("expected unavailable peer to be skipped, got %#v", peerStocks)
	}

	rec = performJSON(authRouter, http.MethodPost, "/api/peer-otc/negotiations", gin.H{
		"sellerId":        gin.H{"routingNumber": handlerOurRouting, "id": "7"},
		"ticker":          "AAPL",
		"amount":          1,
		"pricePerStock":   100,
		"priceCurrency":   "RSD",
		"premium":         5,
		"premiumCurrency": "RSD",
		"settlementDate":  "2030-01-01",
		"accountNumber":   "444000000000000011",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("same-bank negotiation status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performJSON(authRouter, http.MethodGet, "/api/peer-otc/negotiations", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list negotiations status = %d body=%s", rec.Code, rec.Body.String())
	}
	var negotiations []dto.OtcNegotiationView
	if err := json.Unmarshal(rec.Body.Bytes(), &negotiations); err != nil {
		t.Fatalf("decode negotiations: %v", err)
	}
	if len(negotiations) != 1 || negotiations[0].ID.ID != "local-neg" {
		t.Fatalf("unexpected negotiations %#v", negotiations)
	}

	rec = performJSON(authRouter, http.MethodGet, "/api/peer-otc/contracts", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list contracts status = %d body=%s", rec.Code, rec.Body.String())
	}
	var contracts []dto.PeerContract
	if err := json.Unmarshal(rec.Body.Bytes(), &contracts); err != nil {
		t.Fatalf("decode contracts: %v", err)
	}
	if len(contracts) != 1 || contracts[0].ID.ID != "local-contract" {
		t.Fatalf("unexpected contracts %#v", contracts)
	}
}

func TestPeerOtcHandlerRejectsInvalidProtocolRequests(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	peerHandler := NewPeerOtcHandler(nil)
	router := gin.New()
	router.Use(errors.ErrorHandler())
	router.Use(func(c *gin.Context) {
		c.Set(middleware.PeerContextKey, 111)
		c.Next()
	})
	router.POST("/interbank/negotiations", peerHandler.CreateNegotiation)
	router.GET("/interbank/negotiations/:rn/:id", peerHandler.GetNegotiation)
	router.PUT("/interbank/negotiations/:rn/:id", peerHandler.UpdateNegotiation)
	router.DELETE("/interbank/negotiations/:rn/:id", peerHandler.DeleteNegotiation)
	router.GET("/interbank/negotiations/:rn/:id/accept", peerHandler.AcceptNegotiation)
	router.GET("/interbank/user/:rn/:id", peerHandler.UserLookup)

	cases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{name: "create malformed offer", method: http.MethodPost, path: "/interbank/negotiations"},
		{name: "get bad routing number", method: http.MethodGet, path: "/interbank/negotiations/bad/neg-1"},
		{name: "update bad routing number", method: http.MethodPut, path: "/interbank/negotiations/bad/neg-1", body: dto.OtcOffer{}},
		{name: "update malformed offer", method: http.MethodPut, path: "/interbank/negotiations/444/neg-1"},
		{name: "delete bad routing number", method: http.MethodDelete, path: "/interbank/negotiations/bad/neg-1"},
		{name: "accept bad routing number", method: http.MethodGet, path: "/interbank/negotiations/bad/neg-1/accept"},
		{name: "user lookup bad routing number", method: http.MethodGet, path: "/interbank/user/bad/7"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := performJSON(router, tc.method, tc.path, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("%s %s status = %d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
			}
		})
	}

	noPeerRouter := gin.New()
	noPeerRouter.Use(errors.ErrorHandler())
	noPeerRouter.POST("/interbank/negotiations", peerHandler.CreateNegotiation)
	rec := performJSON(noPeerRouter, http.MethodPost, "/interbank/negotiations", dto.OtcOffer{})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing peer context status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPeerOtcFrontendHandlerRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	frontend := NewPeerOtcFrontendHandler(nil)

	router := gin.New()
	router.Use(errors.ErrorHandler())
	router.Use(func(c *gin.Context) {
		auth.SetAuth(c, &auth.AuthContext{IdentityID: 9})
		c.Next()
	})
	router.POST("/api/peer-otc/negotiations", frontend.CreateNegotiation)
	router.PUT("/api/peer-otc/negotiations/:rn/:id/counter", frontend.SendCounterOffer)
	router.POST("/api/peer-otc/negotiations/:rn/:id/accept", frontend.AcceptNegotiation)
	router.DELETE("/api/peer-otc/negotiations/:rn/:id", frontend.Withdraw)
	router.POST("/api/peer-otc/contracts/:rn/:id/exercise", frontend.ExerciseContract)

	cases := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{name: "create missing fields", method: http.MethodPost, path: "/api/peer-otc/negotiations", body: gin.H{"ticker": "AAPL"}},
		{name: "counter bad routing number", method: http.MethodPut, path: "/api/peer-otc/negotiations/bad/neg-1/counter", body: gin.H{}},
		{name: "counter missing terms", method: http.MethodPut, path: "/api/peer-otc/negotiations/111/neg-1/counter", body: gin.H{"amount": 0}},
		{name: "accept bad routing number", method: http.MethodPost, path: "/api/peer-otc/negotiations/bad/neg-1/accept", body: gin.H{}},
		{name: "withdraw bad routing number", method: http.MethodDelete, path: "/api/peer-otc/negotiations/bad/neg-1"},
		{name: "exercise bad routing number", method: http.MethodPost, path: "/api/peer-otc/contracts/bad/contract-1/exercise", body: gin.H{"accountNumber": "444000000000000011"}},
		{name: "exercise missing account", method: http.MethodPost, path: "/api/peer-otc/contracts/111/contract-1/exercise", body: gin.H{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := performJSON(router, tc.method, tc.path, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("%s %s status = %d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
			}
		})
	}

	noAuthRouter := gin.New()
	noAuthRouter.Use(errors.ErrorHandler())
	noAuthRouter.GET("/api/peer-otc/negotiations", frontend.ListMyNegotiations)
	rec := performJSON(noAuthRouter, http.MethodGet, "/api/peer-otc/negotiations", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing auth status = %d body=%s", rec.Code, rec.Body.String())
	}
}
