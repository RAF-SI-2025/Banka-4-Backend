//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaxRepo_AddTaxOwed_CreatesNewRecord(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewTaxRepository(db)
	ctx := context.Background()

	err := repo.AddTaxOwed(ctx, "444000100000000001", nil, 100.0, "RSD")
	require.NoError(t, err)

	tax, err := repo.FindAccumulatedTaxByAccountNumber(ctx, "444000100000000001")
	require.NoError(t, err)
	require.NotNil(t, tax)
	assert.Equal(t, 100.0, tax.TaxOwed)
	assert.Equal(t, "RSD", tax.CurrencyCode)
}

func TestTaxRepo_AddTaxOwed_AccumulatesOnExistingRecord(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewTaxRepository(db)
	ctx := context.Background()

	err := repo.AddTaxOwed(ctx, "444000100000000002", nil, 50.0, "RSD")
	require.NoError(t, err)

	err = repo.AddTaxOwed(ctx, "444000100000000002", nil, 30.0, "RSD")
	require.NoError(t, err)

	tax, err := repo.FindAccumulatedTaxByAccountNumber(ctx, "444000100000000002")
	require.NoError(t, err)
	require.NotNil(t, tax)
	assert.Equal(t, 80.0, tax.TaxOwed)
}

func TestTaxRepo_AddTaxOwed_WithEmployeeID(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewTaxRepository(db)
	ctx := context.Background()

	eid := uint(42)
	err := repo.AddTaxOwed(ctx, "444000100000000003", &eid, 200.0, "RSD")
	require.NoError(t, err)

	taxes, _, err := repo.FindAllAccumulatedTax(ctx, []string{"444000100000000003"}, 1, 10)
	require.NoError(t, err)
	require.Len(t, taxes, 1)
	require.NotNil(t, taxes[0].EmployeeID)
	assert.Equal(t, eid, *taxes[0].EmployeeID)
	assert.Equal(t, 200.0, taxes[0].TaxOwed)
}

func TestTaxRepo_FindAccumulatedTaxByAccountNumber_NotFound(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewTaxRepository(db)
	ctx := context.Background()

	tax, err := repo.FindAccumulatedTaxByAccountNumber(ctx, "999999999999999999")
	require.NoError(t, err)
	assert.Nil(t, tax)
}

