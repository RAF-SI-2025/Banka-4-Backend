package handler

import (
	"net/http"
	"strconv"

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

func (h *EmployeeHandler) Login(c *gin.Context) {
	var req dto.LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	res, err := h.service.Login(c.Request.Context(), &req)

	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, res)
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

func (h *EmployeeHandler) UpdateEmployee(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(errors.BadRequestErr("invalid employee id"))
		return
	}

	var req dto.UpdateEmployeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	employee, svcErr := h.service.UpdateEmployee(c.Request.Context(), uint(id), &req)
	if svcErr != nil {
		c.Error(svcErr)
		return
	}

	c.JSON(http.StatusOK, dto.ToEmployeeResponse(employee))
}

func (h *EmployeeHandler) Activate(c *gin.Context) {
	var req dto.ActivateEmployeeRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	err := h.service.ActivateAccount(c.Request.Context(), req.Token, req.Password)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password set successfully"})
}

func (h *EmployeeHandler) ForgotPassword(c *gin.Context) {
	// Čita iz JSON-a i puni req
	var req dto.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	// Zove forgotPassword iz employee_service-a
	if err := h.service.RequestPasswordReset(c.Request.Context(), req.Email); err != nil {
		c.Error(err)
		return
	}

	// Vraća poruku da je sve prošlo kako treba
	c.JSON(http.StatusOK, gin.H{"message": "If that email is registered, a reset token has been sent"})
}

func (h *EmployeeHandler) ResetPassword(c *gin.Context) {
	var req dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequestErr(err.Error()))
		return
	}
	if err := h.service.ConfirmPasswordReset(c.Request.Context(), req.Token, req.NewPassword); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully"})
}
