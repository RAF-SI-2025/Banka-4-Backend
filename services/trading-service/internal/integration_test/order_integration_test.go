//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

func TestCreateOrder(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, "XNYS")
	listing := seedListing(t, db, "AAPL", ex.MicCode, model.AssetTypeStock, 150.0)
	seedStock(t, db, listing.ListingID)

	auth := authHeaderForSupervisor(t)

	body := map[string]any{
		"listing_id":     listing.ListingID,
		"order_type":     "MARKET",
		"direction":      "BUY",
		"quantity":       5,
		"account_number": "444000100000000001",
	}

	rec := performRequest(t, router, http.MethodPost, "/api/orders", body, auth)
	requireStatus(t, rec, http.StatusCreated)

	resp := decodeResponse[map[string]any](t, rec)
	require.Equal(t, "MARKET", resp["order_type"])
	require.Equal(t, "BUY", resp["direction"])
	require.Equal(t, float64(5), resp["quantity"])
}

func TestCreateOrder_LimitOrder(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, "XNAS")
	listing := seedListing(t, db, "MSFT", ex.MicCode, model.AssetTypeStock, 400.0)
	seedStock(t, db, listing.ListingID)
	// supervisor EmployeeID=10, ownerType=ACTUARY
	seedAssetOwnership(t, db, 10, model.OwnerTypeActuary, listing.AssetID, 20)

	auth := authHeaderForSupervisor(t)

	body := map[string]any{
		"listing_id":     listing.ListingID,
		"order_type":     "LIMIT",
		"direction":      "SELL",
		"quantity":       10,
		"limit_value":    405.0,
		"account_number": "444000100000000001",
	}

	rec := performRequest(t, router, http.MethodPost, "/api/orders", body, auth)
	requireStatus(t, rec, http.StatusCreated)

	resp := decodeResponse[map[string]any](t, rec)
	require.Equal(t, "LIMIT", resp["order_type"])
}

func TestCreateOrder_Unauthorized(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	body := map[string]any{
		"listing_id":     1,
		"order_type":     "MARKET",
		"direction":      "BUY",
		"quantity":       5,
		"account_number": "444000100000000001",
	}

	rec := performRequest(t, router, http.MethodPost, "/api/orders", body, "")
	require.NotEqual(t, http.StatusCreated, rec.Code)
}

func TestCreateOrder_InvalidBody(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForSupervisor(t)

	rec := performRequest(t, router, http.MethodPost, "/api/orders", map[string]any{}, auth)
	require.NotEqual(t, http.StatusCreated, rec.Code)
}

func TestGetOrders_ForbiddenForNonSupervisor(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForAgent(t)

	rec := performRequest(t, router, http.MethodGet, "/api/orders", nil, auth)
	require.NotEqual(t, http.StatusOK, rec.Code)
}

func TestApproveOrder(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, "XNYS")
	listing := seedListing(t, db, "META", ex.MicCode, model.AssetTypeStock, 500.0)
	seedStock(t, db, listing.ListingID)
	order := seedOrder(t, db, 20, listing.ListingID, model.OrderDirectionBuy, model.OrderStatusPending)

	auth := authHeaderForSupervisor(t)

	rec := performRequest(t, router, http.MethodPatch, fmt.Sprintf("/api/orders/%d/approve", order.OrderID), nil, auth)
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[map[string]any](t, rec)
	require.Equal(t, "APPROVED", resp["status"])
}

func TestApproveOrder_NotFound(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForSupervisor(t)

	rec := performRequest(t, router, http.MethodPatch, "/api/orders/99999/approve", nil, auth)
	require.NotEqual(t, http.StatusOK, rec.Code)
}

func TestApproveOrder_InvalidID(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForSupervisor(t)

	rec := performRequest(t, router, http.MethodPatch, "/api/orders/abc/approve", nil, auth)
	require.NotEqual(t, http.StatusOK, rec.Code)
}

func TestDeclineOrder(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, "XNAS")
	listing := seedListing(t, db, "AMZN", ex.MicCode, model.AssetTypeStock, 180.0)
	seedStock(t, db, listing.ListingID)
	order := seedOrder(t, db, 20, listing.ListingID, model.OrderDirectionSell, model.OrderStatusPending)

	auth := authHeaderForSupervisor(t)

	rec := performRequest(t, router, http.MethodPatch, fmt.Sprintf("/api/orders/%d/decline", order.OrderID), nil, auth)
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[map[string]any](t, rec)
	require.Equal(t, "DECLINED", resp["status"])
}

func TestCancelOrder(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, "XNYS")
	listing := seedListing(t, db, "NFLX", ex.MicCode, model.AssetTypeStock, 600.0)
	seedStock(t, db, listing.ListingID)
	order := seedOrder(t, db, 10, listing.ListingID, model.OrderDirectionBuy, model.OrderStatusApproved)

	auth := authHeaderForSupervisor(t)

	rec := performRequest(t, router, http.MethodPatch, fmt.Sprintf("/api/orders/%d/cancel", order.OrderID), nil, auth)
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[map[string]any](t, rec)
	require.Equal(t, true, resp["is_done"])
}

