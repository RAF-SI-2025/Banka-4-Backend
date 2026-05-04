//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// --- Seed helpers ---

func seedAssetAndStock(t *testing.T, db *gorm.DB, ticker string) (*model.Asset, *model.Stock) {
	if len(ticker) > 20 {
		ticker = ticker[:20]
	}
	t.Helper()
	ex := seedExchange(t, db, uniqueMIC(t))
	listing := seedListing(t, db, ticker, ex.MicCode, model.AssetTypeStock, 100.0)
	asset := &model.Asset{}
	require.NoError(t, db.First(asset, listing.AssetID).Error)

	// Insert Stock with StockAssetID forced equal to AssetID so that the service's
	// FindByAssetIDs([]uint{stockAssetID}) lookup (WHERE asset_id = stockAssetID) succeeds
	// and the FK constraint fk_otc_offers_stock (references stocks.asset_id) is satisfied.
	stock := &model.Stock{
		AssetID:           asset.AssetID,
		OutstandingShares: 1_000_000,
		DividendYield:     2.5,
	}
	stock.AssetID = asset.AssetID
	require.NoError(t, db.Create(stock).Error)
	return asset, stock
}

func seedAssetOwnership(t *testing.T, db *gorm.DB, identityID uint, ownerType model.OwnerType, assetID uint, amount float64) *model.AssetOwnership {
	t.Helper()
	o := &model.AssetOwnership{
		UserId:    identityID,
		OwnerType: ownerType,
		AssetID:   assetID,
		Amount:    amount,
		UpdatedAt: time.Now(),
	}
	require.NoError(t, db.Create(o).Error)
	return o
}

func setPublicAmount(t *testing.T, db *gorm.DB, ownershipID uint, publicAmount, reservedAmount float64) {
	t.Helper()
	require.NoError(t, db.Model(&model.AssetOwnership{}).
		Where("asset_ownership_id = ?", ownershipID).
		Updates(map[string]any{
			"public_amount":   publicAmount,
			"reserved_amount": reservedAmount,
		}).Error)
}

func uniqueMIC(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("X%d", uniqueCounter.Add(1))
}

func uniqueTicker(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("T%d", uniqueCounter.Add(1))
}

// clientAuthHeader generates a JWT for a client identity (used as buyer/seller in OTC).
// identityID is what the service reads as the user's ID, clientID is the client table PK.
func clientAuthHeader(t *testing.T, identityID, clientID uint) string {
	return authHeaderForClient(t, identityID, clientID)
}

// --- OTC Offer tests (otc-contracts branch) ---

func TestOTC_CreateOffer_Success(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	asset, _ := seedAssetAndStock(t, db, uniqueValue(t, "CRTO"))
	// Seller identity = 20
	ownership := seedAssetOwnership(t, db, 20, model.OwnerTypeClient, asset.AssetID, 100)
	setPublicAmount(t, db, ownership.AssetOwnershipID, 100, 0)

	body := map[string]any{
		"asset_ownership_id": ownership.AssetOwnershipID,
		"amount":             10,
		"price_per_stock":    50.0,
		"premium":            5.0,
		"settlement_date":    time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
		// "buyer-acc" resolves to clientID=10 via fakeBankingClient
		"buyer_account_number": "buyer-acc",
	}

	// Buyer JWT: identityID=10, clientID=10
	rec := performRequest(t, router, http.MethodPost, "/api/otc/offers", body, clientAuthHeader(t, 10, 10))
	requireStatus(t, rec, http.StatusCreated)

	resp := decodeResponse[map[string]any](t, rec)
	assert.Equal(t, float64(10), resp["buyer_id"])
	assert.Equal(t, float64(20), resp["seller_id"])
	assert.Equal(t, float64(10), resp["amount"])
	assert.Equal(t, "ACTIVE", resp["status"])
}

