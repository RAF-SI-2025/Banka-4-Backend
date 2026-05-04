package dto

import "github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"

type OrderSummaryResponse struct {
	OrderID           uint                 `json:"order_id"`
	UserID            uint                 `json:"user_id"`
	OwnerType         model.OwnerType      `json:"owner_type"`
	AccountNumber     string               `json:"account_number"`
	ListingName       string               `json:"listing_name"`
	AssetType         model.AssetType      `json:"asset_type"`
	OrderType         model.OrderType      `json:"order_type"`
	Quantity          uint                 `json:"quantity"`
	ContractSize      float64              `json:"contract_size"`
	PricePerUnit      *float64             `json:"price_per_unit"`
	Direction         model.OrderDirection `json:"direction"`
	RemainingPortions uint                 `json:"remaining_portions"`
	Status            model.OrderStatus    `json:"status"`
}

func ToOrderSummaryResponse(o model.Order) OrderSummaryResponse {
	return OrderSummaryResponse{
		OrderID:           o.OrderID,
		UserID:            o.OrderOwnerUserID,
		OwnerType:         o.OrderOwnerType,
		AccountNumber:     o.AccountNumber,
		ListingName:       listingAssetName(o.Listing),
		AssetType:         listingAssetType(o.Listing),
		OrderType:         o.OrderType,
		Quantity:          o.Quantity,
		ContractSize:      o.ContractSize,
		PricePerUnit:      o.PricePerUnit,
		Direction:         o.Direction,
		RemainingPortions: o.RemainingPortions(),
		Status:            o.Status,
	}
}

func ToOrderSummaryResponseList(orders []model.Order) []OrderSummaryResponse {
	result := make([]OrderSummaryResponse, len(orders))
	for i, o := range orders {
		result[i] = ToOrderSummaryResponse(o)
	}
	return result
}

type ListOrdersResponse struct {
	Data     []OrderSummaryResponse `json:"data"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}
