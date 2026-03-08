package dto

import "time"

type CreateEmployeeRequest struct {
	FirstName   string    `json:"first_name" binding:"required,max=20"`
	LastName    string    `json:"last_name" binding:"required,max=100"`
	Gender      string    `json:"gender"`
	DateOfBirth time.Time `json:"date_of_birth"`
	Email       string    `json:"email" binding:"required,email"`
	PhoneNumber string    `json:"phone_number"`
	Address     string    `json:"address"`
	Username    string    `json:"username" binding:"required"`
	Department  string    `json:"department"`
	PositionID  uint      `json:"position_id" binding:"required"`
	Active      bool      `json:"active"`
}
