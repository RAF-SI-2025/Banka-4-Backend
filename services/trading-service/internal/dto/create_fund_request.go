package dto

type CreateFundRequest struct {
	Name                string  `json:"name" binding:"required"`
	Description         string  `json:"description" binding:"required"`
	MinimumContribution float64 `json:"minimum_contribution" binding:"gt=0"`
}
