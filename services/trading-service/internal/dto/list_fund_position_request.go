package dto

import "github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"

type FundPositionSummaryResponse struct {
	FundID               uint    `json:"fund_id"`
	FundName             string  `json:"fund_name"`
	FundDescription      string  `json:"fund_description"`
	TotalProfit          float64 `json:"total_profit"`
	ClientsSharePercent  float64 `json:"clients_share_percent"`
	ClientsShareValueRSD float64 `json:"clients_share_value_rsd"`
}

func ToFundPositionSummaryResponse(fp model.ClientFundPosition) FundPositionSummaryResponse {
	return FundPositionSummaryResponse{
		FundID:          fp.FundID,
		FundName:        fp.Fund.Name,
		FundDescription: fp.Fund.Description,
	}
}
