package dto

import (
	"common/pkg/permission"
	"time"
	"user-service/internal/model"
)

type EmployeeResponse struct {
	Id          uint                    `json:"id"`
	FirstName   string                  `json:"first_name"`
	LastName    string                  `json:"last_name"`
	Gender      string                  `json:"gender"`
	DateOfBirth time.Time               `json:"date_of_birth"`
	Email       string                  `json:"email"`
	PhoneNumber string                  `json:"phone_number"`
	Address     string                  `json:"address"`
	Username    string                  `json:"username"`
	Department  string                  `json:"department"`
	PositionID  uint                    `json:"position_id"`
	Active      bool                    `json:"active"`
	Permissions []permission.Permission `json:"permissions"`
}

type ListEmployeesResponse struct {
	Data       []EmployeeResponse `json:"data"`
	Total      int64              `json:"total"`
	Page       int                `json:"page"`
	PageSize   int                `json:"page_size"`
	TotalPages int                `json:"total_pages"`
}

func ToEmployeeResponse(e *model.Employee) *EmployeeResponse {
	permissions := make([]permission.Permission, len(e.Permissions))
	for i, ep := range e.Permissions {
		permissions[i] = ep.Permission
	}

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
		Permissions: permissions,
	}
}

func ToEmployeeResponseList(employees []model.Employee, total int64, page, pageSize int) *ListEmployeesResponse {
	responses := make([]EmployeeResponse, len(employees))
	for i, e := range employees {
		responses[i] = *ToEmployeeResponse(&e)
	}

	totalPages := (int(total) + pageSize - 1) / pageSize

	return &ListEmployeesResponse{
		Data:       responses,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}
}
