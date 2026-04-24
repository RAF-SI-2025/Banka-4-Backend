package dto

import (
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type CreateOtcContractRequest struct {
	SellerID     uint    `json:"seller_id" binding:"required"`
	AssetID      uint    `json:"asset_id" binding:"required"`
	Quantity     float64 `json:"quantity" binding:"required,gt=0"`
	PricePerUnit float64 `json:"price_per_unit" binding:"required,gt=0"`
}

type RejectOtcContractRequest struct {
	Comment string `json:"comment" binding:"required"`
}

type OtcContractResponse struct {
	OtcContractID  uint       `json:"otc_contract_id"`
	BuyerID        uint       `json:"buyer_id"`
	SellerID       uint       `json:"seller_id"`
	AssetID        uint       `json:"asset_id"`
	Ticker         string     `json:"ticker"`
	AssetName      string     `json:"asset_name"`
	Quantity       float64    `json:"quantity"`
	PricePerUnit   float64    `json:"price_per_unit"`
	TotalPrice     float64    `json:"total_price"`
	BankApproved   *bool      `json:"bank_approved"`
	SellerApproved *bool      `json:"seller_approved"`
	Comment        *string    `json:"comment,omitempty"`
	ContractNumber string     `json:"contract_number"`
	FinalizedAt    *time.Time `json:"finalized_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func ToOtcContractResponse(c model.OtcContract) OtcContractResponse {
	ticker := ""
	assetName := ""
	if c.Asset.AssetID != 0 {
		ticker = c.Asset.Ticker
		assetName = c.Asset.Name
	}

	return OtcContractResponse{
		OtcContractID:  c.OtcContractID,
		BuyerID:        c.BuyerID,
		SellerID:       c.SellerID,
		AssetID:        c.AssetID,
		Ticker:         ticker,
		AssetName:      assetName,
		Quantity:       c.Quantity,
		PricePerUnit:   c.PricePerUnit,
		TotalPrice:     c.TotalPrice,
		BankApproved:   c.BankApproved,
		SellerApproved: c.SellerApproved,
		Comment:        c.Comment,
		ContractNumber: c.ContractNumber,
		FinalizedAt:    c.FinalizedAt,
		CreatedAt:      c.CreatedAt,
		UpdatedAt:      c.UpdatedAt,
	}
}

func ToOtcContractResponseList(contracts []model.OtcContract) []OtcContractResponse {
	result := make([]OtcContractResponse, len(contracts))
	for i, c := range contracts {
		result[i] = ToOtcContractResponse(c)
	}
	return result
}
