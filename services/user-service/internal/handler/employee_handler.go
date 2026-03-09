package handler

import (
	"net/http"

	"common/pkg/errors"
	"user-service/internal/dto"
	"user-service/internal/service"

	"github.com/gin-gonic/gin"
)

type EmployeeHandler struct {
	service *service.EmployeeService
}

func NewEmployeeHandler(service *service.EmployeeService) *EmployeeHandler {
	return &EmployeeHandler{service: service}
}

func (h *EmployeeHandler) Register(c *gin.Context) {

	var req dto.CreateEmployeeRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	employee, err := h.service.Register(c.Request.Context(), &req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, dto.ToEmployeeResponse(employee))
}

// Gets list of employees with filtering and pagination
func (h *EmployeeHandler) ListEmployees(c *gin.Context) {
	var query dto.ListEmployeesQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.Error(errors.BadRequestErr(err.Error()))
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
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, result)
}
