package dto

import "time"

type SecurityHoldingResponse struct {
	Ticker            string    `json:"ticker"`
	Amount            float64   `json:"amount"`
	Price             float64   `json:"price"`
	Currency          string    `json:"currency"`
	Change            float64   `json:"change"`
	Volume            uint64    `json:"volume"`
	InitialMarginCost float64   `json:"initial_margin_cost"`
	AcquisitionDate   time.Time `json:"acquisition_date"`
}

type FundPerformanceEntry struct {
	Date         time.Time `json:"date"`
	Value        float64   `json:"value"`
	Profit       float64   `json:"profit"`
	LiquidAssets float64   `json:"liquid_assets"`
}

type FundDetailResponse struct {
	ID                 uint                      `json:"id"`
	Name               string                    `json:"name"`
	Description        string                    `json:"description"`
	Manager            string                    `json:"manager"`
	FundValue          float64                   `json:"fund_value"`
	MinInvestment      float64                   `json:"min_investment"`
	Profit             float64                   `json:"profit"`
	LiquidAssets       float64                   `json:"account_balance"`
	Holdings           []SecurityHoldingResponse `json:"holdings"`
	PerformanceHistory []FundPerformanceEntry    `json:"performance_history"`
}

type TransferManagerFundsRequest struct {
	FromManagerID uint `json:"from_manager_id" binding:"required"`
	ToManagerID   uint `json:"to_manager_id"   binding:"required"`
}
