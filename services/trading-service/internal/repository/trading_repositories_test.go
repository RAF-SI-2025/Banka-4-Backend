package repository

import (
	"context"
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"gorm.io/gorm"
)

func setupTradingRepoDB(t *testing.T) *gorm.DB {
	t.Helper()

	db := setupOtcRepoDB(t)
	if err := db.AutoMigrate(
		&model.ListingDailyPriceInfo{},
		&model.FuturesContract{},
		&model.Option{},
		&model.Watchlist{},
		&model.WatchlistItem{},
		&model.RecurringOrder{},
		&model.AccumulatedTax{},
		&model.TaxCollection{},
	); err != nil {
		t.Fatalf("auto migrate trading repo models: %v", err)
	}
	return db
}

func createRepoExchange(t *testing.T, db *gorm.DB, mic string) model.Exchange {
	t.Helper()

	exchange := model.Exchange{
		Name:           mic + " Exchange",
		Acronym:        mic,
		MicCode:        mic,
		Polity:         "US",
		Currency:       "USD",
		OpenTime:       "09:30",
		CloseTime:      "16:00",
		TradingEnabled: true,
	}
	if err := db.Create(&exchange).Error; err != nil {
		t.Fatalf("create exchange: %v", err)
	}
	return exchange
}

func createRepoListing(t *testing.T, db *gorm.DB, ticker string, assetType model.AssetType, mic string, price float64) model.Listing {
	t.Helper()

	asset := model.Asset{Ticker: ticker, Name: ticker + " Asset", AssetType: assetType}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	listing := model.Listing{
		AssetID:           asset.AssetID,
		ExchangeMIC:       mic,
		LastRefresh:       time.Now().UTC(),
		Price:             price,
		Ask:               price + 1,
		MaintenanceMargin: 0.25,
	}
	if err := db.Create(&listing).Error; err != nil {
		t.Fatalf("create listing: %v", err)
	}
	listing.Asset = &asset
	return listing
}

func createRepoDailyInfo(t *testing.T, db *gorm.DB, listingID uint, date time.Time, price float64, volume uint) model.ListingDailyPriceInfo {
	t.Helper()

	info := model.ListingDailyPriceInfo{
		ListingID: listingID,
		Date:      date,
		Price:     price,
		Ask:       price + 1,
		Bid:       price - 1,
		Change:    1.5,
		Volume:    volume,
	}
	if err := db.Create(&info).Error; err != nil {
		t.Fatalf("create daily price info: %v", err)
	}
	return info
}

