package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/service"
)

type LoanHandler struct {
	loanService *service.LoanService
}

func NewLoanHandler(loanService *service.LoanService) *LoanHandler {
	return &LoanHandler{loanService: loanService}
}

// SubmitLoanRequest godoc
// @Summary Submit a loan request
// @Description Client submits a loan request. Validates repayment period and currency, and calculates the monthly installment based on bank margin.
// @Tags loans
// @Accept json
// @Produce json
// @Param clientId path int true "Client ID"
// @Param request body dto.CreateLoanRequest true "Loan request data"
// @Success 201 {object} dto.CreateLoanResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/{clientId}/loans/request [post]
func (h *LoanHandler) SubmitLoanRequest(c *gin.Context) {
	var req dto.CreateLoanRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	clientID, err := strconv.ParseUint(c.Param("clientId"), 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid client id"))
		return
	}

	resp, err := h.loanService.SubmitLoanRequest(c.Request.Context(), &req, uint(clientID))
	if err != nil {
		_ = c.Error(err)
		return
	}

	// Uspešan odgovor
	c.JSON(http.StatusCreated, resp)
}

// GetLoans godoc
// @Summary List client loans
// @Description Returns a list of loans for a client. Supports sorting by amount.
// @Tags loans
// @Produce json
// @Param clientId path int true "Client ID"
// @Param sort query string false "Sort by amount: 'asc' or 'desc'"
// @Success 200 {array} dto.LoanResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/{clientId}/loans [get]
func (h *LoanHandler) GetLoans(c *gin.Context) {
	clientID, err := strconv.ParseUint(c.Param("clientId"), 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid client id"))
		return
	}

	sortParam := c.Query("sort")
	sortByAmountDesc := sortParam == "desc"

	loans, err := h.loanService.GetClientLoans(c.Request.Context(), uint(clientID), sortByAmountDesc)
	if err != nil {
		_ = c.Error(errors.InternalErr(err))
		return
	}

	c.JSON(http.StatusOK, loans)
}

// GetLoanByID godoc
// @Summary Get loan details
// @Description Returns detailed loan information including the repayment schedule.
// @Tags loans
// @Produce json
// @Param clientId path int true "Client ID"
// @Param loanId path int true "Loan ID"
// @Success 200 {object} dto.LoanDetailsResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/{clientId}/loans/{loanId} [get]
func (h *LoanHandler) GetLoanByID(c *gin.Context) {
	clientID, err := strconv.ParseUint(c.Param("clientId"), 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid client id"))
		return
	}

	loanID, err := strconv.ParseUint(c.Param("loanId"), 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid loan id"))
		return
	}

	details, err := h.loanService.GetLoanDetails(c.Request.Context(), uint(clientID), uint(loanID))
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, details)
}

// ListLoanRequests godoc
// @Summary List loan requests
// @Description Returns a paginated list of all loan requests. Employee access only.
// @Tags loan-requests
// @Produce json
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200 {object} object
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Security BearerAuth
// @Router /api/loan-requests [get]
func (h *LoanHandler) ListLoanRequests(c *gin.Context) {
	var query dto.ListLoanRequestsQuery

	if err := c.ShouldBindQuery(&query); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	if query.Page == 0 {
		query.Page = 1
	}
	if query.PageSize == 0 {
		query.PageSize = 10
	}

	requests, total, err := h.loanService.GetLoanRequests(c.Request.Context(), &query)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      requests,
		"total":     total,
		"page":      query.Page,
		"page_size": query.PageSize,
	})
}

// ApproveLoanRequest godoc
// @Summary Approve a loan request
// @Description Approves a pending loan request. Employee access only.
// @Tags loan-requests
// @Produce json
// @Param id path int true "Loan request ID"
// @Success 200 {object} object
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/loan-requests/{id}/approve [patch]
func (h *LoanHandler) ApproveLoanRequest(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid loan request id"))
		return
	}

	if err := h.loanService.ApproveLoanRequest(c.Request.Context(), uint(id)); err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Loan request approved successfully"})
}

// RejectLoanRequest godoc
// @Summary Reject a loan request
// @Description Rejects a pending loan request. Employee access only.
// @Tags loan-requests
// @Produce json
// @Param id path int true "Loan request ID"
// @Success 200 {object} object
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/loan-requests/{id}/reject [patch]
func (h *LoanHandler) RejectLoanRequest(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid loan request id"))
		return
	}

	if err := h.loanService.RejectLoanRequest(c.Request.Context(), uint(id)); err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Loan request rejected successfully"})
}
