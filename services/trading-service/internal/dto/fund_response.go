package dto

import (
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type FundSummaryResponse struct {
	FundID              uint      `json:"fund_id"`
	Name                string    `json:"name"`
	Description         string    `json:"description"`
	MinimumContribution float64   `json:"minimum_contribution"`
	ManagerID           uint      `json:"manager_id"`
	FundValue           float64   `json:"fund_value"`
	Profit              float64   `json:"profit"`
	LiquidAssets        float64   `json:"liquid_assets"`
	AccountNumber       string    `json:"account_number"`
	CreatedAt           time.Time `json:"created_at"`
}

type ListFundsResponse struct {
	Data     []FundSummaryResponse `json:"data"`
	Total    int64                 `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
}

type ActuaryFundResponse struct {
	FundID        uint    `json:"fund_id"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	FundValue     float64 `json:"fund_value"`
	LiquidAssets  float64 `json:"liquid_assets"`
	AccountNumber string  `json:"account_number"`
}

func ToFundSummaryResponse(fund model.InvestmentFund, securitiesValue, liquidAssets float64) FundSummaryResponse {
	fundValue := liquidAssets + securitiesValue
	var totalInvested float64
	for _, pos := range fund.Positions {
		totalInvested += pos.TotalInvestedAmount
	}
	profit := fundValue - totalInvested
	return FundSummaryResponse{
		FundID:              fund.FundID,
		Name:                fund.Name,
		Description:         fund.Description,
		MinimumContribution: fund.MinimumContribution,
		ManagerID:           fund.ManagerID,
		FundValue:           fundValue,
		Profit:              profit,
		LiquidAssets:        liquidAssets,
		AccountNumber:       fund.AccountNumber,
		CreatedAt:           fund.CreatedAt,
	}
}

func ToActuaryFundResponse(fund model.InvestmentFund, securitiesValue, liquidAssets float64) ActuaryFundResponse {
	fundValue := liquidAssets + securitiesValue
	return ActuaryFundResponse{
		FundID:        fund.FundID,
		Name:          fund.Name,
		Description:   fund.Description,
		FundValue:     fundValue,
		LiquidAssets:  liquidAssets,
		AccountNumber: fund.AccountNumber,
	}
}
