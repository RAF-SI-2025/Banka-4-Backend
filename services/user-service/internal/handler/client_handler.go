package handler

import (
	"net/http"

	"common/pkg/errors"
	"user-service/internal/dto"
	"user-service/internal/service"

	"github.com/gin-gonic/gin"
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
// @Router /api/clients/register [post]
func (h *ClientHandler) Register(c *gin.Context) {
	var req dto.CreateClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.BadRequestErr(err.Error()))
		return
	}

	_, err := h.service.Register(c.Request.Context(), &req)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Registration successful. Please check your email to activate your account."})
}
