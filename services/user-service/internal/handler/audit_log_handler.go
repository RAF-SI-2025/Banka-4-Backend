package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/service"
)

type AuditLogHandler struct {
	service *service.AuditLogService
}

func NewAuditLogHandler(service *service.AuditLogService) *AuditLogHandler {
	return &AuditLogHandler{service: service}
}

// ListAuditLogs godoc
// @Summary List audit log entries
// @Description Returns a paginated list of audit log entries with optional filtering. Accessible only to admins and supervisors.
// @Tags audit-log
// @Produce json
// @Param action_type query string false "Filter by action type"
// @Param performed_by_employee_id query int false "Filter by performer employee ID"
// @Param date_from query string false "Filter entries from this date (YYYY-MM-DD)"
// @Param date_to query string false "Filter entries up to this date (YYYY-MM-DD)"
// @Param page query int false "Page number" minimum(1)
// @Param page_size query int false "Page size" minimum(1) maximum(100)
// @Success 200 {object} dto.ListAuditLogsResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Security BearerAuth
// @Router /api/audit-log [get]
func (h *AuditLogHandler) ListAuditLogs(c *gin.Context) {
	var query dto.ListAuditLogsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 10
	}

	result, err := h.service.GetAll(c.Request.Context(), &query)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, result)
}
