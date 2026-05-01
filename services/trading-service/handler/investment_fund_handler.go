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
		c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	fund, err := h.service.CreateFund(c.Request.Context(), req)
	if err != nil {
		c.Error(err)
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
		c.Error(errors.BadRequestErr("invalid fund id"))
		return
	}

	var req dto.InvestInFundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	resp, err := h.service.InvestInFund(c.Request.Context(), uint(fundID), req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetClientFundPositions godoc
// @Summary Get client's investment fund positions
// @Description Returns all investment fund positions for the specified client, including share percentage and current value in RSD.
// @Tags investment-funds
// @Produce json
// @Param clientId path int true "Client ID"
// @Success 200 {array} dto.FundPositionSummaryResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Security BearerAuth
// @Router /api/client/{clientId}/funds [get]
func (h *InvestmentFundHandler) GetClientFundPositions(c *gin.Context) {
	clientID, err := strconv.ParseUint(c.Param("clientId"), 10, 64)
	if err != nil || clientID == 0 {
		c.Error(errors.BadRequestErr("invalid client id"))
		return
	}

	resp, err := h.service.GetClientFundPositions(c.Request.Context(), uint(clientID))
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, resp)
}
