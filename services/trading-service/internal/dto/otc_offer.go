package dto

import (
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

// CreateOtcOfferRequest — kupac inicira pregovor.
type CreateOtcOfferRequest struct {
	AssetOwnershipID   uint      `json:"asset_ownership_id" binding:"required"`
	Amount             int       `json:"amount" binding:"required,gt=0"`
	PricePerStock      float64   `json:"price_per_stock" binding:"required,gt=0"`
	Premium            float64   `json:"premium" binding:"required,gt=0"`
	SettlementDate     time.Time `json:"settlement_date" binding:"required"`
	BuyerAccountNumber string    `json:"buyer_account_number" binding:"required"`
}

// CounterOfferRequest — bilo koja strana može ažurirati postojeću ponudu.
// AccountNumber je opcioni: prodavac ga šalje samo prvi put da bi postavio
// SellerAccountNumber na ponudi (potreban za kasniji premium transfer).
type CounterOfferRequest struct {
	Amount         int       `json:"amount" binding:"required,gt=0"`
	PricePerStock  float64   `json:"price_per_stock" binding:"required,gt=0"`
	Premium        float64   `json:"premium" binding:"required,gt=0"`
	SettlementDate time.Time `json:"settlement_date" binding:"required"`
	AccountNumber  *string   `json:"account_number,omitempty"`
}

// AcceptOfferRequest — strana suprotna od ModifiedBy prihvata ponudu.
// Ako je SellerAccountNumber još nije postavljen na ponudi, prodavac mora
// proslediti AccountNumber pri prihvatanju.
type AcceptOfferRequest struct {
	AccountNumber *string `json:"account_number,omitempty"`
}

// RejectOfferRequest — bilo koja strana može odustati uz opcioni komentar.
type RejectOfferRequest struct {
	Comment *string `json:"comment,omitempty"`
}

// --- Response DTOs ---

type OtcOfferResponse struct {
	OtcOfferID          uint                 `json:"otc_offer_id"`
	BuyerID             uint                 `json:"buyer_id"`
	SellerID            uint                 `json:"seller_id"`
	StockAssetID        uint                 `json:"stock_asset_id"`
	Ticker              string               `json:"ticker,omitempty"`
	StockName           string               `json:"stock_name,omitempty"`
	Amount              int                  `json:"amount"`
	PricePerStock       float64              `json:"price_per_stock"`
	Premium             float64              `json:"premium"`
	SettlementDate      time.Time            `json:"settlement_date"`
	BuyerAccountNumber  string               `json:"buyer_account_number"`
	SellerAccountNumber *string              `json:"seller_account_number,omitempty"`
	Status              model.OtcOfferStatus `json:"status"`
	LastModified        time.Time            `json:"last_modified"`
	ModifiedBy          uint                 `json:"modified_by"`
	OptionContractID    *uint                `json:"option_contract_id,omitempty"`
	CreatedAt           time.Time            `json:"created_at"`
	UpdatedAt           time.Time            `json:"updated_at"`
}

type OtcOptionContractResponse struct {
	OtcOptionContractID uint       `json:"otc_option_contract_id"`
	OtcOfferID          uint       `json:"otc_offer_id"`
	BuyerID             uint       `json:"buyer_id"`
	SellerID            uint       `json:"seller_id"`
	StockAssetID        uint       `json:"stock_asset_id"`
	Ticker              string     `json:"ticker,omitempty"`
	StockName           string     `json:"stock_name,omitempty"`
	Amount              int        `json:"amount"`
	StrikePrice         float64    `json:"strike_price"`
	Premium             float64    `json:"premium"`
	SettlementDate      time.Time  `json:"settlement_date"`
	IsExercised         bool       `json:"is_exercised"`
	ExercisedAt         *time.Time `json:"exercised_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
}

func ToOtcOfferResponse(o model.OtcOffer) OtcOfferResponse {
	resp := OtcOfferResponse{
		OtcOfferID:          o.OtcOfferID,
		BuyerID:             o.BuyerID,
		SellerID:            o.SellerID,
		StockAssetID:        o.StockAssetID,
		Amount:              o.Amount,
		PricePerStock:       o.PricePerStock,
		Premium:             o.Premium,
		SettlementDate:      o.SettlementDate,
		BuyerAccountNumber:  o.BuyerAccountNumber,
		SellerAccountNumber: o.SellerAccountNumber,
		Status:              o.Status,
		LastModified:        o.LastModified,
		ModifiedBy:          o.ModifiedBy,
		OptionContractID:    o.OptionContractID,
		CreatedAt:           o.CreatedAt,
		UpdatedAt:           o.UpdatedAt,
	}
	if o.Stock.Asset.AssetID != 0 {
		resp.Ticker = o.Stock.Asset.Ticker
		resp.StockName = o.Stock.Asset.Name
	}
	return resp
}

func ToOtcOfferResponseList(offers []model.OtcOffer) []OtcOfferResponse {
	out := make([]OtcOfferResponse, len(offers))
	for i, o := range offers {
		out[i] = ToOtcOfferResponse(o)
	}
	return out
}

func ToOtcOptionContractResponse(c model.OtcOptionContract) OtcOptionContractResponse {
	resp := OtcOptionContractResponse{
		OtcOptionContractID: c.OtcOptionContractID,
		OtcOfferID:          c.OtcOfferID,
		BuyerID:             c.BuyerID,
		SellerID:            c.SellerID,
		StockAssetID:        c.StockAssetID,
		Amount:              c.Amount,
		StrikePrice:         c.StrikePrice,
		Premium:             c.Premium,
		SettlementDate:      c.SettlementDate,
		IsExercised:         c.IsExercised,
		ExercisedAt:         c.ExercisedAt,
		CreatedAt:           c.CreatedAt,
	}
	if c.Stock.Asset.AssetID != 0 {
		resp.Ticker = c.Stock.Asset.Ticker
		resp.StockName = c.Stock.Asset.Name
	}
	return resp
}

func ToOtcOptionContractResponseList(contracts []model.OtcOptionContract) []OtcOptionContractResponse {
	out := make([]OtcOptionContractResponse, len(contracts))
	for i, c := range contracts {
		out[i] = ToOtcOptionContractResponse(c)
	}
	return out
}
