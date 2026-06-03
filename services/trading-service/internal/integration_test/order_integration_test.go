//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

func TestGetOrders_Success(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, "XNYS")
	listing := seedListing(t, db, "AAPL", ex.MicCode, model.AssetTypeStock, 150.0)
	seedStock(t, db, listing.ListingID)
	seedOrder(t, db, 10, listing.ListingID, model.OrderDirectionBuy, model.OrderStatusPending)

	auth := authHeaderForSupervisor(t)

	rec := performRequest(t, router, http.MethodGet, "/api/orders?page=1&page_size=10", nil, auth)
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[map[string]any](t, rec)
	require.NotNil(t, resp["data"])
}

func TestGetOrders_DefaultPagination(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForSupervisor(t)

	rec := performRequest(t, router, http.MethodGet, "/api/orders", nil, auth)
	requireStatus(t, rec, http.StatusOK)
}

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
	// supervisor EmployeeID=10, ownerType=BANK
	seedAssetOwnership(t, db, 10, model.OwnerTypeBank, listing.AssetID, 20)

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
	require.Equal(t, model.OwnerTypeBank, order.OrderOwnerType)
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

func TestGetOrders_ForbiddenForClient(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForClient(t, 50, 1)

	rec := performRequest(t, router, http.MethodGet, "/api/orders", nil, auth)
	require.NotEqual(t, http.StatusOK, rec.Code)
}

func TestGetOrders_Unauthorized(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	rec := performRequest(t, router, http.MethodGet, "/api/orders", nil, "")
	require.NotEqual(t, http.StatusOK, rec.Code)
}

func TestGetMyOrders_Client_Success(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	// Seed listing (stock)
	ex := seedExchange(t, db, "XNYS")
	listing := seedListing(t, db, "AAPL", ex.MicCode, model.AssetTypeStock, 150.0)
	seedStock(t, db, listing.ListingID)

	// Create orders for client with ID 10
	seedOrderForUser(t, db, 10, model.OwnerTypeClient, listing.ListingID, model.OrderDirectionBuy, model.OrderStatusPending)
	seedOrderForUser(t, db, 10, model.OwnerTypeClient, listing.ListingID, model.OrderDirectionSell, model.OrderStatusApproved)

	auth := authHeaderForClient(t, 10, 10) // identityID=10, clientID=10
	rec := performRequest(t, router, http.MethodGet, "/api/orders/my?page=1&page_size=10", nil, auth)
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[map[string]any](t, rec)
	data := resp["data"].([]interface{})
	require.Len(t, data, 2)

	total := resp["total"].(float64)
	require.Equal(t, float64(2), total)
}

func TestGetMyOrders_Employee_Success(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, "XNYS")
	listing := seedListing(t, db, "MSFT", ex.MicCode, model.AssetTypeStock, 400.0)
	seedStock(t, db, listing.ListingID)

	// Create orders for employee with identityID=20 (EmployeeID=20)
	seedOrderForUser(t, db, 20, model.OwnerTypeBank, listing.ListingID, model.OrderDirectionBuy, model.OrderStatusPending)

	auth := authHeaderForAgent(t) // agent has EmployeeID=20
	rec := performRequest(t, router, http.MethodGet, "/api/orders/my?page=1&page_size=10", nil, auth)
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[map[string]any](t, rec)
	data := resp["data"].([]interface{})
	require.Len(t, data, 1)
}

func TestGetMyOrders_FilterByStatus(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, "XNYS")
	listing := seedListing(t, db, "AAPL", ex.MicCode, model.AssetTypeStock, 150.0)
	seedStock(t, db, listing.ListingID)

	seedOrderForUser(t, db, 10, model.OwnerTypeClient, listing.ListingID, model.OrderDirectionBuy, model.OrderStatusPending)
	seedOrderForUser(t, db, 10, model.OwnerTypeClient, listing.ListingID, model.OrderDirectionBuy, model.OrderStatusApproved)

	auth := authHeaderForClient(t, 10, 10)
	rec := performRequest(t, router, http.MethodGet, "/api/orders/my?status=PENDING&page=1&page_size=10", nil, auth)
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[map[string]any](t, rec)
	data := resp["data"].([]interface{})
	require.Len(t, data, 1)

	order := data[0].(map[string]interface{})
	require.Equal(t, "PENDING", order["status"])
}

func TestGetMyOrders_FilterByOrderType(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, "XNYS")
	listing := seedListing(t, db, "AAPL", ex.MicCode, model.AssetTypeStock, 150.0)
	seedStock(t, db, listing.ListingID)

	// Order with type MARKET
	seedOrderWithType(t, db, 10, model.OwnerTypeClient, listing.ListingID, model.OrderTypeMarket, model.OrderDirectionBuy, model.OrderStatusPending)
	// Order with type LIMIT
	seedOrderWithType(t, db, 10, model.OwnerTypeClient, listing.ListingID, model.OrderTypeLimit, model.OrderDirectionBuy, model.OrderStatusPending)

	auth := authHeaderForClient(t, 10, 10)
	rec := performRequest(t, router, http.MethodGet, "/api/orders/my?order_type=LIMIT&page=1&page_size=10", nil, auth)
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[map[string]any](t, rec)
	data := resp["data"].([]interface{})
	require.Len(t, data, 1)

	order := data[0].(map[string]interface{})
	require.Equal(t, "LIMIT", order["order_type"])
}

