package model

import "time"

type Watchlist struct {
	WatchlistID uint      `gorm:"primaryKey;autoIncrement"`
	UserID      uint      `gorm:"not null;uniqueIndex:idx_watchlist_owner_name"`
	OwnerType   OwnerType `gorm:"not null;size:10;uniqueIndex:idx_watchlist_owner_name"`
	Name        string    `gorm:"not null;size:100;uniqueIndex:idx_watchlist_owner_name"`
	CreatedAt   time.Time
	UpdatedAt   time.Time

	Items []WatchlistItem `gorm:"foreignKey:WatchlistID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
type WatchlistItem struct {
	WatchlistItemID uint      `gorm:"primaryKey;autoIncrement"`
	WatchlistID     uint      `gorm:"not null;uniqueIndex:idx_watchlist_item"`
	ListingID       uint      `gorm:"not null;uniqueIndex:idx_watchlist_item;index"`
	Listing         *Listing  `gorm:"foreignKey:ListingID;references:ListingID;constraint:-"`
	CreatedAt       time.Time `gorm:"not null"`
}
