package dto

import "time"

// InvestInFundRequest is the request body for POST /api/funds/:fundId/invest.
type InvestInFundRequest struct {
	// AccountNumber is the source account for the investment.
	// For clients this must be one of their own accounts.
	// For supervisors this must be a bank account.
	AccountNumber string  `json:"account_number" binding:"required"`
	Amount        float64 `json:"amount"         binding:"required,gt=0"`
}

// InvestInFundResponse is returned after a successful investment.
type InvestInFundResponse struct {
	FundID           uint      `json:"fund_id"`
	FundName         string    `json:"fund_name"`
	InvestedNow      float64   `json:"invested_now"`
	CurrencyCode     string    `json:"currency_code"`
	TotalInvestedRSD float64   `json:"total_invested_rsd"`
	CreatedAt        time.Time `json:"created_at"`
}