func TestOTC_CreateOffer_SelfOffer_BadRequest(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	asset, _ := seedAssetAndStock(t, db, uniqueValue(t, "SELF"))
	// Seller identity = 10 (same as buyer below)
	ownership := seedAssetOwnership(t, db, 10, model.OwnerTypeClient, asset.AssetID, 100)
	setPublicAmount(t, db, ownership.AssetOwnershipID, 100, 0)

	body := map[string]any{
		"asset_ownership_id": ownership.AssetOwnershipID,
		"amount":             5,
		"price_per_stock":    50.0,
		"premium":            5.0,
		"settlement_date":    time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
		// "buyer-acc" resolves to clientID=10 — same as seller (identityID=10) → self-offer
		"buyer_account_number": "buyer-acc",
	}

	// Buyer JWT: identityID=10, clientID=10
	rec := performRequest(t, router, http.MethodPost, "/api/otc/offers", body, clientAuthHeader(t, 10, 10))
	requireStatus(t, rec, http.StatusBadRequest)
}

func TestOTC_CreateOffer_Unauthorized(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	body := map[string]any{
		"asset_ownership_id":   1,
		"amount":               5,
		"price_per_stock":      50.0,
		"premium":              5.0,
		"settlement_date":      time.Now().Add(time.Hour * 24).Format(time.RFC3339),
		"buyer_account_number": "buyer-acc",
	}

	rec := performRequest(t, router, http.MethodPost, "/api/otc/offers", body, "")
	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestOTC_SendCounterOffer_Success(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	asset, _ := seedAssetAndStock(t, db, uniqueValue(t, "CNTR"))
	// Seller identity = 20
	ownership := seedAssetOwnership(t, db, 20, model.OwnerTypeClient, asset.AssetID, 100)
	setPublicAmount(t, db, ownership.AssetOwnershipID, 100, 0)

	// Buyer (identityID=10) creates the offer; seller account = ownership owner = 20
	offerBody := map[string]any{
		"asset_ownership_id":   ownership.AssetOwnershipID,
		"amount":               10,
		"price_per_stock":      50.0,
		"premium":              5.0,
		"settlement_date":      time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
		"buyer_account_number": "buyer-acc", // resolves to clientID=10
	}
	rec := performRequest(t, router, http.MethodPost, "/api/otc/offers", offerBody, clientAuthHeader(t, 10, 10))
	requireStatus(t, rec, http.StatusCreated)
	offer := decodeResponse[map[string]any](t, rec)
	offerID := uint(offer["otc_offer_id"].(float64))

	// Seller (identityID=20) sends counter-offer; "seller-acc" resolves to clientID=20
	counterBody := map[string]any{
		"amount":          8,
		"price_per_stock": 55.0,
		"premium":         6.0,
		"settlement_date": time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
		"account_number":  "seller-acc", // resolves to clientID=20
	}
	rec = performRequest(t, router, http.MethodPut, fmt.Sprintf("/api/otc/offers/%d/counter", offerID), counterBody, clientAuthHeader(t, 20, 20))
	requireStatus(t, rec, http.StatusOK)

	updated := decodeResponse[map[string]any](t, rec)
	assert.Equal(t, float64(8), updated["amount"])
	assert.Equal(t, float64(20), updated["modified_by"])
}

func TestOTC_SendCounterOffer_SameUserTwice_BadRequest(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	asset, _ := seedAssetAndStock(t, db, uniqueValue(t, "DBL"))
	ownership := seedAssetOwnership(t, db, 20, model.OwnerTypeClient, asset.AssetID, 100)
	setPublicAmount(t, db, ownership.AssetOwnershipID, 100, 0)

	offerBody := map[string]any{
		"asset_ownership_id":   ownership.AssetOwnershipID,
		"amount":               10,
		"price_per_stock":      50.0,
		"premium":              5.0,
		"settlement_date":      time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
		"buyer_account_number": "buyer-acc", // resolves to clientID=10
	}
	rec := performRequest(t, router, http.MethodPost, "/api/otc/offers", offerBody, clientAuthHeader(t, 10, 10))
	requireStatus(t, rec, http.StatusCreated)
	offer := decodeResponse[map[string]any](t, rec)
	offerID := uint(offer["otc_offer_id"].(float64))

	// Buyer (identityID=10) tries to counter their own offer → bad request
	counterBody := map[string]any{
		"amount":          9,
		"price_per_stock": 50.0,
		"premium":         5.0,
		"settlement_date": time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
	}
	rec = performRequest(t, router, http.MethodPut, fmt.Sprintf("/api/otc/offers/%d/counter", offerID), counterBody, clientAuthHeader(t, 10, 10))
	requireStatus(t, rec, http.StatusBadRequest)
}

func TestOTC_AcceptOffer_Success_CreatesContract(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	asset, _ := seedAssetAndStock(t, db, uniqueValue(t, "ACPT"))
	// Seller identity = 20
	ownership := seedAssetOwnership(t, db, 20, model.OwnerTypeClient, asset.AssetID, 100)
	setPublicAmount(t, db, ownership.AssetOwnershipID, 100, 0)

	offerBody := map[string]any{
		"asset_ownership_id":   ownership.AssetOwnershipID,
		"amount":               10,
		"price_per_stock":      50.0,
		"premium":              5.0,
		"settlement_date":      time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
		"buyer_account_number": "buyer-acc", // resolves to clientID=10
	}
	rec := performRequest(t, router, http.MethodPost, "/api/otc/offers", offerBody, clientAuthHeader(t, 10, 10))
	requireStatus(t, rec, http.StatusCreated)
	offer := decodeResponse[map[string]any](t, rec)
	offerID := uint(offer["otc_offer_id"].(float64))

	// Seller (identityID=20) accepts; "seller-acc" resolves to clientID=20
	acceptBody := map[string]any{"account_number": "seller-acc"}
	rec = performRequest(t, router, http.MethodPatch, fmt.Sprintf("/api/otc/offers/%d/accept", offerID), acceptBody, clientAuthHeader(t, 20, 20))
	requireStatus(t, rec, http.StatusCreated)

	contract := decodeResponse[map[string]any](t, rec)
	require.NotNil(t, contract["otc_option_contract_id"])
	assert.Equal(t, float64(10), contract["amount"])
	assert.Equal(t, float64(50.0), contract["strike_price"])

	var updatedOwnership model.AssetOwnership
	err := db.Where("user_id = ? AND owner_type = ? AND asset_id = ?", 20, model.OwnerTypeClient, asset.AssetID).
		First(&updatedOwnership).Error
	require.NoError(t, err)
	assert.Equal(t, float64(10), updatedOwnership.ReservedAmount)
}

func TestOTC_AcceptOffer_BuyerCannotAcceptOwnOffer(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	asset, _ := seedAssetAndStock(t, db, uniqueValue(t, "SELF2"))
	ownership := seedAssetOwnership(t, db, 20, model.OwnerTypeClient, asset.AssetID, 100)
	setPublicAmount(t, db, ownership.AssetOwnershipID, 100, 0)

	offerBody := map[string]any{
		"asset_ownership_id":   ownership.AssetOwnershipID,
		"amount":               10,
		"price_per_stock":      50.0,
		"premium":              5.0,
		"settlement_date":      time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
		"buyer_account_number": "buyer-acc", // resolves to clientID=10
	}
	rec := performRequest(t, router, http.MethodPost, "/api/otc/offers", offerBody, clientAuthHeader(t, 10, 10))
	requireStatus(t, rec, http.StatusCreated)
	offer := decodeResponse[map[string]any](t, rec)
	offerID := uint(offer["otc_offer_id"].(float64))

	// Buyer (identityID=10) tries to accept their own offer → bad request
	rec = performRequest(t, router, http.MethodPatch, fmt.Sprintf("/api/otc/offers/%d/accept", offerID), nil, clientAuthHeader(t, 10, 10))
	requireStatus(t, rec, http.StatusBadRequest)
}

func TestOTC_RejectOffer_Success(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	asset, _ := seedAssetAndStock(t, db, uniqueValue(t, "REJ"))
	ownership := seedAssetOwnership(t, db, 20, model.OwnerTypeClient, asset.AssetID, 100)
	setPublicAmount(t, db, ownership.AssetOwnershipID, 100, 0)

	offerBody := map[string]any{
		"asset_ownership_id":   ownership.AssetOwnershipID,
		"amount":               10,
		"price_per_stock":      50.0,
		"premium":              5.0,
		"settlement_date":      time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
		"buyer_account_number": "buyer-acc", // resolves to clientID=10
	}
	rec := performRequest(t, router, http.MethodPost, "/api/otc/offers", offerBody, clientAuthHeader(t, 10, 10))
	requireStatus(t, rec, http.StatusCreated)
	offer := decodeResponse[map[string]any](t, rec)
	offerID := uint(offer["otc_offer_id"].(float64))

	// Seller (identityID=20) rejects
	rec = performRequest(t, router, http.MethodPatch, fmt.Sprintf("/api/otc/offers/%d/reject", offerID), nil, clientAuthHeader(t, 20, 20))
	requireStatus(t, rec, http.StatusOK)

	rejected := decodeResponse[map[string]any](t, rec)
	assert.Equal(t, "REJECTED", rejected["status"])
}

func TestOTC_GetMyActiveOffers_ReturnsOnlyOwnOffers(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	asset, _ := seedAssetAndStock(t, db, uniqueValue(t, "ACTIVE"))
	ownership := seedAssetOwnership(t, db, 20, model.OwnerTypeClient, asset.AssetID, 100)
	setPublicAmount(t, db, ownership.AssetOwnershipID, 100, 0)

	for i := 0; i < 2; i++ {
		body := map[string]any{
			"asset_ownership_id":   ownership.AssetOwnershipID,
			"amount":               5,
			"price_per_stock":      50.0,
			"premium":              5.0,
			"settlement_date":      time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
			"buyer_account_number": "buyer-acc", // resolves to clientID=10
		}
		rec := performRequest(t, router, http.MethodPost, "/api/otc/offers", body, clientAuthHeader(t, 10, 10))
		requireStatus(t, rec, http.StatusCreated)
	}

	// Buyer (identityID=10) should see their 2 offers
	rec := performRequest(t, router, http.MethodGet, "/api/otc/offers/active", nil, clientAuthHeader(t, 10, 10))
	requireStatus(t, rec, http.StatusOK)
	offers := decodeResponse[[]map[string]any](t, rec)
	assert.Len(t, offers, 2)

	// Unrelated user (identityID=99) should see none
	rec = performRequest(t, router, http.MethodGet, "/api/otc/offers/active", nil, clientAuthHeader(t, 99, 99))
	requireStatus(t, rec, http.StatusOK)
	other := decodeResponse[[]map[string]any](t, rec)
	assert.Empty(t, other)
}

func TestOTC_GetMyOptionContracts_AfterAccept(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	asset, _ := seedAssetAndStock(t, db, uniqueValue(t, "CTCT"))
	// Seller identity = 20
	ownership := seedAssetOwnership(t, db, 20, model.OwnerTypeClient, asset.AssetID, 100)
	setPublicAmount(t, db, ownership.AssetOwnershipID, 100, 0)

	offerBody := map[string]any{
		"asset_ownership_id":   ownership.AssetOwnershipID,
		"amount":               10,
		"price_per_stock":      50.0,
		"premium":              5.0,
		"settlement_date":      time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
		"buyer_account_number": "buyer-acc", // resolves to clientID=10
	}
	rec := performRequest(t, router, http.MethodPost, "/api/otc/offers", offerBody, clientAuthHeader(t, 10, 10))
	requireStatus(t, rec, http.StatusCreated)
	offer := decodeResponse[map[string]any](t, rec)
	offerID := uint(offer["otc_offer_id"].(float64))

	// Seller (identityID=20) accepts; "seller-acc" resolves to clientID=20
	acceptBody := map[string]any{"account_number": "seller-acc"}
	rec = performRequest(t, router, http.MethodPatch, fmt.Sprintf("/api/otc/offers/%d/accept", offerID), acceptBody, clientAuthHeader(t, 20, 20))
	requireStatus(t, rec, http.StatusCreated)

	// Both buyer and seller should each see 1 contract
	rec = performRequest(t, router, http.MethodGet, "/api/otc/contracts", nil, clientAuthHeader(t, 10, 10))
	requireStatus(t, rec, http.StatusOK)
	contracts := decodeResponse[[]map[string]any](t, rec)
	assert.Len(t, contracts, 1)

	rec = performRequest(t, router, http.MethodGet, "/api/otc/contracts", nil, clientAuthHeader(t, 20, 20))
	requireStatus(t, rec, http.StatusOK)
	contracts = decodeResponse[[]map[string]any](t, rec)
	assert.Len(t, contracts, 1)
}

// --- Publish endpoint tests (main branch) ---

func TestOTCHandler_PublishAsset_ClientSuccess(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, uniqueMIC(t))
	listing := seedListing(t, db, uniqueTicker(t), ex.MicCode, model.AssetTypeStock, 100.0)
	ownership := seedAssetOwnership(t, db, 50, model.OwnerTypeClient, listing.AssetID, 20)

	path := fmt.Sprintf("/api/client/50/assets/%d/publish", ownership.AssetOwnershipID)
	rec := performRequest(t, router, http.MethodPatch, path, map[string]any{"amount": 5}, authHeaderForClient(t, 50, 50))
	requireStatus(t, rec, http.StatusNoContent)
}

