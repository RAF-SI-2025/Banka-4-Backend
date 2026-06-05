//go:build integration

package integration_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/stretchr/testify/require"
)

// ── GET /api/dividends ─────────────────────────────────────────────

func TestGetAllDividendPayouts_EmptyList(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	rec := performRequest(t, router, http.MethodGet, "/api/dividends", nil, authHeaderForSupervisor(t))
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[dto.ListDividendPayoutsResponse](t, rec)
	require.NotNil(t, resp.Data)
	require.Empty(t, resp.Data)
}

func TestGetAllDividendPayouts_ReturnsSavedPayouts(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	exchange := seedExchange(t, db, uniqueValue(t, "MIC"))
	listing := seedListing(t, db, uniqueValue(t, "TICK"), exchange.MicCode, model.AssetTypeStock, 100.0)
	_ = seedStock(t, db, listing.ListingID)

	ownership := &model.AssetOwnership{
		UserId:    1,
		OwnerType: model.OwnerTypeClient,
		AssetID:   listing.AssetID,
		Amount:    10,
	}
	require.NoError(t, db.Create(ownership).Error)

	payout := &model.DividendPayout{
		AssetOwnershipID: ownership.AssetOwnershipID,
		Quantity:         10,
		PricePerShare:    100,
		GrossAmount:      25,
		TaxAmount:        3.75,
		NetAmount:        21.25,
		CurrencyCode:     "RSD",
		AccountNumber:    "444000100000000001",
	}
	require.NoError(t, db.Create(payout).Error)

	rec := performRequest(t, router, http.MethodGet, "/api/dividends", nil, authHeaderForSupervisor(t))
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[dto.ListDividendPayoutsResponse](t, rec)
	require.NotEmpty(t, resp.Data)
}

func TestGetAllDividendPayouts_ForbiddenForClient(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	rec := performRequest(t, router, http.MethodGet, "/api/dividends", nil, authHeaderForClient(t, 1, 1))
	requireStatus(t, rec, http.StatusForbidden)
}

func TestGetAllDividendPayouts_ForbiddenForAgent(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	rec := performRequest(t, router, http.MethodGet, "/api/dividends", nil, authHeaderForAgent(t))
	requireStatus(t, rec, http.StatusForbidden)
}

func TestGetAllDividendPayouts_Unauthorized(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	rec := performRequest(t, router, http.MethodGet, "/api/dividends", nil, "")
	requireStatus(t, rec, http.StatusUnauthorized)
}

// ── GET /api/client/:clientId/assets/:assetOwnershipId/dividends ───

func TestGetDividendPayoutsForAssetOwnership_ReturnsPayouts(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	exchange := seedExchange(t, db, uniqueValue(t, "MIC"))
	listing := seedListing(t, db, uniqueValue(t, "TICK"), exchange.MicCode, model.AssetTypeStock, 100.0)
	_ = seedStock(t, db, listing.ListingID)

	const clientID = uint(1)
	ownership := &model.AssetOwnership{
		UserId:    clientID,
		OwnerType: model.OwnerTypeClient,
		AssetID:   listing.AssetID,
		Amount:    10,
	}
	require.NoError(t, db.Create(ownership).Error)

	payout := &model.DividendPayout{
		AssetOwnershipID: ownership.AssetOwnershipID,
		Quantity:         10,
		PricePerShare:    100,
		GrossAmount:      25,
		TaxAmount:        3.75,
		NetAmount:        21.25,
		CurrencyCode:     "RSD",
		AccountNumber:    "444000100000000001",
	}
	require.NoError(t, db.Create(payout).Error)

	path := fmt.Sprintf("/api/client/%d/assets/%d/dividends", clientID, ownership.AssetOwnershipID)
	rec := performRequest(t, router, http.MethodGet, path, nil, authHeaderForClient(t, clientID, clientID))
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[dto.ListDividendPayoutsResponse](t, rec)
	require.Len(t, resp.Data, 1)
	require.Equal(t, ownership.AssetOwnershipID, resp.Data[0].AssetOwnershipID)
	require.InDelta(t, 25.0, resp.Data[0].GrossAmount, 0.001)
	require.InDelta(t, 3.75, resp.Data[0].TaxAmount, 0.001)
	require.InDelta(t, 21.25, resp.Data[0].NetAmount, 0.001)
	require.Equal(t, "RSD", resp.Data[0].CurrencyCode)
}

