//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateFund_Success(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForSupervisor(t)

	body := map[string]any{
		"name":                 "Alpha Growth Fund",
		"description":          "Fund focused on the IT sector.",
		"minimum_contribution": 1000.0,
	}

	rec := performRequest(t, router, http.MethodPost, "/api/investment-funds", body, auth)
	requireStatus(t, rec, http.StatusCreated)

	resp := decodeResponse[map[string]any](t, rec)
	require.Equal(t, "Alpha Growth Fund", resp["name"])
	require.Equal(t, "Fund focused on the IT sector.", resp["description"])
	require.Equal(t, 1000.0, resp["minimum_contribution"])
	require.NotEmpty(t, resp["account_number"])
	require.Equal(t, float64(10), resp["manager_id"])
}

func TestCreateFund_Unauthorized(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	body := map[string]any{
		"name":                 "Unauthorized Fund",
		"description":          "Should fail.",
		"minimum_contribution": 500.0,
	}

	rec := performRequest(t, router, http.MethodPost, "/api/investment-funds", body, "")
	require.NotEqual(t, http.StatusCreated, rec.Code)
}

func TestCreateFund_ForbiddenForAgent(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForAgent(t)

	body := map[string]any{
		"name":                 "Agent Fund",
		"description":          "Should be forbidden.",
		"minimum_contribution": 500.0,
	}

	rec := performRequest(t, router, http.MethodPost, "/api/investment-funds", body, auth)
	require.NotEqual(t, http.StatusCreated, rec.Code)
}

func TestCreateFund_ForbiddenForClient(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForClient(t, 1, 1)

	body := map[string]any{
		"name":                 "Client Fund",
		"description":          "Should be forbidden.",
		"minimum_contribution": 500.0,
	}

	rec := performRequest(t, router, http.MethodPost, "/api/investment-funds", body, auth)
	require.NotEqual(t, http.StatusCreated, rec.Code)
}

func TestCreateFund_DuplicateName(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForSupervisor(t)
	name := fmt.Sprintf("Unique Fund %d", uniqueCounter.Add(1))

	// Seed a fund with the same name directly in DB
	seedInvestmentFund(t, db, name, 10)

	body := map[string]any{
		"name":                 name,
		"description":          "Duplicate name.",
		"minimum_contribution": 1000.0,
	}

	rec := performRequest(t, router, http.MethodPost, "/api/investment-funds", body, auth)
	require.Equal(t, http.StatusConflict, rec.Code)
}

func TestCreateFund_InvalidBody(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForSupervisor(t)

	// Empty body — missing required fields
	rec := performRequest(t, router, http.MethodPost, "/api/investment-funds", map[string]any{}, auth)
	require.NotEqual(t, http.StatusCreated, rec.Code)
}

func TestCreateFund_AccountNumberIsUnique(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForSupervisor(t)

	body1 := map[string]any{
		"name":                 fmt.Sprintf("Fund One %d", uniqueCounter.Add(1)),
		"description":          "First fund.",
		"minimum_contribution": 1000.0,
	}
	body2 := map[string]any{
		"name":                 fmt.Sprintf("Fund Two %d", uniqueCounter.Add(1)),
		"description":          "Second fund.",
		"minimum_contribution": 2000.0,
	}

	rec1 := performRequest(t, router, http.MethodPost, "/api/investment-funds", body1, auth)
	requireStatus(t, rec1, http.StatusCreated)

	rec2 := performRequest(t, router, http.MethodPost, "/api/investment-funds", body2, auth)
	requireStatus(t, rec2, http.StatusCreated)

	resp1 := decodeResponse[map[string]any](t, rec1)
	resp2 := decodeResponse[map[string]any](t, rec2)

	require.NotEqual(t, resp1["account_number"], resp2["account_number"])
}

func TestGetFundDetail_Success_AsClient(t *testing.T) {
	t.Skip("Skipping: requires banking service with account balance; will be re-enabled later")
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	supervisorAuth := authHeaderForSupervisor(t)
	fundName := fmt.Sprintf("GetFundTest %d", uniqueCounter.Add(1))
	createBody := map[string]any{
		"name":                 fundName,
		"description":          "Test fund for GET endpoint",
		"minimum_contribution": 2000.0,
	}
	createResp := performRequest(t, router, http.MethodPost, "/api/investment-funds", createBody, supervisorAuth)
	requireStatus(t, createResp, http.StatusCreated)
	createData := decodeResponse[map[string]any](t, createResp)
	fundID := int(createData["fund_id"].(float64)) // FIXED: "fund_id" instead of "id"

	clientAuth := authHeaderForClient(t, 10, 100)
	getPath := fmt.Sprintf("/api/investment-funds/%d", fundID)
	rec := performRequest(t, router, http.MethodGet, getPath, nil, clientAuth)
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[map[string]any](t, rec)
	require.Equal(t, fundName, resp["name"])
	require.Equal(t, "Test fund for GET endpoint", resp["description"])
	require.Equal(t, 2000.0, resp["min_investment"])
	require.Contains(t, resp["manager"], "Manager")
	require.NotNil(t, resp["fund_value"])
	require.NotNil(t, resp["profit"])
	require.NotNil(t, resp["account_balance"])
	require.IsType(t, []interface{}{}, resp["holdings"])
	require.IsType(t, []interface{}{}, resp["performance_history"])
}

