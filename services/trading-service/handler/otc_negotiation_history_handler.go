package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/service"
	"github.com/gin-gonic/gin"
)

type OtcNegotiationHistoryHandler struct {
	otcNegotiationHistoryService service.OtcNegotiationHistoryService
}

func NewOtcNegotiationHistoryHandler(otcNegotiationHistoryService service.OtcNegotiationHistoryService) *OtcNegotiationHistoryHandler {
	return &OtcNegotiationHistoryHandler{otcNegotiationHistoryService: otcNegotiationHistoryService}
}

// GetNegotiationHistory retrieves the negotiation history for a given offer.
// @Summary Get negotiation history
// @Description Retrieves the negotiation history for a given offer.
// @Tags otc
// @Produce json
// @Param id path int true "Offer ID"
// @Param status query string false "Filter by status"
// @Param from query string false "Filter by date from"
// @Param to query string false "Filter by date to"
// @Param counterparty query int false "Filter by counterparty"
// @Success 200 {array} model.OtcNegotiationHistory
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Router /api/otc/offers/{id}/history [get]
func (h *OtcNegotiationHistoryHandler) GetNegotiationHistory(c *gin.Context) {
	offerID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid offer id"))
		return
	}

	status := c.Query("status")
	dateFromStr := c.Query("from")
	dateToStr := c.Query("to")
	counterpartyStr := c.Query("counterparty")

	var dateFrom, dateTo *time.Time
	if dateFromStr != "" {
		t, err := time.Parse(time.RFC3339, dateFromStr)
		if err != nil {
			_ = c.Error(errors.BadRequestErr("invalid from date"))
			return
		}
		dateFrom = &t
	}
	if dateToStr != "" {
		t, err := time.Parse(time.RFC3339, dateToStr)
		if err != nil {
			_ = c.Error(errors.BadRequestErr("invalid to date"))
			return
		}
		dateTo = &t
	}

	var counterparty uint64
	if counterpartyStr != "" {
		counterparty, err = strconv.ParseUint(counterpartyStr, 10, 32)
		if err != nil {
			_ = c.Error(errors.BadRequestErr("invalid counterparty"))
			return
		}
	}

	history, err := h.otcNegotiationHistoryService.GetNegotiationHistory(c.Request.Context(), uint(offerID), status, dateFrom, dateTo, uint(counterparty))
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, history)
}