func TestGetDividendPayoutsForAssetOwnership_EmptyForUnknownOwnership(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	const clientID = uint(1)
	path := fmt.Sprintf("/api/client/%d/assets/999999/dividends", clientID)
	rec := performRequest(t, router, http.MethodGet, path, nil, authHeaderForClient(t, clientID, clientID))
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[dto.ListDividendPayoutsResponse](t, rec)
	require.Empty(t, resp.Data)
}

func TestGetDividendPayoutsForAssetOwnership_MultiplePayouts(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	exchange := seedExchange(t, db, uniqueValue(t, "MIC"))
	listing := seedListing(t, db, uniqueValue(t, "TICK"), exchange.MicCode, model.AssetTypeStock, 100.0)
	_ = seedStock(t, db, listing.ListingID)

	const clientID = uint(1)
	ownership := &model.AssetOwnership{
		UserId:    clientID,
		OwnerType: model.OwnerTypeClient,
		AssetID:   listing.AssetID,
		Amount:    10,
	}
	require.NoError(t, db.Create(ownership).Error)

	for i := 0; i < 3; i++ {
		p := &model.DividendPayout{
			AssetOwnershipID: ownership.AssetOwnershipID,
			Quantity:         10,
			PricePerShare:    float64(100 + i*10),
			GrossAmount:      float64(25 + i*5),
			TaxAmount:        float64(25+i*5) * 0.15,
			NetAmount:        float64(25+i*5) * 0.85,
			CurrencyCode:     "RSD",
			AccountNumber:    "444000100000000001",
		}
		require.NoError(t, db.Create(p).Error)
	}

	path := fmt.Sprintf("/api/client/%d/assets/%d/dividends", clientID, ownership.AssetOwnershipID)
	rec := performRequest(t, router, http.MethodGet, path, nil, authHeaderForClient(t, clientID, clientID))
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[dto.ListDividendPayoutsResponse](t, rec)
	require.Len(t, resp.Data, 3)
}

func TestGetDividendPayoutsForAssetOwnership_DoesNotReturnOtherOwnershipsPayouts(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	exchange := seedExchange(t, db, uniqueValue(t, "MIC"))
	listing := seedListing(t, db, uniqueValue(t, "TICK"), exchange.MicCode, model.AssetTypeStock, 100.0)
	_ = seedStock(t, db, listing.ListingID)

	const clientID = uint(1)
	ownership1 := &model.AssetOwnership{
		UserId:    clientID,
		OwnerType: model.OwnerTypeClient,
		AssetID:   listing.AssetID,
		Amount:    10,
	}
	require.NoError(t, db.Create(ownership1).Error)

	ownership2 := &model.AssetOwnership{
		UserId:    2,
		OwnerType: model.OwnerTypeClient,
		AssetID:   listing.AssetID,
		Amount:    20,
	}
	require.NoError(t, db.Create(ownership2).Error)

	require.NoError(t, db.Create(&model.DividendPayout{
		AssetOwnershipID: ownership1.AssetOwnershipID,
		Quantity:         10,
		PricePerShare:    100,
		GrossAmount:      25,
		TaxAmount:        3.75,
		NetAmount:        21.25,
		CurrencyCode:     "RSD",
		AccountNumber:    "444000100000000001",
	}).Error)

	require.NoError(t, db.Create(&model.DividendPayout{
		AssetOwnershipID: ownership2.AssetOwnershipID,
		Quantity:         20,
		PricePerShare:    100,
		GrossAmount:      50,
		TaxAmount:        7.5,
		NetAmount:        42.5,
		CurrencyCode:     "RSD",
		AccountNumber:    "444000100000000002",
	}).Error)

	path := fmt.Sprintf("/api/client/%d/assets/%d/dividends", clientID, ownership1.AssetOwnershipID)
	rec := performRequest(t, router, http.MethodGet, path, nil, authHeaderForClient(t, clientID, clientID))
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[dto.ListDividendPayoutsResponse](t, rec)
	require.Len(t, resp.Data, 1)
	require.Equal(t, ownership1.AssetOwnershipID, resp.Data[0].AssetOwnershipID)
}

