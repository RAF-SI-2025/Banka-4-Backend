package dto

type ListFundsQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Name     string `form:"name"`
	SortBy   string `form:"sort_by"`
	SortDir  string `form:"sort_dir"`
}
