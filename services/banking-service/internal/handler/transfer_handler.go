package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/service"
)

type TransferHandler struct {
	service *service.TransferService
}

func NewTransferHandler(service *service.TransferService) *TransferHandler {
	return &TransferHandler{service: service}
}

// ExecuteTransfer godoc
// @Summary Execute an internal transfer
// @Description Executes transfer between two authenticated-client accounts and persists transaction and transfer records atomically.
// @Tags transfers
// @Accept json
// @Produce json
// @Param clientId path int true "Client ID"
// @Param request body dto.TransferRequest true "Transfer details"
// @Success 201 {object} dto.TransferResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/{clientId}/transfers [post]
func (h *TransferHandler) ExecuteTransfer(c *gin.Context) {
	var req dto.TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	response, err := h.service.ExecuteTransfer(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, response)
}

// GetTransferHistory godoc
// @Summary Get client transfer history
// @Description Returns transfer history for a client, newest first. Supports pagination.
// @Tags transfers
// @Produce json
// @Param clientId path int true "Client ID"
// @Param page query int false "Page number" minimum(1)
// @Param page_size query int false "Page size" minimum(1) maximum(100)
// @Success 200 {object} dto.ListTransfersResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/{clientId}/transfers [get]
func (h *TransferHandler) GetTransferHistory(c *gin.Context) {
	clientID, err := parseClientID(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	var query dto.ListTransfersQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	response, err := h.service.GetTransferHistory(c.Request.Context(), clientID, query.Page, query.PageSize)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, response)
}

func parseClientID(c *gin.Context) (uint, error) {
	value, err := strconv.ParseUint(c.Param("clientId"), 10, 64)
	if err != nil {
		return 0, errors.BadRequestErr("invalid client id")
	}
	return uint(value), nil
}
