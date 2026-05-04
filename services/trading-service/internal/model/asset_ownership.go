package model

import "time"

type OwnerType string

const (
	OwnerTypeClient  OwnerType = "CLIENT"
	OwnerTypeActuary OwnerType = "ACTUARY"
	OwnerTypeFund    OwnerType = "FUND"
)

type AssetOwnership struct {
	AssetOwnershipID uint      `gorm:"primaryKey;autoIncrement"`
	UserId           uint      `gorm:"not null;uniqueIndex:idx_ao_identity"`
	OwnerType        OwnerType `gorm:"not null;size:10;uniqueIndex:idx_ao_identity"`
	AssetID          uint      `gorm:"not null;uniqueIndex:idx_ao_identity"`
	Asset            Asset
	Amount           float64 `gorm:"not null;default:0"`
	AvgBuyPriceRSD   float64 `gorm:"not null;default:0"`
	PublicAmount     float64 `gorm:"not null;default:0"`
	ReservedAmount   float64 `gorm:"not null;default:0"`
	UpdatedAt        time.Time
}