func TestGetMyOrders_FilterByAssetType(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	// Stock listing
	exStock := seedExchange(t, db, "XNYS")
	stockListing := seedListing(t, db, "AAPL", exStock.MicCode, model.AssetTypeStock, 150.0)
	seedStock(t, db, stockListing.ListingID)

	// Forex listing (needs asset with type forexPair)
	exForex := seedExchange(t, db, "FOREX")
	forexListing := seedListing(t, db, "EUR/USD", exForex.MicCode, model.AssetTypeForexPair, 1.2)
	// ForexPair may not need additional seed, but asset already has type

	seedOrderForUser(t, db, 10, model.OwnerTypeClient, stockListing.ListingID, model.OrderDirectionBuy, model.OrderStatusPending)
	seedOrderForUser(t, db, 10, model.OwnerTypeClient, forexListing.ListingID, model.OrderDirectionBuy, model.OrderStatusPending)

	auth := authHeaderForClient(t, 10, 10)
	rec := performRequest(t, router, http.MethodGet, "/api/orders/my?asset_type=stock&page=1&page_size=10", nil, auth)
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[map[string]any](t, rec)
	data := resp["data"].([]interface{})
	require.Len(t, data, 1)

	order := data[0].(map[string]interface{})
	require.Equal(t, "AAPL", order["ticker"])
}

func TestGetMyOrders_FilterByDateRange(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, "XNYS")
	listing := seedListing(t, db, "AAPL", ex.MicCode, model.AssetTypeStock, 150.0)
	seedStock(t, db, listing.ListingID)

	// Create two orders for the client (userID 10)
	clientID := uint(10)
	ownerType := model.OwnerTypeClient
	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1)
	tomorrow := now.AddDate(0, 0, 1)

	// Order created today (should be in range)
	orderToday := seedOrderWithCustomDate(t, db, clientID, ownerType, listing.ListingID, model.OrderDirectionBuy, model.OrderStatusPending, now)
	// Order created 5 days ago (should be out of range)
	orderOld := seedOrderWithCustomDate(t, db, clientID, ownerType, listing.ListingID, model.OrderDirectionBuy, model.OrderStatusPending, now.AddDate(0, 0, -5))
	_ = orderOld // to avoid unused variable warning

	auth := authHeaderForClient(t, 10, 10)
	from := yesterday.Format("2006-01-02")
	to := tomorrow.Format("2006-01-02")
	url := fmt.Sprintf("/api/orders/my?from_date=%s&to_date=%s&page=1&page_size=10", from, to)
	rec := performRequest(t, router, http.MethodGet, url, nil, auth)
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[map[string]any](t, rec)
	data := resp["data"].([]interface{})
	require.Len(t, data, 1) // Only the order from today should be returned

	// Optional: verify it's the correct order
	orderMap := data[0].(map[string]interface{})
	require.Equal(t, float64(orderToday.OrderID), orderMap["order_id"])
}

func TestGetMyOrders_Pagination(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	ex := seedExchange(t, db, "XNYS")
	listing := seedListing(t, db, "AAPL", ex.MicCode, model.AssetTypeStock, 150.0)
	seedStock(t, db, listing.ListingID)

	// Create 3 orders
	for i := 0; i < 3; i++ {
		seedOrderForUser(t, db, 10, model.OwnerTypeClient, listing.ListingID, model.OrderDirectionBuy, model.OrderStatusPending)
	}

	auth := authHeaderForClient(t, 10, 10)

	// First page, size 2
	rec1 := performRequest(t, router, http.MethodGet, "/api/orders/my?page=1&page_size=2", nil, auth)
	requireStatus(t, rec1, http.StatusOK)
	resp1 := decodeResponse[map[string]any](t, rec1)
	data1 := resp1["data"].([]interface{})
	require.Len(t, data1, 2)
	require.Equal(t, float64(3), resp1["total"])
	require.Equal(t, float64(1), resp1["page"])
	require.Equal(t, float64(2), resp1["page_size"])

	// Second page, size 2
	rec2 := performRequest(t, router, http.MethodGet, "/api/orders/my?page=2&page_size=2", nil, auth)
	requireStatus(t, rec2, http.StatusOK)
	resp2 := decodeResponse[map[string]any](t, rec2)
	data2 := resp2["data"].([]interface{})
	require.Len(t, data2, 1) // third order
}

// TestGetMyOrders_Unauthorized – no token returns 401
func TestGetMyOrders_Unauthorized(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	rec := performRequest(t, router, http.MethodGet, "/api/orders/my", nil, "")
	requireStatus(t, rec, http.StatusUnauthorized)
}