func TestTaxRepo_FindAllAccumulatedTax_FiltersByAccountNumbers(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewTaxRepository(db)
	ctx := context.Background()

	_ = repo.AddTaxOwed(ctx, "444000100000000010", nil, 10.0, "RSD")
	_ = repo.AddTaxOwed(ctx, "444000100000000011", nil, 20.0, "RSD")
	_ = repo.AddTaxOwed(ctx, "444000100000000012", nil, 30.0, "RSD")

	taxes, count, err := repo.FindAllAccumulatedTax(ctx, []string{"444000100000000010", "444000100000000011"}, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
	assert.Len(t, taxes, 2)
}

func TestTaxRepo_FindAllAccumulatedTax_NoFilter(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewTaxRepository(db)
	ctx := context.Background()

	_ = repo.AddTaxOwed(ctx, "444000200000000001", nil, 5.0, "RSD")
	_ = repo.AddTaxOwed(ctx, "444000200000000002", nil, 15.0, "RSD")

	taxes, count, err := repo.FindAllAccumulatedTax(ctx, nil, 1, 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(2))
	assert.GreaterOrEqual(t, len(taxes), 2)
}

func TestTaxRepo_FindAllAccumulatedTax_Pagination(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewTaxRepository(db)
	ctx := context.Background()

	accounts := []string{
		"444000300000000001",
		"444000300000000002",
		"444000300000000003",
	}
	for _, acc := range accounts {
		_ = repo.AddTaxOwed(ctx, acc, nil, 1.0, "RSD")
	}

	page1, count, err := repo.FindAllAccumulatedTax(ctx, accounts, 1, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
	assert.Len(t, page1, 2)

	page2, _, err := repo.FindAllAccumulatedTax(ctx, accounts, 2, 2)
	require.NoError(t, err)
	assert.Len(t, page2, 1)
}

func TestTaxRepo_ClearTax(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewTaxRepository(db)
	ctx := context.Background()

	err := repo.AddTaxOwed(ctx, "444000100000000020", nil, 500.0, "RSD")
	require.NoError(t, err)

	clearedAt := time.Now()
	err = repo.ClearTax(ctx, "444000100000000020", clearedAt)
	require.NoError(t, err)

	tax, err := repo.FindAccumulatedTaxByAccountNumber(ctx, "444000100000000020")
	require.NoError(t, err)
	require.NotNil(t, tax)
	assert.Equal(t, 0.0, tax.TaxOwed)
	require.NotNil(t, tax.LastClearedAt)
}

func TestTaxRepo_SaveAccumulatedTax(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewTaxRepository(db)
	ctx := context.Background()

	err := repo.AddTaxOwed(ctx, "444000100000000030", nil, 100.0, "RSD")
	require.NoError(t, err)

	tax, err := repo.FindAccumulatedTaxByAccountNumber(ctx, "444000100000000030")
	require.NoError(t, err)
	require.NotNil(t, tax)

	tax.TaxOwed = 250.0
	err = repo.SaveAccumulatedTax(ctx, tax)
	require.NoError(t, err)

	updated, err := repo.FindAccumulatedTaxByAccountNumber(ctx, "444000100000000030")
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, 250.0, updated.TaxOwed)
}

func TestTaxRepo_CreateTaxCollection(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewTaxRepository(db)
	ctx := context.Background()

	periodStart := time.Now().AddDate(0, -1, 0)
	collection := &model.TaxCollection{
		AccountNumber:     "444000100000000040",
		TaxOwed:           75.0,
		CurrencyCode:      "RSD",
		Status:            model.TaxStatusCollected,
		TaxingPeriodStart: periodStart,
	}

	err := repo.CreateTaxCollection(ctx, collection)
	require.NoError(t, err)
	assert.NotZero(t, collection.TaxCollectionID)

	collections, err := repo.FindTaxCollectionsByAccountNumber(ctx, "444000100000000040")
	require.NoError(t, err)
	require.Len(t, collections, 1)
	assert.Equal(t, 75.0, collections[0].TaxOwed)
	assert.Equal(t, model.TaxStatusCollected, collections[0].Status)
}

func TestTaxRepo_FindTaxCollectionsByAccountNumber_Empty(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewTaxRepository(db)
	ctx := context.Background()

	collections, err := repo.FindTaxCollectionsByAccountNumber(ctx, "000000000000000000")
	require.NoError(t, err)
	assert.Empty(t, collections)
}

func TestTaxRepo_FindTaxCollectionsByAccountNumber_OrderedDesc(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewTaxRepository(db)
	ctx := context.Background()

	acct := "444000100000000050"
	for i := 0; i < 3; i++ {
		c := &model.TaxCollection{
			AccountNumber:     acct,
			TaxOwed:           float64(10 * (i + 1)),
			CurrencyCode:      "RSD",
			Status:            model.TaxStatusCollected,
			TaxingPeriodStart: time.Now().AddDate(0, -(i + 1), 0),
		}
		err := repo.CreateTaxCollection(ctx, c)
		require.NoError(t, err)
	}

	collections, err := repo.FindTaxCollectionsByAccountNumber(ctx, acct)
	require.NoError(t, err)
	require.Len(t, collections, 3)
	assert.Greater(t, collections[0].TaxCollectionID, collections[1].TaxCollectionID)
	assert.Greater(t, collections[1].TaxCollectionID, collections[2].TaxCollectionID)
}

func TestTaxRepo_FindLatestTaxCollection(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewTaxRepository(db)
	ctx := context.Background()

	acct := "444000100000000060"
	for i := 0; i < 3; i++ {
		c := &model.TaxCollection{
			AccountNumber:     acct,
			TaxOwed:           float64(100 + i),
			CurrencyCode:      "RSD",
			Status:            model.TaxStatusCollected,
			TaxingPeriodStart: time.Now().AddDate(0, -(3 - i), 0),
		}
		err := repo.CreateTaxCollection(ctx, c)
		require.NoError(t, err)
	}

	latest, err := repo.FindLatestTaxCollection(ctx, acct)
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, 102.0, latest.TaxOwed)
}