func TestGetDividendPayoutsForAssetOwnership_InvalidID(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	const clientID = uint(1)
	path := fmt.Sprintf("/api/client/%d/assets/abc/dividends", clientID)
	rec := performRequest(t, router, http.MethodGet, path, nil, authHeaderForClient(t, clientID, clientID))
	requireStatus(t, rec, http.StatusBadRequest)
}

func TestGetDividendPayoutsForAssetOwnership_Unauthorized(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	rec := performRequest(t, router, http.MethodGet, "/api/client/1/assets/1/dividends", nil, "")
	requireStatus(t, rec, http.StatusUnauthorized)
}

// ── GET /api/actuary/:actId/assets/:assetOwnershipId/dividends ─────

func TestGetActuaryDividendPayoutsForAssetOwnership_ReturnsPayouts(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	exchange := seedExchange(t, db, uniqueValue(t, "MIC"))
	listing := seedListing(t, db, uniqueValue(t, "TICK"), exchange.MicCode, model.AssetTypeStock, 100.0)
	_ = seedStock(t, db, listing.ListingID)

	const actuaryID = uint(10)
	ownership := &model.AssetOwnership{
		UserId:    actuaryID,
		OwnerType: model.OwnerTypeBank,
		AssetID:   listing.AssetID,
		Amount:    10,
	}
	require.NoError(t, db.Create(ownership).Error)

	payout := &model.DividendPayout{
		AssetOwnershipID: ownership.AssetOwnershipID,
		Quantity:         10,
		PricePerShare:    100,
		GrossAmount:      25,
		TaxAmount:        0,
		NetAmount:        25,
		CurrencyCode:     "RSD",
		AccountNumber:    "444000000000000099",
	}
	require.NoError(t, db.Create(payout).Error)

	path := fmt.Sprintf("/api/actuary/%d/assets/%d/dividends", actuaryID, ownership.AssetOwnershipID)
	rec := performRequest(t, router, http.MethodGet, path, nil, authHeaderForSupervisor(t))
	requireStatus(t, rec, http.StatusOK)

	resp := decodeResponse[dto.ListDividendPayoutsResponse](t, rec)
	require.Len(t, resp.Data, 1)
	require.Equal(t, ownership.AssetOwnershipID, resp.Data[0].AssetOwnershipID)
}

func TestGetActuaryDividendPayoutsForAssetOwnership_Unauthorized(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	rec := performRequest(t, router, http.MethodGet, "/api/actuary/10/assets/1/dividends", nil, "")
	requireStatus(t, rec, http.StatusUnauthorized)
}

// ── POST /api/dividends/process ────────────────────────────────────

func TestTriggerDividends_ProcessesAndSavesClientPayout(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	exchange := seedExchange(t, db, uniqueValue(t, "MIC"))
	listing := seedListing(t, db, uniqueValue(t, "TICK"), exchange.MicCode, model.AssetTypeStock, 200.0)
	stock := seedStock(t, db, listing.ListingID)

	ownership := &model.AssetOwnership{
		UserId:    1,
		OwnerType: model.OwnerTypeClient,
		AssetID:   stock.AssetID,
		Amount:    100,
	}
	require.NoError(t, db.Create(ownership).Error)

	rec := performRequest(t, router, http.MethodPost, "/api/dividends/process", nil, authHeaderForSupervisor(t))
	requireStatus(t, rec, http.StatusOK)

	var payouts []model.DividendPayout
	require.NoError(t, db.Where("asset_ownership_id = ?", ownership.AssetOwnershipID).Find(&payouts).Error)
	require.Len(t, payouts, 1)

	require.InDelta(t, 125.0, payouts[0].GrossAmount, 0.001)
	require.InDelta(t, 125.0*0.15, payouts[0].TaxAmount, 0.001)
	require.InDelta(t, 125.0*0.85, payouts[0].NetAmount, 0.001)
}