func TestGetFundDetail_Unauthorized(t *testing.T) {
	t.Skip("Skipping: requires banking service with account balance; will be re-enabled later")
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	supervisorAuth := authHeaderForSupervisor(t)
	createBody := map[string]any{
		"name":                 fmt.Sprintf("NoAuthFund %d", uniqueCounter.Add(1)),
		"description":          "For unauthorized test",
		"minimum_contribution": 500.0,
	}
	createResp := performRequest(t, router, http.MethodPost, "/api/investment-funds", createBody, supervisorAuth)
	requireStatus(t, createResp, http.StatusCreated)
	createData := decodeResponse[map[string]any](t, createResp)
	fundID := int(createData["fund_id"].(float64)) // FIXED: "fund_id"

	rec := performRequest(t, router, http.MethodGet, fmt.Sprintf("/api/investment-funds/%d", fundID), nil, "")
	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestGetFundDetail_NotFound(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	clientAuth := authHeaderForClient(t, 10, 100)
	rec := performRequest(t, router, http.MethodGet, "/api/investment-funds/999999", nil, clientAuth)
	requireStatus(t, rec, http.StatusNotFound)
}

func TestGetFundDetail_HoldingsFormat(t *testing.T) {
	t.Skip("Skipping: requires banking service with account balance; will be re-enabled later")
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	supervisorAuth := authHeaderForSupervisor(t)
	fundName := fmt.Sprintf("HoldingsFund %d", uniqueCounter.Add(1))
	createBody := map[string]any{
		"name":                 fundName,
		"description":          "Fund with holdings",
		"minimum_contribution": 1000.0,
	}
	createResp := performRequest(t, router, http.MethodPost, "/api/investment-funds", createBody, supervisorAuth)
	requireStatus(t, createResp, http.StatusCreated)
	createData := decodeResponse[map[string]any](t, createResp)
	fundID := int(createData["fund_id"].(float64)) // FIXED: "fund_id"

	clientAuth := authHeaderForClient(t, 10, 100)
	rec := performRequest(t, router, http.MethodGet, fmt.Sprintf("/api/investment-funds/%d", fundID), nil, clientAuth)
	requireStatus(t, rec, http.StatusOK)
	resp := decodeResponse[map[string]any](t, rec)
	holdings := resp["holdings"].([]interface{})
	if len(holdings) > 0 {
		first := holdings[0].(map[string]interface{})
		require.Contains(t, first, "ticker")
		require.Contains(t, first, "price")
		require.Contains(t, first, "change")
		require.Contains(t, first, "volume")
		require.Contains(t, first, "initialMarginCost")
		require.Contains(t, first, "acquisitionDate")
	}
}

// ── GET /api/funds tests ──────────────────────────────────────────

func TestGetAllFunds_Success(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	seedInvestmentFund(t, db, fmt.Sprintf("Discovery Fund %d", uniqueCounter.Add(1)), 10)

	auth := authHeaderForSupervisor(t)
	rec := performRequest(t, router, http.MethodGet, "/api/investment-funds", nil, auth)
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[map[string]any](t, rec)
	require.NotNil(t, resp["data"])
	require.NotNil(t, resp["total"])
	require.NotNil(t, resp["page"])
	require.NotNil(t, resp["page_size"])
}

func TestGetAllFunds_Pagination(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	prefix := fmt.Sprintf("Paginate%d", uniqueCounter.Add(1))
	for i := 0; i < 3; i++ {
		seedInvestmentFund(t, db, fmt.Sprintf("%s Fund %d", prefix, i), 10)
	}

	auth := authHeaderForSupervisor(t)
	rec := performRequest(t, router, http.MethodGet, "/api/investment-funds?page=1&page_size=2", nil, auth)
	requireStatus(t, rec, http.StatusOK)

	type listResp struct {
		Data     []map[string]any `json:"data"`
		Total    float64          `json:"total"`
		Page     float64          `json:"page"`
		PageSize float64          `json:"page_size"`
	}
	resp := decodeResponse[listResp](t, rec)
	require.LessOrEqual(t, len(resp.Data), 2)
	require.Equal(t, float64(1), resp.Page)
	require.Equal(t, float64(2), resp.PageSize)
}

func TestGetAllFunds_NameFilter(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	unique := fmt.Sprintf("UniqueAlpha%d", uniqueCounter.Add(1))
	seedInvestmentFund(t, db, unique+" Fund", 10)
	seedInvestmentFund(t, db, fmt.Sprintf("OtherFund%d", uniqueCounter.Add(1)), 10)

	auth := authHeaderForSupervisor(t)
	rec := performRequest(t, router, http.MethodGet, "/api/investment-funds?name="+unique, nil, auth)
	requireStatus(t, rec, http.StatusOK)

	type listResp struct {
		Data []map[string]any `json:"data"`
	}
	resp := decodeResponse[listResp](t, rec)
	require.NotEmpty(t, resp.Data)
	for _, f := range resp.Data {
		require.Contains(t, f["name"], unique)
	}
}

func TestGetAllFunds_AccessibleToAgent(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForAgent(t)
	rec := performRequest(t, router, http.MethodGet, "/api/investment-funds", nil, auth)
	requireStatus(t, rec, http.StatusOK)
}

func TestGetAllFunds_AccessibleToClient(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForClient(t, 1, 1)
	rec := performRequest(t, router, http.MethodGet, "/api/investment-funds", nil, auth)
	requireStatus(t, rec, http.StatusOK)
}

func TestGetAllFunds_Unauthorized(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	rec := performRequest(t, router, http.MethodGet, "/api/investment-funds", nil, "")
	require.NotEqual(t, http.StatusOK, rec.Code)
}

func TestGetAllFunds_FundValueAndProfitPresent(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	seedInvestmentFund(t, db, fmt.Sprintf("ValueFund%d", uniqueCounter.Add(1)), 10)

	auth := authHeaderForSupervisor(t)
	rec := performRequest(t, router, http.MethodGet, "/api/investment-funds", nil, auth)
	requireStatus(t, rec, http.StatusOK)

	type listResp struct {
		Data []map[string]any `json:"data"`
	}
	resp := decodeResponse[listResp](t, rec)
	require.NotEmpty(t, resp.Data)
	fund := resp.Data[0]
	_, hasFundValue := fund["fund_value"]
	_, hasProfit := fund["profit"]
	_, hasMinContrib := fund["minimum_contribution"]
	require.True(t, hasFundValue)
	require.True(t, hasProfit)
	require.True(t, hasMinContrib)
}

// ── GET /api/actuary/:actId/assets/funds tests ────────────────────

func TestGetActuaryFunds_Success(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	seedInvestmentFund(t, db, fmt.Sprintf("ActuaryFund%d", uniqueCounter.Add(1)), 10)

	auth := authHeaderForSupervisor(t)
	rec := performRequest(t, router, http.MethodGet, "/api/actuary/10/assets/funds", nil, auth)
	requireStatus(t, rec, http.StatusOK)

	var funds []map[string]any
	funds = decodeResponse[[]map[string]any](t, rec)
	require.NotEmpty(t, funds)
	require.NotNil(t, funds[0]["fund_value"])
	require.NotNil(t, funds[0]["liquid_assets"])
}

func TestGetActuaryFunds_Empty(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForSupervisor(t)
	// Manager ID 999 has no funds
	rec := performRequest(t, router, http.MethodGet, "/api/actuary/999/assets/funds", nil, auth)
	requireStatus(t, rec, http.StatusOK)

	var funds []map[string]any
	funds = decodeResponse[[]map[string]any](t, rec)
	require.Empty(t, funds)
}

func TestGetActuaryFunds_OnlyReturnsManagedFunds(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	// Seed funds for manager 10 and manager 20
	seedInvestmentFund(t, db, fmt.Sprintf("Manager10Fund%d", uniqueCounter.Add(1)), 10)
	seedInvestmentFund(t, db, fmt.Sprintf("Manager20Fund%d", uniqueCounter.Add(1)), 20)

	auth := authHeaderForSupervisor(t)
	rec := performRequest(t, router, http.MethodGet, "/api/actuary/20/assets/funds", nil, auth)
	requireStatus(t, rec, http.StatusOK)

	var funds []map[string]any
	funds = decodeResponse[[]map[string]any](t, rec)
	for _, f := range funds {
		// All returned funds should belong to manager 20
		require.Contains(t, f["name"].(string), "Manager20Fund")
	}
}

func TestGetActuaryFunds_ForbiddenForClient(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForClient(t, 1, 1)
	rec := performRequest(t, router, http.MethodGet, "/api/actuary/10/assets/funds", nil, auth)
	require.NotEqual(t, http.StatusOK, rec.Code)
}

func TestGetActuaryFunds_Unauthorized(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	rec := performRequest(t, router, http.MethodGet, "/api/actuary/10/assets/funds", nil, "")
	require.NotEqual(t, http.StatusOK, rec.Code)
}