func TestTaxRepo_FindLatestTaxCollection_NotFound(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewTaxRepository(db)
	ctx := context.Background()

	latest, err := repo.FindLatestTaxCollection(ctx, "000000000000000001")
	require.NoError(t, err)
	assert.Nil(t, latest)
}

func TestTaxRepo_RecordCollectionResult_WithClearTax(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewTaxRepository(db)
	ctx := context.Background()

	acct := "444000100000000070"
	err := repo.AddTaxOwed(ctx, acct, nil, 300.0, "RSD")
	require.NoError(t, err)

	clearedAt := time.Now()
	collection := &model.TaxCollection{
		AccountNumber:     acct,
		TaxOwed:           300.0,
		CurrencyCode:      "RSD",
		Status:            model.TaxStatusCollected,
		TaxingPeriodStart: time.Now().AddDate(0, -1, 0),
	}

	err = repo.RecordCollectionResult(ctx, collection, true, 300.0, clearedAt)
	require.NoError(t, err)
	assert.NotZero(t, collection.TaxCollectionID)

	tax, err := repo.FindAccumulatedTaxByAccountNumber(ctx, acct)
	require.NoError(t, err)
	require.NotNil(t, tax)
	assert.Equal(t, 0.0, tax.TaxOwed)
}

func TestTaxRepo_RecordCollectionResult_WithoutClearTax(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewTaxRepository(db)
	ctx := context.Background()

	acct := "444000100000000080"
	err := repo.AddTaxOwed(ctx, acct, nil, 400.0, "RSD")
	require.NoError(t, err)

	reason := "payment failed"
	collection := &model.TaxCollection{
		AccountNumber:     acct,
		TaxOwed:           400.0,
		CurrencyCode:      "RSD",
		Status:            model.TaxStatusFailed,
		FailureReason:     &reason,
		TaxingPeriodStart: time.Now().AddDate(0, -1, 0),
	}

	err = repo.RecordCollectionResult(ctx, collection, false, 0, time.Now())
	require.NoError(t, err)

	tax, err := repo.FindAccumulatedTaxByAccountNumber(ctx, acct)
	require.NoError(t, err)
	require.NotNil(t, tax)
	assert.Equal(t, 400.0, tax.TaxOwed)

	collections, err := repo.FindTaxCollectionsByAccountNumber(ctx, acct)
	require.NoError(t, err)
	require.Len(t, collections, 1)
	assert.Equal(t, model.TaxStatusFailed, collections[0].Status)
}

func TestListingRepo_Count(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewListingRepository(db)
	ctx := context.Background()

	exchange := seedExchange(t, db, "XCNT")

	before, err := repo.Count(ctx)
	require.NoError(t, err)

	seedListing(t, db, "AAAA", exchange.MicCode, model.AssetTypeStock, 100.0)
	seedListing(t, db, "BBBB", exchange.MicCode, model.AssetTypeStock, 200.0)

	after, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, before+2, after)
}

func TestListingRepo_CreateDailyPriceInfo(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewListingRepository(db)
	ctx := context.Background()

	exchange := seedExchange(t, db, "XCDP")
	listing := seedListing(t, db, "CDPI", exchange.MicCode, model.AssetTypeStock, 50.0)

	info := &model.ListingDailyPriceInfo{
		ListingID: listing.ListingID,
		Date:      time.Now().AddDate(0, 0, -1).Truncate(24 * time.Hour),
		Price:     51.0,
		Ask:       52.0,
		Bid:       50.0,
		Change:    1.0,
		Volume:    1000,
	}

	err := repo.CreateDailyPriceInfo(ctx, info)
	require.NoError(t, err)
	assert.NotZero(t, info.ID)
}

