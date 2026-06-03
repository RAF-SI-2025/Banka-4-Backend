package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/service"
)

type RecurringOrderHandler struct {
	service *service.RecurringOrderService
}

func NewRecurringOrderHandler(service *service.RecurringOrderService) *RecurringOrderHandler {
	return &RecurringOrderHandler{service: service}
}

// CreateRecurringOrder godoc
// @Summary Create a recurring order
// @Description Creates a recurring market order that executes automatically at the given cadence
// @Tags recurring-orders
// @Accept json
// @Produce json
// @Param request body dto.CreateRecurringOrderRequest true "Recurring order details"
// @Success 201 {object} dto.RecurringOrderResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Security BearerAuth
// @Router /api/recurring-orders [post]
func (h *RecurringOrderHandler) CreateRecurringOrder(c *gin.Context) {
	var req dto.CreateRecurringOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	ro, err := h.service.CreateRecurringOrder(c.Request.Context(), req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToRecurringOrderResponse(*ro))
}

// DeleteRecurringOrder godoc
// @Summary Delete a recurring order
// @Description Cancels and deletes a recurring order owned by the authenticated user
// @Tags recurring-orders
// @Produce json
// @Param id path int true "Recurring Order ID"
// @Success 204
// @Failure 400 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/recurring-orders/{id} [delete]
func (h *RecurringOrderHandler) DeleteRecurringOrder(c *gin.Context) {
	id, err := parseRecurringOrderID(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	if err := h.service.DeleteRecurringOrder(c.Request.Context(), id); err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusNoContent)
}

// GetMyRecurringOrders godoc
// @Summary Get recurring orders for the authenticated user
// @Description Returns all recurring orders belonging to the authenticated client or actuary
// @Tags recurring-orders
// @Produce json
// @Success 200 {array} dto.RecurringOrderResponse
// @Failure 401 {object} errors.AppError
// @Security BearerAuth
// @Router /api/recurring-orders [get]
func (h *RecurringOrderHandler) GetMyRecurringOrders(c *gin.Context) {
	orders, err := h.service.GetMyRecurringOrders(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, dto.ToRecurringOrderResponseList(orders))
}

// PauseRecurringOrder godoc
// @Summary Pause or resume a recurring order
// @Description Toggles the active state of a recurring order owned by the authenticated user
// @Tags recurring-orders
// @Produce json
// @Param id path int true "Recurring Order ID"
// @Success 200 {object} dto.RecurringOrderResponse
// @Failure 400 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/recurring-orders/{id}/pause [patch]
func (h *RecurringOrderHandler) PauseRecurringOrder(c *gin.Context) {
	id, err := parseRecurringOrderID(c)
	if err != nil {
		_ = c.Error(err)
		return
	}

	ro, err := h.service.PauseRecurringOrder(c.Request.Context(), id)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, dto.ToRecurringOrderResponse(*ro))
}

func parseRecurringOrderID(c *gin.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return 0, errors.BadRequestErr("invalid recurring order id")
	}
	return uint(id), nil
}
