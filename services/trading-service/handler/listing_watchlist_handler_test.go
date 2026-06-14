package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	appErrors "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/permission"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/service"
	tradingvalidator "github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/validator"
)

type handlerListingFixtures struct {
	stockListingID  uint
	futureListingID uint
	forexListingID  uint
	optionListingID uint
}

func setupTradingHandlerDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
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
		&model.Watchlist{},
		&model.WatchlistItem{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func seedTradingHandlerListings(t *testing.T, db *gorm.DB) handlerListingFixtures {
	t.Helper()

	now := time.Now().UTC()
	exchanges := []model.Exchange{
		{Name: "Nasdaq", Acronym: "NASDAQ", MicCode: "XNAS", Polity: "US", Currency: "USD", OpenTime: "09:30", CloseTime: "16:00", TradingEnabled: true},
		{Name: "Simulation", Acronym: "SIM", MicCode: model.SimulatedExchangeMIC, Polity: "US", Currency: "USD", OpenTime: "00:00", CloseTime: "23:59", TradingEnabled: true},
	}
	if err := db.Create(&exchanges).Error; err != nil {
		t.Fatalf("create exchanges: %v", err)
	}

	stockAsset := model.Asset{Ticker: "AAPL", Name: "Apple Inc", AssetType: model.AssetTypeStock}
	futureAsset := model.Asset{Ticker: "ESM6", Name: "S&P Future", AssetType: model.AssetTypeFuture}
	forexAsset := model.Asset{Ticker: "EUR/USD", Name: "EUR/USD", AssetType: model.AssetTypeForexPair}
	optionAsset := model.Asset{Ticker: "AAPL260C", Name: "AAPL Call", AssetType: model.AssetTypeOption}
	if err := db.Create(&[]*model.Asset{&stockAsset, &futureAsset, &forexAsset, &optionAsset}).Error; err != nil {
		t.Fatalf("create assets: %v", err)
	}

	listings := []model.Listing{
		{AssetID: stockAsset.AssetID, ExchangeMIC: "XNAS", Price: 150, Ask: 151, MaintenanceMargin: 10, LastRefresh: now},
		{AssetID: futureAsset.AssetID, ExchangeMIC: "XNAS", Price: 5200, Ask: 5201, MaintenanceMargin: 100, LastRefresh: now},
		{AssetID: forexAsset.AssetID, ExchangeMIC: model.SimulatedExchangeMIC, Price: 1.08, Ask: 1.081, MaintenanceMargin: 0, LastRefresh: now},
		{AssetID: optionAsset.AssetID, ExchangeMIC: model.SimulatedExchangeMIC, Price: 12, Ask: 13, MaintenanceMargin: 5, LastRefresh: now},
	}
	if err := db.Create(&listings).Error; err != nil {
		t.Fatalf("create listings: %v", err)
	}

	stock := model.Stock{AssetID: stockAsset.AssetID, OutstandingShares: 1_000_000, DividendYield: 0.5}
	if err := db.Create(&stock).Error; err != nil {
		t.Fatalf("create stock: %v", err)
	}
	settlement := now.AddDate(0, 1, 0).Truncate(24 * time.Hour)
	if err := db.Create(&model.FuturesContract{AssetID: futureAsset.AssetID, ContractSize: 50, ContractUnit: "index", SettlementDate: settlement}).Error; err != nil {
		t.Fatalf("create future: %v", err)
	}
	if err := db.Create(&model.ForexPair{AssetID: forexAsset.AssetID, Base: "EUR", Quote: "USD", Rate: 1.08, ProviderUpdatedAt: now, ProviderNextUpdateAt: now.Add(time.Hour)}).Error; err != nil {
		t.Fatalf("create forex pair: %v", err)
	}
	if err := db.Create(&model.Option{AssetID: optionAsset.AssetID, StockID: stock.StockID, OptionType: model.OptionTypeCall, StrikePrice: 160, ContractSize: 100, SettlementDate: settlement}).Error; err != nil {
		t.Fatalf("create option: %v", err)
	}

	priceInfos := []model.ListingDailyPriceInfo{
		{ListingID: listings[0].ListingID, Date: now.Add(-24 * time.Hour), Price: 149, Ask: 150, Bid: 148, Change: -1, Volume: 100},
		{ListingID: listings[0].ListingID, Date: now, Price: 150, Ask: 151, Bid: 149.5, Change: 2, Volume: 500},
		{ListingID: listings[1].ListingID, Date: now, Price: 5200, Ask: 5201, Bid: 5199, Change: 5, Volume: 250},
		{ListingID: listings[2].ListingID, Date: now, Price: 1.08, Ask: 1.081, Bid: 1.079, Change: 0.01, Volume: 50},
		{ListingID: listings[3].ListingID, Date: now, Price: 12, Ask: 13, Bid: 11, Change: 0.5, Volume: 80},
	}
	if err := db.Create(&priceInfos).Error; err != nil {
		t.Fatalf("create daily price infos: %v", err)
	}

	return handlerListingFixtures{
		stockListingID:  listings[0].ListingID,
		futureListingID: listings[1].ListingID,
		forexListingID:  listings[2].ListingID,
		optionListingID: listings[3].ListingID,
	}
}

func newListingHandlerForDB(db *gorm.DB) *ListingHandler {
	return NewListingHandler(service.NewListingService(
		repository.NewListingRepository(db),
		repository.NewFuturesContractRepository(db),
		repository.NewForexRepository(db),
		repository.NewOptionRepository(db),
	))
}

func newWatchlistHandlerForDB(db *gorm.DB) *WatchlistHandler {
	return NewWatchlistHandler(service.NewWatchlistService(
		repository.NewWatchlistRepository(db),
		repository.NewListingRepository(db),
	))
}

func performTradingJSON(router http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, _ := json.Marshal(body)
		reader = bytes.NewReader(payload)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestListingHandlerListsAndDetails(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	db := setupTradingHandlerDB(t)
	fixtures := seedTradingHandlerListings(t, db)
	h := newListingHandlerForDB(db)
	router := gin.New()
	router.Use(appErrors.ErrorHandler())
	router.GET("/stocks", h.GetStocks)
	router.GET("/stocks/:listingId", h.GetStockDetails)
	router.GET("/futures", h.GetFutures)
	router.GET("/futures/:listingId", h.GetFutureDetails)
	router.GET("/forex", h.GetForex)
	router.GET("/forex/:listingId", h.GetForexDetails)
	router.GET("/options", h.GetOptions)
	router.GET("/options/:listingId", h.GetOptionDetails)

	rec := performTradingJSON(router, http.MethodGet, "/stocks?search=AAPL&page=0&page_size=500", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("stocks status = %d body=%s", rec.Code, rec.Body.String())
	}
	var stockPage struct {
		Data []struct {
			Ticker string `json:"ticker"`
			Volume uint   `json:"volume"`
		} `json:"data"`
		Page int `json:"page"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &stockPage); err != nil {
		t.Fatalf("decode stocks: %v", err)
	}
	if len(stockPage.Data) != 1 || stockPage.Data[0].Ticker != "AAPL" || stockPage.Data[0].Volume != 500 || stockPage.Page != 1 {
		t.Fatalf("unexpected stocks response %#v", stockPage)
	}

	detailPaths := []string{
		"/stocks/" + uintPath(fixtures.stockListingID) + "?days_back=7",
		"/futures/" + uintPath(fixtures.futureListingID),
		"/forex/" + uintPath(fixtures.forexListingID),
		"/options/" + uintPath(fixtures.optionListingID),
	}
	for _, path := range detailPaths {
		rec = performTradingJSON(router, http.MethodGet, path, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d body=%s", path, rec.Code, rec.Body.String())
		}
	}

	listPaths := []string{
		"/futures?settlement_date=" + time.Now().UTC().AddDate(0, 1, 0).Format("2006-01-02"),
		"/forex?search=EUR&sort_dir=desc",
		"/options?search=AAPL",
	}
	for _, path := range listPaths {
		rec = performTradingJSON(router, http.MethodGet, path, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d body=%s", path, rec.Code, rec.Body.String())
		}
	}

	rec = performTradingJSON(router, http.MethodGet, "/stocks/not-a-number", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad stock id status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = performTradingJSON(router, http.MethodGet, "/options/999999", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing option status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = performTradingJSON(router, http.MethodGet, "/futures?settlement_date=bad-date", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad settlement date status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWatchlistHandlerLifecycle(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	db := setupTradingHandlerDB(t)
	fixtures := seedTradingHandlerListings(t, db)
	h := newWatchlistHandlerForDB(db)
	clientID := uint(12)
	router := gin.New()
	router.Use(appErrors.ErrorHandler())
	router.Use(func(c *gin.Context) {
		auth.SetAuth(c, &auth.AuthContext{
			IdentityID:   1200,
			IdentityType: auth.IdentityClient,
			ClientID:     &clientID,
			Permissions:  []permission.Permission{permission.Trading},
		})
		c.Next()
	})
	router.GET("/watchlists", h.GetWatchlists)
	router.POST("/watchlists", h.CreateWatchlist)
	router.GET("/watchlists/:watchlistId", h.GetWatchlistDetail)
	router.DELETE("/watchlists/:watchlistId", h.DeleteWatchlist)
	router.POST("/watchlists/:watchlistId/items", h.AddListing)
	router.DELETE("/watchlists/:watchlistId/items/:listingId", h.RemoveListing)

	rec := performTradingJSON(router, http.MethodPost, "/watchlists", map[string]any{"name": "Tech"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create watchlist status = %d body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		WatchlistID uint `json:"watchlist_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode watchlist: %v", err)
	}
	if created.WatchlistID == 0 {
		t.Fatal("expected watchlist id")
	}

	watchlistPath := "/watchlists/" + uintPath(created.WatchlistID)
	rec = performTradingJSON(router, http.MethodPost, watchlistPath+"/items", map[string]any{"listing_id": fixtures.stockListingID})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("add item status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = performTradingJSON(router, http.MethodGet, "/watchlists", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", rec.Code, rec.Body.String())
	}
	var watchlists []struct {
		Name      string `json:"name"`
		ItemCount int    `json:"item_count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &watchlists); err != nil {
		t.Fatalf("decode watchlists: %v", err)
	}
	if len(watchlists) != 1 || watchlists[0].Name != "Tech" || watchlists[0].ItemCount != 1 {
		t.Fatalf("unexpected watchlists %#v", watchlists)
	}

	rec = performTradingJSON(router, http.MethodGet, watchlistPath+"?asset_type=stock", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail status = %d body=%s", rec.Code, rec.Body.String())
	}
	var detail struct {
		Listings []struct {
			Ticker    string `json:"ticker"`
			AssetType string `json:"asset_type"`
		} `json:"listings"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if len(detail.Listings) != 1 || detail.Listings[0].Ticker != "AAPL" || detail.Listings[0].AssetType != "stock" {
		t.Fatalf("unexpected detail %#v", detail)
	}

	rec = performTradingJSON(router, http.MethodGet, watchlistPath+"?asset_type=invalid", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad asset type status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = performTradingJSON(router, http.MethodDelete, watchlistPath+"/items/"+uintPath(fixtures.stockListingID), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("remove status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = performTradingJSON(router, http.MethodDelete, watchlistPath, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = performTradingJSON(router, http.MethodDelete, "/watchlists/bad", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad watchlist id status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestExchangeHandlerListAndToggle(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	db := setupTradingHandlerDB(t)
	exchanges := []model.Exchange{
		{Name: "Nasdaq", Acronym: "NASDAQ", MicCode: "XNAS", Polity: "US", Currency: "USD", OpenTime: "09:30", CloseTime: "16:00", TradingEnabled: true},
		{Name: "Simulation", Acronym: "SIM", MicCode: model.SimulatedExchangeMIC, Polity: "US", Currency: "USD", OpenTime: "00:00", CloseTime: "23:59", TradingEnabled: false},
	}
	if err := db.Create(&exchanges).Error; err != nil {
		t.Fatalf("create exchanges: %v", err)
	}

	h := NewExchangeHandler(service.NewExchangeService(repository.NewExchangeRepository(db)))
	router := gin.New()
	router.Use(appErrors.ErrorHandler())
	router.GET("/exchanges", h.GetAll)
	router.PATCH("/exchanges/:micCode/toggle", h.ToggleTradingEnabled)

	rec := performTradingJSON(router, http.MethodGet, "/exchanges?page=0&page_size=0", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list exchanges status = %d body=%s", rec.Code, rec.Body.String())
	}
	var list struct {
		Total    int64 `json:"total"`
		Page     int   `json:"page"`
		PageSize int   `json:"page_size"`
		Data     []struct {
			MicCode        string `json:"mic_code"`
			TradingEnabled bool   `json:"trading_enabled"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode exchanges: %v", err)
	}
	if list.Total != 2 || list.Page != 1 || list.PageSize != 10 {
		t.Fatalf("unexpected exchange list %#v", list)
	}

	rec = performTradingJSON(router, http.MethodPatch, "/exchanges/XNAS/toggle", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle exchange status = %d body=%s", rec.Code, rec.Body.String())
	}
	var toggled struct {
		MicCode        string `json:"mic_code"`
		TradingEnabled bool   `json:"trading_enabled"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &toggled); err != nil {
		t.Fatalf("decode toggled exchange: %v", err)
	}
	if toggled.MicCode != "XNAS" || toggled.TradingEnabled {
		t.Fatalf("unexpected toggled exchange %#v", toggled)
	}

	rec = performTradingJSON(router, http.MethodPatch, "/exchanges/MISS/toggle", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing exchange status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestTradingHandlersRejectInvalidRequests(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	tradingvalidator.RegisterValidators()
	orderHandler := NewOrderHandler(nil)
	priceAlertHandler := NewPriceAlertHandler(nil)
	recurringHandler := NewRecurringOrderHandler(nil)

	router := gin.New()
	router.Use(appErrors.ErrorHandler())
	router.GET("/orders", orderHandler.GetOrders)
	router.POST("/orders", orderHandler.CreateOrder)
	router.POST("/orders/invest", orderHandler.CreateFundOrder)
	router.PATCH("/orders/:id/approve", orderHandler.ApproveOrder)
	router.PATCH("/orders/:id/decline", orderHandler.DeclineOrder)
	router.PATCH("/orders/:id/cancel", orderHandler.CancelOrder)
	router.GET("/orders/my", orderHandler.GetMyOrders)
	router.POST("/price-alerts", priceAlertHandler.CreatePriceAlert)
	router.DELETE("/price-alerts/:priceAlertId", priceAlertHandler.DeletePriceAlert)
	router.POST("/recurring-orders", recurringHandler.CreateRecurringOrder)
	router.DELETE("/recurring-orders/:id", recurringHandler.DeleteRecurringOrder)
	router.PATCH("/recurring-orders/:id/pause", recurringHandler.PauseRecurringOrder)

	cases := []struct {
		name   string
		method string
		path   string
		body   any
		status int
	}{
		{name: "orders invalid bool filter", method: http.MethodGet, path: "/orders?is_done=not-bool", status: http.StatusBadRequest},
		{name: "create order missing fields", method: http.MethodPost, path: "/orders", body: gin.H{"listing_id": 1}, status: http.StatusBadRequest},
		{name: "create fund order invalid direction", method: http.MethodPost, path: "/orders/invest", body: gin.H{"fund_id": 1, "listing_id": 1, "order_type": "MARKET", "direction": "SIDEWAYS", "quantity": 1}, status: http.StatusBadRequest},
		{name: "approve bad id", method: http.MethodPatch, path: "/orders/bad/approve", status: http.StatusBadRequest},
		{name: "decline bad id", method: http.MethodPatch, path: "/orders/bad/decline", status: http.StatusBadRequest},
		{name: "cancel bad id", method: http.MethodPatch, path: "/orders/bad/cancel", status: http.StatusBadRequest},
		{name: "my orders unauthenticated", method: http.MethodGet, path: "/orders/my", status: http.StatusUnauthorized},
		{name: "create price alert invalid threshold", method: http.MethodPost, path: "/price-alerts", body: gin.H{"listing_id": 1, "condition": "ABOVE", "threshold": 0}, status: http.StatusBadRequest},
		{name: "delete price alert bad id", method: http.MethodDelete, path: "/price-alerts/bad", status: http.StatusBadRequest},
		{name: "create recurring order invalid cadence", method: http.MethodPost, path: "/recurring-orders", body: gin.H{"listing_id": 1, "account_number": "444000100000000001", "direction": "BUY", "mode": "BY_QUANTITY", "value": 1, "cadence": "YEARLY"}, status: http.StatusBadRequest},
		{name: "delete recurring order bad id", method: http.MethodDelete, path: "/recurring-orders/bad", status: http.StatusBadRequest},
		{name: "pause recurring order bad id", method: http.MethodPatch, path: "/recurring-orders/bad/pause", status: http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := performTradingJSON(router, tc.method, tc.path, tc.body)
			if rec.Code != tc.status {
				t.Fatalf("%s %s status = %d, want %d body=%s", tc.method, tc.path, rec.Code, tc.status, rec.Body.String())
			}
		})
	}
}

func TestTradingFundPortfolioOTCAndTaxHandlersRejectInvalidRequests(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	fundHandler := NewInvestmentFundHandler(nil)
	portfolioHandler := NewPortfolioHandler(nil)
	otcHandler := NewOTCHandler(nil)
	offerHandler := NewOtcOfferHandler(nil)
	taxHandler := NewTaxHandler(nil, nil)
	dividendHandler := NewDividendHandler(nil)

	router := gin.New()
	router.Use(appErrors.ErrorHandler())
	router.GET("/funds", fundHandler.GetAllFunds)
	router.POST("/funds", fundHandler.CreateFund)
	router.GET("/funds/:fundId", fundHandler.GetFundDetail)
	router.DELETE("/funds/:fundId", fundHandler.DeleteFund)
	router.POST("/funds/:fundId/invest", fundHandler.InvestInFund)
	router.POST("/funds/:fundId/withdraw", fundHandler.WithdrawFromFund)
	router.GET("/actuary/:actId/assets/funds", fundHandler.GetActuaryFunds)
	router.GET("/client/:clientId/funds", fundHandler.GetClientFundPositions)

	router.GET("/client/:clientId/assets", portfolioHandler.GetClientPortfolio)
	router.GET("/actuary/:actId/assets", portfolioHandler.GetActuaryPortfolio)
	router.GET("/client/:clientId/assets/profit", portfolioHandler.GetClientPortfolioProfit)
	router.GET("/actuary/:actId/assets/profit", portfolioHandler.GetActuaryPortfolioProfit)
	router.GET("/profit/actuaries", portfolioHandler.GetAllActuaryProfits)
	router.POST("/actuary/:actId/options/:assetId/exercise", portfolioHandler.ExerciseOption)

	router.PATCH("/client/:clientId/assets/:ownershipId/publish", otcHandler.PublishAssetClient)
	router.PATCH("/actuary/:actId/assets/:ownershipId/publish", otcHandler.PublishAssetActuary)
	router.GET("/otc/public", otcHandler.GetPublicOTCAssets)

	router.POST("/otc/offers", offerHandler.CreateOffer)
	router.PUT("/otc/offers/:id/counter", offerHandler.SendCounterOffer)
	router.PATCH("/otc/offers/:id/accept", offerHandler.AcceptOffer)
	router.PATCH("/otc/offers/:id/reject", offerHandler.RejectOffer)
	router.GET("/otc/offers/active", offerHandler.GetMyActiveOffers)
	router.GET("/otc/contracts", offerHandler.GetMyOptionContracts)
	router.POST("/otc/contracts/:id/exercise", offerHandler.ExerciseContract)
	router.GET("/otc/executions/:id", offerHandler.GetExecution)

	router.GET("/tax", taxHandler.ListTaxUsers)
	router.GET("/client/:clientId/accumulated-tax", taxHandler.GetClientAccumulatedTax)
	router.GET("/actuary/:actId/accumulated-tax", taxHandler.GetActuaryAccumulatedTax)
	router.GET("/portfolio/assets/:assetOwnershipId/dividends", dividendHandler.GetDividendPayoutsForAssetOwnership)
	router.GET("/client/:clientId/assets/:assetOwnershipId/dividends", dividendHandler.GetClientDividendPayoutsForAssetOwnership)
	router.GET("/actuary/:actId/assets/:assetOwnershipId/dividends", dividendHandler.GetActuaryDividendPayoutsForAssetOwnership)

	cases := []struct {
		name   string
		method string
		path   string
		body   any
		status int
	}{
		{name: "funds bad page", method: http.MethodGet, path: "/funds?page=bad", status: http.StatusBadRequest},
		{name: "create fund missing fields", method: http.MethodPost, path: "/funds", body: gin.H{"name": "Growth"}, status: http.StatusBadRequest},
		{name: "actuary funds bad id", method: http.MethodGet, path: "/actuary/bad/assets/funds", status: http.StatusBadRequest},
		{name: "fund detail bad id", method: http.MethodGet, path: "/funds/bad", status: http.StatusBadRequest},
		{name: "delete fund zero id", method: http.MethodDelete, path: "/funds/0", status: http.StatusBadRequest},
		{name: "invest fund bad id", method: http.MethodPost, path: "/funds/bad/invest", body: gin.H{"account_number": "444000100000000001", "amount": 100}, status: http.StatusBadRequest},
		{name: "invest fund invalid body", method: http.MethodPost, path: "/funds/1/invest", body: gin.H{"account_number": "444000100000000001", "amount": 0}, status: http.StatusBadRequest},
		{name: "withdraw fund zero id", method: http.MethodPost, path: "/funds/0/withdraw", body: gin.H{"account_number": "444000100000000001", "amount": 100}, status: http.StatusBadRequest},
		{name: "withdraw fund invalid body", method: http.MethodPost, path: "/funds/1/withdraw", body: gin.H{"amount": 100}, status: http.StatusBadRequest},
		{name: "client funds bad id", method: http.MethodGet, path: "/client/bad/funds", status: http.StatusBadRequest},
		{name: "client portfolio bad id", method: http.MethodGet, path: "/client/bad/assets", status: http.StatusBadRequest},
		{name: "actuary portfolio bad id", method: http.MethodGet, path: "/actuary/bad/assets", status: http.StatusBadRequest},
		{name: "client profit bad id", method: http.MethodGet, path: "/client/bad/assets/profit", status: http.StatusBadRequest},
		{name: "actuary profit bad id", method: http.MethodGet, path: "/actuary/bad/assets/profit", status: http.StatusBadRequest},
		{name: "all actuary profits bad page", method: http.MethodGet, path: "/profit/actuaries?page=bad", status: http.StatusBadRequest},
		{name: "all actuary profits bad page size", method: http.MethodGet, path: "/profit/actuaries?page_size=bad", status: http.StatusBadRequest},
		{name: "exercise option bad actuary id", method: http.MethodPost, path: "/actuary/bad/options/1/exercise", body: gin.H{"account_number": "444000100000000001"}, status: http.StatusBadRequest},
		{name: "exercise option bad asset id", method: http.MethodPost, path: "/actuary/1/options/bad/exercise", body: gin.H{"account_number": "444000100000000001"}, status: http.StatusBadRequest},
		{name: "exercise option invalid body", method: http.MethodPost, path: "/actuary/1/options/2/exercise", body: gin.H{}, status: http.StatusBadRequest},
		{name: "publish client bad ownership id", method: http.MethodPatch, path: "/client/1/assets/bad/publish", body: gin.H{"amount": 1}, status: http.StatusBadRequest},
		{name: "publish client bad client id", method: http.MethodPatch, path: "/client/bad/assets/1/publish", body: gin.H{"amount": 1}, status: http.StatusBadRequest},
		{name: "publish client invalid body", method: http.MethodPatch, path: "/client/1/assets/1/publish", body: gin.H{}, status: http.StatusBadRequest},
		{name: "publish actuary bad ownership id", method: http.MethodPatch, path: "/actuary/1/assets/bad/publish", body: gin.H{"amount": 1}, status: http.StatusBadRequest},
		{name: "publish actuary bad actuary id", method: http.MethodPatch, path: "/actuary/bad/assets/1/publish", body: gin.H{"amount": 1}, status: http.StatusBadRequest},
		{name: "public otc bad query", method: http.MethodGet, path: "/otc/public?page=bad", status: http.StatusBadRequest},
		{name: "create offer invalid body", method: http.MethodPost, path: "/otc/offers", body: gin.H{"amount": 1}, status: http.StatusBadRequest},
		{name: "counter bad offer id", method: http.MethodPut, path: "/otc/offers/bad/counter", body: gin.H{}, status: http.StatusBadRequest},
		{name: "counter invalid body", method: http.MethodPut, path: "/otc/offers/1/counter", body: gin.H{"amount": 1}, status: http.StatusBadRequest},
		{name: "accept bad offer id", method: http.MethodPatch, path: "/otc/offers/bad/accept", status: http.StatusBadRequest},
		{name: "reject bad offer id", method: http.MethodPatch, path: "/otc/offers/bad/reject", status: http.StatusBadRequest},
		{name: "active offers unauthenticated", method: http.MethodGet, path: "/otc/offers/active", status: http.StatusUnauthorized},
		{name: "contracts unauthenticated", method: http.MethodGet, path: "/otc/contracts", status: http.StatusUnauthorized},
		{name: "exercise contract bad id", method: http.MethodPost, path: "/otc/contracts/bad/exercise", status: http.StatusBadRequest},
		{name: "execution bad id", method: http.MethodGet, path: "/otc/executions/bad", status: http.StatusBadRequest},
		{name: "tax list bad page", method: http.MethodGet, path: "/tax?page=bad", status: http.StatusBadRequest},
		{name: "client tax bad id", method: http.MethodGet, path: "/client/bad/accumulated-tax", status: http.StatusBadRequest},
		{name: "actuary tax bad id", method: http.MethodGet, path: "/actuary/bad/accumulated-tax", status: http.StatusBadRequest},
		{name: "dividend bad ownership id", method: http.MethodGet, path: "/portfolio/assets/bad/dividends", status: http.StatusBadRequest},
		{name: "client dividend bad ownership id", method: http.MethodGet, path: "/client/1/assets/bad/dividends", status: http.StatusBadRequest},
		{name: "actuary dividend bad ownership id", method: http.MethodGet, path: "/actuary/1/assets/bad/dividends", status: http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := performTradingJSON(router, tc.method, tc.path, tc.body)
			if rec.Code != tc.status {
				t.Fatalf("%s %s status = %d, want %d body=%s", tc.method, tc.path, rec.Code, tc.status, rec.Body.String())
			}
		})
	}
}

func TestMapPayoutsIncludesAssetTicker(t *testing.T) {
	payouts := mapPayouts([]model.DividendPayout{
		{
			DividendPayoutID: 9,
			AssetOwnershipID: 7,
			AssetOwnership: model.AssetOwnership{
				Asset: model.Asset{Ticker: "AAPL"},
			},
			Quantity:      4,
			GrossAmount:   12.5,
			TaxAmount:     1.875,
			NetAmount:     10.625,
			CurrencyCode:  "RSD",
			AccountNumber: "444000100000000001",
			PaymentDate:   time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC),
		},
	})

	if len(payouts) != 1 {
		t.Fatalf("len(payouts) = %d", len(payouts))
	}
	if payouts[0].DividendPayoutID != 9 || payouts[0].Stock != "AAPL" || payouts[0].NetAmount != 10.625 {
		t.Fatalf("unexpected payout response %#v", payouts[0])
	}
}

func uintPath(v uint) string {
	return strings.TrimSpace(strings.ReplaceAll(jsonNumber(v), "\"", ""))
}

func jsonNumber(v uint) string {
	b, _ := json.Marshal(v)
	return string(b)
}