func TestListingRepositoryQueriesAndFilters(t *testing.T) {
	t.Parallel()

	db := setupTradingRepoDB(t)
	createRepoExchange(t, db, "XNYS")
	ctx := context.Background()
	repo := NewListingRepository(db)
	now := time.Now().UTC()

	stockListing := createRepoListing(t, db, "AAPL", model.AssetTypeStock, "XNYS", 150)
	stock := model.Stock{AssetID: stockListing.AssetID, OutstandingShares: 1000}
	if err := db.Create(&stock).Error; err != nil {
		t.Fatalf("create stock: %v", err)
	}
	oldInfo := createRepoDailyInfo(t, db, stockListing.ListingID, now.AddDate(0, 0, -2), 148, 100)
	latestInfo := createRepoDailyInfo(t, db, stockListing.ListingID, now.AddDate(0, 0, -1), 151, 500)

	futureListing := createRepoListing(t, db, "ESM6", model.AssetTypeFuture, "XNYS", 5200)
	settlement := now.AddDate(0, 1, 0).Truncate(24 * time.Hour)
	if err := db.Create(&model.FuturesContract{
		AssetID:        futureListing.AssetID,
		ContractSize:   50,
		ContractUnit:   "index",
		SettlementDate: settlement,
	}).Error; err != nil {
		t.Fatalf("create future: %v", err)
	}
	createRepoDailyInfo(t, db, futureListing.ListingID, now.AddDate(0, 0, -1), 5201, 250)

	optionListing := createRepoListing(t, db, "AAPL260C", model.AssetTypeOption, "XNYS", 12)
	if err := db.Create(&model.Option{
		AssetID:        optionListing.AssetID,
		StockID:        stock.StockID,
		OptionType:     model.OptionTypeCall,
		StrikePrice:    160,
		ContractSize:   100,
		SettlementDate: settlement,
	}).Error; err != nil {
		t.Fatalf("create option: %v", err)
	}
	createRepoDailyInfo(t, db, optionListing.ListingID, now.AddDate(0, 0, -1), 13, 80)

	listings, err := repo.FindAll(ctx)
	if err != nil {
		t.Fatalf("find all: %v", err)
	}
	if len(listings) != 3 {
		t.Fatalf("len(listings) = %d, want 3", len(listings))
	}

	found, err := repo.FindByID(ctx, stockListing.ListingID, 7)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if found == nil || found.Asset.Ticker != "AAPL" || len(found.DailyPriceInfos) != 2 {
		t.Fatalf("unexpected listing %#v", found)
	}

	latest, err := repo.FindLatestDailyPriceInfo(ctx, stockListing.ListingID)
	if err != nil {
		t.Fatalf("find latest daily info: %v", err)
	}
	if latest == nil || latest.ID != latestInfo.ID {
		t.Fatalf("latest daily info = %#v, want %#v", latest, latestInfo)
	}

	lastBefore, err := repo.FindLastDailyPriceInfo(ctx, stockListing.ListingID, latestInfo.Date)
	if err != nil {
		t.Fatalf("find last daily info: %v", err)
	}
	if lastBefore == nil || lastBefore.ID != oldInfo.ID {
		t.Fatalf("last before = %#v, want %#v", lastBefore, oldInfo)
	}

	if err := repo.UpdatePriceAndAsk(ctx, &stockListing, 155, 156); err != nil {
		t.Fatalf("update price and ask: %v", err)
	}
	updated, err := repo.FindByID(ctx, stockListing.ListingID, 0)
	if err != nil {
		t.Fatalf("find updated listing: %v", err)
	}
	if updated.Price != 155 || updated.Ask != 156 {
		t.Fatalf("updated price/ask = %.2f/%.2f", updated.Price, updated.Ask)
	}

	count, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Fatalf("count = %d, want 3", count)
	}

	stockFilter := ListingFilter{Search: "AAP", Exchange: "XN", PriceMin: 100, PriceMax: 200, BidMin: 100, BidMax: 200, VolumeMin: 400, VolumeMax: 600, SortBy: "volume", SortDir: "desc", Page: 1, PageSize: 10}
	stocks, stockCount, err := repo.FindStocks(ctx, stockFilter)
	if err != nil {
		t.Fatalf("find stocks: %v", err)
	}
	if stockCount != 1 || len(stocks) != 1 || stocks[0].Asset.Ticker != "AAPL" {
		t.Fatalf("unexpected stocks count=%d listings=%#v", stockCount, stocks)
	}

	futures, futuresCount, err := repo.FindFutures(ctx, ListingFilter{SettlementDate: &settlement, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("find futures: %v", err)
	}
	if futuresCount != 1 || len(futures) != 1 || futures[0].Asset.Ticker != "ESM6" {
		t.Fatalf("unexpected futures count=%d listings=%#v", futuresCount, futures)
	}

	options, optionsCount, err := repo.FindOptions(ctx, ListingFilter{SettlementDate: &settlement, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("find options: %v", err)
	}
	if optionsCount != 1 || len(options) != 1 || options[0].Asset.Ticker != "AAPL260C" {
		t.Fatalf("unexpected options count=%d listings=%#v", optionsCount, options)
	}

	byAssets, err := repo.FindByAssetIDs(ctx, []uint{stockListing.AssetID, futureListing.AssetID})
	if err != nil {
		t.Fatalf("find by asset ids: %v", err)
	}
	if len(byAssets) != 2 {
		t.Fatalf("len(byAssets) = %d, want 2", len(byAssets))
	}

	byType, err := repo.FindByAssetType(ctx, model.AssetTypeOption)
	if err != nil {
		t.Fatalf("find by asset type: %v", err)
	}
	if len(byType) != 1 || byType[0].Asset.Ticker != "AAPL260C" {
		t.Fatalf("unexpected by type %#v", byType)
	}

	newListing := createRepoListing(t, db, "IBM", model.AssetTypeStock, "XNYS", 180)
	newListing.Price = 181
	if err := repo.Upsert(ctx, &newListing); err != nil {
		t.Fatalf("upsert existing listing: %v", err)
	}
}

func TestWatchlistRepositoryLifecycle(t *testing.T) {
	t.Parallel()

	db := setupTradingRepoDB(t)
	createRepoExchange(t, db, "XNAS")
	ctx := context.Background()
	listing := createRepoListing(t, db, "MSFT", model.AssetTypeStock, "XNAS", 300)
	createRepoDailyInfo(t, db, listing.ListingID, time.Now().UTC(), 301, 400)
	repo := NewWatchlistRepository(db)

	watchlist := &model.Watchlist{UserID: 42, OwnerType: model.OwnerTypeClient, Name: "tech"}
	if err := repo.Create(ctx, watchlist); err != nil {
		t.Fatalf("create watchlist: %v", err)
	}

	if err := repo.AddItem(ctx, &model.WatchlistItem{WatchlistID: watchlist.WatchlistID, ListingID: listing.ListingID, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("add watchlist item: %v", err)
	}

	found, err := repo.FindByID(ctx, watchlist.WatchlistID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if found == nil || found.Name != "tech" {
		t.Fatalf("unexpected watchlist %#v", found)
	}

	byOwnerName, err := repo.FindByOwnerAndName(ctx, 42, model.OwnerTypeClient, "tech")
	if err != nil {
		t.Fatalf("find by owner and name: %v", err)
	}
	if byOwnerName == nil || byOwnerName.WatchlistID != watchlist.WatchlistID {
		t.Fatalf("unexpected owner/name watchlist %#v", byOwnerName)
	}

	owned, err := repo.FindByOwner(ctx, 42, model.OwnerTypeClient)
	if err != nil {
		t.Fatalf("find by owner: %v", err)
	}
	if len(owned) != 1 || len(owned[0].Items) != 1 {
		t.Fatalf("unexpected owned watchlists %#v", owned)
	}

	item, err := repo.FindItem(ctx, watchlist.WatchlistID, listing.ListingID)
	if err != nil {
		t.Fatalf("find item: %v", err)
	}
	if item == nil || item.ListingID != listing.ListingID {
		t.Fatalf("unexpected item %#v", item)
	}

	assetType := model.AssetTypeStock
	detail, err := repo.FindDetail(ctx, watchlist.WatchlistID, &assetType)
	if err != nil {
		t.Fatalf("find detail: %v", err)
	}
	if detail == nil || len(detail.Items) != 1 || len(detail.Items[0].Listing.DailyPriceInfos) != 1 {
		t.Fatalf("unexpected detail %#v", detail)
	}

	removed, err := repo.RemoveItem(ctx, watchlist.WatchlistID, listing.ListingID)
	if err != nil {
		t.Fatalf("remove item: %v", err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}

	if err := repo.Delete(ctx, watchlist.WatchlistID); err != nil {
		t.Fatalf("delete watchlist: %v", err)
	}
	missing, err := repo.FindByID(ctx, watchlist.WatchlistID)
	if err != nil {
		t.Fatalf("find deleted watchlist: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected deleted watchlist nil, got %#v", missing)
	}
}

func TestRecurringOrderRepositoryLifecycle(t *testing.T) {
	t.Parallel()

	db := setupTradingRepoDB(t)
	createRepoExchange(t, db, "XREC")
	ctx := context.Background()
	listing := createRepoListing(t, db, "VTI", model.AssetTypeStock, "XREC", 250)
	repo := NewRecurringOrderRepository(db)
	now := time.Now().UTC()

	order := &model.RecurringOrder{
		UserID:        7,
		OwnerType:     model.OwnerTypeClient,
		ListingID:     listing.ListingID,
		Direction:     model.OrderDirectionBuy,
		Mode:          model.RecurringOrderModeByAmount,
		Value:         1000,
		AccountNumber: "444000100000000001",
		Cadence:       model.RecurringOrderCadenceWeekly,
		NextRun:       now.Add(-time.Minute),
		Active:        true,
	}
	if err := repo.Create(ctx, order); err != nil {
		t.Fatalf("create recurring order: %v", err)
	}

	found, err := repo.FindByID(ctx, order.RecurringOrderID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if found == nil || found.Listing.Asset.Ticker != "VTI" {
		t.Fatalf("unexpected recurring order %#v", found)
	}

	order.Value = 1200
	order.NextRun = now.Add(time.Hour)
	if err := repo.Save(ctx, order); err != nil {
		t.Fatalf("save recurring order: %v", err)
	}

	byUser, err := repo.FindByUser(ctx, 7, model.OwnerTypeClient)
	if err != nil {
		t.Fatalf("find by user: %v", err)
	}
	if len(byUser) != 1 || byUser[0].Value != 1200 {
		t.Fatalf("unexpected user recurring orders %#v", byUser)
	}

	due, err := repo.FindDue(ctx, now)
	if err != nil {
		t.Fatalf("find due: %v", err)
	}
	if len(due) != 0 {
		t.Fatalf("len(due) = %d, want 0", len(due))
	}

	order.NextRun = now.Add(-time.Minute)
	if err := repo.Save(ctx, order); err != nil {
		t.Fatalf("save due recurring order: %v", err)
	}
	due, err = repo.FindDue(ctx, now)
	if err != nil {
		t.Fatalf("find due after update: %v", err)
	}
	if len(due) != 1 || due[0].RecurringOrderID != order.RecurringOrderID {
		t.Fatalf("unexpected due orders %#v", due)
	}

	if err := repo.Delete(ctx, order.RecurringOrderID); err != nil {
		t.Fatalf("delete recurring order: %v", err)
	}
	missing, err := repo.FindByID(ctx, order.RecurringOrderID)
	if err != nil {
		t.Fatalf("find deleted recurring order: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected deleted recurring order nil, got %#v", missing)
	}
}

func TestOrderRepositoryQueries(t *testing.T) {
	t.Parallel()

	db := setupTradingRepoDB(t)
	createRepoExchange(t, db, "XORD")
	ctx := context.Background()
	listing := createRepoListing(t, db, "NFLX", model.AssetTypeStock, "XORD", 600)
	repo := NewOrderRepository(db)
	now := time.Now().UTC()
	nextRun := now.Add(-time.Minute)
	limit := 605.0

	order := &model.Order{
		OrderOwnerUserID: 9,
		OrderOwnerType:   model.OwnerTypeClient,
		AccountNumber:    "444000100000000002",
		ListingID:        listing.ListingID,
		OrderType:        model.OrderTypeLimit,
		Direction:        model.OrderDirectionBuy,
		Quantity:         10,
		LimitValue:       &limit,
		Status:           model.OrderStatusApproved,
		NextExecutionAt:  &nextRun,
		CreatedAt:        now,
		OwnerType:        model.OwnerTypeClient,
	}
	if err := repo.Create(ctx, order); err != nil {
		t.Fatalf("create order: %v", err)
	}

	found, err := repo.FindByID(ctx, order.OrderID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if found == nil || found.Listing.Asset.Ticker != "NFLX" {
		t.Fatalf("unexpected order %#v", found)
	}

	order.FilledQty = 2
	if err := repo.Save(ctx, order); err != nil {
		t.Fatalf("save order: %v", err)
	}

	status := model.OrderStatusApproved
	direction := model.OrderDirectionBuy
	done := false
	all, count, err := repo.FindAll(ctx, 1, 10, nil, nil, &status, &direction, &done)
	if err != nil {
		t.Fatalf("find all: %v", err)
	}
	if count != 1 || len(all) != 1 || all[0].FilledQty != 2 {
		t.Fatalf("unexpected all orders count=%d orders=%#v", count, all)
	}

	ready, err := repo.FindReadyForExecution(ctx, now, 5)
	if err != nil {
		t.Fatalf("find ready: %v", err)
	}
	if len(ready) != 1 || ready[0].OrderID != order.OrderID {
		t.Fatalf("unexpected ready orders %#v", ready)
	}

	assetType := model.AssetTypeStock
	orderType := model.OrderTypeLimit
	fromDate := now.Add(-24 * time.Hour)
	toDate := now
	userOrders, userCount, err := repo.FindUserOrders(ctx, 9, model.OwnerTypeClient, dto.UserOrdersQuery{
		Status:    &status,
		OrderType: &orderType,
		AssetType: &assetType,
		FromDate:  &fromDate,
		ToDate:    &toDate,
		Page:      1,
		PageSize:  10,
	})
	if err != nil {
		t.Fatalf("find user orders: %v", err)
	}
	if userCount != 1 || len(userOrders) != 1 || userOrders[0].OrderID != order.OrderID {
		t.Fatalf("unexpected user orders count=%d orders=%#v", userCount, userOrders)
	}

	missing, err := repo.FindByID(ctx, 9999)
	if err != nil {
		t.Fatalf("find missing order: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected missing order nil, got %#v", missing)
	}
}

func TestTaxRepositoryLifecycle(t *testing.T) {
	t.Parallel()

	db := setupTradingRepoDB(t)
	ctx := context.Background()
	repo := NewTaxRepository(db)
	employeeID := uint(77)
	now := time.Now().UTC()

	if err := repo.AddTaxOwed(ctx, "444000100000000003", nil, 250, "RSD"); err != nil {
		t.Fatalf("add client tax: %v", err)
	}
	if err := repo.AddTaxOwed(ctx, "444000100000000003", nil, 125, "RSD"); err != nil {
		t.Fatalf("add client tax again: %v", err)
	}
	if err := repo.AddTaxOwed(ctx, "444000100000000004", &employeeID, 400, "RSD"); err != nil {
		t.Fatalf("add employee tax: %v", err)
	}

	clientTax, err := repo.FindAccumulatedTaxByAccountNumber(ctx, "444000100000000003")
	if err != nil {
		t.Fatalf("find client tax: %v", err)
	}
	if clientTax == nil || clientTax.TaxOwed != 375 {
		t.Fatalf("unexpected client tax %#v", clientTax)
	}

	page, count, err := repo.FindAllAccumulatedTax(ctx, []string{"444000100000000003", "444000100000000004"}, 1, 10)
	if err != nil {
		t.Fatalf("find all accumulated tax: %v", err)
	}
	if count != 2 || len(page) != 2 {
		t.Fatalf("unexpected accumulated page count=%d taxes=%#v", count, page)
	}

	positive, err := repo.FindAllPositiveAccumulatedTax(ctx)
	if err != nil {
		t.Fatalf("find positive accumulated tax: %v", err)
	}
	if len(positive) != 2 {
		t.Fatalf("len(positive) = %d, want 2", len(positive))
	}

	employeeTaxes, err := repo.FindAccumulatedTaxByEmployeeID(ctx, employeeID)
	if err != nil {
		t.Fatalf("find employee tax: %v", err)
	}
	if len(employeeTaxes) != 1 || employeeTaxes[0].EmployeeID == nil {
		t.Fatalf("unexpected employee taxes %#v", employeeTaxes)
	}

	clientTaxes, err := repo.FindAccumulatedTaxByClientAccountNumbers(ctx, []string{"444000100000000003", "444000100000000004"})
	if err != nil {
		t.Fatalf("find client taxes: %v", err)
	}
	if len(clientTaxes) != 1 || clientTaxes[0].AccountNumber != "444000100000000003" {
		t.Fatalf("unexpected client taxes %#v", clientTaxes)
	}

	if err := repo.ClearTax(ctx, "444000100000000003", now); err != nil {
		t.Fatalf("clear tax: %v", err)
	}
	clientTax, err = repo.FindAccumulatedTaxByAccountNumber(ctx, "444000100000000003")
	if err != nil {
		t.Fatalf("find cleared client tax: %v", err)
	}
	if clientTax.TaxOwed != 0 || clientTax.LastClearedAt == nil {
		t.Fatalf("unexpected cleared tax %#v", clientTax)
	}

	clientTax.TaxOwed = 50
	if err := repo.SaveAccumulatedTax(ctx, clientTax); err != nil {
		t.Fatalf("save accumulated tax: %v", err)
	}

	collection := &model.TaxCollection{
		AccountNumber:     "444000100000000003",
		TaxOwed:           50,
		CurrencyCode:      "RSD",
		Status:            model.TaxStatusCollected,
		TaxingPeriodStart: now.AddDate(0, -1, 0),
		TaxingPeriodEnd:   &now,
	}
	if err := repo.CreateTaxCollection(ctx, collection); err != nil {
		t.Fatalf("create tax collection: %v", err)
	}

	failedReason := "insufficient funds"
	failed := &model.TaxCollection{
		AccountNumber:     "444000100000000004",
		TaxOwed:           400,
		CurrencyCode:      "RSD",
		EmployeeID:        &employeeID,
		Status:            model.TaxStatusFailed,
		FailureReason:     &failedReason,
		TaxingPeriodStart: now.AddDate(0, -1, 0),
		TaxingPeriodEnd:   &now,
	}
	if err := repo.RecordCollectionResult(ctx, failed, false, 0, now); err != nil {
		t.Fatalf("record collection result: %v", err)
	}

	collections, err := repo.FindTaxCollectionsByAccountNumber(ctx, "444000100000000003")
	if err != nil {
		t.Fatalf("find tax collections: %v", err)
	}
	if len(collections) != 1 || collections[0].TaxCollectionID != collection.TaxCollectionID {
		t.Fatalf("unexpected collections %#v", collections)
	}

	latest, err := repo.FindLatestTaxCollection(ctx, "444000100000000003")
	if err != nil {
		t.Fatalf("find latest tax collection: %v", err)
	}
	if latest == nil || latest.TaxCollectionID != collection.TaxCollectionID {
		t.Fatalf("unexpected latest collection %#v", latest)
	}

	missing, err := repo.FindLatestTaxCollection(ctx, "missing")
	if err != nil {
		t.Fatalf("find missing latest tax collection: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected missing latest collection nil, got %#v", missing)
	}
}
