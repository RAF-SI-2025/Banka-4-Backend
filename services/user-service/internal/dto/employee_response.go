package dto

import (
	"time"
	"user-service/internal/model"
)

type EmployeeResponse struct {
	Id          uint      `json:"id"`
	FirstName   string    `json:"first_name"`
	LastName    string    `json:"last_name"`
	Gender      string    `json:"gender"`
	DateOfBirth time.Time `json:"date_of_birth"`
	Email       string    `json:"email"`
	PhoneNumber string    `json:"phone_number"`
	Address     string    `json:"address"`
	Username    string    `json:"username"`
	Department  string    `json:"department"`
	PositionID  uint      `json:"position_id"`
	Active      bool      `json:"active"`
}

func ToEmployeeResponse(e *model.Employee) *EmployeeResponse {
	return &EmployeeResponse{
		Id:          e.EmployeeID,
		FirstName:   e.FirstName,
		LastName:    e.LastName,
		Gender:      e.Gender,
		DateOfBirth: e.DateOfBirth,
		Email:       e.Email,
		PhoneNumber: e.PhoneNumber,
		Address:     e.Address,
		Username:    e.Username,
		Department:  e.Department,
		PositionID:  e.PositionID,
		Active:      e.Active,
	}
}
