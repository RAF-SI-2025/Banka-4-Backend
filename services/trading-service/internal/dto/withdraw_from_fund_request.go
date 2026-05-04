package dto

import (
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type WithdrawFromFundRequest struct {
	AccountNumber string  `json:"account_number" binding:"required"`
	Amount        float64 `json:"amount"         binding:"required,gt=0"`
}

type WithdrawFromFundResponse struct {
	FundID                   uint                       `json:"fund_id"`
	FundName                 string                     `json:"fund_name"`
	DestinationAccountNumber string                     `json:"destination_account_number"`
	DestinationCurrencyCode  string                     `json:"destination_currency_code"`
	RequestedAmountRSD       float64                    `json:"requested_amount_rsd"`
	WithdrawnAmountRSD       float64                    `json:"withdrawn_amount_rsd"`
	TotalInvestedRSD         float64                    `json:"total_invested_rsd"`
	Status                   model.FundRedemptionStatus `json:"status"`
	Message                  string                     `json:"message,omitempty"`
	CreatedAt                time.Time                  `json:"created_at"`
	CompletedAt              *time.Time                 `json:"completed_at,omitempty"`
}
