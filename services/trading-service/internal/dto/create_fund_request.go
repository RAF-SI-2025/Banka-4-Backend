package dto

type CreateFundRequest struct {
	Name                        string   `json:"name" binding:"required"`
	Description                 string   `json:"description" binding:"required"`
	MinimumContribution         float64  `json:"minimum_contribution" binding:"gt=0"`
	DividendReinvestmentPercent *float64 `json:"dividend_reinvestment_percent,omitempty" binding:"omitempty,min=0,max=100"`
}
