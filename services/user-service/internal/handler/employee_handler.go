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
// @Router /api/employees/register [post]
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

// Login godoc
// @Summary Authenticate employee
// @Description Authenticates an employee and returns authentication tokens or session data
// @Tags employees
// @Accept json
// @Produce json
// @Param credentials body dto.LoginRequest true "Employee login credentials"
// @Success 200 {object} dto.LoginResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Router /api/employees/login [post]
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

// ListEmployees godoc
// @Summary List employees
// @Description Returns a paginated list of employees with optional filtering
// @Tags employees
// @Produce json
// @Param query body dto.ListEmployeesQuery true "Employee list and filtering query"
// @Success 200 {object} dto.ListEmployeesResponse
// @Failure 400 {object} errors.AppError
// @Router /api/employees [get]
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
// @Router /api/employees/{id} [patch]
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

// Activate godoc
// @Summary Activate employee account
// @Description Activates an employee account by setting the initial password using an activation token
// @Tags employees
// @Accept json
// @Produce json
// @Param activation body dto.ActivateEmployeeRequest true "Activation token and new password"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Router /api/employees/activate [post]
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

// ForgotPassword godoc
// @Summary Request password reset
// @Description Sends a password reset token to the employee email if it exists
// @Tags employees
// @Accept json
// @Produce json
// @Param request body dto.ForgotPasswordRequest true "Email address for password reset"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.AppError
// @Router /api/employees/forgot-password [post]
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

// ResetPassword godoc
// @Summary Reset employee password
// @Description Resets the employee password using a valid reset token
// @Tags employees
// @Accept json
// @Produce json
// @Param request body dto.ResetPasswordRequest true "Password reset token and new password"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Router /api/employees/reset-password [post]
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

// ChangePassword godoc
// @Summary Change employee password
// @Description Allows an authenticated employee to change their password by providing the current password and a new password.
// @Tags employees
// @Accept json
// @Produce json
// @Param request body dto.ChangePasswordRequest true "Current and new password"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Router /api/employees/change-password [post]
func (h *EmployeeHandler) ChangePassword(c *gin.Context) {
	var req dto.ChangePasswordRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequestErr(err.Error()))
		return
	}
	// proveravamo sa servisom da li je doslo do neke greske pri promeni
	if err := h.service.ConfirmChangePassword(c.Request.Context(), req.OldPassword, req.NewPassword); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "New Password set successfully"})
}
