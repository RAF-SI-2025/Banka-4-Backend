package seed

import (
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"gorm.io/gorm"
)

func InvestmentFunds(db *gorm.DB) error {
	now := time.Now()

	funds := []model.InvestmentFund{
		{
			Name:                "Alpha Growth Fund",
			Description:         "Fond fokusiran na IT sektor sa agresivnom strategijom rasta.",
			MinimumContribution: 1000.00,
			ManagerID:           3,
			AccountNumber:       "444000000000000010",
			CreatedAt:           now,
		},
		{
			Name:                "Beta Stable Fund",
			Description:         "Konzervativni fond fokusiran na stabilne prihode i obveznice.",
			MinimumContribution: 5000.00,
			ManagerID:           7,
			AccountNumber:       "444000000000000011",
			CreatedAt:           now,
		},
	}

	// Seed funds
	for i := range funds {
		if err := db.FirstOrCreate(&funds[i], model.InvestmentFund{Name: funds[i].Name}).Error; err != nil {
			return err
		}
		investment := &model.ClientFundInvestment{
			ClientID:      funds[i].ManagerID,
			OwnerType:     model.OwnerTypeActuary,
			FundID:        funds[i].FundID,
			AccountNumber: funds[i].AccountNumber,
			Amount:        1500000,
			CurrencyCode:  "RSD",
			CreatedAt:     now,
		}
		if err := db.FirstOrCreate(
			investment,
			model.ClientFundInvestment{
				ClientID:  funds[i].ManagerID,
				OwnerType: model.OwnerTypeActuary,
				FundID:    funds[i].FundID,
			},
		).Error; err != nil {
			return err
		}

		position := &model.ClientFundPosition{
			ClientID:            funds[i].ManagerID,
			OwnerType:           model.OwnerTypeActuary,
			FundID:              funds[i].FundID,
			UnitsOwned:          1500000,
			TotalInvestedAmount: 1500000,
			UpdatedAt:           now,
		}

		if err := db.FirstOrCreate(
			position,
			model.ClientFundPosition{
				ClientID:  funds[i].ManagerID,
				OwnerType: model.OwnerTypeActuary,
				FundID:    funds[i].FundID,
			},
		).Error; err != nil {
			return err
		}
	}

	// Seed performance history.
	// 13 monthly snapshots per fund (MinSnapshotsForMetrics = 12).
	// Values are hand-crafted with realistic oscillations so that all 4 metrics
	// are non-trivial:
	//   Alpha: ~73% annual return, ~14% volatility, RtV ~5.2, max drawdown -7.14%
	//   Beta:  ~11% annual return,  ~0.7% volatility, RtV ~16.4, max drawdown  0%

	type perfSeed struct {
		fundName string
		// values[0] = 12 months ago (oldest), values[12] = current month (newest)
		values       []float64
		liquidRatios []float64
	}

	perfSeeds := []perfSeed{
		{
			fundName: "Alpha Growth Fund",
			// Aggressive growth with one drawdown in month 8 (index 7->8)
			// Expected: annual_return~73%, volatility~14%, RtV~5.2, max_drawdown~-7.14%
			values: []float64{
				1500000.00, // month -12
				1590000.00, // month -11  +6.0%
				1650000.00, // month -10  +3.8%
				1750000.00, // month  -9  +6.1%
				1820000.00, // month  -8  +4.0%
				1900000.00, // month  -7  +4.4%
				2050000.00, // month  -6  +7.9%
				2100000.00, // month  -5  +2.4%
				1950000.00, // month  -4  -7.1%  <- drawdown
				2100000.00, // month  -3  +7.7%
				2300000.00, // month  -2  +9.5%
				2450000.00, // month  -1  +6.5%
				2600000.00, // month   0  +6.1%
			},
			liquidRatios: []float64{
				0.15, 0.15, 0.14, 0.16, 0.15, 0.14, 0.16, 0.15,
				0.20, 0.17, 0.15, 0.14, 0.15,
			},
		},
		{
			fundName: "Beta Stable Fund",
			// Conservative steady growth, very low volatility
			// Expected: annual_return~11%, volatility~0.7%, RtV~16.4, max_drawdown~0%
			values: []float64{
				1500000.00, // month -12
				1510000.00, // month -11  +0.67%
				1528000.00, // month -10  +1.19%
				1535000.00, // month  -9  +0.46%
				1550000.00, // month  -8  +0.98%
				1562000.00, // month  -7  +0.77%
				1578000.00, // month  -6  +1.02%
				1590000.00, // month  -5  +0.76%
				1608000.00, // month  -4  +1.13%
				1622000.00, // month  -3  +0.87%
				1635000.00, // month  -2  +0.80%
				1650000.00, // month  -1  +0.92%
				1665000.00, // month   0  +0.91%
			},
			liquidRatios: []float64{
				0.30, 0.30, 0.31, 0.30, 0.31, 0.30, 0.30, 0.31,
				0.30, 0.30, 0.31, 0.30, 0.30,
			},
		},
	}

	totalInvested := 1500000.0

	for _, ps := range perfSeeds {
		var fund model.InvestmentFund
		if err := db.First(&fund, "name = ?", ps.fundName).Error; err != nil {
			return err
		}

		// Skip if already enough snapshots
		var count int64
		if err := db.Model(&model.FundPerformance{}).
			Where("fund_id = ?", fund.FundID).
			Count(&count).Error; err != nil {
			return err
		}
		if count >= 12 {
			continue
		}

		// values[0] = oldest (12 months ago), values[12] = newest (now)
		for i, value := range ps.values {
			monthsAgo := 12 - i
			date := now.AddDate(0, -monthsAgo, 0)
			liquid := value * ps.liquidRatios[i]
			profit := value - totalInvested

			fp := model.FundPerformance{
				FundID:       fund.FundID,
				Date:         date,
				FundValue:    value,
				LiquidAssets: liquid,
				Profit:       profit,
			}
			if err := db.FirstOrCreate(
				&fp,
				model.FundPerformance{
					FundID: fund.FundID,
					Date:   date,
				},
			).Error; err != nil {
				return err
			}
		}
	}

	return nil
}
