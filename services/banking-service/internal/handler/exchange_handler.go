package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/service"
)

type ExchangeHandler struct {
	service *service.ExchangeService
}

func NewExchangeHandler(service *service.ExchangeService) *ExchangeHandler {
	return &ExchangeHandler{service: service}
}

// GetRates godoc
// @Summary Get exchange rates
// @Description Returns the latest available currency exchange rates.
// @Tags exchange
// @Produce json
// @Success 200 {object} dto.ExchangeRatesResponse
// @Failure 500 {object} errors.AppError
// @Router /exchange/rates [get]
func (h *ExchangeHandler) GetRates(c *gin.Context) {
	rates, err := h.service.GetRates(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, dto.ToExchangeRatesResponse(rates))
}

// Calculate godoc
// @Summary Calculate currency conversion
// @Description Converts a given amount from one currency to another using current exchange rates.
// @Tags exchange
// @Produce json
// @Param amount query number true "Amount to convert" minimum(0)
// @Param from_currency query string true "Source currency code (e.g. USD, EUR)"
// @Param to_currency query string true "Target currency code (e.g. RSD, EUR)"
// @Success 200 {object} dto.ConvertResponse
// @Failure 400 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Router /exchange/calculate [get]
func (h *ExchangeHandler) Calculate(c *gin.Context) {
	amountStr := c.Query("amount")
	fromStr := c.Query("from_currency")
	toStr := c.Query("to_currency")
	if amountStr == "" || fromStr == "" || toStr == "" {
		_ = c.Error(errors.BadRequestErr("amount, from_currency, and to_currency are required"))
		return
	}

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amount <= 0 {
		_ = c.Error(errors.BadRequestErr("amount must be a positive number"))
		return
	}

	from := model.CurrencyCode(fromStr)
	to := model.CurrencyCode(toStr)
	if !model.AllowedCurrencies[from] {
		_ = c.Error(errors.BadRequestErr("unsupported from_currency: " + fromStr))
		return
	}

	if !model.AllowedCurrencies[to] {
		_ = c.Error(errors.BadRequestErr("unsupported to_currency: " + toStr))
		return
	}

	total, err := h.service.Convert(c.Request.Context(), amount, from, to)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, dto.ToConvertResponse(amount, from, to, total))
}
