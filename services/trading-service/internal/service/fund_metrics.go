package service

import (
	"math"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

const MinSnapshotsForMetrics = 12

type FundMetrics struct {
	AnnualReturn        *float64
	RewardToVariability *float64
	MaxDrawdown         *float64
	Volatility          *float64
}

func calculateFundMetrics(snapshots []model.FundPerformance) FundMetrics {
	if len(snapshots) < MinSnapshotsForMetrics {
		return FundMetrics{}
	}

	monthlyReturns := make([]float64, 0, len(snapshots)-1)
	for i := 1; i < len(snapshots); i++ {
		prev := snapshots[i-1].FundValue
		curr := snapshots[i].FundValue
		if prev <= 0 {
			continue
		}
		monthlyReturns = append(monthlyReturns, (curr-prev)/prev)
	}

	if len(monthlyReturns) == 0 {
		return FundMetrics{}
	}

	first := snapshots[0].FundValue
	last := snapshots[len(snapshots)-1].FundValue
	nMonths := float64(len(snapshots) - 1)
	var annualReturn *float64
	if first > 0 && nMonths > 0 {
		ar := (math.Pow(last/first, 12.0/nMonths) - 1) * 100
		annualReturn = &ar
	}

	mean := 0.0
	for _, r := range monthlyReturns {
		mean += r
	}
	mean /= float64(len(monthlyReturns))

	variance := 0.0
	for _, r := range monthlyReturns {
		diff := r - mean
		variance += diff * diff
	}
	variance /= float64(len(monthlyReturns))
	monthlyStdDev := math.Sqrt(variance)
	annualizedVol := monthlyStdDev * math.Sqrt(12) * 100

	var volatility *float64
	if !math.IsNaN(annualizedVol) && !math.IsInf(annualizedVol, 0) {
		volatility = &annualizedVol
	}

	var rewardToVariability *float64
	if volatility != nil && *volatility > 0 && annualReturn != nil {
		rtv := *annualReturn / *volatility
		rewardToVariability = &rtv
	}

	var maxDrawdown *float64
	peak := snapshots[0].FundValue
	maxDD := 0.0
	for _, s := range snapshots[1:] {
		if s.FundValue > peak {
			peak = s.FundValue
		}
		if peak > 0 {
			dd := (peak - s.FundValue) / peak * 100
			if dd > maxDD {
				maxDD = dd
			}
		}
	}
	negMaxDD := -maxDD
	maxDrawdown = &negMaxDD

	return FundMetrics{
		AnnualReturn:        annualReturn,
		RewardToVariability: rewardToVariability,
		MaxDrawdown:         maxDrawdown,
		Volatility:          volatility,
	}
}

func averagePerformanceHistory(allHistories map[uint][]model.FundPerformance) []model.FundPerformance {
	if len(allHistories) == 0 {
		return nil
	}

	maxLen := 0
	for _, h := range allHistories {
		if len(h) > maxLen {
			maxLen = len(h)
		}
	}

	result := make([]model.FundPerformance, maxLen)
	counts := make([]int, maxLen)

	for _, snapshots := range allHistories {
		for i, s := range snapshots {
			result[i].FundValue += s.FundValue
			result[i].Profit += s.Profit
			result[i].LiquidAssets += s.LiquidAssets
			result[i].Date = s.Date
			counts[i]++
		}
	}

	for i := range result {
		if counts[i] > 0 {
			result[i].FundValue /= float64(counts[i])
			result[i].Profit /= float64(counts[i])
			result[i].LiquidAssets /= float64(counts[i])
		}
	}

	return result
}
