package seed

import (
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"gorm.io/gorm"
)

func SeedAssetOwnerships(db *gorm.DB) error {
	now := time.Now()

	assets := []model.AssetOwnership{
		{
			UserId:         1,
			OwnerType:      model.OwnerTypeClient,
			AssetID:        1,
			Amount:         100,
			AvgBuyPriceRSD: 1200,
			PublicAmount:   0,
			ReservedAmount: 0,
			UpdatedAt:      now,
		},
		{
			UserId:         1,
			OwnerType:      model.OwnerTypeClient,
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

	return nil
}
