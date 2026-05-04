package repository

import (
	"context"
	"errors"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"gorm.io/gorm"
)

type listingRepository struct {
	db *gorm.DB
}

func NewListingRepository(db *gorm.DB) ListingRepository {
	return &listingRepository{db: db}
}

func (r *listingRepository) FindAll(ctx context.Context) ([]model.Listing, error) {
	var listings []model.Listing
	if err := r.db.WithContext(ctx).Preload("Asset").Find(&listings).Error; err != nil {
		return nil, err
	}
	return listings, nil
}

func (r *listingRepository) FindByID(ctx context.Context, id uint, daysBack int) (*model.Listing, error) {
	var listing model.Listing

	result := r.db.WithContext(ctx).
		Preload("Asset").
		Preload("Stock").
		Preload("DailyPriceInfos", func(db *gorm.DB) *gorm.DB {
			q := db.Order("date ASC")
			if daysBack >= 0 {
				now := time.Now().UTC()
				since := time.Date(
					now.Year(), now.Month(), now.Day(),
					0, 0, 0, 0, now.Location(),
				).AddDate(0, 0, -daysBack)
				q = q.Where("listing_daily_price_infos.date >= ?", since)
			}
			return q
		}).
		First(&listing, id)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &listing, result.Error
}

func (r *listingRepository) FindLatestDailyPriceInfo(ctx context.Context, listingID uint) (*model.ListingDailyPriceInfo, error) {
	var dailyInfo model.ListingDailyPriceInfo
	result := r.db.WithContext(ctx).
		Where("listing_id = ?", listingID).
		Order("date DESC").
		First(&dailyInfo)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &dailyInfo, result.Error
}

func (r *listingRepository) Upsert(ctx context.Context, listing *model.Listing) error {
	return r.db.WithContext(ctx).
		Where(model.Listing{AssetID: listing.AssetID}).
		Assign(*listing).
		FirstOrCreate(listing).Error
}

func (r *listingRepository) UpdatePriceAndAsk(ctx context.Context, listing *model.Listing, price, ask float64) error {
	return r.db.WithContext(ctx).Model(listing).Updates(map[string]interface{}{
		"price":        price,
		"ask":          ask,
		"last_refresh": time.Now(),
	}).Error
}

