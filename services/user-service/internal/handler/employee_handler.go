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

func (h *EmployeeHandler) Activate(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if len(req.Password) < 8 {
		c.JSON(400, gin.H{"error": "Password too short"})
		return
	}

	err := h.service.SetPassword(req.Email, req.Password)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	es := service.NewEmailService()
	es.Send(req.Email, "Account activated", "Vaš nalog je uspešno aktiviran.")

	c.JSON(200, gin.H{"message": "Password set successfully"})
}
