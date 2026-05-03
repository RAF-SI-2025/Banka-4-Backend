package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/service"
)

type EmployeeHandler struct {
	service *service.EmployeeService
}

func NewEmployeeHandler(service *service.EmployeeService) *EmployeeHandler {
	return &EmployeeHandler{service: service}
}

// Register godoc
// @Summary Register a new employee
// @Description Creates a new employee account and returns the created employee
// @Tags employees
// @Accept json
// @Produce json
// @Param employee body dto.CreateEmployeeRequest true "Employee registration data"
// @Success 201 {object} dto.EmployeeResponse
// @Failure 400 {object} errors.AppError
// @Failure 409 {object} errors.AppError
// @Security BearerAuth
// @Router /api/employees/register [post]
func (h *EmployeeHandler) Register(c *gin.Context) {
	var req dto.CreateEmployeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	employee, err := h.service.Register(c.Request.Context(), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToEmployeeResponse(employee))
}

// ListEmployees godoc
// @Summary List employees
// @Description Returns a paginated list of employees with optional filtering
// @Tags employees
// @Produce json
// @Param email query string false "Filter by email"
// @Param first_name query string false "Filter by first name"
// @Param last_name query string false "Filter by last name"
// @Param position query string false "Filter by position"
// @Param page query int false "Page number" minimum(1)
// @Param page_size query int false "Page size" minimum(1) maximum(100)
// @Success 200 {object} dto.ListEmployeesResponse
// @Failure 400 {object} errors.AppError
// @Security BearerAuth
// @Router /api/employees [get]
func (h *EmployeeHandler) ListEmployees(c *gin.Context) {
	var query dto.ListEmployeesQuery
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

	result, err := h.service.GetAllEmployees(c.Request.Context(), &query)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetEmployee godoc
// @Summary Get employee by ID
// @Description Returns a single employee by their ID
// @Tags employees
// @Produce json
// @Param id path int true "Employee ID"
// @Success 200 {object} dto.EmployeeResponse
// @Failure 400 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/employees/{id} [get]
func (h *EmployeeHandler) GetEmployee(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid employee id"))
		return
	}

	result, svcErr := h.service.GetEmployeeByID(c.Request.Context(), uint(id))
	if svcErr != nil {
		_ = c.Error(svcErr)
		return
	}

	c.JSON(http.StatusOK, result)
}

// UpdateEmployee godoc
// @Summary Update employee profile
// @Description Updates employee information by ID
// @Tags employees
// @Accept json
// @Produce json
// @Param id path int true "Employee ID"
// @Param employee body dto.UpdateEmployeeRequest true "Employee update data"
// @Success 200 {object} dto.EmployeeResponse
// @Failure 400 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Failure 409 {object} errors.AppError
// @Security BearerAuth
// @Router /api/employees/{id} [patch]
func (h *EmployeeHandler) UpdateEmployee(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid employee id"))
		return
	}

	var req dto.UpdateEmployeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	employee, svcErr := h.service.UpdateEmployee(c.Request.Context(), uint(id), &req)
	if svcErr != nil {
		_ = c.Error(svcErr)
		return
	}

	c.JSON(http.StatusOK, dto.ToEmployeeResponse(employee))
}

// DeactivateEmployee godoc
// @Summary Deactivate employee
// @Description Sets the employee's active status to false
// @Tags employees
// @Produce json
// @Param id path int true "Employee ID"
// @Success 204
// @Failure 400 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/employees/{id}/deactivate [post]
func (h *EmployeeHandler) DeactivateEmployee(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid employee id"))
		return
	}

	if err := h.service.DeactivateEmployee(c.Request.Context(), uint(id)); err != nil {
		_ = c.Error(err)
		return
	}

	c.Status(http.StatusNoContent)
}