func TestListingRepo_FindLatestDailyPriceInfo(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewListingRepository(db)
	ctx := context.Background()

	exchange := seedExchange(t, db, "XFLD")
	listing := seedListing(t, db, "FLDP", exchange.MicCode, model.AssetTypeStock, 80.0)

	older := &model.ListingDailyPriceInfo{
		ListingID: listing.ListingID,
		Date:      time.Now().AddDate(0, 0, -3).Truncate(24 * time.Hour),
		Price:     78.0, Ask: 79.0, Bid: 77.0, Change: -1.0, Volume: 500,
	}
	newer := &model.ListingDailyPriceInfo{
		ListingID: listing.ListingID,
		Date:      time.Now().AddDate(0, 0, -1).Truncate(24 * time.Hour),
		Price:     82.0, Ask: 83.0, Bid: 81.0, Change: 2.0, Volume: 900,
	}
	require.NoError(t, repo.CreateDailyPriceInfo(ctx, older))
	require.NoError(t, repo.CreateDailyPriceInfo(ctx, newer))

	latest, err := repo.FindLatestDailyPriceInfo(ctx, listing.ListingID)
	require.NoError(t, err)
	require.NotNil(t, latest)
	assert.Equal(t, 82.0, latest.Price)
}

func TestListingRepo_FindLatestDailyPriceInfo_NotFound(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewListingRepository(db)
	ctx := context.Background()

	exchange := seedExchange(t, db, "XNOD")
	listing := seedListing(t, db, "NODP", exchange.MicCode, model.AssetTypeStock, 10.0)

	latest, err := repo.FindLatestDailyPriceInfo(ctx, listing.ListingID)
	require.NoError(t, err)
	assert.Nil(t, latest)
}

func TestListingRepo_FindLastDailyPriceInfo(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewListingRepository(db)
	ctx := context.Background()

	exchange := seedExchange(t, db, "XFLB")
	listing := seedListing(t, db, "FLBI", exchange.MicCode, model.AssetTypeStock, 120.0)

	d1 := time.Now().AddDate(0, 0, -5).Truncate(24 * time.Hour)
	d2 := time.Now().AddDate(0, 0, -2).Truncate(24 * time.Hour)

	info1 := &model.ListingDailyPriceInfo{
		ListingID: listing.ListingID, Date: d1,
		Price: 110.0, Ask: 111.0, Bid: 109.0, Change: -1.0, Volume: 200,
	}
	info2 := &model.ListingDailyPriceInfo{
		ListingID: listing.ListingID, Date: d2,
		Price: 118.0, Ask: 119.0, Bid: 117.0, Change: 1.5, Volume: 300,
	}
	require.NoError(t, repo.CreateDailyPriceInfo(ctx, info1))
	require.NoError(t, repo.CreateDailyPriceInfo(ctx, info2))

	beforeDate := time.Now().AddDate(0, 0, -1)
	found, err := repo.FindLastDailyPriceInfo(ctx, listing.ListingID, beforeDate)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, 118.0, found.Price)
}

