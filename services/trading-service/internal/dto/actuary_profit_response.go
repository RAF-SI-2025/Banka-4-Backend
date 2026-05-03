package dto

type ActuaryProfitResponse struct {
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	ProfitRSD float64 `json:"profit_rsd"`
}

type PaginatedActuaryProfitResponse struct {
	Data     []ActuaryProfitResponse `json:"data"`
	Total    int64                   `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"pageSize"`
}
