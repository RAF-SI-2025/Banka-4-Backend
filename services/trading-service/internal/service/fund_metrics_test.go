package service

import (
	"context"
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/stretchr/testify/require"
)

func makeSnapshots(values []float64) []model.FundPerformance {
	snaps := make([]model.FundPerformance, len(values))
	for i, v := range values {
		snaps[i] = model.FundPerformance{
			FundID:    1,
			Date:      time.Now().AddDate(0, -len(values)+i, 0),
			FundValue: v,
			Profit:    v - values[0],
		}
	}
	return snaps
}

func TestCalculateFundMetrics_NotEnoughSnapshots(t *testing.T) {
	snaps := makeSnapshots([]float64{1000, 1100, 1200})
	metrics := calculateFundMetrics(snaps)

	require.Nil(t, metrics.AnnualReturn)
	require.Nil(t, metrics.RewardToVariability)
	require.Nil(t, metrics.MaxDrawdown)
	require.Nil(t, metrics.Volatility)
}

func TestCalculateFundMetrics_ExactlyMinSnapshots(t *testing.T) {
	values := make([]float64, MinSnapshotsForMetrics)
	for i := range values {
		values[i] = 1000.0 + float64(i)*10
	}
	snaps := makeSnapshots(values)
	metrics := calculateFundMetrics(snaps)

	require.NotNil(t, metrics.AnnualReturn)
	require.NotNil(t, metrics.Volatility)
	require.NotNil(t, metrics.MaxDrawdown)
}

func TestCalculateFundMetrics_SteadyGrowth(t *testing.T) {
	values := make([]float64, 13) // 12 mesečnih prinosa
	values[0] = 1000.0
	for i := 1; i < 13; i++ {
		values[i] = values[i-1] * 1.10
	}
	snaps := makeSnapshots(values)
	metrics := calculateFundMetrics(snaps)

	require.NotNil(t, metrics.AnnualReturn)
	require.InDelta(t, 213.8, *metrics.AnnualReturn, 1.0)

	require.NotNil(t, metrics.MaxDrawdown)
	require.InDelta(t, 0.0, *metrics.MaxDrawdown, 0.0001)
}

func TestCalculateFundMetrics_ConstantValue(t *testing.T) {
	values := make([]float64, 13)
	for i := range values {
		values[i] = 1000.0
	}
	snaps := makeSnapshots(values)
	metrics := calculateFundMetrics(snaps)

	require.NotNil(t, metrics.Volatility)
	require.InDelta(t, 0.0, *metrics.Volatility, 0.0001)
	require.Nil(t, metrics.RewardToVariability)
}

func TestCalculateFundMetrics_MaxDrawdown_PeakThenDrop(t *testing.T) {
	// Fond ide do 2000 pa pada na 1000 — drawdown 50%
	values := []float64{
		1000, 1100, 1200, 1300, 1400, 1500, 1600, 1700, 1800, 1900, 2000,
		1500, 1000,
	}
	snaps := makeSnapshots(values)
	metrics := calculateFundMetrics(snaps)

	require.NotNil(t, metrics.MaxDrawdown)
	require.InDelta(t, -50.0, *metrics.MaxDrawdown, 0.1)
}

func TestCalculateFundMetrics_EmptySnapshots(t *testing.T) {
	metrics := calculateFundMetrics([]model.FundPerformance{})
	require.Nil(t, metrics.AnnualReturn)
	require.Nil(t, metrics.RewardToVariability)
	require.Nil(t, metrics.MaxDrawdown)
	require.Nil(t, metrics.Volatility)
}

func TestCalculateFundMetrics_ZeroFirstValue(t *testing.T) {
	values := make([]float64, 13)
	values[0] = 0
	for i := 1; i < 13; i++ {
		values[i] = float64(i * 100)
	}
	snaps := makeSnapshots(values)
	metrics := calculateFundMetrics(snaps)

	require.Nil(t, metrics.AnnualReturn)
}