func TestListingRepo_FindLastDailyPriceInfo_NotFound(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewListingRepository(db)
	ctx := context.Background()

	exchange := seedExchange(t, db, "XNLB")
	listing := seedListing(t, db, "NLBI", exchange.MicCode, model.AssetTypeStock, 10.0)

	found, err := repo.FindLastDailyPriceInfo(ctx, listing.ListingID, time.Now())
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestListingRepo_FindByType(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewListingRepository(db)
	ctx := context.Background()

	exchange := seedExchange(t, db, "XFBT")
	seedListing(t, db, "STKA", exchange.MicCode, model.AssetTypeStock, 100.0)
	seedListing(t, db, "STKB", exchange.MicCode, model.AssetTypeStock, 200.0)
	seedListing(t, db, "FUTA", exchange.MicCode, model.AssetTypeFuture, 300.0)

	stocks, err := repo.FindByAssetType(ctx, model.AssetTypeStock)
	require.NoError(t, err)
	for _, l := range stocks {
		require.NotNil(t, l.Asset)
		assert.Equal(t, model.AssetTypeStock, l.Asset.AssetType)
	}

	futures, err := repo.FindByAssetType(ctx, model.AssetTypeFuture)
	require.NoError(t, err)
	for _, l := range futures {
		require.NotNil(t, l.Asset)
		assert.Equal(t, model.AssetTypeFuture, l.Asset.AssetType)
	}
}

func TestOrderRepo_FindReadyForExecution(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewOrderRepository(db)
	ctx := context.Background()

	exchange := seedExchange(t, db, "XORD")
	listing := seedListing(t, db, "ORDT", exchange.MicCode, model.AssetTypeStock, 50.0)

	pastTime := time.Now().Add(-10 * time.Minute)
	futureTime := time.Now().Add(10 * time.Minute)

	readyOrder := &model.Order{
		OrderOwnerUserID: 1,
		AccountNumber:    "444000100000000001",
		ListingID:        listing.ListingID,
		OrderType:        model.OrderTypeMarket,
		Direction:        model.OrderDirectionBuy,
		Quantity:         5,
		ContractSize:     1,
		Status:           model.OrderStatusApproved,
		IsDone:           false,
		NextExecutionAt:  &pastTime,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	require.NoError(t, db.Create(readyOrder).Error)

	notYetOrder := &model.Order{
		OrderOwnerUserID: 1,
		AccountNumber:    "444000100000000001",
		ListingID:        listing.ListingID,
		OrderType:        model.OrderTypeMarket,
		Direction:        model.OrderDirectionBuy,
		Quantity:         5,
		ContractSize:     1,
		Status:           model.OrderStatusApproved,
		IsDone:           false,
		NextExecutionAt:  &futureTime,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	require.NoError(t, db.Create(notYetOrder).Error)

	pendingOrder := &model.Order{
		OrderOwnerUserID: 1,
		AccountNumber:    "444000100000000001",
		ListingID:        listing.ListingID,
		OrderType:        model.OrderTypeMarket,
		Direction:        model.OrderDirectionBuy,
		Quantity:         5,
		ContractSize:     1,
		Status:           model.OrderStatusPending,
		IsDone:           false,
		NextExecutionAt:  &pastTime,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	require.NoError(t, db.Create(pendingOrder).Error)

	doneOrder := &model.Order{
		OrderOwnerUserID: 1,
		AccountNumber:    "444000100000000001",
		ListingID:        listing.ListingID,
		OrderType:        model.OrderTypeMarket,
		Direction:        model.OrderDirectionBuy,
		Quantity:         5,
		ContractSize:     1,
		Status:           model.OrderStatusApproved,
		IsDone:           true,
		NextExecutionAt:  &pastTime,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	require.NoError(t, db.Create(doneOrder).Error)

	results, err := repo.FindReadyForExecution(ctx, time.Now(), 0)
	require.NoError(t, err)

	foundIDs := make(map[uint]bool)
	for _, o := range results {
		foundIDs[o.OrderID] = true
	}

	assert.True(t, foundIDs[readyOrder.OrderID], "ready order should be returned")
	assert.False(t, foundIDs[notYetOrder.OrderID], "future order should not be returned")
	assert.False(t, foundIDs[pendingOrder.OrderID], "pending order should not be returned")
	assert.False(t, foundIDs[doneOrder.OrderID], "done order should not be returned")
}

func TestOrderRepo_FindReadyForExecution_WithLimit(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewOrderRepository(db)
	ctx := context.Background()

	exchange := seedExchange(t, db, "XORL")
	listing := seedListing(t, db, "ORLT", exchange.MicCode, model.AssetTypeStock, 50.0)

	for i := 0; i < 5; i++ {
		execAt := time.Now().Add(-time.Duration(i+1) * time.Minute)
		o := &model.Order{
			OrderOwnerUserID: 1,
			AccountNumber:    "444000100000000001",
			ListingID:        listing.ListingID,
			OrderType:        model.OrderTypeMarket,
			Direction:        model.OrderDirectionBuy,
			Quantity:         1,
			ContractSize:     1,
			Status:           model.OrderStatusApproved,
			IsDone:           false,
			NextExecutionAt:  &execAt,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		require.NoError(t, db.Create(o).Error)
	}

	results, err := repo.FindReadyForExecution(ctx, time.Now(), 3)
	require.NoError(t, err)
	assert.Len(t, results, 3)
}

func TestOrderTransactionRepo_Create(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewOrderTransactionRepository(db)
	ctx := context.Background()

	exchange := seedExchange(t, db, "XOTX")
	listing := seedListing(t, db, "OTXL", exchange.MicCode, model.AssetTypeStock, 100.0)

	order := &model.Order{
		OrderOwnerUserID: 1,
		AccountNumber:    "444000100000000001",
		ListingID:        listing.ListingID,
		OrderType:        model.OrderTypeMarket,
		Direction:        model.OrderDirectionBuy,
		Quantity:         10,
		ContractSize:     1,
		Status:           model.OrderStatusApproved,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	require.NoError(t, db.Create(order).Error)

	tx := &model.OrderTransaction{
		OrderID:      order.OrderID,
		Quantity:     10,
		PricePerUnit: 100.0,
		TotalPrice:   1000.0,
		Commission:   5.0,
		ExecutedAt:   time.Now(),
		CreatedAt:    time.Now(),
	}

	err := repo.Create(ctx, tx)
	require.NoError(t, err)
	assert.NotZero(t, tx.OrderTransactionID)

	var found model.OrderTransaction
	require.NoError(t, db.First(&found, tx.OrderTransactionID).Error)
	assert.Equal(t, order.OrderID, found.OrderID)
	assert.Equal(t, uint(10), found.Quantity)
	assert.Equal(t, 100.0, found.PricePerUnit)
	assert.Equal(t, 1000.0, found.TotalPrice)
	assert.Equal(t, 5.0, found.Commission)
}

func TestStockRepo_FindByListingIDs(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewStockRepository(db)
	ctx := context.Background()

	exchange := seedExchange(t, db, "XSTK")
	listing1 := seedListing(t, db, "STKI", exchange.MicCode, model.AssetTypeStock, 100.0)
	listing2 := seedListing(t, db, "STKJ", exchange.MicCode, model.AssetTypeStock, 200.0)
	listing3 := seedListing(t, db, "STKK", exchange.MicCode, model.AssetTypeStock, 300.0)

	stock1 := seedStock(t, db, listing1.ListingID)
	stock2 := seedStock(t, db, listing2.ListingID)
	_ = seedStock(t, db, listing3.ListingID)

	results, err := repo.FindByAssetIDs(ctx, []uint{listing1.AssetID, listing2.AssetID})
	require.NoError(t, err)
	require.Len(t, results, 2)

	foundIDs := make(map[uint]bool)
	for _, s := range results {
		foundIDs[s.StockID] = true
		assert.NotNil(t, s.Listing)
	}

	assert.True(t, foundIDs[stock1.StockID])
	assert.True(t, foundIDs[stock2.StockID])
}

func TestStockRepo_FindByAssetIDs_Empty(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewStockRepository(db)
	ctx := context.Background()

	results, err := repo.FindByAssetIDs(ctx, []uint{99999, 99998})
	require.NoError(t, err)
	assert.Empty(t, results)
}
