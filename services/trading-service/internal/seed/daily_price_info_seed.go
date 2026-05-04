package seed

import (
	"log"
	"math/rand"
	"time"

	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

func SeedDailyPriceHistory(db *gorm.DB, daysBack int) error {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	var listings []model.Listing
	if err := db.Find(&listings).Error; err != nil {
		return err
	}

	endDate := time.Now().Truncate(24*time.Hour).AddDate(0, 0, -1) // yesterday
	startDate := endDate.AddDate(0, 0, -daysBack+1)

	for _, listing := range listings {
		if err := db.Where("listing_id = ?", listing.ListingID).Delete(&model.ListingDailyPriceInfo{}).Error; err != nil {
			return err
		}

		dates := make([]time.Time, daysBack)
		for i := 0; i < daysBack; i++ {
			dates[i] = startDate.AddDate(0, 0, i)
		}

		// Generate prices backwards from the current listing price
		currentPrice := listing.Price
		if currentPrice <= 0 {
			currentPrice = 100.0
		}
		prices := make([]float64, daysBack)
		prices[daysBack-1] = currentPrice

		const volatility = 0.03
		for i := daysBack - 2; i >= 0; i-- {
			returnPct := (rng.Float64()*2 - 1) * volatility
			prevPrice := prices[i+1] / (1 + returnPct)
			if prevPrice <= 0 {
				prevPrice = prices[i+1] * 0.5
			}
			prices[i] = prevPrice
		}

		var dailyInfos []model.ListingDailyPriceInfo
		for idx, date := range dates {
			price := prices[idx]

			var change float64
			if idx == 0 {
				change = 0
			} else {
				change = price - prices[idx-1]
			}

			spread := 0.001 + rng.Float64()*0.007
			ask := price * (1 + spread/2)
			bid := price * (1 - spread/2)

			volume := uint(1000 + rng.Int63n(199000))

			dailyInfos = append(dailyInfos, model.ListingDailyPriceInfo{
				ListingID: listing.ListingID,
				Date:      date,
				Price:     price,
				Ask:       ask,
				Bid:       bid,
				Change:    change,
				Volume:    volume,
			})
		}

		if err := db.CreateInBatches(dailyInfos, 100).Error; err != nil {
			return err
		}
		log.Printf("Seeded %d daily price records for listing %d", len(dailyInfos), listing.ListingID)
	}

	return nil
}
