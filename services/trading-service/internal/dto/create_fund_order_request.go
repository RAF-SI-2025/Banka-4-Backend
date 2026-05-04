package dto

import "github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"

type CreateFundOrderRequest struct {
	FundID     uint                 `json:"fund_id"     binding:"required"`
	ListingID  uint                 `json:"listing_id"  binding:"required"`
	OrderType  model.OrderType      `json:"order_type"  binding:"required,oneof=MARKET LIMIT STOP STOP_LIMIT"`
	Direction  model.OrderDirection `json:"direction"   binding:"required,oneof=BUY SELL"`
	Quantity   uint                 `json:"quantity"    binding:"required,min=1"`
	LimitValue *float64             `json:"limit_value,omitempty"`
	StopValue  *float64             `json:"stop_value,omitempty"`
	AllOrNone  bool                 `json:"all_or_none"`
	Margin     bool                 `json:"margin"`
}
