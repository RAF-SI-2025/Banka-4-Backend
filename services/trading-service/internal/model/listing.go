package model

import (
	"time"
)

type Listing struct {
	ListingID         uint      `gorm:"primaryKey;autoIncrement"`
	AssetID           uint      `gorm:"not null;index"`
	Asset             *Asset    `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	ExchangeMIC       string    `gorm:"not null;size:10;index"`
	LastRefresh       time.Time `gorm:"not null"`
	Price             float64   `gorm:"not null;default:0"`
	Ask               float64   `gorm:"not null;default:0"`
	MaintenanceMargin float64   `gorm:"not null;default:0"`

	Exchange        *Exchange               `gorm:"foreignKey:ExchangeMIC;references:MicCode;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	Stock           *Stock                  `gorm:"foreignKey:AssetID;references:AssetID;constraint:-"`
	DailyPriceInfos []ListingDailyPriceInfo `gorm:"foreignKey:ListingID"`
}

type ListingDailyPriceInfo struct {
	ID        uint `gorm:"primaryKey;autoIncrement"`
	ListingID uint `gorm:"not null;index"`
	Listing   Listing
	Date      time.Time `gorm:"not null;index"`
	Price     float64   `gorm:"not null;default:0"`
	Ask       float64   `gorm:"not null;default:0"`
	Bid       float64   `gorm:"not null;default:0"`
	Change    float64   `gorm:"not null;default:0"`
	Volume    uint      `gorm:"not null;default:0"`
}
