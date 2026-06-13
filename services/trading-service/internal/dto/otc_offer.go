package dto

import (
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

// CreateOtcOfferRequest — kupac inicira pregovor.
type CreateOtcOfferRequest struct {
	AssetOwnershipID   uint      `json:"asset_ownership_id" binding:"required"`
	Amount             int       `json:"amount" binding:"required,gt=0"`
	PricePerStockRSD   float64   `json:"price_per_stock_rsd" binding:"required,gt=0"`
	PremiumRSD         float64   `json:"premium_rsd" binding:"required,gt=0"`
	SettlementDate     time.Time `json:"settlement_date" binding:"required"`
	BuyerAccountNumber string    `json:"buyer_account_number" binding:"required"`
}

// CounterOfferRequest — bilo koja strana može ažurirati postojeću ponudu.
// AccountNumber je opcioni: prodavac ga šalje samo prvi put da bi postavio
// SellerAccountNumber na ponudi (potreban za kasniji premium transfer).
type CounterOfferRequest struct {
	Amount           int       `json:"amount" binding:"required,gt=0"`
	PricePerStockRSD float64   `json:"price_per_stock_rsd" binding:"required,gt=0"`
	PremiumRSD       float64   `json:"premium_rsd" binding:"required,gt=0"`
	SettlementDate   time.Time `json:"settlement_date" binding:"required"`
	AccountNumber    *string   `json:"account_number,omitempty"`
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
	OtcOfferID       uint    `json:"otc_offer_id"`
	BuyerID          uint    `json:"buyer_id"`
	SellerID         uint    `json:"seller_id"`
	StockAssetID     uint    `json:"stock_asset_id"`
	Ticker           string  `json:"ticker,omitempty"`
	StockName        string  `json:"stock_name,omitempty"`
	Amount           int     `json:"amount"`
	PricePerStockRSD float64 `json:"price_per_stock_rsd"`

	CurrentPrice    *float64 `json:"current_price,omitempty"`
	ListingCurrency string   `json:"listing_currency,omitempty"`

	CurrentPriceRSD *float64 `json:"current_price_rsd,omitempty"`

	PremiumRSD          float64              `json:"premium_rsd"`
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
	OtcOptionContractID uint `json:"otc_option_contract_id"`
	OtcOfferID          uint `json:"otc_offer_id"`

	BuyerID       uint   `json:"buyer_id"`
	BuyerFullName string `json:"buyer_full_name"`
	BuyerBank     string `json:"buyer_bank"`

	SellerID       uint   `json:"seller_id"`
	SellerFullName string `json:"seller_full_name"`
	SellerBank     string `json:"seller_bank"`

	StockAssetID        uint                          `json:"stock_asset_id"`
	Ticker              string                        `json:"ticker,omitempty"`
	StockName           string                        `json:"stock_name,omitempty"`
	Amount              int                           `json:"amount"`
	StrikePriceRSD      float64                       `json:"strike_price_rsd"`
	PremiumRSD          float64                       `json:"premium_rsd"`
	ListingCurrency     string                        `json:"listing_currency"`
	CurrentPrice        *float64                      `json:"current_price"`
	SettlementDate      time.Time                     `json:"settlement_date"`
	BuyerAccountNumber  string                        `json:"buyer_account_number"`
	SellerAccountNumber string                        `json:"seller_account_number"`
	Status              model.OtcOptionContractStatus `json:"status"`
	ExercisedAt         *time.Time                    `json:"exercised_at,omitempty"`
	CreatedAt           time.Time                     `json:"created_at"`
}

type OtcExecutionSagaResponse struct {
	OtcExecutionSagaID uint                           `json:"otc_execution_saga_id"`
	ContractID         uint                           `json:"contract_id"`
	ExecutionKey       string                         `json:"execution_key"`
	CurrentStep        model.OtcExecutionStep         `json:"current_step"`
	Status             model.OtcExecutionStatus       `json:"status"`
	RetryCount         int                            `json:"retry_count"`
	NextRetryAt        *time.Time                     `json:"next_retry_at,omitempty"`
	LastError          string                         `json:"last_error,omitempty"`
	CompletedAt        *time.Time                     `json:"completed_at,omitempty"`
	CreatedAt          time.Time                      `json:"created_at"`
	UpdatedAt          time.Time                      `json:"updated_at"`
	Log                []OtcExecutionLogEntryResponse `json:"log,omitempty"`
}

type OtcExecutionLogEntryResponse struct {
	Step      string                       `json:"step"`
	Outcome   model.OtcExecutionLogOutcome `json:"outcome"`
	Error     string                       `json:"error,omitempty"`
	CreatedAt time.Time                    `json:"created_at"`
}

func ToOtcExecutionLogEntryResponses(entries []model.OtcExecutionSagaLogEntry) []OtcExecutionLogEntryResponse {
	out := make([]OtcExecutionLogEntryResponse, len(entries))
	for i, e := range entries {
		out[i] = OtcExecutionLogEntryResponse{
			Step:      e.Step,
			Outcome:   e.Outcome,
			Error:     e.Error,
			CreatedAt: e.CreatedAt,
		}
	}
	return out
}

func ToOtcOfferResponse(o model.OtcOffer) OtcOfferResponse {
	resp := OtcOfferResponse{
		OtcOfferID:          o.OtcOfferID,
		BuyerID:             o.BuyerID,
		SellerID:            o.SellerID,
		StockAssetID:        o.StockAssetID,
		Amount:              o.Amount,
		PricePerStockRSD:    o.PricePerStockRSD,
		PremiumRSD:          o.PremiumRSD,
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
		StrikePriceRSD:      c.StrikePriceRSD,
		PremiumRSD:          c.PremiumRSD,
		SettlementDate:      c.SettlementDate,
		BuyerAccountNumber:  c.BuyerAccountNumber,
		SellerAccountNumber: c.SellerAccountNumber,
		Status:              c.Status,
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

func ToOtcExecutionSagaResponse(saga model.OtcExecutionSaga) OtcExecutionSagaResponse {
	return OtcExecutionSagaResponse{
		OtcExecutionSagaID: saga.OtcExecutionSagaID,
		ContractID:         saga.ContractID,
		ExecutionKey:       saga.ExecutionKey,
		CurrentStep:        saga.CurrentStep,
		Status:             saga.Status,
		RetryCount:         saga.RetryCount,
		NextRetryAt:        saga.NextRetryAt,
		LastError:          saga.LastError,
		CompletedAt:        saga.CompletedAt,
		CreatedAt:          saga.CreatedAt,
		UpdatedAt:          saga.UpdatedAt,
	}
}
