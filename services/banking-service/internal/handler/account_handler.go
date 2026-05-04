package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/service"
)

type AccountHandler struct {
	service *service.AccountService
}

func NewAccountHandler(service *service.AccountService) *AccountHandler {
	return &AccountHandler{service: service}
}

// Create godoc
// @Summary Create a new bank account
// @Description Creates a new bank account for a client. Account kind must be "Current" or "Foreign". Current accounts require a subtype; foreign accounts require a currency code.
// @Tags accounts
// @Accept json
// @Produce json
// @Param account body dto.CreateAccountRequest true "Account creation data"
// @Success 201 {object} dto.AccountResponse
// @Failure 400 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Failure 409 {object} errors.AppError
// @Security BearerAuth
// @Router /api/accounts [post]
func (h *AccountHandler) Create(c *gin.Context) {
	var req dto.CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	account, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToAccountResponse(account))
}

// ListAccounts godoc
// @Summary List all accounts
// @Description Returns a paginated list of all accounts. Employee access only. Supports filtering by client, account type, kind, status and currency.
// @Tags accounts
// @Produce json
// @Param client_id query int false "Filter by client ID"
// @Param account_type query string false "Filter by account type"
// @Param account_kind query string false "Filter by account kind"
// @Param status query string false "Filter by status"
// @Param currency_id query int false "Filter by currency ID"
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200 {object} object
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Security BearerAuth
// @Router /api/accounts [get]
func (h *AccountHandler) ListAccounts(c *gin.Context) {
	var req dto.ListAccountsQuery

	if err := c.ShouldBindQuery(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	if req.Page < 0 {
		req.Page = 1
	}

	if req.PageSize < 0 {
		req.PageSize = 10
	}

	accounts, total, err := h.service.GetAllAccounts(c.Request.Context(), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      accounts,
		"total":     total,
		"page":      req.Page,
		"page_size": req.PageSize,
	})
}

func GetParamUint(c *gin.Context, key string) (uint, bool) {
	valStr := c.Param(key)

	val, err := strconv.ParseUint(valStr, 10, 64)

	if err != nil {
		_ = c.Error(errors.BadRequestErr("client id must be a number"))
		return 0, false
	}

	return uint(val), true
}

// GetClientAccounts godoc
// @Summary List client accounts
// @Description Returns all active accounts belonging to the authenticated client
// @Tags accounts
// @Produce json
// @Param clientId path string true "Client id"
// @Success 200 {array} dto.AccountSummaryResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/{clientId}/accounts [get]
func (h *AccountHandler) GetClientAccounts(c *gin.Context) {

	clientId, ok := GetParamUint(c, "clientId")
	if !ok {
		return
	}

	accounts, err := h.service.GetClientAccounts(c.Request.Context(), clientId)
	if err != nil {
		_ = c.Error(err)
		return
	}

	resp := make([]dto.AccountSummaryResponse, len(accounts))
	for i := range accounts {
		resp[i] = dto.ToAccountSummaryResponse(&accounts[i])
	}
	c.JSON(http.StatusOK, resp)
}

// GetAccountDetails godoc
// @Summary Get account details
// @Description Returns full details for a specific account owned by the authenticated client
// @Tags accounts
// @Produce json
// @Param accountNumber path string true "Account number"
// @Param clientId path string true "Client id"
// @Success 200 {object} dto.AccountResponse
// @Failure 400 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/{clientId}/accounts/{accountNumber} [get]
func (h *AccountHandler) GetAccountDetails(c *gin.Context) {
	clientId, ok := GetParamUint(c, "clientId")
	if !ok {
		return
	}

	accountNumber := c.Param("accountNumber")
	account, err := h.service.GetAccountDetails(c.Request.Context(), accountNumber, clientId)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, dto.ToAccountResponse(account))
}

// UpdateAccountName godoc
// @Summary Update account name
// @Description Updates the display name of an account owned by the authenticated client
// @Tags accounts
// @Accept json
// @Produce json
// @Param clientId path int true "Client ID"
// @Param accountNumber path string true "Account number"
// @Param request body dto.UpdateAccountNameRequest true "New account name"
// @Success 200
// @Failure 400 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/{clientId}/accounts/{accountNumber}/name [put]
func (h *AccountHandler) UpdateAccountName(c *gin.Context) {
	clientId, ok := GetParamUint(c, "clientId")
	if !ok {
		return
	}

	accountNumber := c.Param("accountNumber")

	var req dto.UpdateAccountNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	if err := h.service.UpdateAccountName(c.Request.Context(), accountNumber, clientId, req.Name); err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusOK)
}

// RequestLimitsChange godoc
// @Summary Request account limit change
// @Description Initiates a limit change request for an account. Confirmation uses TOTP code generated in the mobile app.
// @Tags accounts
// @Accept json
// @Produce json
// @Param clientId path int true "Client ID"
// @Param accountNumber path string true "Account number"
// @Param request body dto.RequestLimitsChangeRequest true "New daily and monthly limits"
// @Success 200
// @Failure 400 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/{clientId}/accounts/{accountNumber}/limits/request [post]
func (h *AccountHandler) RequestLimitsChange(c *gin.Context) {
	clientId, ok := GetParamUint(c, "clientId")
	if !ok {
		return
	}

	accountNumber := c.Param("accountNumber")

	var req dto.RequestLimitsChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	if err := h.service.RequestLimitsChange(c.Request.Context(), accountNumber, clientId, req.DailyLimit, req.MonthlyLimit); err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusOK)
}

// ConfirmLimitsChange godoc
// @Summary Confirm account limit change
// @Description Confirms a pending limit change request using the verification code from the mobile app
// @Tags accounts
// @Accept json
// @Produce json
// @Param clientId path int true "Client ID"
// @Param accountNumber path string true "Account number"
// @Param request body dto.ConfirmLimitsChangeRequest true "Verification code generated in mobile app"
// @Success 200
// @Failure 400 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/{clientId}/accounts/{accountNumber}/limits [put]
func (h *AccountHandler) ConfirmLimitsChange(c *gin.Context) {
	clientId, ok := GetParamUint(c, "clientId")
	if !ok {
		return
	}

	accountNumber := c.Param("accountNumber")

	var req dto.ConfirmLimitsChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	if err := h.service.ConfirmLimitsChange(c.Request.Context(), accountNumber, clientId, req.Code, c.GetHeader("Authorization")); err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusOK)
}
