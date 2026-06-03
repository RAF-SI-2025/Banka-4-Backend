//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

func seedRecurringOrder(t *testing.T, db *gorm.DB, userID uint, ownerType model.OwnerType, listingID uint, cadence model.RecurringOrderCadence, active bool) *model.RecurringOrder {
	t.Helper()

	ro := &model.RecurringOrder{
		UserID:        userID,
		OwnerType:     ownerType,
		ListingID:     listingID,
		Direction:     model.OrderDirectionBuy,
		Mode:          model.RecurringOrderModeByAmount,
		Value:         500.0,
		AccountNumber: "444000100000000001",
		Cadence:       cadence,
		NextRun:       time.Now().AddDate(0, 0, 1),
		Active:        active,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := db.Create(ro).Error; err != nil {
		t.Fatalf("seed recurring order: %v", err)
	}
	return ro
}

// ── GET /api/recurring-orders ─────────────────────────────────────

func TestGetMyRecurringOrders_Client_ReturnsOwn(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, uniqueValue(t, "XREC"))
	listing := seedListing(t, db, uniqueValue(t, "REC"), ex.MicCode, model.AssetTypeStock, 100.0)

	clientID := uint(42)
	seedRecurringOrder(t, db, clientID, model.OwnerTypeClient, listing.ListingID, model.RecurringOrderCadenceDaily, true)
	seedRecurringOrder(t, db, clientID, model.OwnerTypeClient, listing.ListingID, model.RecurringOrderCadenceWeekly, false)

	auth := authHeaderForClient(t, 42, clientID)
	rec := performRequest(t, router, http.MethodGet, "/api/recurring-orders", nil, auth)
	requireStatus(t, rec, http.StatusOK)

	var resp []map[string]any
	resp = decodeResponse[[]map[string]any](t, rec)
	require.Len(t, resp, 2)
}

func TestGetMyRecurringOrders_Client_DoesNotSeeOthers(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, uniqueValue(t, "XREC"))
	listing := seedListing(t, db, uniqueValue(t, "REC"), ex.MicCode, model.AssetTypeStock, 100.0)

	seedRecurringOrder(t, db, 99, model.OwnerTypeClient, listing.ListingID, model.RecurringOrderCadenceDaily, true)

	clientID := uint(55)
	auth := authHeaderForClient(t, 55, clientID)
	rec := performRequest(t, router, http.MethodGet, "/api/recurring-orders", nil, auth)
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[[]map[string]any](t, rec)
	require.Empty(t, resp)
}

func TestGetMyRecurringOrders_Unauthorized(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	rec := performRequest(t, router, http.MethodGet, "/api/recurring-orders", nil, "")
	requireStatus(t, rec, http.StatusUnauthorized)
}

// ── POST /api/recurring-orders ────────────────────────────────────

func TestCreateRecurringOrder_Client_Success(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, uniqueValue(t, "XREC"))
	listing := seedListing(t, db, uniqueValue(t, "REC"), ex.MicCode, model.AssetTypeStock, 100.0)

	clientID := uint(60)
	auth := authHeaderForClient(t, 60, clientID)

	body := map[string]any{
		"listing_id":     listing.ListingID,
		"account_number": "444000100000000001",
		"direction":      "BUY",
		"mode":           "BY_AMOUNT",
		"value":          250.0,
		"cadence":        "WEEKLY",
	}
	rec := performRequest(t, router, http.MethodPost, "/api/recurring-orders", body, auth)
	requireStatus(t, rec, http.StatusCreated)

	resp := decodeResponse[map[string]any](t, rec)
	require.Equal(t, "WEEKLY", resp["cadence"])
	require.Equal(t, "BY_AMOUNT", resp["mode"])
	require.Equal(t, true, resp["active"])
}

func TestCreateRecurringOrder_InvalidBody(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	clientID := uint(61)
	auth := authHeaderForClient(t, 61, clientID)

	body := map[string]any{
		"direction": "BUY",
	}
	rec := performRequest(t, router, http.MethodPost, "/api/recurring-orders", body, auth)
	requireStatus(t, rec, http.StatusBadRequest)
}

func TestCreateRecurringOrder_ListingNotFound(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	clientID := uint(62)
	auth := authHeaderForClient(t, 62, clientID)

	body := map[string]any{
		"listing_id":     999999,
		"account_number": "444000100000000001",
		"direction":      "BUY",
		"mode":           "BY_AMOUNT",
		"value":          100.0,
		"cadence":        "DAILY",
	}
	rec := performRequest(t, router, http.MethodPost, "/api/recurring-orders", body, auth)
	requireStatus(t, rec, http.StatusNotFound)
}

