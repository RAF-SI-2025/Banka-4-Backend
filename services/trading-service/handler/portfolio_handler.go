package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	pkgerrors "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/service"
)

type PortfolioHandler struct {
	service *service.PortfolioService
}

func NewPortfolioHandler(service *service.PortfolioService) *PortfolioHandler {
	return &PortfolioHandler{service: service}
}

// GetClientPortfolio godoc
// @Summary Get portfolio for a client
// @Description Returns all currently held asset positions for a client, aggregated from all orders. Only approved orders with fills are counted. Includes stocks, futures, options, and forex pairs.
// @Tags portfolio
// @Security BearerAuth
// @Produce json
// @Param clientId path int true "Client ID"
// @Success 200 {array} dto.PortfolioAssetResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Router /api/client/{clientId}/assets [get]
func (h *PortfolioHandler) GetClientPortfolio(c *gin.Context) {
	clientID, err := strconv.ParseUint(c.Param("clientId"), 10, 64)
	if err != nil {
		_ = c.Error(pkgerrors.BadRequestErr("invalid client id"))
		return
	}

	assets, err := h.service.GetClientPortfolio(c.Request.Context(), uint(clientID))
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, assets)
}

// GetActuaryPortfolio godoc
// @Summary Get portfolio for an actuary/agent
// @Description Returns all currently held asset positions for an actuary (employee agent/supervisor), aggregated from all orders. Only approved orders with fills are counted. Includes stocks, futures, options, and forex pairs.
// @Tags portfolio
// @Security BearerAuth
// @Produce json
// @Param actId path int true "Actuary ID"
// @Success 200 {array} dto.PortfolioAssetResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Router /api/actuary/{actId}/assets [get]
func (h *PortfolioHandler) GetActuaryPortfolio(c *gin.Context) {
	actID, err := strconv.ParseUint(c.Param("actId"), 10, 64)
	if err != nil {
		_ = c.Error(pkgerrors.BadRequestErr("invalid actuary id"))
		return
	}

	assets, err := h.service.GetActuaryPortfolio(c.Request.Context(), uint(actID))
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, assets)
}

// GetClientPortfolioProfit godoc
// @Summary Get total profit for a client portfolio
// @Description Returns the total accumulated profit across all currently held asset positions for a client.
// @Tags portfolio
// @Security BearerAuth
// @Produce json
// @Param clientId path int true "Client ID"
// @Success 200 {object} dto.PortfolioProfitResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Router /api/client/{clientId}/assets/profit [get]
func (h *PortfolioHandler) GetClientPortfolioProfit(c *gin.Context) {
	clientID, err := strconv.ParseUint(c.Param("clientId"), 10, 64)
	if err != nil {
		_ = c.Error(pkgerrors.BadRequestErr("invalid client id"))
		return
	}

	assets, err := h.service.GetClientPortfolio(c.Request.Context(), uint(clientID))
	if err != nil {
		_ = c.Error(err)
		return
	}

	var total float64
	for _, a := range assets {
		total += a.Profit
	}

	c.JSON(http.StatusOK, dto.PortfolioProfitResponse{TotalProfitRSD: total})
}

// GetActuaryPortfolioProfit godoc
// @Summary Get total profit for an actuary portfolio
// @Description Returns the total accumulated profit across all currently held asset positions for an actuary.
// @Tags portfolio
// @Security BearerAuth
// @Produce json
// @Param actId path int true "Actuary ID"
// @Success 200 {object} dto.PortfolioProfitResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Router /api/actuary/{actId}/assets/profit [get]
func (h *PortfolioHandler) GetActuaryPortfolioProfit(c *gin.Context) {
	actID, err := strconv.ParseUint(c.Param("actId"), 10, 64)
	if err != nil {
		_ = c.Error(pkgerrors.BadRequestErr("invalid actuary id"))
		return
	}

	assets, err := h.service.GetActuaryPortfolio(c.Request.Context(), uint(actID))
	if err != nil {
		_ = c.Error(err)
		return
	}

	var total float64
	for _, a := range assets {
		total += a.Profit
	}

	c.JSON(http.StatusOK, dto.PortfolioProfitResponse{TotalProfitRSD: total})
}

// GetAllActuaryProfits godoc
// @Summary Get actuary profits
// @Description Returns paginated list of actuaries with their profits (agents and supervisors)
// @Tags profit
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(10)
// @Param first_name query string false "Filter by first name"
// @Param last_name query string false "Filter by last name"
// @Success 200 {array} dto.ActuaryProfitResponse
// @Failure 400 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Router /api/profit/actuaries [get]
func (h *PortfolioHandler) GetAllActuaryProfits(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		_ = c.Error(pkgerrors.BadRequestErr("invalid page"))
		return
	}

	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if err != nil {
		_ = c.Error(pkgerrors.BadRequestErr("invalid page size"))
		return
	}

	firstName := c.Query("first_name")
	lastName := c.Query("last_name")

	res, err := h.service.GetAllActuaryProfits(
		c.Request.Context(),
		int32(page),
		int32(pageSize),
		firstName,
		lastName,
	)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, res)
}

// ExerciseOption godoc
// @Summary Exercise an owned option
// @Description Exercises one contract of an actuary-owned in-the-money call option and buys the underlying stock at the strike price.
// @Tags portfolio
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param actId path int true "Actuary ID"
// @Param assetId path int true "Option asset ID"
// @Param request body dto.ExerciseOptionRequest true "Exercise request"
// @Success 200 {object} dto.ExerciseOptionResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Router /api/actuary/{actId}/options/{assetId}/exercise [post]
func (h *PortfolioHandler) ExerciseOption(c *gin.Context) {
	actID, err := strconv.ParseUint(c.Param("actId"), 10, 64)
	if err != nil {
		_ = c.Error(pkgerrors.BadRequestErr("invalid actuary id"))
		return
	}

	assetID, err := strconv.ParseUint(c.Param("assetId"), 10, 64)
	if err != nil {
		_ = c.Error(pkgerrors.BadRequestErr("invalid asset id"))
		return
	}

	var req dto.ExerciseOptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(pkgerrors.BadRequestErr(err.Error()))
		return
	}

	resp, err := h.service.ExerciseOption(c.Request.Context(), uint(actID), model.OwnerTypeActuary, uint(assetID), req.AccountNumber)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, resp)
}
