package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/service"
)

type InvestmentFundHandler struct {
	service *service.InvestmentFundService
}

func NewInvestmentFundHandler(service *service.InvestmentFundService) *InvestmentFundHandler {
	return &InvestmentFundHandler{service: service}
}

// GetAllFunds godoc
// @Summary Get all investment funds
// @Description Returns a paginated list of all investment funds with optional filtering and sorting. Accessible to actuaries and clients.
// @Tags investment-funds
// @Produce json
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Param name query string false "Filter by fund name (case-insensitive substring)"
// @Param sort_by query string false "Sort by field: name, minimum_contribution, created_at, liquid_assets"
// @Param sort_dir query string false "Sort direction: asc, desc"
// @Success 200 {object} dto.ListFundsResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Router /api/funds [get]
func (h *InvestmentFundHandler) GetAllFunds(c *gin.Context) {
	var query dto.ListFundsQuery
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

	response, err := h.service.GetAllFunds(c.Request.Context(), query)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetBankFundPositions godoc
// @Summary Get investment fund positions
// @Description Returns all investment funds with bank share, manager info and profit calculation
// @Tags profit
// @Security BearerAuth
// @Accept json
// @Produce json
// @Success 200 {array} dto.FundPositionResponse
// @Failure 500 {object} errors.AppError
// @Router /api/profit/funds [get]
func (h *InvestmentFundHandler) GetBankFundPositions(c *gin.Context) {
	res, err := h.service.GetBankFundPositions(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, res)
}

// GetActuaryFunds godoc
// @Summary Get funds managed by an actuary
// @Description Returns all investment funds managed by the specified actuary (supervisor). Shows fund value and liquidity.
// @Tags investment-funds
// @Produce json
// @Param actId path int true "Actuary ID"
// @Success 200 {array} dto.ActuaryFundResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Router /api/actuary/{actId}/assets/funds [get]
func (h *InvestmentFundHandler) GetActuaryFunds(c *gin.Context) {
	actID, err := strconv.ParseUint(c.Param("actId"), 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid actuary id"))
		return
	}

	funds, err := h.service.GetActuaryFunds(c.Request.Context(), uint(actID))
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, funds)
}

// CreateFund godoc
// @Summary Create a new investment fund
// @Description Supervisor creates a new investment fund. An RSD account is automatically created for the fund.
// @Tags investment-funds
// @Accept json
// @Produce json
// @Param request body dto.CreateFundRequest true "Fund details"
// @Success 201 {object} dto.CreateFundResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Router /api/investment-funds [post]
func (h *InvestmentFundHandler) CreateFund(c *gin.Context) {
	var req dto.CreateFundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	fund, err := h.service.CreateFund(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, fund)
}

// InvestInFund godoc
//
//	@Summary		Invest into a fund
//	@Description	Allows a client or supervisor to invest money into an investment fund.
//	@Description	Clients must provide one of their own accounts; supervisors must provide a bank account.
//	@Tags			investment-funds
//	@Accept			json
//	@Produce		json
//	@Param			fundId	path		int						true	"Fund ID"
//	@Param			body	body		dto.InvestInFundRequest	true	"Investment details"
//	@Success		200		{object}	dto.InvestInFundResponse
//	@Failure		400		{object}	errors.AppError
//	@Failure		401		{object}	errors.AppError
//	@Failure		403		{object}	errors.AppError
//	@Failure		404		{object}	errors.AppError
//	@Router			/api/investment-funds/{fundId}/invest [post]
func (h *InvestmentFundHandler) InvestInFund(c *gin.Context) {
	fundID, err := strconv.ParseUint(c.Param("fundId"), 10, 64)
	if err != nil || fundID == 0 {
		_ = c.Error(errors.BadRequestErr("invalid fund id"))
		return
	}

	var req dto.InvestInFundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	resp, err := h.service.InvestInFund(c.Request.Context(), uint(fundID), req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetFundDetail godoc
// @Summary Get investment fund details
// @Description Retrieves detailed information about an investment fund, including holdings, current value, performance history, and account balance.
// @Tags investment-funds
// @Accept json
// @Produce json
// @Param fundId path int true "Fund ID"
// @Success 200 {object} dto.FundDetailResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Router /api/investment-funds/{fundId} [get]
func (h *InvestmentFundHandler) GetFundDetail(c *gin.Context) {
	fundIDStr := c.Param("fundId")
	fundID, err := strconv.ParseUint(fundIDStr, 10, 32)
	if err != nil {
		_ = c.Error(err)
		return
	}

	resp, err := h.service.GetFundDetail(c.Request.Context(), uint(fundID))
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// TransferManagerFunds godoc
// @Summary Transfer funds from one manager to another
// @Description Transfers all investment funds managed by fromManagerID to toManagerID. Called internally when admin removes isSupervisor permission.
// @Tags investment-funds
// @Accept json
// @Produce json
// @Param request body dto.TransferManagerFundsRequest true "Transfer details"
// @Success 204
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Router /api/investment-funds/transfer-manager [post]
func (h *InvestmentFundHandler) TransferManagerFunds(c *gin.Context) {
	var req dto.TransferManagerFundsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	if err := h.service.TransferManagerFunds(c.Request.Context(), req.FromManagerID, req.ToManagerID); err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusNoContent)
}
