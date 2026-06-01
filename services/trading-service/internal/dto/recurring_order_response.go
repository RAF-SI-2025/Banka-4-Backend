package dto

import (
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type RecurringOrderResponse struct {
	RecurringOrderID uint                        `json:"recurring_order_id"`
	UserID           uint                        `json:"user_id"`
	OwnerType        model.OwnerType             `json:"owner_type"`
	ListingID        uint                        `json:"listing_id"`
	Ticker           string                      `json:"ticker"`
	ListingName      string                      `json:"listing_name"`
	Direction        model.OrderDirection        `json:"direction"`
	Mode             model.RecurringOrderMode    `json:"mode"`
	Value            float64                     `json:"value"`
	AccountNumber    string                      `json:"account_number"`
	Cadence          model.RecurringOrderCadence `json:"cadence"`
	NextRun          time.Time                   `json:"next_run"`
	Active           bool                        `json:"active"`
	CreatedAt        time.Time                   `json:"created_at"`
	UpdatedAt        time.Time                   `json:"updated_at"`
}

func ToRecurringOrderResponse(ro model.RecurringOrder) RecurringOrderResponse {
	return RecurringOrderResponse{
		RecurringOrderID: ro.RecurringOrderID,
		UserID:           ro.UserID,
		OwnerType:        ro.OwnerType,
		ListingID:        ro.ListingID,
		Ticker:           listingAssetTicker(ro.Listing),
		ListingName:      listingAssetName(ro.Listing),
		Direction:        ro.Direction,
		Mode:             ro.Mode,
		Value:            ro.Value,
		AccountNumber:    ro.AccountNumber,
		Cadence:          ro.Cadence,
		NextRun:          ro.NextRun,
		Active:           ro.Active,
		CreatedAt:        ro.CreatedAt,
		UpdatedAt:        ro.UpdatedAt,
	}
}

func ToRecurringOrderResponseList(orders []model.RecurringOrder) []RecurringOrderResponse {
	result := make([]RecurringOrderResponse, len(orders))
	for i, ro := range orders {
		result[i] = ToRecurringOrderResponse(ro)
	}
	return result
}