func TestOTCHandler_PublishAsset_ActuarySuccess(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, uniqueMIC(t))
	listing := seedListing(t, db, uniqueTicker(t), ex.MicCode, model.AssetTypeStock, 50.0)
	ownership := seedAssetOwnership(t, db, 20, model.OwnerTypeActuary, listing.AssetID, 15)

	path := fmt.Sprintf("/api/actuary/20/assets/%d/publish", ownership.AssetOwnershipID)
	rec := performRequest(t, router, http.MethodPatch, path, map[string]any{"amount": 3}, authHeaderForAgent(t))
	requireStatus(t, rec, http.StatusNoContent)
}

func TestOTCHandler_PublishAsset_Unauthenticated(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	rec := performRequest(t, router, http.MethodPatch, "/api/client/50/assets/1/publish", map[string]any{"amount": 5}, "")
	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestOTCHandler_PublishAsset_WrongOwner(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, uniqueMIC(t))
	listing := seedListing(t, db, uniqueTicker(t), ex.MicCode, model.AssetTypeStock, 100.0)
	ownership := seedAssetOwnership(t, db, 50, model.OwnerTypeClient, listing.AssetID, 20)

	path := fmt.Sprintf("/api/client/99/assets/%d/publish", ownership.AssetOwnershipID)
	rec := performRequest(t, router, http.MethodPatch, path, map[string]any{"amount": 5}, authHeaderForClient(t, 99, 99))
	requireStatus(t, rec, http.StatusForbidden)
}

