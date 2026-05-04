package handler

import (
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/service"
)

type PaymentHandler struct {
	service        *service.PaymentService
	accountService *service.AccountService
}

func NewPaymentHandler(paymentService *service.PaymentService, accountService *service.AccountService) *PaymentHandler {
	return &PaymentHandler{service: paymentService, accountService: accountService}
}

// CreatePayment godoc
// @Summary Create a new payment
// @Description Creates a new payment from the request body. Only the authenticated client can create payments.
// @Tags payments
// @Accept json
// @Produce json
// @Param clientId path int true "Client ID"
// @Param request body dto.CreatePaymentRequest true "Payment data"
// @Success 200 {object} dto.CreatePaymentResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/{clientId}/payments [post]
func (h *PaymentHandler) CreatePayment(c *gin.Context) {
	var req dto.CreatePaymentRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	payment, err := h.service.CreatePayment(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, dto.CreatePaymentResponse{
		PaymentID: payment.PaymentID,
	})
}

// GetPaymentByID godoc
// @Summary Get payment details
// @Description Returns payment details by ID. Clients can only access their own payments, employees can access any.
// @Tags payments
// @Produce json
// @Param clientId path int true "Client ID"
// @Param id path int true "Payment ID"
// @Success 200 {object} dto.PaymentResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/{clientId}/payments/{id} [get]
func (h *PaymentHandler) GetPaymentByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid payment id"))
		return
	}

	payment, err := h.service.GetPaymentByID(c.Request.Context(), uint(id))
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, dto.ToPaymentResponse(payment))
}

// GetReceipt godoc
// @Summary Download payment receipt
// @Description Generates and returns a PDF receipt for a completed payment.
// @Tags payments
// @Produce application/pdf
// @Param clientId path int true "Client ID"
// @Param id path int true "Payment ID"
// @Success 200 {file} file
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/{clientId}/payments/{id}/receipt [get]
func (h *PaymentHandler) GetReceipt(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid payment id"))
		return
	}

	pdfBytes, err := h.service.GenerateReceipt(c.Request.Context(), uint(id))
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=receipt-%d.pdf", id))
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}

// VerifyPayment godoc
// @Summary Verify a payment
// @Description Confirms a payment using a verification code. Only the client who created the payment can verify it.
// @Tags payments
// @Accept json
// @Produce json
// @Param clientId path int true "Client ID"
// @Param id path int true "Payment ID"
// @Param request body dto.VerifyPaymentRequest true "Verification code"
// @Success 200 {object} dto.VerifyPaymentResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/{clientId}/payments/{id}/verify [post]
func (h *PaymentHandler) VerifyPayment(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid payment id"))
		return
	}

	var req dto.VerifyPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	payment, err := h.service.VerifyPayment(
		c.Request.Context(),
		uint(id),
		req.Code,
		c.GetHeader("Authorization"),
	)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, dto.VerifyPaymentResponse{
		PaymentID: payment.PaymentID,
	})
}

