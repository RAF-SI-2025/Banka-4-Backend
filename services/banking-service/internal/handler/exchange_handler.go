package handler

import (
	"banking-service/internal/dto"
	"banking-service/internal/model"
	"banking-service/internal/service"
	"common/pkg/errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ExchangeHandler struct {
	service *service.ExchangeService
}

func NewExchangeHandler(service *service.ExchangeService) *ExchangeHandler {
	return &ExchangeHandler{service: service}
}

func (h *ExchangeHandler) GetRates(c *gin.Context) {
	rates, err := h.service.GetRates(c.Request.Context())
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, dto.ToExchangeRatesResponse(rates))
}

func (h *ExchangeHandler) Calculate(c *gin.Context) {
	amountStr := c.Query("amount")
	fromStr := c.Query("from_currency")
	toStr := c.Query("to_currency")
	if amountStr == "" || fromStr == "" || toStr == "" {
		c.Error(errors.BadRequestErr("amount, from_currency, and to_currency are required"))
		return
	}

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amount <= 0 {
		c.Error(errors.BadRequestErr("amount must be a positive number"))
		return
	}

	from := model.CurrencyCode(fromStr)
	to := model.CurrencyCode(toStr)
	if !model.AllowedCurrencies[from] {
		c.Error(errors.BadRequestErr("unsupported from_currency: " + fromStr))
		return
	}

	if !model.AllowedCurrencies[to] {
		c.Error(errors.BadRequestErr("unsupported to_currency: " + toStr))
		return
	}

	total, err := h.service.Convert(c.Request.Context(), amount, from, to)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, dto.ToConvertResponse(amount, from, to, total))
}
