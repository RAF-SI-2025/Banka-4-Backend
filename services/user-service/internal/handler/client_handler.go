package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/service"
)

type ClientHandler struct {
	service *service.ClientService
}

func NewClientHandler(service *service.ClientService) *ClientHandler {
	return &ClientHandler{service: service}
}

// Register godoc
// @Summary Register a new client
// @Description Creates a new client account and sends an activation email
// @Tags clients
// @Accept json
// @Produce json
// @Param client body dto.CreateClientRequest true "Client registration data"
// @Success 201 {object} map[string]string
// @Failure 400 {object} errors.AppError
// @Failure 409 {object} errors.AppError
// @Failure 503 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/register [post]
func (h *ClientHandler) Register(c *gin.Context) {
	var req dto.CreateClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	_, err := h.service.Register(c.Request.Context(), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Registration successful. Please check your email to activate your account."})
}

// ListClients godoc
// @Summary List all clients
// @Description Returns a paginated list of clients. Supports filtering by email, first name, and last name. Requires ClientView permission.
// @Tags clients
// @Produce json
// @Param email query string false "Filter by email"
// @Param first_name query string false "Filter by first name"
// @Param last_name query string false "Filter by last name"
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200 {object} object
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients [get]
func (h *ClientHandler) ListClients(c *gin.Context) {
	var query dto.ListClientsQuery
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

	result, err := h.service.GetAllClients(c.Request.Context(), &query)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// UpdateClient godoc
// @Summary Update a client
// @Description Updates client details by ID. Requires ClientUpdate permission.
// @Tags clients
// @Accept json
// @Produce json
// @Param id path int true "Client ID"
// @Param client body dto.UpdateClientRequest true "Fields to update"
// @Success 200 {object} object
// @Failure 400 {object} errors.AppError
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Security BearerAuth
// @Router /api/clients/{id} [patch]
func (h *ClientHandler) UpdateClient(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		_ = c.Error(errors.BadRequestErr("invalid client id"))
		return
	}

	var req dto.UpdateClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	client, err := h.service.UpdateClient(c.Request.Context(), uint(id), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, client)
}

// GetMobileSecret godoc
// @Summary Get mobile verification secret
// @Description Returns TOTP secret for currently authenticated client
// @Tags clients
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.MobileSecretResponse
// @Failure 401 {object} errors.AppError
// @Failure 403 {object} errors.AppError
// @Failure 404 {object} errors.AppError
// @Router /api/secret-mobile [get]
func (h *ClientHandler) GetMobileSecret(c *gin.Context) {
	authCtx := auth.GetAuth(c)

	secret, err := h.service.GetMobileVerificationSecret(c.Request.Context(), *authCtx.ClientID)
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, dto.MobileSecretResponse{Secret: secret})
}