// GetAccountPayments godoc
// @Summary List account payments
// @Description Returns a paginated list of payments for an account, filterable by status, date range, and amount. Clients can access only their own accounts, employees can access any.
// @Tags payments
// @Produce json
// @Param clientId path int true "Client ID"
// @Param accountNumber path string true "Account number"
// @Param status query string false "Filter by status (processing, completed, rejected)"
// @Param start_date query string false "Filter from date (YYYY-MM-DD)"
// @Param end_date query string false "Filter to date (YYYY-MM-DD)"
// @Param min_amount query number false "Minimum amount filter"
// @Param max_amount query number false "Maximum amount filter"
// @Param page query int false "Page number" minimum(1)
// @Param page_size query int false "Page size" minimum(1) maximum(100)
// @Success 200 {object} dto.ListPaymentsResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/{clientId}/accounts/{accountNumber}/payments [get]
func (h *PaymentHandler) GetAccountPayments(c *gin.Context) {
	valStr := c.Param("clientId")

	_, err := strconv.ParseUint(valStr, 10, 64)

	if err != nil {
		_ = c.Error(errors.BadRequestErr("client id must be a number"))
		return
	}

	accountNumber := c.Param("accountNumber")

	var filters dto.PaymentFilters
	if err := c.ShouldBindQuery(&filters); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}
	if filters.Page < 1 {
		filters.Page = 1
	}
	if filters.PageSize < 1 {
		filters.PageSize = 10
	}

	payments, total, err := h.service.GetAccountPayments(c.Request.Context(), accountNumber, &filters)
	if err != nil {
		_ = c.Error(err)
		return
	}

	data := make([]dto.PaymentSummaryResponse, len(payments))
	for i, p := range payments {
		data[i] = dto.PaymentSummaryResponse{
			ID:               p.PaymentID,
			RecipientName:    p.RecipientName,
			RecipientAccount: p.Transaction.RecipientAccountNumber,
			PayerAccount:     p.Transaction.PayerAccountNumber,
			Amount:           p.Transaction.StartAmount,
			Currency:         string(p.Transaction.StartCurrencyCode),
			Status:           string(p.Transaction.Status),
			Purpose:          p.Purpose,
			PaymentCode:      p.PaymentCode,
			CreatedAt:        p.Transaction.CreatedAt,
		}
	}

	totalPages := int(math.Ceil(float64(total) / float64(filters.PageSize)))
	c.JSON(http.StatusOK, dto.ListPaymentsResponse{
		Data:       data,
		Total:      total,
		Page:       filters.Page,
		PageSize:   filters.PageSize,
		TotalPages: totalPages,
	})
}

// GetClientPayments godoc
// @Summary List all client payments
// @Description Returns a paginated list of all payments for a client. Supports filtering by status, date range, and amount.
// @Tags payments
// @Produce json
// @Param clientId path int true "Client ID"
// @Param status query string false "Filter by status (processing, completed, rejected)"
// @Param start_date query string false "Filter from date (YYYY-MM-DD)"
// @Param end_date query string false "Filter to date (YYYY-MM-DD)"
// @Param min_amount query number false "Minimum amount filter"
// @Param max_amount query number false "Maximum amount filter"
// @Param page query int false "Page number" minimum(1)
// @Param page_size query int false "Page size" minimum(1) maximum(100)
// @Success 200 {object} dto.ListPaymentsResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/{clientId}/payments [get]
func (h *PaymentHandler) GetClientPayments(c *gin.Context) {
	valStr := c.Param("clientId")

	clientID, err := strconv.ParseUint(valStr, 10, 64)

	if err != nil {
		_ = c.Error(errors.BadRequestErr("client id must be a number"))
		return
	}

	var filters dto.PaymentFilters
	if err := c.ShouldBindQuery(&filters); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}
	if filters.Page < 1 {
		filters.Page = 1
	}
	if filters.PageSize < 1 {
		filters.PageSize = 10
	}

	payments, total, err := h.service.GetClientPayments(c.Request.Context(), uint(clientID), &filters)
	if err != nil {
		_ = c.Error(err)
		return
	}

	data := make([]dto.PaymentSummaryResponse, len(payments))
	for i, p := range payments {
		data[i] = dto.PaymentSummaryResponse{
			ID:               p.PaymentID,
			RecipientName:    p.RecipientName,
			RecipientAccount: p.Transaction.RecipientAccountNumber,
			PayerAccount:     p.Transaction.PayerAccountNumber,
			Amount:           p.Transaction.StartAmount,
			Currency:         string(p.Transaction.StartCurrencyCode),
			Status:           string(p.Transaction.Status),
			Purpose:          p.Purpose,
			PaymentCode:      p.PaymentCode,
			CreatedAt:        p.Transaction.CreatedAt,
		}
	}

	totalPages := int(math.Ceil(float64(total) / float64(filters.PageSize)))
	c.JSON(http.StatusOK, dto.ListPaymentsResponse{
		Data:       data,
		Total:      total,
		Page:       filters.Page,
		PageSize:   filters.PageSize,
		TotalPages: totalPages,
	})
}
