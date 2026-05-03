package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/service"
)

type ExchangeHandler struct {
	service *service.ExchangeService
}

func NewExchangeHandler(service *service.ExchangeService) *ExchangeHandler {
	return &ExchangeHandler{service: service}
}

// GetAll godoc
// @Summary Get all exchanges
// @Description Returns a paginated list of all stock exchanges
// @Tags exchange
// @Produce json
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/exchanges [get]
func (h *ExchangeHandler) GetAll(c *gin.Context) {
	var query dto.ListExchangesQuery
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

	exchanges, total, err := h.service.GetAll(c.Request.Context(), query.Page, query.PageSize)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      dto.ToExchangeResponseList(exchanges),
		"total":     total,
		"page":      query.Page,
		"page_size": query.PageSize,
	})
}

// ToggleTradingEnabled godoc
// @Summary Toggle trading enabled for an exchange
// @Description Enables or disables trading time enforcement for a specific exchange (for testing purposes)
// @Tags exchange
// @Produce json
// @Param micCode path string true "Exchange MIC code"
// @Success 200 {object} dto.ExchangeResponse
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/exchanges/{micCode}/toggle [patch]
func (h *ExchangeHandler) ToggleTradingEnabled(c *gin.Context) {
	micCode := c.Param("micCode")

	exchange, err := h.service.ToggleTradingEnabled(c.Request.Context(), micCode)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.ToExchangeResponse(*exchange))
}