func (r *listingRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.Listing{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func joinAssets(q *gorm.DB) *gorm.DB {
	return q.Joins("INNER JOIN assets ON assets.asset_id = listings.asset_id")
}

func joinLatestDaily(q *gorm.DB) *gorm.DB {
	return q.Joins(`
		LEFT JOIN listing_daily_price_infos AS ldpi
		ON ldpi.listing_id = listings.listing_id
		AND ldpi.date = (
			SELECT MAX(d.date)
			FROM listing_daily_price_infos d
			WHERE d.listing_id = listings.listing_id
		)
	`)
}

func applyListingFilters(q *gorm.DB, filter ListingFilter) *gorm.DB {
	if filter.Search != "" {
		like := "%" + filter.Search + "%"
		q = q.Where("assets.ticker LIKE ? OR assets.name LIKE ?", like, like)
	}
	if filter.Exchange != "" {
		q = q.Where("listings.exchange_mic LIKE ?", filter.Exchange+"%")
	}
	if filter.PriceMin > 0 {
		q = q.Where("listings.price >= ?", filter.PriceMin)
	}
	if filter.PriceMax > 0 {
		q = q.Where("listings.price <= ?", filter.PriceMax)
	}
	if filter.AskMin > 0 {
		q = q.Where("listings.ask >= ?", filter.AskMin)
	}
	if filter.AskMax > 0 {
		q = q.Where("listings.ask <= ?", filter.AskMax)
	}
	if filter.BidMin > 0 {
		q = q.Where("ldpi.bid >= ?", filter.BidMin)
	}
	if filter.BidMax > 0 {
		q = q.Where("ldpi.bid <= ?", filter.BidMax)
	}
	if filter.VolumeMin > 0 {
		q = q.Where("ldpi.volume >= ?", filter.VolumeMin)
	}
	if filter.VolumeMax > 0 {
		q = q.Where("ldpi.volume <= ?", filter.VolumeMax)
	}
	return q
}

func sortColumn(filter ListingFilter) string {
	col := "listings.price"
	switch filter.SortBy {
	case "volume":
		col = "ldpi.volume"
	case "maintenance_margin":
		col = "listings.maintenance_margin"
	}
	dir := "ASC"
	if filter.SortDir == "desc" {
		dir = "DESC"
	}
	return col + " " + dir
}

func (r *listingRepository) FindStocks(ctx context.Context, filter ListingFilter) ([]model.Listing, int64, error) {
	var listings []model.Listing
	var count int64

	db := r.db.WithContext(ctx).
		Model(&model.Listing{})

	db = joinAssets(db)
	db = db.Joins("INNER JOIN stocks ON stocks.asset_id = assets.asset_id")
	db = joinLatestDaily(db)
	db = applyListingFilters(db, filter)

	if err := db.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	err := db.
		Preload("Asset").
		Preload("Stock").
		Preload("DailyPriceInfos", func(db *gorm.DB) *gorm.DB {
			return db.Order("date DESC").Limit(1)
		}).
		Order(sortColumn(filter)).
		Limit(filter.PageSize).
		Offset((filter.Page - 1) * filter.PageSize).
		Find(&listings).Error

	return listings, count, err
}

func (r *listingRepository) FindFutures(ctx context.Context, filter ListingFilter) ([]model.Listing, int64, error) {
	var listings []model.Listing
	var count int64

	db := r.db.WithContext(ctx).
		Model(&model.Listing{})

	db = joinAssets(db)
	db = db.Joins("INNER JOIN futures_contracts ON futures_contracts.asset_id = assets.asset_id")
	db = joinLatestDaily(db)
	db = applyListingFilters(db, filter)

	// FIX: zamena PostgreSQL-specifičnog ::date cast-a sa date range
	if filter.SettlementDate != nil {
		start := filter.SettlementDate.Truncate(24 * time.Hour)
		end := start.Add(24 * time.Hour)
		db = db.Where("futures_contracts.settlement_date >= ? AND futures_contracts.settlement_date < ?", start, end)
	}

	if err := db.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	err := db.
		Preload("Asset").
		Preload("DailyPriceInfos", func(db *gorm.DB) *gorm.DB {
			return db.Order("date DESC").Limit(1)
		}).
		Order(sortColumn(filter)).
		Limit(filter.PageSize).
		Offset((filter.Page - 1) * filter.PageSize).
		Find(&listings).Error

	return listings, count, err
}

func (r *listingRepository) FindOptions(ctx context.Context, filter ListingFilter) ([]model.Listing, int64, error) {
	var listings []model.Listing
	var count int64

	db := r.db.WithContext(ctx).
		Model(&model.Listing{})

	db = joinAssets(db)
	db = db.Joins("INNER JOIN options ON options.asset_id = assets.asset_id")
	db = joinLatestDaily(db)
	db = applyListingFilters(db, filter)

	if filter.SettlementDate != nil {
		start := filter.SettlementDate.Truncate(24 * time.Hour)
		end := start.Add(24 * time.Hour)
		db = db.Where("options.settlement_date >= ? AND options.settlement_date < ?", start, end)
	}

	if err := db.Count(&count).Error; err != nil {
		return nil, 0, err
	}

	err := db.
		Preload("Asset").
		Preload("DailyPriceInfos", func(db *gorm.DB) *gorm.DB {
			return db.Order("date DESC").Limit(1)
		}).
		Order(sortColumn(filter)).
		Limit(filter.PageSize).
		Offset((filter.Page - 1) * filter.PageSize).
		Find(&listings).Error

	return listings, count, err
}

func (r *listingRepository) CreateDailyPriceInfo(ctx context.Context, info *model.ListingDailyPriceInfo) error {
	return r.db.WithContext(ctx).Create(info).Error
}

func (r *listingRepository) FindLastDailyPriceInfo(ctx context.Context, listingID uint, beforeDate time.Time) (*model.ListingDailyPriceInfo, error) {
	var info model.ListingDailyPriceInfo
	err := r.db.WithContext(ctx).
		Where("listing_id = ? AND date < ?", listingID, beforeDate).
		Order("date DESC").
		First(&info).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &info, err
}

func (r *listingRepository) FindByAssetIDs(ctx context.Context, assetIDs []uint) ([]model.Listing, error) {
	var listings []model.Listing
	err := r.db.WithContext(ctx).
		Where("asset_id IN ?", assetIDs).
		Preload("Asset").
		Preload("Exchange").
		Preload("DailyPriceInfos", func(db *gorm.DB) *gorm.DB {
			return db.Order("date DESC").Limit(1)
		}).
		Find(&listings).Error
	return listings, err
}

func (r *listingRepository) FindByAssetType(ctx context.Context, assetType model.AssetType) ([]model.Listing, error) {
	var listings []model.Listing
	err := r.db.WithContext(ctx).
		Joins("INNER JOIN assets ON assets.asset_id = listings.asset_id").
		Where("assets.asset_type = ?", assetType).
		Preload("Asset").
		Find(&listings).Error
	return listings, err
}