func TestOTCHandler_PublishAsset_NotFound(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	rec := performRequest(t, router, http.MethodPatch, "/api/client/50/assets/99999/publish", map[string]any{"amount": 1}, authHeaderForClient(t, 50, 50))
	requireStatus(t, rec, http.StatusNotFound)
}

func TestOTCHandler_PublishAsset_AmountExceedsOwned(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, uniqueMIC(t))
	listing := seedListing(t, db, uniqueTicker(t), ex.MicCode, model.AssetTypeStock, 100.0)
	ownership := seedAssetOwnership(t, db, 50, model.OwnerTypeClient, listing.AssetID, 10)

	path := fmt.Sprintf("/api/client/50/assets/%d/publish", ownership.AssetOwnershipID)
	rec := performRequest(t, router, http.MethodPatch, path, map[string]any{"amount": 9999}, authHeaderForClient(t, 50, 50))
	requireStatus(t, rec, http.StatusBadRequest)
}

func TestOTCHandler_PublishAsset_UpdatesExistingPublicAmount(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, uniqueMIC(t))
	listing := seedListing(t, db, uniqueTicker(t), ex.MicCode, model.AssetTypeStock, 100.0)
	ownership := seedAssetOwnership(t, db, 50, model.OwnerTypeClient, listing.AssetID, 20)
	setPublicAmount(t, db, ownership.AssetOwnershipID, 3, 0)

	path := fmt.Sprintf("/api/client/50/assets/%d/publish", ownership.AssetOwnershipID)
	rec := performRequest(t, router, http.MethodPatch, path, map[string]any{"amount": 8}, authHeaderForClient(t, 50, 50))
	requireStatus(t, rec, http.StatusNoContent)

	var updated model.AssetOwnership
	require.NoError(t, db.First(&updated, ownership.AssetOwnershipID).Error)
	require.Equal(t, float64(11), updated.PublicAmount)
}

