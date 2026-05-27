package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

// WatchlistRepository persists watchlists and their items.
type WatchlistRepository interface {
	// Create persists a new watchlist (without items).
	Create(ctx context.Context, watchlist *model.Watchlist) error
	// FindByID loads a single watchlist by its primary key, without items.
	// Returns (nil, nil) when no such watchlist exists.
	FindByID(ctx context.Context, id uint) (*model.Watchlist, error)
	// FindByOwnerAndName loads a watchlist by owner identity and name, used to
	// reject duplicate names. Returns (nil, nil) when none matches.
	FindByOwnerAndName(ctx context.Context, userID uint, ownerType model.OwnerType, name string) (*model.Watchlist, error)
	// FindByOwner lists every watchlist owned by the given identity, with their
	// items preloaded so the caller can report item counts.
	FindByOwner(ctx context.Context, userID uint, ownerType model.OwnerType) ([]model.Watchlist, error)
	// FindDetail loads a watchlist together with its items fully hydrated with
	// listing data (asset, stock and the latest daily price info). When
	// assetType is non-nil, only items of that asset type are returned.
	FindDetail(ctx context.Context, id uint, assetType *model.AssetType) (*model.Watchlist, error)
	// Delete removes a watchlist (cascading to its items).
	Delete(ctx context.Context, id uint) error

	// AddItem adds a listing to a watchlist.
	AddItem(ctx context.Context, item *model.WatchlistItem) error
	// FindItem loads the item linking the given listing to the given watchlist.
	// Returns (nil, nil) when the listing is not in the watchlist.
	FindItem(ctx context.Context, watchlistID, listingID uint) (*model.WatchlistItem, error)
	// RemoveItem removes a listing from a watchlist. Returns the number of
	// deleted rows so callers can distinguish "not present" from success.
	RemoveItem(ctx context.Context, watchlistID, listingID uint) (int64, error)
}
