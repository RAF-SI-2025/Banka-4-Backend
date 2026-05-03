package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/service"
)

type AuthHandler struct {
	service *service.AuthService
}

func NewAuthHandler(service *service.AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

// Login godoc
// @Summary Authenticate user
// @Description Authenticates a user by email and password, returns JWT and refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param credentials body dto.LoginRequest true "Login credentials"
// @Success 200 {object} dto.LoginResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Router /api/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	res, err := h.service.Login(c.Request.Context(), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, res)
}

// RefreshToken godoc
// @Summary Refresh access token
// @Description Generates a new access token using a valid refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.RefreshRequest true "Refresh token"
// @Success 200 {object} dto.RefreshResponse
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Router /api/auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	res, err := h.service.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, res)
}

// Activate godoc
// @Summary Activate account
// @Description Activates an account by setting the initial password using an activation token
// @Tags auth
// @Accept json
// @Produce json
// @Param activation body dto.ActivateAccountRequest true "Activation token and new password"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Router /api/auth/activate [post]
func (h *AuthHandler) Activate(c *gin.Context) {
	var req dto.ActivateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	if err := h.service.ActivateAccount(c.Request.Context(), req.Token, req.Password); err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password set successfully"})
}

// ResendActivation godoc
// @Summary Resend activation email
// @Description Resends an activation email
// @Tags auth
// @Accept json
// @Produce json
// @Param activation body dto.ResendActivationRequest true "Email"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Router /api/auth/resend-activation [post]
func (h *AuthHandler) ResendActivation(c *gin.Context) {
	var req dto.ResendActivationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	if err := h.service.ResendActivation(c.Request.Context(), req.Email); err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "If that email is registered, an activation link has been sent"})
}

// ForgotPassword godoc
// @Summary Request password reset
// @Description Sends a password reset token to the email if it exists
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.ForgotPasswordRequest true "Email address for password reset"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.AppError
// @Router /api/auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req dto.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	if err := h.service.RequestPasswordReset(c.Request.Context(), req.Email); err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "If that email is registered, a reset token has been sent"})
}

// ResetPassword godoc
// @Summary Reset password
// @Description Resets the password using a valid reset token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.ResetPasswordRequest true "Password reset token and new password"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Router /api/auth/reset-password [post]
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	if err := h.service.ConfirmPasswordReset(c.Request.Context(), req.Token, req.NewPassword); err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully"})
}

// ChangePassword godoc
// @Summary Change password
// @Description Allows an authenticated user to change their password
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.ChangePasswordRequest true "Current and new password"
// @Success 200 {object} map[string]string
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Security BearerAuth
// @Router /api/auth/change-password [post]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	if err := h.service.ChangePassword(c.Request.Context(), req.OldPassword, req.NewPassword); err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}
