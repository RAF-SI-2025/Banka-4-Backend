//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/permission"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

// seedWatchlistListing seeds a listing + matching asset sub-row + one daily
// price point so the response shape (price/bid/change/volume) is fully
// populated, matching how listing_integration_test.go composes fixtures.
func seedWatchlistListing(t *testing.T, db *gorm.DB, ticker, micCode string, assetType model.AssetType, price float64) *model.Listing {
	t.Helper()

	listing := seedListing(t, db, ticker, micCode, assetType, price)
	switch assetType {
	case model.AssetTypeStock:
		seedStock(t, db, listing.ListingID)
	case model.AssetTypeForexPair:
		seedForex(t, db, listing.ListingID)
	case model.AssetTypeFuture:
		seedFuture(t, db, listing.ListingID)
	}
	seedDailyPriceInfo(t, db, listing.ListingID)
	return listing
}

func TestWatchlist_CreateAndList_AsClient(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)
	auth := authHeaderForClient(t, 50, 1)

	rec := performRequest(t, router, http.MethodPost, "/api/watchlists",
		dto.CreateWatchlistRequest{Name: "tech stocks"}, auth)
	requireStatus(t, rec, http.StatusCreated)

	created := decodeResponse[dto.WatchlistResponse](t, rec)
	assert.NotZero(t, created.WatchlistID)
	assert.Equal(t, "tech stocks", created.Name)
	assert.Equal(t, 0, created.ItemCount)

	rec = performRequest(t, router, http.MethodGet, "/api/watchlists", nil, auth)
	requireStatus(t, rec, http.StatusOK)
	list := decodeResponse[[]dto.WatchlistResponse](t, rec)
	require.Len(t, list, 1)
	assert.Equal(t, "tech stocks", list[0].Name)
}

// Lead's explicit requirement: supervisor must be able to use these endpoints
// — Trading permission alone is the authorization check.
func TestWatchlist_AccessibleToSupervisor(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)
	auth := authHeaderForSupervisor(t)

	rec := performRequest(t, router, http.MethodPost, "/api/watchlists",
		dto.CreateWatchlistRequest{Name: "supervisor watchlist"}, auth)
	requireStatus(t, rec, http.StatusCreated)

	rec = performRequest(t, router, http.MethodGet, "/api/watchlists", nil, auth)
	requireStatus(t, rec, http.StatusOK)
	list := decodeResponse[[]dto.WatchlistResponse](t, rec)
	require.Len(t, list, 1)
	assert.Equal(t, "supervisor watchlist", list[0].Name)
}

func TestWatchlist_AccessibleToAgent(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)
	auth := authHeaderForAgent(t)

	rec := performRequest(t, router, http.MethodPost, "/api/watchlists",
		dto.CreateWatchlistRequest{Name: "agent picks"}, auth)
	requireStatus(t, rec, http.StatusCreated)
}

func TestWatchlist_Unauthorized(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	rec := performRequest(t, router, http.MethodGet, "/api/watchlists", nil, "")
	require.NotEqual(t, http.StatusOK, rec.Code)
}

func TestWatchlist_ForbiddenWithoutTradingPermission(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouterWithPermissions(t, db, []permission.Permission{})
	auth := authHeaderForClient(t, 50, 1)

	rec := performRequest(t, router, http.MethodGet, "/api/watchlists", nil, auth)
	requireStatus(t, rec, http.StatusForbidden)
}

func TestWatchlist_AddListingAndGetDetail(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)
	auth := authHeaderForClient(t, 50, 1)

	ex := seedExchange(t, db, "XNYS")
	listing := seedWatchlistListing(t, db, "AAPL", ex.MicCode, model.AssetTypeStock, 150.0)

	// create watchlist
	rec := performRequest(t, router, http.MethodPost, "/api/watchlists",
		dto.CreateWatchlistRequest{Name: "core"}, auth)
	requireStatus(t, rec, http.StatusCreated)
	wl := decodeResponse[dto.WatchlistResponse](t, rec)

	// add listing
	rec = performRequest(t, router, http.MethodPost,
		fmt.Sprintf("/api/watchlists/%d/items", wl.WatchlistID),
		dto.AddWatchlistItemRequest{ListingID: listing.ListingID}, auth)
	requireStatus(t, rec, http.StatusNoContent)

	// detail
	rec = performRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/watchlists/%d", wl.WatchlistID), nil, auth)
	requireStatus(t, rec, http.StatusOK)
	detail := decodeResponse[dto.WatchlistDetailResponse](t, rec)
	require.Len(t, detail.Listings, 1)
	assert.Equal(t, "AAPL", detail.Listings[0].Ticker)
	assert.Equal(t, string(model.AssetTypeStock), detail.Listings[0].AssetType)

	// item count reflected in list view
	rec = performRequest(t, router, http.MethodGet, "/api/watchlists", nil, auth)
	requireStatus(t, rec, http.StatusOK)
	list := decodeResponse[[]dto.WatchlistResponse](t, rec)
	require.Len(t, list, 1)
	assert.Equal(t, 1, list[0].ItemCount)
}

