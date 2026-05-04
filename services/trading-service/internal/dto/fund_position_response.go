package dto

type FundPositionResponse struct {
	FundName       string  `json:"fund_name"`
	ManagerName    string  `json:"manager_name"`
	BankSharePct   float64 `json:"bank_share_pct"`
	BankShareValue float64 `json:"bank_share_value"`
	Profit         float64 `json:"profit"`
}
