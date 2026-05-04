package dto

type PublishAssetRequest struct {
	Amount float64 `json:"amount" binding:"required"`
}

type OTCListRequest struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
}

func (q *OTCListRequest) Normalize() {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 10
	}
}
