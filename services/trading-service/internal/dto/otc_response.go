package dto

import (
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type OTCAssetResponse struct {
	AssetOwnershipID uint            `json:"asset_ownership_id"`
	Name             string          `json:"name"`
	Ticker           string          `json:"ticker"`
	SecurityType     model.AssetType `json:"security_type"`
	Price            float64         `json:"price"`
	Currency         string          `json:"currency"`
	AvailableAmount  float64         `json:"available_amount"`
	UpdatedAt        time.Time       `json:"updated_at"`
	OwnerName        string          `json:"owner_name"`
}
