package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/service"
)

type PayeeHandler struct {
	service *service.PayeeService
}

func NewPayeeHandler(service *service.PayeeService) *PayeeHandler {
	return &PayeeHandler{service: service}
}

// GetAll godoc
// @Summary List all payees
// @Description Returns a list of all payees for the authenticated client.
// @Tags payees
// @Produce json
// @Success 200 {array} dto.PayeeResponse
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Security BearerAuth
// @Router /payees [get]
func (h *PayeeHandler) GetAll(c *gin.Context) {
	payees, err := h.service.GetAll(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}

	response := make([]dto.PayeeResponse, len(payees))
	for i, p := range payees {
		response[i] = dto.ToPayeeResponse(&p)
	}

	c.JSON(http.StatusOK, response)
}

// Create godoc
// @Summary Create a new payee
// @Description Creates a new payee for the authenticated client.
// @Tags payees
// @Accept json
// @Produce json
// @Param request body dto.CreatePayeeRequest true "Payee data"
// @Success 201 {object} dto.PayeeResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Security BearerAuth
// @Router /payees [post]
func (h *PayeeHandler) Create(c *gin.Context) {
	var req dto.CreatePayeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	payee, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToPayeeResponse(payee))
}

// Update godoc
// @Summary Update a payee
// @Description Updates an existing payee by ID.
// @Tags payees
// @Accept json
// @Produce json
// @Param id path int true "Payee ID"
// @Param request body dto.UpdatePayeeRequest true "Updated payee data"
// @Success 200 {object} dto.PayeeResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Security BearerAuth
// @Router /payees/{id} [patch]
func (h *PayeeHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid payee id"))
		return
	}

	var req dto.UpdatePayeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	payee, err := h.service.Update(c.Request.Context(), uint(id), req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, dto.ToPayeeResponse(payee))
}

// Delete godoc
// @Summary Delete a payee
// @Description Deletes a payee by ID.
// @Tags payees
// @Produce json
// @Param id path int true "Payee ID"
// @Success 204 "No Content"
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Failure 500 {object} errors.AppError
// @Security BearerAuth
// @Router /payees/{id} [delete]
func (h *PayeeHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid payee id"))
		return
	}

	if err := h.service.Delete(c.Request.Context(), uint(id)); err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusNoContent)
}
