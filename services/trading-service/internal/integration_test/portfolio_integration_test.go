//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

func TestGetClientPortfolio_Empty(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForClient(t, 50, 1)

	rec := performRequest(t, router, http.MethodGet, "/api/client/1/assets", nil, auth)
	requireStatus(t, rec, http.StatusOK)

	var body []any
	body = decodeResponse[[]any](t, rec)
	require.Empty(t, body)
}

func TestGetClientPortfolio_WithOwnership(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, "XNYS")
	listing := seedListing(t, db, "AAPL", ex.MicCode, model.AssetTypeStock, 150.0)
	seedStock(t, db, listing.ListingID)

	_ = seedOrder(t, db, 1, listing.ListingID, model.OrderDirectionBuy, model.OrderStatusApproved)

	ownership := &model.AssetOwnership{
		UserId:         1,
		OwnerType:      model.OwnerTypeClient,
		AssetID:        listing.AssetID,
		Amount:         10.0,
		AvgBuyPriceRSD: 100.0,
	}
	if err := db.Create(ownership).Error; err != nil {
		t.Fatalf("seed ownership: %v", err)
	}

	auth := authHeaderForClient(t, 50, 1)
	rec := performRequest(t, router, http.MethodGet, "/api/client/1/assets", nil, auth)
	requireStatus(t, rec, http.StatusOK)
}

func TestGetActuaryPortfolio_Empty(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForSupervisor(t)

	rec := performRequest(t, router, http.MethodGet, "/api/actuary/10/assets", nil, auth)
	requireStatus(t, rec, http.StatusOK)
}

func TestGetClientPortfolio_Unauthorized(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	rec := performRequest(t, router, http.MethodGet, "/api/client/1/assets", nil, "")
	require.NotEqual(t, http.StatusOK, rec.Code)
}

func TestGetClientPortfolio_InvalidID(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForClient(t, 50, 1)

	rec := performRequest(t, router, http.MethodGet, "/api/client/abc/assets", nil, auth)
	require.NotEqual(t, http.StatusOK, rec.Code)
}

func TestGetActuaryPortfolio_InvalidID(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForSupervisor(t)

	rec := performRequest(t, router, http.MethodGet, "/api/actuary/abc/assets", nil, auth)
	require.NotEqual(t, http.StatusOK, rec.Code)
}

func TestExerciseOption_Success(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	exchange := seedExchange(t, db, "XNYS")
	stockListing := seedListing(t, db, "AAPL", exchange.MicCode, model.AssetTypeStock, 190.0)
	stock := seedStock(t, db, stockListing.ListingID)

	optionListing := seedListing(t, db, "AAPL:CALL:150.00", exchange.MicCode, model.AssetTypeOption, 15.0)
	seedOption(t, db, optionListing.ListingID, stock.StockID)

	optionOwnership := &model.AssetOwnership{
		UserId:         10,
		OwnerType:      model.OwnerTypeActuary,
		AssetID:        optionListing.AssetID,
		Amount:         100,
		AvgBuyPriceRSD: 12,
	}
	if err := db.Create(optionOwnership).Error; err != nil {
		t.Fatalf("seed option ownership: %v", err)
	}

	auth := authHeaderForSupervisor(t)
	rec := performRequest(
		t,
		router,
		http.MethodPost,
		fmt.Sprintf("/api/actuary/10/options/%d/exercise", optionListing.AssetID),
		map[string]any{"account_number": "444000100000000001"},
		auth,
	)
	requireStatus(t, rec, http.StatusOK)

	response := decodeResponse[struct {
		OptionAssetID      uint    `json:"option_asset_id"`
		StockAssetID       uint    `json:"stock_asset_id"`
		ExercisedContracts uint    `json:"exercised_contracts"`
		PurchasedShares    float64 `json:"purchased_shares"`
		TotalCost          float64 `json:"total_cost"`
		RemainingContracts uint    `json:"remaining_contracts"`
	}](t, rec)

	require.Equal(t, optionListing.AssetID, response.OptionAssetID)
	require.Equal(t, stock.AssetID, response.StockAssetID)
	require.Equal(t, uint(1), response.ExercisedContracts)
	require.Equal(t, 100.0, response.PurchasedShares)
	require.Equal(t, 15000.0, response.TotalCost)
	require.Equal(t, uint(0), response.RemainingContracts)

	var updatedOptionOwnership model.AssetOwnership
	if err := db.Where("user_id = ? AND owner_type = ? AND asset_id = ?", 10, model.OwnerTypeActuary, optionListing.AssetID).
		First(&updatedOptionOwnership).Error; err != nil {
		t.Fatalf("load updated option ownership: %v", err)
	}
	require.Equal(t, 0.0, updatedOptionOwnership.Amount)

	var stockOwnership model.AssetOwnership
	if err := db.Where("user_id = ? AND owner_type = ? AND asset_id = ?", 10, model.OwnerTypeActuary, stock.AssetID).
		First(&stockOwnership).Error; err != nil {
		t.Fatalf("load stock ownership: %v", err)
	}
	require.Equal(t, 100.0, stockOwnership.Amount)
	require.Equal(t, 150.0, stockOwnership.AvgBuyPriceRSD)
}
