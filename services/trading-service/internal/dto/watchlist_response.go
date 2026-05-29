package dto

import "time"

// WatchlistResponse is a watchlist summary returned by the list endpoint.
type WatchlistResponse struct {
	WatchlistID uint      `json:"watchlist_id"`
	Name        string    `json:"name"`
	ItemCount   int       `json:"item_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// WatchlistListingResponse is a tracked listing enriched with the same market
// data exposed by the listing endpoints (price, ask, bid, daily change and
// volume), plus the asset type and the time it was added to the watchlist.
type WatchlistListingResponse struct {
	BaseListingResponse
	AssetType string    `json:"asset_type"`
	AddedAt   time.Time `json:"added_at"`
}

// WatchlistDetailResponse is a single watchlist with all of its tracked
// listings' data.
type WatchlistDetailResponse struct {
	WatchlistID uint                       `json:"watchlist_id"`
	Name        string                     `json:"name"`
	CreatedAt   time.Time                  `json:"created_at"`
	Listings    []WatchlistListingResponse `json:"listings"`
}
