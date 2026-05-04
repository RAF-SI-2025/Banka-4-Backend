//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInvestInFund_ClientSuccess(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	fund := seedInvestmentFund(t, db, fmt.Sprintf("Fund %d", uniqueCounter.Add(1)), 10)

	auth := authHeaderForClient(t, 1, 1)

	body := map[string]any{
		"account_number": "444000100000000001",
		"amount":         1000.0,
	}

	rec := performRequest(t, router, http.MethodPost, fmt.Sprintf("/api/investment-funds/%d/invest", fund.FundID), body, auth)
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[map[string]any](t, rec)
	require.Equal(t, float64(fund.FundID), resp["fund_id"])
	require.Equal(t, 1000.0, resp["invested_now"])
	require.Equal(t, 1000.0, resp["total_invested_rsd"])
}

func TestInvestInFund_SupervisorSuccess(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	fund := seedInvestmentFund(t, db, fmt.Sprintf("Fund %d", uniqueCounter.Add(1)), 10)

	auth := authHeaderForSupervisor(t)

	body := map[string]any{
		"account_number": "444000000000000000",
		"amount":         5000.0,
	}

	rec := performRequest(t, router, http.MethodPost, fmt.Sprintf("/api/investment-funds/%d/invest", fund.FundID), body, auth)
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[map[string]any](t, rec)
	require.Equal(t, 5000.0, resp["invested_now"])
	require.Equal(t, 5000.0, resp["total_invested_rsd"])
}

func TestInvestInFund_AccumulatesPosition(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	fund := seedInvestmentFund(t, db, fmt.Sprintf("Fund %d", uniqueCounter.Add(1)), 10)
	auth := authHeaderForClient(t, 1, 1)

	body := map[string]any{
		"account_number": "444000100000000001",
		"amount":         1000.0,
	}

	rec1 := performRequest(t, router, http.MethodPost, fmt.Sprintf("/api/investment-funds/%d/invest", fund.FundID), body, auth)
	requireStatus(t, rec1, http.StatusOK)

	rec2 := performRequest(t, router, http.MethodPost, fmt.Sprintf("/api/investment-funds/%d/invest", fund.FundID), body, auth)
	requireStatus(t, rec2, http.StatusOK)

	resp := decodeResponse[map[string]any](t, rec2)
	require.Equal(t, 1000.0, resp["invested_now"])
	require.Equal(t, 2000.0, resp["total_invested_rsd"])
}

func TestInvestInFund_BelowMinimum(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	fund := seedInvestmentFund(t, db, fmt.Sprintf("Fund %d", uniqueCounter.Add(1)), 10)
	auth := authHeaderForClient(t, 1, 1)

	body := map[string]any{
		"account_number": "444000100000000001",
		"amount":         1.0,
	}

	rec := performRequest(t, router, http.MethodPost, fmt.Sprintf("/api/investment-funds/%d/invest", fund.FundID), body, auth)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestInvestInFund_FundNotFound(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	auth := authHeaderForClient(t, 1, 1)

	body := map[string]any{
		"account_number": "444000100000000001",
		"amount":         1000.0,
	}

	rec := performRequest(t, router, http.MethodPost, "/api/investment-funds/99999/invest", body, auth)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestInvestInFund_Unauthorized(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	fund := seedInvestmentFund(t, db, fmt.Sprintf("Fund %d", uniqueCounter.Add(1)), 10)

	body := map[string]any{
		"account_number": "444000100000000001",
		"amount":         1000.0,
	}

	rec := performRequest(t, router, http.MethodPost, fmt.Sprintf("/api/investment-funds/%d/invest", fund.FundID), body, "")
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestInvestInFund_AgentForbidden(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	fund := seedInvestmentFund(t, db, fmt.Sprintf("Fund %d", uniqueCounter.Add(1)), 10)
	auth := authHeaderForAgent(t)

	body := map[string]any{
		"account_number": "444000100000000001",
		"amount":         1000.0,
	}

	rec := performRequest(t, router, http.MethodPost, fmt.Sprintf("/api/investment-funds/%d/invest", fund.FundID), body, auth)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestInvestInFund_InvalidBody(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	fund := seedInvestmentFund(t, db, fmt.Sprintf("Fund %d", uniqueCounter.Add(1)), 10)
	auth := authHeaderForClient(t, 1, 1)

	rec := performRequest(t, router, http.MethodPost, fmt.Sprintf("/api/investment-funds/%d/invest", fund.FundID), map[string]any{}, auth)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestInvestInFund_CurrencyConversion(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	fund := seedInvestmentFund(t, db, fmt.Sprintf("Fund %d", uniqueCounter.Add(1)), 10)
	auth := authHeaderForClient(t, 1, 1)

	body := map[string]any{
		"account_number": "444000100000000001",
		"amount":         1000.0,
	}

	rec := performRequest(t, router, http.MethodPost, fmt.Sprintf("/api/investment-funds/%d/invest", fund.FundID), body, auth)
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[map[string]any](t, rec)
	// invested_now je u originalnoj valuti racuna
	require.Equal(t, 1000.0, resp["invested_now"])
	// total_invested je u RSD
	require.Equal(t, resp["total_invested"], resp["total_invested"])
	require.Equal(t, "RSD", resp["currency_code"])
}

func TestInvestInFund_SupervisorWithDifferentBankAccount(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	fund := seedInvestmentFund(t, db, fmt.Sprintf("Fund %d", uniqueCounter.Add(1)), 10)

	auth := authHeaderForSupervisor(t)

	body := map[string]any{
		"account_number": "444000000000000099",
		"amount":         5000.0,
	}

	rec := performRequest(t, router, http.MethodPost, fmt.Sprintf("/api/investment-funds/%d/invest", fund.FundID), body, auth)
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[map[string]any](t, rec)
	require.Equal(t, float64(fund.FundID), resp["fund_id"])
	require.Equal(t, 5000.0, resp["invested_now"])
}
