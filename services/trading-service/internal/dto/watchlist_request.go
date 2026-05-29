package dto

// CreateWatchlistRequest is the body for creating a new watchlist.
type CreateWatchlistRequest struct {
	Name string `json:"name" binding:"required,min=1,max=100"`
}

// AddWatchlistItemRequest is the body for adding a listing to a watchlist.
type AddWatchlistItemRequest struct {
	ListingID uint `json:"listing_id" binding:"required"`
}
