package dto

type UserTaxEntry struct {
	ID         uint64  `json:"id"`
	FirstName  string  `json:"firstName"`
	LastName   string  `json:"lastName"`
	Email      string  `json:"email"`
	UserType   string  `json:"userType"`
	TaxOwedRSD float64 `json:"taxOwedRsd"`
}

type ListTaxUsersResponse struct {
	Data       []UserTaxEntry `json:"data"`
	Total      int64          `json:"total"`
	Page       int32          `json:"page"`
	PageSize   int32          `json:"pageSize"`
	TotalPages int32          `json:"totalPages"`
}

type CollectTaxesResponse struct {
	Message string `json:"message"`
}
type TaxInfoResponse struct {
	TotalTax float64 `json:"totalTax"`
}
