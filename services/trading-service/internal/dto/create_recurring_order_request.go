package dto

import "github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"

type CreateRecurringOrderRequest struct {
	ListingID     uint                        `json:"listing_id" binding:"required"`
	AccountNumber string                      `json:"account_number" binding:"required"`
	Direction     model.OrderDirection        `json:"direction" binding:"required,oneof=BUY SELL"`
	Mode          model.RecurringOrderMode    `json:"mode" binding:"required,oneof=BY_QUANTITY BY_AMOUNT"`
	Value         float64                     `json:"value" binding:"required,gt=0"`
	Cadence       model.RecurringOrderCadence `json:"cadence" binding:"required,oneof=DAILY WEEKLY MONTHLY"`
}
