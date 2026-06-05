package repository

import (
	"context"
	"errors"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"gorm.io/gorm"
)

type watchlistRepository struct {
	db *gorm.DB
}

func NewWatchlistRepository(db *gorm.DB) WatchlistRepository {
	return &watchlistRepository{db: db}
}

func (r *watchlistRepository) Create(ctx context.Context, watchlist *model.Watchlist) error {
	return r.db.WithContext(ctx).Create(watchlist).Error
}

func (r *watchlistRepository) FindByID(ctx context.Context, id uint) (*model.Watchlist, error) {
	var watchlist model.Watchlist
	err := r.db.WithContext(ctx).First(&watchlist, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &watchlist, nil
}

func (r *watchlistRepository) FindByOwnerAndName(ctx context.Context, userID uint, ownerType model.OwnerType, name string) (*model.Watchlist, error) {
	var watchlist model.Watchlist
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND owner_type = ? AND name = ?", userID, ownerType, name).
		First(&watchlist).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &watchlist, nil
}

func (r *watchlistRepository) FindByOwner(ctx context.Context, userID uint, ownerType model.OwnerType) ([]model.Watchlist, error) {
	var watchlists []model.Watchlist
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND owner_type = ?", userID, ownerType).
		Preload("Items").
		Order("created_at ASC, watchlist_id ASC").
		Find(&watchlists).Error
	return watchlists, err
}

func (r *watchlistRepository) FindDetail(ctx context.Context, id uint, assetType *model.AssetType) (*model.Watchlist, error) {
	watchlist, err := r.FindByID(ctx, id)
	if err != nil || watchlist == nil {
		return nil, err
	}

	itemsQuery := r.db.WithContext(ctx).
		Where("watchlist_items.watchlist_id = ?", id).
		Preload("Listing.Asset").
		Preload("Listing.Exchange").
		Order("watchlist_items.created_at ASC, watchlist_items.watchlist_item_id ASC")

	if assetType != nil {
		itemsQuery = itemsQuery.
			Joins("INNER JOIN listings ON listings.listing_id = watchlist_items.listing_id").
			Joins("INNER JOIN assets ON assets.asset_id = listings.asset_id").
			Where("assets.asset_type = ?", *assetType)
	}

	var items []model.WatchlistItem
	if err := itemsQuery.Find(&items).Error; err != nil {
		return nil, err
	}

	listingIDs := make([]uint, 0, len(items))
	for _, item := range items {
		listingIDs = append(listingIDs, item.ListingID)
	}

	latest, err := r.latestDailyForListings(ctx, listingIDs)
	if err != nil {
		return nil, err
	}

	for i := range items {
		if items[i].Listing == nil {
			continue
		}
		if info, ok := latest[items[i].ListingID]; ok {
			items[i].Listing.DailyPriceInfos = []model.ListingDailyPriceInfo{info}
		}
	}

	watchlist.Items = items
	return watchlist, nil
}

// latestDailyForListings returns the most recent daily price info per listing,
// keyed by listing id. It uses a correlated subquery so it stays portable
// across SQLite and PostgreSQL (matching the existing listing repository).
func (r *watchlistRepository) latestDailyForListings(ctx context.Context, listingIDs []uint) (map[uint]model.ListingDailyPriceInfo, error) {
	result := make(map[uint]model.ListingDailyPriceInfo)
	if len(listingIDs) == 0 {
		return result, nil
	}

	var infos []model.ListingDailyPriceInfo
	err := r.db.WithContext(ctx).
		Where("listing_id IN ?", listingIDs).
		Where(`date = (
			SELECT MAX(d.date)
			FROM listing_daily_price_infos d
			WHERE d.listing_id = listing_daily_price_infos.listing_id
		)`).
		Find(&infos).Error
	if err != nil {
		return nil, err
	}

	for _, info := range infos {
		result[info.ListingID] = info
	}
	return result, nil
}

func (r *watchlistRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("watchlist_id = ?", id).Delete(&model.WatchlistItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Watchlist{}, id).Error
	})
}

func (r *watchlistRepository) AddItem(ctx context.Context, item *model.WatchlistItem) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *watchlistRepository) FindItem(ctx context.Context, watchlistID, listingID uint) (*model.WatchlistItem, error) {
	var item model.WatchlistItem
	err := r.db.WithContext(ctx).
		Where("watchlist_id = ? AND listing_id = ?", watchlistID, listingID).
		First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *watchlistRepository) RemoveItem(ctx context.Context, watchlistID, listingID uint) (int64, error) {
	res := r.db.WithContext(ctx).
		Where("watchlist_id = ? AND listing_id = ?", watchlistID, listingID).
		Delete(&model.WatchlistItem{})
	return res.RowsAffected, res.Error
}