func TestAveragePerformanceHistory_EmptyInput(t *testing.T) {
	result := averagePerformanceHistory(map[uint][]model.FundPerformance{})
	require.Nil(t, result)
}

func TestAveragePerformanceHistory_SingleFund(t *testing.T) {
	snaps := makeSnapshots([]float64{1000, 1100, 1200})
	histories := map[uint][]model.FundPerformance{1: snaps}

	result := averagePerformanceHistory(histories)

	require.Len(t, result, 3)
	require.InDelta(t, 1000.0, result[0].FundValue, 0.001)
	require.InDelta(t, 1100.0, result[1].FundValue, 0.001)
	require.InDelta(t, 1200.0, result[2].FundValue, 0.001)
}

func TestAveragePerformanceHistory_TwoFunds(t *testing.T) {
	snaps1 := makeSnapshots([]float64{1000, 1200, 1400})
	snaps2 := makeSnapshots([]float64{2000, 2200, 2400})
	histories := map[uint][]model.FundPerformance{
		1: snaps1,
		2: snaps2,
	}

	result := averagePerformanceHistory(histories)

	require.Len(t, result, 3)
	require.InDelta(t, 1500.0, result[0].FundValue, 0.001) // (1000+2000)/2
	require.InDelta(t, 1700.0, result[1].FundValue, 0.001) // (1200+2200)/2
	require.InDelta(t, 1900.0, result[2].FundValue, 0.001) // (1400+2400)/2
}

func TestGetAllFunds_WithMetrics(t *testing.T) {
	fund := model.InvestmentFund{
		FundID:        1,
		Name:          "Metrics Fund",
		AccountNumber: "ACC-001",
		Positions:     []model.ClientFundPosition{{TotalInvestedAmount: 1000}},
	}

	values := make([]float64, 13)
	values[0] = 1000.0
	for i := 1; i < 13; i++ {
		values[i] = values[i-1] * 1.05
	}
	snaps := makeSnapshots(values)

	fundRepo := &fakeFundRepo{
		findAllResult:                    []model.InvestmentFund{fund},
		findAllTotal:                     1,
		getAllPerformanceHistoriesResult: map[uint][]model.FundPerformance{1: snaps},
	}
	bankingClient := &fakeFundBankingClient{
		getAccountResult: &pb.GetAccountByNumberResponse{AvailableBalance: 500},
	}
	svc := newTestFundService(fundRepo, &fakeAssetOwnershipRepo{}, &fakeListingRepo{}, bankingClient, &fakeFundUserClient{})

	resp, err := svc.GetAllFunds(fundSupervisorCtx(), dto.ListFundsQuery{Page: 1, PageSize: 10})

	require.NoError(t, err)
	require.Len(t, resp.Data, 1)
	require.NotNil(t, resp.Data[0].AnnualReturn)
	require.NotNil(t, resp.Data[0].Volatility)
	require.NotNil(t, resp.Data[0].MaxDrawdown)
}

func TestGetAllFunds_MetricsNilWhenNotEnoughHistory(t *testing.T) {
	fund := model.InvestmentFund{
		FundID:        1,
		Name:          "Young Fund",
		AccountNumber: "ACC-002",
	}

	fundRepo := &fakeFundRepo{
		findAllResult: []model.InvestmentFund{fund},
		findAllTotal:  1,
		// nema dovoljno snimaka — fond se filtrira iz mape
		getAllPerformanceHistoriesResult: map[uint][]model.FundPerformance{},
	}
	bankingClient := &fakeFundBankingClient{
		getAccountResult: &pb.GetAccountByNumberResponse{AvailableBalance: 0},
	}
	svc := newTestFundService(fundRepo, &fakeAssetOwnershipRepo{}, &fakeListingRepo{}, bankingClient, &fakeFundUserClient{})

	resp, err := svc.GetAllFunds(fundSupervisorCtx(), dto.ListFundsQuery{Page: 1, PageSize: 10})

	require.NoError(t, err)
	require.Len(t, resp.Data, 1)
	require.Nil(t, resp.Data[0].AnnualReturn)
	require.Nil(t, resp.Data[0].Volatility)
	require.Nil(t, resp.Data[0].MaxDrawdown)
	require.Nil(t, resp.Data[0].RewardToVariability)
}