func TestTriggerDividends_ActuaryPayoutGoesToBankAccount(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	exchange := seedExchange(t, db, uniqueValue(t, "MIC"))
	listing := seedListing(t, db, uniqueValue(t, "TICK"), exchange.MicCode, model.AssetTypeStock, 200.0)
	stock := seedStock(t, db, listing.ListingID)

	ownership := &model.AssetOwnership{
		UserId:    10, // EmployeeID supervizora
		OwnerType: model.OwnerTypeBank,
		AssetID:   stock.AssetID,
		Amount:    100,
	}
	require.NoError(t, db.Create(ownership).Error)

	rec := performRequest(t, router, http.MethodPost, "/api/dividends/process", nil, authHeaderForSupervisor(t))
	requireStatus(t, rec, http.StatusOK)

	var payouts []model.DividendPayout
	require.NoError(t, db.Where("asset_ownership_id = ?", ownership.AssetOwnershipID).Find(&payouts).Error)
	require.Len(t, payouts, 1)

	require.Equal(t, 0.0, payouts[0].TaxAmount)
	require.Equal(t, payouts[0].GrossAmount, payouts[0].NetAmount)
	require.Equal(t, "444000000000000099", payouts[0].AccountNumber)
}

func TestTriggerDividends_SkipsStockWithZeroDividendYield(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	exchange := seedExchange(t, db, uniqueValue(t, "MIC"))
	listing := seedListing(t, db, uniqueValue(t, "TICK"), exchange.MicCode, model.AssetTypeStock, 200.0)

	stock := &model.Stock{
		AssetID:           listing.AssetID,
		OutstandingShares: 1_000_000,
		DividendYield:     0,
	}
	require.NoError(t, db.Create(stock).Error)

	ownership := &model.AssetOwnership{
		UserId:    1,
		OwnerType: model.OwnerTypeClient,
		AssetID:   stock.AssetID,
		Amount:    100,
	}
	require.NoError(t, db.Create(ownership).Error)

	rec := performRequest(t, router, http.MethodPost, "/api/dividends/process", nil, authHeaderForSupervisor(t))
	requireStatus(t, rec, http.StatusOK)

	var payouts []model.DividendPayout
	require.NoError(t, db.Where("asset_ownership_id = ?", ownership.AssetOwnershipID).Find(&payouts).Error)
	require.Empty(t, payouts)
}

func TestTriggerDividends_ForbiddenForAgent(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	rec := performRequest(t, router, http.MethodPost, "/api/dividends/process", nil, authHeaderForAgent(t))
	requireStatus(t, rec, http.StatusForbidden)
}

func TestTriggerDividends_ForbiddenForClient(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	rec := performRequest(t, router, http.MethodPost, "/api/dividends/process", nil, authHeaderForClient(t, 1, 1))
	requireStatus(t, rec, http.StatusForbidden)
}

func TestTriggerDividends_Unauthorized(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	rec := performRequest(t, router, http.MethodPost, "/api/dividends/process", nil, "")
	requireStatus(t, rec, http.StatusUnauthorized)
}

// ── Fund Dividend Integration Tests ────────────────────────────────────

func TestTriggerDividends_FundOwnedStockCreatesPayoutForFundAccount(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	// Seed exchange in simulated MIC so currency resolves to RSD
	exchange := seedExchange(t, db, uniqueValue(t, "MIC"))
	listing := seedListing(t, db, uniqueValue(t, "TICK"), exchange.MicCode, model.AssetTypeStock, 200.0)
	stock := seedStock(t, db, listing.ListingID)

	// Create a fund that owns the stock
	fund := seedInvestmentFund(t, db, uniqueValue(t, "Fund"), 10)
	ownership := &model.AssetOwnership{
		UserId:    fund.FundID,
		OwnerType: model.OwnerTypeFund,
		AssetID:   stock.AssetID,
		Amount:    100,
	}
	require.NoError(t, db.Create(ownership).Error)

	rec := performRequest(t, router, http.MethodPost, "/api/dividends/process", nil, authHeaderForSupervisor(t))
	requireStatus(t, rec, http.StatusOK)

	// The fund-level DividendPayout should reference the fund's account number
	var payouts []model.DividendPayout
	require.NoError(t, db.Where("asset_ownership_id = ?", ownership.AssetOwnershipID).Find(&payouts).Error)
	require.Len(t, payouts, 1)

	// gross = 100 shares * 200 price * (2.5/400) = 125.0
	require.InDelta(t, 125.0, payouts[0].GrossAmount, 0.001)
	require.Equal(t, 0.0, payouts[0].TaxAmount)
	require.Equal(t, payouts[0].GrossAmount, payouts[0].NetAmount)
	require.Equal(t, fund.AccountNumber, payouts[0].AccountNumber)
}