func TestCreateRecurringOrder_Unauthorized(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	rec := performRequest(t, router, http.MethodPost, "/api/recurring-orders", map[string]any{}, "")
	requireStatus(t, rec, http.StatusUnauthorized)
}

// ── DELETE /api/recurring-orders/:id ─────────────────────────────

func TestDeleteRecurringOrder_Owner_Success(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, uniqueValue(t, "XREC"))
	listing := seedListing(t, db, uniqueValue(t, "REC"), ex.MicCode, model.AssetTypeStock, 100.0)

	clientID := uint(70)
	ro := seedRecurringOrder(t, db, clientID, model.OwnerTypeClient, listing.ListingID, model.RecurringOrderCadenceDaily, true)

	auth := authHeaderForClient(t, 70, clientID)
	rec := performRequest(t, router, http.MethodDelete, fmt.Sprintf("/api/recurring-orders/%d", ro.RecurringOrderID), nil, auth)
	requireStatus(t, rec, http.StatusNoContent)
}

func TestDeleteRecurringOrder_NotOwner_Forbidden(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, uniqueValue(t, "XREC"))
	listing := seedListing(t, db, uniqueValue(t, "REC"), ex.MicCode, model.AssetTypeStock, 100.0)

	ro := seedRecurringOrder(t, db, 99, model.OwnerTypeClient, listing.ListingID, model.RecurringOrderCadenceDaily, true)

	clientID := uint(71)
	auth := authHeaderForClient(t, 71, clientID)
	rec := performRequest(t, router, http.MethodDelete, fmt.Sprintf("/api/recurring-orders/%d", ro.RecurringOrderID), nil, auth)
	requireStatus(t, rec, http.StatusForbidden)
}

func TestDeleteRecurringOrder_NotFound(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	clientID := uint(72)
	auth := authHeaderForClient(t, 72, clientID)
	rec := performRequest(t, router, http.MethodDelete, "/api/recurring-orders/999999", nil, auth)
	requireStatus(t, rec, http.StatusNotFound)
}

func TestDeleteRecurringOrder_InvalidID(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	clientID := uint(73)
	auth := authHeaderForClient(t, 73, clientID)
	rec := performRequest(t, router, http.MethodDelete, "/api/recurring-orders/abc", nil, auth)
	requireStatus(t, rec, http.StatusBadRequest)
}

// ── PATCH /api/recurring-orders/:id/pause ────────────────────────

func TestPauseRecurringOrder_Owner_TogglesActive(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, uniqueValue(t, "XREC"))
	listing := seedListing(t, db, uniqueValue(t, "REC"), ex.MicCode, model.AssetTypeStock, 100.0)

	clientID := uint(80)
	ro := seedRecurringOrder(t, db, clientID, model.OwnerTypeClient, listing.ListingID, model.RecurringOrderCadenceWeekly, true)

	auth := authHeaderForClient(t, 80, clientID)
	rec := performRequest(t, router, http.MethodPatch, fmt.Sprintf("/api/recurring-orders/%d/pause", ro.RecurringOrderID), nil, auth)
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[map[string]any](t, rec)
	require.Equal(t, false, resp["active"])
}

func TestPauseRecurringOrder_NotOwner_Forbidden(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, uniqueValue(t, "XREC"))
	listing := seedListing(t, db, uniqueValue(t, "REC"), ex.MicCode, model.AssetTypeStock, 100.0)

	ro := seedRecurringOrder(t, db, 99, model.OwnerTypeClient, listing.ListingID, model.RecurringOrderCadenceWeekly, true)

	clientID := uint(81)
	auth := authHeaderForClient(t, 81, clientID)
	rec := performRequest(t, router, http.MethodPatch, fmt.Sprintf("/api/recurring-orders/%d/pause", ro.RecurringOrderID), nil, auth)
	requireStatus(t, rec, http.StatusForbidden)
}

func TestPauseRecurringOrder_NotFound(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	clientID := uint(82)
	auth := authHeaderForClient(t, 82, clientID)
	rec := performRequest(t, router, http.MethodPatch, "/api/recurring-orders/999999/pause", nil, auth)
	requireStatus(t, rec, http.StatusNotFound)
}
