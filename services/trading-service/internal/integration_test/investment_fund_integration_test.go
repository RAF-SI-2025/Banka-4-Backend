//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
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

func TestGetClientFundPositions_Empty(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForClient(t, 1, 1)

	rec := performRequest(t, router, http.MethodGet, "/api/client/1/funds", nil, auth)
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[[]any](t, rec)
	require.Empty(t, resp)
}

func TestGetClientFundPositions_WithPosition(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	fund := seedInvestmentFund(t, db, fmt.Sprintf("MyFund %d", uniqueCounter.Add(1)), 10)

	position := &model.ClientFundPosition{
		ClientID:            1,
		OwnerType:           model.OwnerTypeClient,
		FundID:              fund.FundID,
		TotalInvestedAmount: 2000,
	}
	require.NoError(t, db.Create(position).Error)

	auth := authHeaderForClient(t, 1, 1)

	rec := performRequest(t, router, http.MethodGet, "/api/client/1/funds", nil, auth)
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[[]map[string]any](t, rec)
	require.Len(t, resp, 1)
	require.Equal(t, float64(fund.FundID), resp[0]["fund_id"])
	require.Equal(t, fund.Name, resp[0]["fund_name"])
	require.Equal(t, 1.0, resp[0]["clients_share_percent"])
}

func TestGetClientFundPositions_Unauthorized(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	rec := performRequest(t, router, http.MethodGet, "/api/client/1/funds", nil, "")
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGetClientFundPositions_InvalidClientId(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForClient(t, 1, 1)

	rec := performRequest(t, router, http.MethodGet, "/api/client/abc/funds", nil, auth)
	require.Equal(t, http.StatusBadRequest, rec.Code)
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
