package dto

type ActivateEmployeeRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,password"`
}
