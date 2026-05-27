package service

import (
	"context"
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/permission"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupWatchlistTestDB(t *testing.T) *gorm.DB {
	dsn := "file:testdb_watchlist_" + time.Now().Format("150405.000000") + "?mode=memory&_pragma=foreign_keys(1)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	require.NoError(t, db.AutoMigrate(
		&model.Exchange{},
		&model.Asset{},
		&model.Listing{},
		&model.Stock{},
		&model.ListingDailyPriceInfo{},
		&model.Watchlist{},
		&model.WatchlistItem{},
	))
	return db
}

// seedWatchlistListings creates two listings (a stock and a forex pair) with a
// latest daily price info row each, and returns their listing ids.
func seedWatchlistListings(t *testing.T, db *gorm.DB) (stockListingID, forexListingID uint) {
	exchange := model.Exchange{
		Name: "Nasdaq", Acronym: "NASDAQ", MicCode: "XNAS", Polity: "USA",
		Currency: "USD", TimeZone: -4, OpenTime: "09:30", CloseTime: "16:00", TradingEnabled: true,
	}
	require.NoError(t, db.Create(&exchange).Error)

	stockAsset := model.Asset{Ticker: "AAPL", Name: "Apple Inc", AssetType: model.AssetTypeStock}
	forexAsset := model.Asset{Ticker: "EURUSD", Name: "EUR/USD", AssetType: model.AssetTypeForexPair}
	require.NoError(t, db.Create(&stockAsset).Error)
	require.NoError(t, db.Create(&forexAsset).Error)

	stockListing := model.Listing{AssetID: stockAsset.AssetID, ExchangeMIC: "XNAS", Price: 150.0, Ask: 151.0, MaintenanceMargin: 10.0, LastRefresh: time.Now()}
	forexListing := model.Listing{AssetID: forexAsset.AssetID, ExchangeMIC: "XNAS", Price: 1.08, Ask: 1.081, MaintenanceMargin: 0.1, LastRefresh: time.Now()}
	require.NoError(t, db.Create(&stockListing).Error)
	require.NoError(t, db.Create(&forexListing).Error)

	require.NoError(t, db.Omit("Asset").Create(&model.Stock{AssetID: stockAsset.AssetID, OutstandingShares: 1_000_000, DividendYield: 0.5}).Error)

	// Two daily rows for the stock so we can verify the *latest* surfaces.
	require.NoError(t, db.Omit("Listing").Create(&model.ListingDailyPriceInfo{ListingID: stockListing.ListingID, Date: time.Now().Add(-24 * time.Hour), Price: 148, Bid: 147, Change: 1.0, Volume: 1000}).Error)
	require.NoError(t, db.Omit("Listing").Create(&model.ListingDailyPriceInfo{ListingID: stockListing.ListingID, Date: time.Now(), Price: 150, Bid: 149.5, Change: 2.5, Volume: 4200}).Error)
	require.NoError(t, db.Omit("Listing").Create(&model.ListingDailyPriceInfo{ListingID: forexListing.ListingID, Date: time.Now(), Price: 1.08, Bid: 1.079, Change: -0.3, Volume: 99}).Error)

	return stockListing.ListingID, forexListing.ListingID
}

func newWatchlistService(db *gorm.DB) *WatchlistService {
	return NewWatchlistService(
		repository.NewWatchlistRepository(db),
		repository.NewListingRepository(db),
	)
}

func clientCtx(clientID uint) context.Context {
	cid := clientID
	return auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityID:   clientID,
		IdentityType: auth.IdentityClient,
		ClientID:     &cid,
		Permissions:  []permission.Permission{permission.Trading},
	})
}

func actuaryCtx(employeeID uint) context.Context {
	eid := employeeID
	return auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityID:   employeeID,
		IdentityType: auth.IdentityEmployee,
		EmployeeID:   &eid,
		Permissions:  []permission.Permission{permission.Trading},
	})
}

func TestWatchlist_CreateAndList(t *testing.T) {
	db := setupWatchlistTestDB(t)
	svc := newWatchlistService(db)
	ctx := clientCtx(1)

	created, err := svc.CreateWatchlist(ctx, dto.CreateWatchlistRequest{Name: "tech stocks"})
	require.NoError(t, err)
	assert.NotZero(t, created.WatchlistID)
	assert.Equal(t, "tech stocks", created.Name)
	assert.Equal(t, 0, created.ItemCount)
	assert.False(t, created.CreatedAt.IsZero())

	_, err = svc.CreateWatchlist(ctx, dto.CreateWatchlistRequest{Name: "forex pairs"})
	require.NoError(t, err)

	lists, err := svc.GetWatchlists(ctx)
	require.NoError(t, err)
	require.Len(t, lists, 2)
	assert.Equal(t, "tech stocks", lists[0].Name)
	assert.Equal(t, "forex pairs", lists[1].Name)
}