func TestGetAllFunds_SortByAnnualReturn(t *testing.T) {
	fund1 := model.InvestmentFund{FundID: 1, Name: "Slow Fund", AccountNumber: "ACC-001"}
	fund2 := model.InvestmentFund{FundID: 2, Name: "Fast Fund", AccountNumber: "ACC-002"}

	slowSnaps := makeSnapshots(func() []float64 {
		v := make([]float64, 13)
		v[0] = 1000
		for i := 1; i < 13; i++ {
			v[i] = v[i-1] * 1.01
		}
		return v
	}())
	fastSnaps := makeSnapshots(func() []float64 {
		v := make([]float64, 13)
		v[0] = 1000
		for i := 1; i < 13; i++ {
			v[i] = v[i-1] * 1.10
		}
		return v
	}())

	fundRepo := &fakeFundRepo{
		findAllResult: []model.InvestmentFund{fund1, fund2},
		findAllTotal:  2,
		getAllPerformanceHistoriesResult: map[uint][]model.FundPerformance{
			1: slowSnaps,
			2: fastSnaps,
		},
	}
	bankingClient := &fakeFundBankingClient{
		getAccountResult: &pb.GetAccountByNumberResponse{AvailableBalance: 0},
	}
	svc := newTestFundService(fundRepo, &fakeAssetOwnershipRepo{}, &fakeListingRepo{}, bankingClient, &fakeFundUserClient{})

	resp, err := svc.GetAllFunds(fundSupervisorCtx(), dto.ListFundsQuery{
		Page: 1, PageSize: 10,
		SortBy:  "annual_return",
		SortDir: "desc",
	})

	require.NoError(t, err)
	require.Len(t, resp.Data, 2)
	// Fast Fund treba biti prvi (veći godišnji prinos)
	require.Equal(t, "Fast Fund", resp.Data[0].Name)
	require.Equal(t, "Slow Fund", resp.Data[1].Name)
}

func TestGetFundDetail_IncludesMetricsAndAverageHistory(t *testing.T) {
	values := make([]float64, 13)
	values[0] = 1000.0
	for i := 1; i < 13; i++ {
		values[i] = values[i-1] * 1.05
	}

	snaps := makeSnapshots(values)
	reversed := make([]model.FundPerformance, len(snaps))
	for i, s := range snaps {
		reversed[len(snaps)-1-i] = s
	}

	fund := &model.InvestmentFund{
		FundID:        1,
		Name:          "Detail Fund",
		AccountNumber: "ACC-001",
		ManagerID:     10,
	}
	fundRepo := &fakeFundRepo{
		findByIDResult:              fund,
		getPerformanceHistoryResult: reversed,
		getAllPerformanceHistoriesResult: map[uint][]model.FundPerformance{
			1: snaps,
			2: makeSnapshots(values),
		},
	}
	bankingClient := &fakeFundBankingClient{
		getAccountResult: &pb.GetAccountByNumberResponse{AvailableBalance: 5000},
	}
	userClient := &fakeUserClient{}
	svc := newTestFundServiceWithListing(fundRepo, &fakeListingRepo{}, bankingClient, userClient)

	resp, err := svc.GetFundDetail(context.Background(), 1)

	require.NoError(t, err)
	require.NotNil(t, resp.AnnualReturn)
	require.NotNil(t, resp.Volatility)
	require.NotNil(t, resp.MaxDrawdown)
	require.NotEmpty(t, resp.AverageMarketHistory)
}
