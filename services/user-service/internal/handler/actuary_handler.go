package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/service"
)

type ActuaryHandler struct {
	service *service.ActuaryService
}

func NewActuaryHandler(service *service.ActuaryService) *ActuaryHandler {
	return &ActuaryHandler{service: service}
}

// ListActuaries godoc
// @Summary List actuaries
// @Description Returns a paginated list of actuaries with optional filtering by employee and actuary fields
// @Tags actuaries
// @Produce json
// @Param email query string false "Filter by email"
// @Param first_name query string false "Filter by first name"
// @Param last_name query string false "Filter by last name"
// @Param position query string false "Filter by position"
// @Param department query string false "Filter by department"
// @Param type query string false "Filter by actuary type" Enums(agent, supervisor)
// @Param active query boolean false "Filter by active status"
// @Param need_approval query boolean false "Filter by need approval flag"
// @Param page query int false "Page number" minimum(1)
// @Param page_size query int false "Page size" minimum(1) maximum(100)
// @Success 200 {object} dto.ListActuariesResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Security BearerAuth
// @Router /api/actuaries [get]
func (h *ActuaryHandler) ListActuaries(c *gin.Context) {
	var query dto.ListActuariesQuery
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

	result, err := h.service.GetAllActuaries(c.Request.Context(), &query)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// UpdateActuarySettings godoc
// @Summary Update actuary settings
// @Description Updates an agent's limit and approval settings. Only supervisors can perform this action.
// @Tags actuaries
// @Accept json
// @Produce json
// @Param id path int true "Employee ID"
// @Param settings body dto.UpdateActuarySettingsRequest true "Actuary settings"
// @Success 200 {object} dto.ActuaryResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/actuaries/{id} [patch]
func (h *ActuaryHandler) UpdateActuarySettings(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid employee id"))
		return
	}

	var req dto.UpdateActuarySettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	actuary, svcErr := h.service.UpdateActuarySettings(c.Request.Context(), uint(id), &req)
	if svcErr != nil {
		_ = c.Error(svcErr)
		return
	}

	c.JSON(http.StatusOK, actuary)
}

// ResetUsedLimit godoc
// @Summary Reset actuary used limit
// @Description Resets an agent's used limit to zero. Only supervisors can perform this action.
// @Tags actuaries
// @Produce json
// @Param id path int true "Employee ID"
// @Success 200 {object} dto.ActuaryResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/actuaries/{id}/reset-used-limit [post]
func (h *ActuaryHandler) ResetUsedLimit(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid employee id"))
		return
	}

	actuary, svcErr := h.service.ResetUsedLimit(c.Request.Context(), uint(id))
	if svcErr != nil {
		_ = c.Error(svcErr)
		return
	}

	c.JSON(http.StatusOK, actuary)
}
