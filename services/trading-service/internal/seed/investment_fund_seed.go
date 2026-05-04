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
	}

	// --- ASSUME first fund exists ---
	var fund model.InvestmentFund
	if err := db.First(&fund, "name = ?", "Alpha Growth Fund").Error; err != nil {
		return err
	}

	// -------------------------------
	// 1. Seed Asset Ownership (FUND owns assets)
	// -------------------------------
	assets := []model.AssetOwnership{
		{
			UserId:         fund.FundID, // fund acts as "user"
			OwnerType:      model.OwnerTypeFund,
			AssetID:        1,
			Amount:         100,
			AvgBuyPriceRSD: 1200,
			PublicAmount:   0,
			ReservedAmount: 0,
			UpdatedAt:      now,
		},
		{
			UserId:         fund.FundID,
			OwnerType:      model.OwnerTypeFund,
			AssetID:        2,
			Amount:         50,
			AvgBuyPriceRSD: 2500,
			PublicAmount:   0,
			ReservedAmount: 0,
			UpdatedAt:      now,
		},
	}

	for _, ao := range assets {
		if err := db.FirstOrCreate(
			&ao,
			model.AssetOwnership{
				UserId:    ao.UserId,
				OwnerType: ao.OwnerType,
				AssetID:   ao.AssetID,
			},
		).Error; err != nil {
			return err
		}
	}

	// -------------------------------
	// 2. Seed Fund Performance (last 3 days)
	// -------------------------------
	performances := []model.FundPerformance{
		{
			FundID:       fund.FundID,
			Date:         now.AddDate(0, 0, -2),
			FundValue:    200000,
			LiquidAssets: 20000,
			Profit:       5000,
			CreatedAt:    now,
		},
		{
			FundID:       fund.FundID,
			Date:         now.AddDate(0, 0, -1),
			FundValue:    210000,
			LiquidAssets: 25000,
			Profit:       8000,
			CreatedAt:    now,
		},
		{
			FundID:       fund.FundID,
			Date:         now,
			FundValue:    220000,
			LiquidAssets: 30000,
			Profit:       12000,
			CreatedAt:    now,
		},
	}

	var count int64
	err := db.Model(&model.FundPerformance{}).Count(&count).Error
	if err != nil {
		return err
	}

	if count == 0 {
		for _, fp := range performances {
			if err := db.FirstOrCreate(
				&fp,
				model.FundPerformance{
					FundID: fp.FundID,
					Date:   fp.Date,
				},
			).Error; err != nil {
				return err
			}
		}
	}

	return nil
}