func TestWatchlist_CreateDuplicateNameConflicts(t *testing.T) {
	db := setupWatchlistTestDB(t)
	svc := newWatchlistService(db)
	ctx := clientCtx(1)

	_, err := svc.CreateWatchlist(ctx, dto.CreateWatchlistRequest{Name: "tech stocks"})
	require.NoError(t, err)

	_, err = svc.CreateWatchlist(ctx, dto.CreateWatchlistRequest{Name: "tech stocks"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestWatchlist_SameNameAllowedForDifferentOwners(t *testing.T) {
	db := setupWatchlistTestDB(t)
	svc := newWatchlistService(db)

	_, err := svc.CreateWatchlist(clientCtx(1), dto.CreateWatchlistRequest{Name: "tech stocks"})
	require.NoError(t, err)
	// A different client may reuse the name.
	_, err = svc.CreateWatchlist(clientCtx(2), dto.CreateWatchlistRequest{Name: "tech stocks"})
	require.NoError(t, err)
	// An actuary with the same numeric id as client 1 lives in a separate namespace.
	_, err = svc.CreateWatchlist(actuaryCtx(1), dto.CreateWatchlistRequest{Name: "tech stocks"})
	require.NoError(t, err)
}

func TestWatchlist_AddListingAndDetail(t *testing.T) {
	db := setupWatchlistTestDB(t)
	svc := newWatchlistService(db)
	ctx := clientCtx(1)
	stockListingID, forexListingID := seedWatchlistListings(t, db)

	wl, err := svc.CreateWatchlist(ctx, dto.CreateWatchlistRequest{Name: "mixed"})
	require.NoError(t, err)

	require.NoError(t, svc.AddListing(ctx, wl.WatchlistID, dto.AddWatchlistItemRequest{ListingID: stockListingID}))
	require.NoError(t, svc.AddListing(ctx, wl.WatchlistID, dto.AddWatchlistItemRequest{ListingID: forexListingID}))

	detail, err := svc.GetWatchlistDetail(ctx, wl.WatchlistID, "")
	require.NoError(t, err)
	require.Len(t, detail.Listings, 2)

	// The stock should carry the latest daily price info (change 2.5, volume 4200).
	var stock dto.WatchlistListingResponse
	for _, l := range detail.Listings {
		if l.ListingID == stockListingID {
			stock = l
		}
	}
	assert.Equal(t, "AAPL", stock.Ticker)
	assert.Equal(t, string(model.AssetTypeStock), stock.AssetType)
	assert.Equal(t, 150.0, stock.Price)
	assert.Equal(t, 149.5, stock.Bid)
	assert.Equal(t, 2.5, stock.Change)
	assert.Equal(t, uint(4200), stock.Volume)
	assert.False(t, stock.AddedAt.IsZero())

	// Item count is reflected in the list view.
	lists, err := svc.GetWatchlists(ctx)
	require.NoError(t, err)
	require.Len(t, lists, 1)
	assert.Equal(t, 2, lists[0].ItemCount)
}

func TestWatchlist_DetailFilteredByAssetType(t *testing.T) {
	db := setupWatchlistTestDB(t)
	svc := newWatchlistService(db)
	ctx := clientCtx(1)
	stockListingID, forexListingID := seedWatchlistListings(t, db)

	wl, err := svc.CreateWatchlist(ctx, dto.CreateWatchlistRequest{Name: "mixed"})
	require.NoError(t, err)
	require.NoError(t, svc.AddListing(ctx, wl.WatchlistID, dto.AddWatchlistItemRequest{ListingID: stockListingID}))
	require.NoError(t, svc.AddListing(ctx, wl.WatchlistID, dto.AddWatchlistItemRequest{ListingID: forexListingID}))

	detail, err := svc.GetWatchlistDetail(ctx, wl.WatchlistID, string(model.AssetTypeForexPair))
	require.NoError(t, err)
	require.Len(t, detail.Listings, 1)
	assert.Equal(t, forexListingID, detail.Listings[0].ListingID)
	assert.Equal(t, string(model.AssetTypeForexPair), detail.Listings[0].AssetType)

	_, err = svc.GetWatchlistDetail(ctx, wl.WatchlistID, "bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid asset_type")
}

func TestWatchlist_AddDuplicateListingConflicts(t *testing.T) {
	db := setupWatchlistTestDB(t)
	svc := newWatchlistService(db)
	ctx := clientCtx(1)
	stockListingID, _ := seedWatchlistListings(t, db)

	wl, err := svc.CreateWatchlist(ctx, dto.CreateWatchlistRequest{Name: "mixed"})
	require.NoError(t, err)
	require.NoError(t, svc.AddListing(ctx, wl.WatchlistID, dto.AddWatchlistItemRequest{ListingID: stockListingID}))

	err = svc.AddListing(ctx, wl.WatchlistID, dto.AddWatchlistItemRequest{ListingID: stockListingID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already in this watchlist")
}

func TestWatchlist_AddNonexistentListingNotFound(t *testing.T) {
	db := setupWatchlistTestDB(t)
	svc := newWatchlistService(db)
	ctx := clientCtx(1)
	seedWatchlistListings(t, db)

	wl, err := svc.CreateWatchlist(ctx, dto.CreateWatchlistRequest{Name: "mixed"})
	require.NoError(t, err)

	err = svc.AddListing(ctx, wl.WatchlistID, dto.AddWatchlistItemRequest{ListingID: 99999})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing not found")
}

func TestWatchlist_OwnershipIsolation(t *testing.T) {
	db := setupWatchlistTestDB(t)
	svc := newWatchlistService(db)
	ownerCtx := clientCtx(1)
	otherCtx := clientCtx(2)
	stockListingID, _ := seedWatchlistListings(t, db)

	wl, err := svc.CreateWatchlist(ownerCtx, dto.CreateWatchlistRequest{Name: "private"})
	require.NoError(t, err)

	// Another user cannot see it.
	otherLists, err := svc.GetWatchlists(otherCtx)
	require.NoError(t, err)
	assert.Empty(t, otherLists)

	// ...nor read its detail.
	_, err = svc.GetWatchlistDetail(otherCtx, wl.WatchlistID, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// ...nor add to it.
	err = svc.AddListing(otherCtx, wl.WatchlistID, dto.AddWatchlistItemRequest{ListingID: stockListingID})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// ...nor delete it.
	err = svc.DeleteWatchlist(otherCtx, wl.WatchlistID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestWatchlist_RemoveListing(t *testing.T) {
	db := setupWatchlistTestDB(t)
	svc := newWatchlistService(db)
	ctx := clientCtx(1)
	stockListingID, _ := seedWatchlistListings(t, db)

	wl, err := svc.CreateWatchlist(ctx, dto.CreateWatchlistRequest{Name: "mixed"})
	require.NoError(t, err)
	require.NoError(t, svc.AddListing(ctx, wl.WatchlistID, dto.AddWatchlistItemRequest{ListingID: stockListingID}))

	require.NoError(t, svc.RemoveListing(ctx, wl.WatchlistID, stockListingID))

	detail, err := svc.GetWatchlistDetail(ctx, wl.WatchlistID, "")
	require.NoError(t, err)
	assert.Empty(t, detail.Listings)

	// Removing again reports not found.
	err = svc.RemoveListing(ctx, wl.WatchlistID, stockListingID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestWatchlist_Delete(t *testing.T) {
	db := setupWatchlistTestDB(t)
	svc := newWatchlistService(db)
	ctx := clientCtx(1)
	stockListingID, _ := seedWatchlistListings(t, db)

	wl, err := svc.CreateWatchlist(ctx, dto.CreateWatchlistRequest{Name: "mixed"})
	require.NoError(t, err)
	require.NoError(t, svc.AddListing(ctx, wl.WatchlistID, dto.AddWatchlistItemRequest{ListingID: stockListingID}))

	require.NoError(t, svc.DeleteWatchlist(ctx, wl.WatchlistID))

	lists, err := svc.GetWatchlists(ctx)
	require.NoError(t, err)
	assert.Empty(t, lists)

	// The items were removed with the watchlist.
	var itemCount int64
	require.NoError(t, db.Model(&model.WatchlistItem{}).Where("watchlist_id = ?", wl.WatchlistID).Count(&itemCount).Error)
	assert.Equal(t, int64(0), itemCount)
}

func TestWatchlist_RequiresAuth(t *testing.T) {
	db := setupWatchlistTestDB(t)
	svc := newWatchlistService(db)

	_, err := svc.CreateWatchlist(context.Background(), dto.CreateWatchlistRequest{Name: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not authenticated")
}