func TestWatchlist_DetailFilteredByAssetType(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)
	auth := authHeaderForClient(t, 50, 1)

	ex := seedExchange(t, db, "XNYS")
	stock := seedWatchlistListing(t, db, "AAPL", ex.MicCode, model.AssetTypeStock, 150.0)
	forex := seedWatchlistListing(t, db, "EURUSD", ex.MicCode, model.AssetTypeForexPair, 1.09)

	rec := performRequest(t, router, http.MethodPost, "/api/watchlists",
		dto.CreateWatchlistRequest{Name: "mixed"}, auth)
	requireStatus(t, rec, http.StatusCreated)
	wl := decodeResponse[dto.WatchlistResponse](t, rec)

	for _, id := range []uint{stock.ListingID, forex.ListingID} {
		rec = performRequest(t, router, http.MethodPost,
			fmt.Sprintf("/api/watchlists/%d/items", wl.WatchlistID),
			dto.AddWatchlistItemRequest{ListingID: id}, auth)
		requireStatus(t, rec, http.StatusNoContent)
	}

	// no filter — both
	rec = performRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/watchlists/%d", wl.WatchlistID), nil, auth)
	requireStatus(t, rec, http.StatusOK)
	detail := decodeResponse[dto.WatchlistDetailResponse](t, rec)
	require.Len(t, detail.Listings, 2)

	// stock filter — one
	rec = performRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/watchlists/%d?asset_type=stock", wl.WatchlistID), nil, auth)
	requireStatus(t, rec, http.StatusOK)
	detail = decodeResponse[dto.WatchlistDetailResponse](t, rec)
	require.Len(t, detail.Listings, 1)
	assert.Equal(t, string(model.AssetTypeStock), detail.Listings[0].AssetType)

	// invalid filter — 400
	rec = performRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/watchlists/%d?asset_type=bogus", wl.WatchlistID), nil, auth)
	requireStatus(t, rec, http.StatusBadRequest)
}

func TestWatchlist_DuplicateNameConflicts(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)
	auth := authHeaderForClient(t, 50, 1)

	rec := performRequest(t, router, http.MethodPost, "/api/watchlists",
		dto.CreateWatchlistRequest{Name: "dupes"}, auth)
	requireStatus(t, rec, http.StatusCreated)

	rec = performRequest(t, router, http.MethodPost, "/api/watchlists",
		dto.CreateWatchlistRequest{Name: "dupes"}, auth)
	requireStatus(t, rec, http.StatusConflict)
}

func TestWatchlist_AddDuplicateListingConflicts(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)
	auth := authHeaderForClient(t, 50, 1)

	ex := seedExchange(t, db, "XNYS")
	listing := seedWatchlistListing(t, db, "AAPL", ex.MicCode, model.AssetTypeStock, 150.0)

	rec := performRequest(t, router, http.MethodPost, "/api/watchlists",
		dto.CreateWatchlistRequest{Name: "core"}, auth)
	requireStatus(t, rec, http.StatusCreated)
	wl := decodeResponse[dto.WatchlistResponse](t, rec)

	rec = performRequest(t, router, http.MethodPost,
		fmt.Sprintf("/api/watchlists/%d/items", wl.WatchlistID),
		dto.AddWatchlistItemRequest{ListingID: listing.ListingID}, auth)
	requireStatus(t, rec, http.StatusNoContent)

	rec = performRequest(t, router, http.MethodPost,
		fmt.Sprintf("/api/watchlists/%d/items", wl.WatchlistID),
		dto.AddWatchlistItemRequest{ListingID: listing.ListingID}, auth)
	requireStatus(t, rec, http.StatusConflict)
}