func TestTriggerDividends_FundDistributesPayoutToClients(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	exchange := seedExchange(t, db, uniqueValue(t, "MIC"))
	listing := seedListing(t, db, uniqueValue(t, "TICK"), exchange.MicCode, model.AssetTypeStock, 200.0)
	stock := seedStock(t, db, listing.ListingID)

	reinvestPct := 0.0 // 0% reinvestment → 100% to clients
	fund := &model.InvestmentFund{
		Name:                        uniqueValue(t, "Fund"),
		Description:                 "test fund",
		MinimumContribution:         100,
		ManagerID:                   10,
		AccountNumber:               fmt.Sprintf("444000199999%06d", uniqueCounter.Add(1)),
		DividendReinvestmentPercent: &reinvestPct,
		CreatedAt:                   time.Now(),
	}
	require.NoError(t, db.Create(fund).Error)

	// Fund owns 40 shares of the stock
	ownership := &model.AssetOwnership{
		UserId:    fund.FundID,
		OwnerType: model.OwnerTypeFund,
		AssetID:   stock.AssetID,
		Amount:    40,
	}
	require.NoError(t, db.Create(ownership).Error)

	// Two clients: 3 and 1 unit — 75% and 25% of payout
	pos1 := &model.ClientFundPosition{FundID: fund.FundID, ClientID: 1, OwnerType: model.OwnerTypeClient, UnitsOwned: 3, UpdatedAt: time.Now()}
	pos2 := &model.ClientFundPosition{FundID: fund.FundID, ClientID: 2, OwnerType: model.OwnerTypeClient, UnitsOwned: 1, UpdatedAt: time.Now()}
	require.NoError(t, db.Create(pos1).Error)
	require.NoError(t, db.Create(pos2).Error)

	rec := performRequest(t, router, http.MethodPost, "/api/dividends/process", nil, authHeaderForSupervisor(t))
	requireStatus(t, rec, http.StatusOK)

	// gross = 40 * 200 * (2.5/400) = 50
	// 0% reinvestment → all 50 distributed to clients via banking payments
	// Client distributions do NOT create DividendPayout records (no valid AssetOwnershipID).
	// Only the fund-level payout record is persisted.
	var fundPayouts []model.DividendPayout
	require.NoError(t, db.Where("asset_ownership_id = ? AND account_number = ?", ownership.AssetOwnershipID, fund.AccountNumber).Find(&fundPayouts).Error)
	require.Len(t, fundPayouts, 1)
	require.InDelta(t, 50.0, fundPayouts[0].GrossAmount, 0.001)
	require.Equal(t, 0.0, fundPayouts[0].TaxAmount)
}

func TestTriggerDividends_MultipleOwnersGetSeparatePayouts(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	router, _ := setupTestRouter(t, db)

	exchange := seedExchange(t, db, uniqueValue(t, "MIC"))
	listing := seedListing(t, db, uniqueValue(t, "TICK"), exchange.MicCode, model.AssetTypeStock, 100.0)
	stock := seedStock(t, db, listing.ListingID)

	ownership1 := &model.AssetOwnership{
		UserId:    1,
		OwnerType: model.OwnerTypeClient,
		AssetID:   stock.AssetID,
		Amount:    50,
	}
	require.NoError(t, db.Create(ownership1).Error)

	ownership2 := &model.AssetOwnership{
		UserId:    2,
		OwnerType: model.OwnerTypeClient,
		AssetID:   stock.AssetID,
		Amount:    100,
	}
	require.NoError(t, db.Create(ownership2).Error)

	rec := performRequest(t, router, http.MethodPost, "/api/dividends/process", nil, authHeaderForSupervisor(t))
	requireStatus(t, rec, http.StatusOK)

	var payouts1 []model.DividendPayout
	require.NoError(t, db.Where("asset_ownership_id = ?", ownership1.AssetOwnershipID).Find(&payouts1).Error)
	require.Len(t, payouts1, 1)

	var payouts2 []model.DividendPayout
	require.NoError(t, db.Where("asset_ownership_id = ?", ownership2.AssetOwnershipID).Find(&payouts2).Error)
	require.Len(t, payouts2, 1)

	require.InDelta(t, payouts1[0].GrossAmount*2, payouts2[0].GrossAmount, 0.001)
}
