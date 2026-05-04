package dto

import (
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type OrderResponse struct {
	OrderID           uint                 `json:"order_id"`
	UserID            uint                 `json:"user_id"`
	OwnerType         model.OwnerType      `json:"owner_type"`
	AccountNumber     string               `json:"account_number"`
	ListingID         uint                 `json:"listing_id"`
	Ticker            string               `json:"ticker"`
	ListingName       string               `json:"listing_name"`
	OrderType         model.OrderType      `json:"order_type"`
	Direction         model.OrderDirection `json:"direction"`
	Quantity          uint                 `json:"quantity"`
	ContractSize      float64              `json:"contract_size"`
	PricePerUnit      *float64             `json:"price_per_unit"`
	LimitValue        *float64             `json:"limit_value,omitempty"`
	StopValue         *float64             `json:"stop_value,omitempty"`
	AllOrNone         bool                 `json:"all_or_none"`
	Margin            bool                 `json:"margin"`
	Status            model.OrderStatus    `json:"status"`
	ApprovedBy        *uint                `json:"approved_by,omitempty"`
	IsDone            bool                 `json:"is_done"`
	AfterHours        bool                 `json:"after_hours"`
	RemainingPortions uint                 `json:"remaining_portions"`
	CreatedAt         time.Time            `json:"created_at"`
	UpdatedAt         time.Time            `json:"updated_at"`
}

func listingAssetTicker(l model.Listing) string {
	if l.Asset != nil {
		return l.Asset.Ticker
	}
	return ""
}

func listingAssetName(l model.Listing) string {
	if l.Asset != nil {
		return l.Asset.Name
	}
	return ""
}

func listingAssetType(l model.Listing) model.AssetType {
	if l.Asset != nil {
		return l.Asset.AssetType
	}
	return ""
}

func ToOrderResponse(o model.Order) OrderResponse {
	return OrderResponse{
		OrderID:           o.OrderID,
		UserID:            o.OrderOwnerUserID,
		OwnerType:         o.OrderOwnerType,
		AccountNumber:     o.AccountNumber,
		ListingID:         o.ListingID,
		Ticker:            listingAssetTicker(o.Listing),
		ListingName:       listingAssetName(o.Listing),
		OrderType:         o.OrderType,
		Direction:         o.Direction,
		Quantity:          o.Quantity,
		ContractSize:      o.ContractSize,
		PricePerUnit:      o.PricePerUnit,
		LimitValue:        o.LimitValue,
		StopValue:         o.StopValue,
		AllOrNone:         o.AllOrNone,
		Margin:            o.Margin,
		Status:            o.Status,
		ApprovedBy:        o.ApprovedBy,
		IsDone:            o.IsDone,
		AfterHours:        o.AfterHours,
		RemainingPortions: o.RemainingPortions(),
		CreatedAt:         o.CreatedAt,
		UpdatedAt:         o.UpdatedAt,
	}
}

func ToOrderResponseList(orders []model.Order) []OrderResponse {
	result := make([]OrderResponse, len(orders))
	for i, o := range orders {
		result[i] = ToOrderResponse(o)
	}
	return result
}