func TestWatchlist_AddNonexistentListingNotFound(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)
	auth := authHeaderForClient(t, 50, 1)

	rec := performRequest(t, router, http.MethodPost, "/api/watchlists",
		dto.CreateWatchlistRequest{Name: "ghosts"}, auth)
	requireStatus(t, rec, http.StatusCreated)
	wl := decodeResponse[dto.WatchlistResponse](t, rec)

	rec = performRequest(t, router, http.MethodPost,
		fmt.Sprintf("/api/watchlists/%d/items", wl.WatchlistID),
		dto.AddWatchlistItemRequest{ListingID: 99999}, auth)
	requireStatus(t, rec, http.StatusNotFound)
}

// Another user's watchlist must look like it does not exist — no probing.
func TestWatchlist_OwnershipIsolation(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)
	alice := authHeaderForClient(t, 50, 1)
	bob := authHeaderForClient(t, 51, 2)

	rec := performRequest(t, router, http.MethodPost, "/api/watchlists",
		dto.CreateWatchlistRequest{Name: "alice list"}, alice)
	requireStatus(t, rec, http.StatusCreated)
	wl := decodeResponse[dto.WatchlistResponse](t, rec)

	rec = performRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/watchlists/%d", wl.WatchlistID), nil, bob)
	requireStatus(t, rec, http.StatusNotFound)

	rec = performRequest(t, router, http.MethodDelete,
		fmt.Sprintf("/api/watchlists/%d", wl.WatchlistID), nil, bob)
	requireStatus(t, rec, http.StatusNotFound)

	// bob's listing should not appear in alice's view
	rec = performRequest(t, router, http.MethodGet, "/api/watchlists", nil, bob)
	requireStatus(t, rec, http.StatusOK)
	bobList := decodeResponse[[]dto.WatchlistResponse](t, rec)
	require.Len(t, bobList, 0)
}

func TestWatchlist_RemoveListingAndDelete(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)
	auth := authHeaderForClient(t, 50, 1)

	ex := seedExchange(t, db, "XNYS")
	listing := seedWatchlistListing(t, db, "AAPL", ex.MicCode, model.AssetTypeStock, 150.0)

	rec := performRequest(t, router, http.MethodPost, "/api/watchlists",
		dto.CreateWatchlistRequest{Name: "core"}, auth)
	requireStatus(t, rec, http.StatusCreated)
	wl := decodeResponse[dto.WatchlistResponse](t, rec)

	rec = performRequest(t, router, http.MethodPost,
		fmt.Sprintf("/api/watchlists/%d/items", wl.WatchlistID),
		dto.AddWatchlistItemRequest{ListingID: listing.ListingID}, auth)
	requireStatus(t, rec, http.StatusNoContent)

	// remove listing
	rec = performRequest(t, router, http.MethodDelete,
		fmt.Sprintf("/api/watchlists/%d/items/%d", wl.WatchlistID, listing.ListingID), nil, auth)
	requireStatus(t, rec, http.StatusNoContent)

	// detail now empty
	rec = performRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/watchlists/%d", wl.WatchlistID), nil, auth)
	requireStatus(t, rec, http.StatusOK)
	detail := decodeResponse[dto.WatchlistDetailResponse](t, rec)
	assert.Len(t, detail.Listings, 0)

	// removing again -> 404
	rec = performRequest(t, router, http.MethodDelete,
		fmt.Sprintf("/api/watchlists/%d/items/%d", wl.WatchlistID, listing.ListingID), nil, auth)
	requireStatus(t, rec, http.StatusNotFound)

	// delete watchlist
	rec = performRequest(t, router, http.MethodDelete,
		fmt.Sprintf("/api/watchlists/%d", wl.WatchlistID), nil, auth)
	requireStatus(t, rec, http.StatusNoContent)

	// gone
	rec = performRequest(t, router, http.MethodGet,
		fmt.Sprintf("/api/watchlists/%d", wl.WatchlistID), nil, auth)
	requireStatus(t, rec, http.StatusNotFound)
}

func TestWatchlist_SameNameAllowedForDifferentOwners(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)
	alice := authHeaderForClient(t, 50, 1)
	bob := authHeaderForClient(t, 51, 2)

	rec := performRequest(t, router, http.MethodPost, "/api/watchlists",
		dto.CreateWatchlistRequest{Name: "favorites"}, alice)
	requireStatus(t, rec, http.StatusCreated)

	rec = performRequest(t, router, http.MethodPost, "/api/watchlists",
		dto.CreateWatchlistRequest{Name: "favorites"}, bob)
	requireStatus(t, rec, http.StatusCreated)
}
