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

func TestWithdrawFromFund_ClientSuccess(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	fund := seedInvestmentFund(t, db, fmt.Sprintf("Fund %d", uniqueCounter.Add(1)), 10)
	position := &model.ClientFundPosition{
		ClientID:            1,
		OwnerType:           model.OwnerTypeClient,
		FundID:              fund.FundID,
		TotalInvestedAmount: 2000,
		UpdatedAt:           time.Now(),
	}
	require.NoError(t, db.Create(position).Error)

	body := map[string]any{
		"account_number": "444000100000000001",
		"amount":         750.0,
	}

	rec := performRequest(t, router, http.MethodPost, fmt.Sprintf("/api/investment-funds/%d/withdraw", fund.FundID), body, authHeaderForClient(t, 1, 1))
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[map[string]any](t, rec)
	require.Equal(t, float64(fund.FundID), resp["fund_id"])
	require.Equal(t, "COMPLETED", resp["status"])
	require.Equal(t, 750.0, resp["withdrawn_amount_rsd"])
	require.Equal(t, 1250.0, resp["total_invested_rsd"])

	var updated model.ClientFundPosition
	require.NoError(t, db.First(&updated, position.PositionID).Error)
	require.Equal(t, 1250.0, updated.TotalInvestedAmount)
}

func TestWithdrawFromFund_SupervisorSuccess(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	fund := seedInvestmentFund(t, db, fmt.Sprintf("Fund %d", uniqueCounter.Add(1)), 10)
	position := &model.ClientFundPosition{
		ClientID:            10,
		OwnerType:           model.OwnerTypeActuary,
		FundID:              fund.FundID,
		TotalInvestedAmount: 3000,
		UpdatedAt:           time.Now(),
	}
	require.NoError(t, db.Create(position).Error)

	body := map[string]any{
		"account_number": "444000000000000000",
		"amount":         1000.0,
	}

	rec := performRequest(t, router, http.MethodPost, fmt.Sprintf("/api/investment-funds/%d/withdraw", fund.FundID), body, authHeaderForSupervisor(t))
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[map[string]any](t, rec)
	require.Equal(t, "COMPLETED", resp["status"])
	require.Equal(t, 2000.0, resp["total_invested_rsd"])
}

func TestWithdrawFromFund_ExceedsPosition(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	fund := seedInvestmentFund(t, db, fmt.Sprintf("Fund %d", uniqueCounter.Add(1)), 10)
	position := &model.ClientFundPosition{
		ClientID:            1,
		OwnerType:           model.OwnerTypeClient,
		FundID:              fund.FundID,
		TotalInvestedAmount: 500,
		UpdatedAt:           time.Now(),
	}
	require.NoError(t, db.Create(position).Error)

	body := map[string]any{
		"account_number": "444000100000000001",
		"amount":         1000.0,
	}

	rec := performRequest(t, router, http.MethodPost, fmt.Sprintf("/api/investment-funds/%d/withdraw", fund.FundID), body, authHeaderForClient(t, 1, 1))
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