func TestCancelOrder_AlreadyDeclined(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, "XNYS")
	listing := seedListing(t, db, "DIS", ex.MicCode, model.AssetTypeStock, 100.0)
	seedStock(t, db, listing.ListingID)
	order := seedOrder(t, db, 10, listing.ListingID, model.OrderDirectionBuy, model.OrderStatusDeclined)

	auth := authHeaderForSupervisor(t)

	rec := performRequest(t, router, http.MethodPatch, fmt.Sprintf("/api/orders/%d/cancel", order.OrderID), nil, auth)
	require.NotEqual(t, http.StatusOK, rec.Code)
}

// ── POST /api/orders/invest ───────────────────────────────────────

func TestCreateFundOrder_BuyMarket_Success(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, "XNYS")
	listing := seedListing(t, db, "GOOG", ex.MicCode, model.AssetTypeStock, 170.0)
	seedStock(t, db, listing.ListingID)
	fund := seedInvestmentFund(t, db, "Alpha Fund", 10) // supervisor EmployeeID=10

	auth := authHeaderForSupervisor(t)

	body := map[string]any{
		"fund_id":    fund.FundID,
		"listing_id": listing.ListingID,
		"order_type": "MARKET",
		"direction":  "BUY",
		"quantity":   5,
	}

	rec := performRequest(t, router, http.MethodPost, "/api/orders/invest", body, auth)
	requireStatus(t, rec, http.StatusCreated)

	resp := decodeResponse[map[string]any](t, rec)
	require.Equal(t, "MARKET", resp["order_type"])
	require.Equal(t, "BUY", resp["direction"])
	require.Equal(t, float64(5), resp["quantity"])

	// asset owner should be the fund, not the supervisor
	var order model.Order
	require.NoError(t, db.Last(&order).Error)
	require.Equal(t, fund.FundID, order.AssetOwnerUserID)
	require.Equal(t, model.OwnerTypeFund, order.AssetOwnerType)
	require.Equal(t, uint(10), order.OrderOwnerUserID)
	require.Equal(t, model.OwnerTypeActuary, order.OrderOwnerType)
	require.Equal(t, fund.AccountNumber, order.AccountNumber)
}

func TestCreateFundOrder_NotSupervisor_Forbidden(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, "XNYS")
	listing := seedListing(t, db, "AMZN", ex.MicCode, model.AssetTypeStock, 180.0)
	seedStock(t, db, listing.ListingID)
	fund := seedInvestmentFund(t, db, "Beta Fund", 10)

	body := map[string]any{
		"fund_id":    fund.FundID,
		"listing_id": listing.ListingID,
		"order_type": "MARKET",
		"direction":  "BUY",
		"quantity":   1,
	}

	// agent (EmployeeID=20) is not a supervisor
	rec := performRequest(t, router, http.MethodPost, "/api/orders/invest", body, authHeaderForAgent(t))
	requireStatus(t, rec, http.StatusForbidden)
}

func TestCreateFundOrder_Unauthorized(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	body := map[string]any{
		"fund_id":    1,
		"listing_id": 1,
		"order_type": "MARKET",
		"direction":  "BUY",
		"quantity":   1,
	}

	rec := performRequest(t, router, http.MethodPost, "/api/orders/invest", body, "")
	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestCreateFundOrder_FundNotFound(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, "XNYS")
	listing := seedListing(t, db, "NFLX", ex.MicCode, model.AssetTypeStock, 500.0)
	seedStock(t, db, listing.ListingID)

	body := map[string]any{
		"fund_id":    99999,
		"listing_id": listing.ListingID,
		"order_type": "MARKET",
		"direction":  "BUY",
		"quantity":   1,
	}

	rec := performRequest(t, router, http.MethodPost, "/api/orders/invest", body, authHeaderForSupervisor(t))
	requireStatus(t, rec, http.StatusNotFound)
}

func TestCreateFundOrder_NotFundManager_Forbidden(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, "XNYS")
	listing := seedListing(t, db, "META", ex.MicCode, model.AssetTypeStock, 500.0)
	seedStock(t, db, listing.ListingID)
	// fund manager is EmployeeID=99, but supervisor token is EmployeeID=10
	fund := seedInvestmentFund(t, db, "Gamma Fund", 99)

	body := map[string]any{
		"fund_id":    fund.FundID,
		"listing_id": listing.ListingID,
		"order_type": "MARKET",
		"direction":  "BUY",
		"quantity":   1,
	}

	rec := performRequest(t, router, http.MethodPost, "/api/orders/invest", body, authHeaderForSupervisor(t))
	requireStatus(t, rec, http.StatusForbidden)
}

func TestCreateFundOrder_LimitOrder_MissingLimitValue(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, "XNYS")
	listing := seedListing(t, db, "TSLA", ex.MicCode, model.AssetTypeStock, 200.0)
	seedStock(t, db, listing.ListingID)
	fund := seedInvestmentFund(t, db, "Delta Fund", 10)

	body := map[string]any{
		"fund_id":    fund.FundID,
		"listing_id": listing.ListingID,
		"order_type": "LIMIT",
		"direction":  "BUY",
		"quantity":   1,
		// limit_value intentionally omitted
	}

	rec := performRequest(t, router, http.MethodPost, "/api/orders/invest", body, authHeaderForSupervisor(t))
	requireStatus(t, rec, http.StatusBadRequest)
}
