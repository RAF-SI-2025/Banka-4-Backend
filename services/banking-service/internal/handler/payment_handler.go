package handler

import (
	"banking-service/internal/dto"
	"banking-service/internal/service"
	"common/pkg/errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type PaymentHandler struct {
	service *service.PaymentService
}

func NewPaymentHandler(service *service.PaymentService) *PaymentHandler {
	return &PaymentHandler{service: service}
}

func (h *PaymentHandler) CreatePayment(c *gin.Context) {
	var req dto.CreatePaymentRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	payment, err := h.service.CreatePayment(c.Request.Context(), req)
	if err != nil {
		c.Error(errors.InternalErr(err))
		return
	}

	c.JSON(http.StatusOK, dto.CreatePaymentResponse{
		PaymentID: payment.PaymentID,
	})
}

func (h *PaymentHandler) VerifyPayment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("payment_id"), 10, 64)
	if err != nil {
		c.Error(errors.BadRequestErr("invalid payment id"))
		return
	}

	var req dto.VerifyPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	payment, err := h.service.VerifyPayment(c.Request.Context(), uint(id), req.Code)
	if err != nil {
		c.Error(errors.InternalErr(err))
		return
	}

	c.JSON(http.StatusOK, dto.VerifyPaymentResponse{
		PaymentID: payment.PaymentID,
	})
}