// --- GetPublicOTCAssets tests (main branch) ---

func TestOTCHandler_GetPublicOTCAssets_ReturnsList(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, uniqueMIC(t))
	ticker := uniqueTicker(t)
	listing := seedListing(t, db, ticker, ex.MicCode, model.AssetTypeStock, 120.0)
	ownership := seedAssetOwnership(t, db, 50, model.OwnerTypeClient, listing.AssetID, 10)
	setPublicAmount(t, db, ownership.AssetOwnershipID, 6, 1)

	rec := performRequest(t, router, http.MethodGet, "/api/otc/public?page=1&page_size=10", nil, authHeaderForClient(t, 50, 50))
	requireStatus(t, rec, http.StatusOK)

	body := decodeResponse[map[string]interface{}](t, rec)
	data, ok := body["data"].([]interface{})
	require.True(t, ok)
	require.GreaterOrEqual(t, len(data), 1)

	entry := data[0].(map[string]interface{})
	require.Equal(t, float64(5), entry["available_amount"]) // 6 - 1
	require.NotEmpty(t, entry["ticker"])
	require.NotEmpty(t, entry["name"])
	require.NotEmpty(t, entry["security_type"])
}

func TestOTCHandler_GetPublicOTCAssets_UnpublishedNotIncluded(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, uniqueMIC(t))
	ticker := uniqueTicker(t)
	listing := seedListing(t, db, ticker, ex.MicCode, model.AssetTypeStock, 100.0)
	ownership := seedAssetOwnership(t, db, 50, model.OwnerTypeClient, listing.AssetID, 10)
	setPublicAmount(t, db, ownership.AssetOwnershipID, 0, 0)

	rec := performRequest(t, router, http.MethodGet, "/api/otc/public?page=1&page_size=10", nil, authHeaderForClient(t, 50, 50))
	requireStatus(t, rec, http.StatusOK)

	body := decodeResponse[map[string]interface{}](t, rec)
	data := body["data"].([]interface{})
	for _, item := range data {
		entry := item.(map[string]interface{})
		require.NotEqual(t, ticker, entry["ticker"])
	}
}

func TestOTCHandler_GetPublicOTCAssets_Unauthenticated(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	rec := performRequest(t, router, http.MethodGet, "/api/otc/public", nil, "")
	requireStatus(t, rec, http.StatusUnauthorized)
}
