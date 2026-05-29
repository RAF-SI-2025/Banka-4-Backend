package dto

import (
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/audit"
)

type ListAuditLogsQuery struct {
	ActionType            string     `form:"action_type"`
	PerformedByEmployeeID *uint      `form:"performed_by_employee_id"`
	DateFrom              *time.Time `form:"date_from" time_format:"2006-01-02"`
	DateTo                *time.Time `form:"date_to" time_format:"2006-01-02"`
	Page                  int        `form:"page" binding:"min=1"`
	PageSize              int        `form:"page_size" binding:"min=1,max=100"`
}

type AuditLogResponse struct {
	ID                    uint      `json:"id"`
	ActionType            string    `json:"action_type"`
	PerformedByEmployeeID uint      `json:"performed_by_employee_id"`
	Details               string    `json:"details"`
	CreatedAt             time.Time `json:"created_at"`
}

type ListAuditLogsResponse struct {
	Data       []AuditLogResponse `json:"data"`
	Total      int64              `json:"total"`
	Page       int                `json:"page"`
	PageSize   int                `json:"page_size"`
	TotalPages int                `json:"total_pages"`
}

func ToAuditLogResponse(entry audit.AuditLog) AuditLogResponse {
	return AuditLogResponse{
		ID:                    entry.ID,
		ActionType:            entry.ActionType,
		PerformedByEmployeeID: entry.PerformedByEmployeeID,
		Details:               entry.Details,
		CreatedAt:             entry.CreatedAt,
	}
}

func ToAuditLogResponseList(entries []audit.AuditLog, total int64, page, pageSize int) ListAuditLogsResponse {
	data := make([]AuditLogResponse, len(entries))
	for i, e := range entries {
		data[i] = ToAuditLogResponse(e)
	}

	totalPages := (int(total) + pageSize - 1) / pageSize

	return ListAuditLogsResponse{
		Data:       data,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}
}
