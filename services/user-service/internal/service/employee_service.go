package service

import (
	"context"

	"common/pkg/errors"
	"user-service/internal/dto"
	"user-service/internal/model"
	"user-service/internal/repository"
)

type EmployeeService struct {
	repo repository.EmployeeRepository // <-- no pointer
}

func NewEmployeeService(repo repository.EmployeeRepository) *EmployeeService {
	return &EmployeeService{repo: repo}
}

func (s *EmployeeService) Register(ctx context.Context, req *dto.CreateEmployeeRequest) (*model.Employee, error) {

	existing, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	if existing != nil {
		return nil, errors.ConflictErr("email already in use")
	}

	existingByUsername, err := s.repo.FindByUserName(ctx, req.Username)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if existingByUsername != nil {
		return nil, errors.ConflictErr("username already in use")
	}
	employee := &model.Employee{
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		Gender:      req.Gender,
		DateOfBirth: req.DateOfBirth,
		Email:       req.Email,
		PhoneNumber: req.PhoneNumber,
		Address:     req.Address,
		Username:    req.Username,
		Department:  req.Department,
		PositionID:  req.PositionID,
		Active:      req.Active,
	}

	if err := s.repo.Create(ctx, employee); err != nil {
		return nil, errors.InternalErr(err)
	}

	return employee, nil
}

func (s *EmployeeService) GetAllEmployees(ctx context.Context, query *dto.ListEmployeesQuery) (*dto.ListEmployeesResponse, error) {
	employees, total, err := s.repo.GetAll(ctx, query.Email, query.FirstName, query.LastName, query.Position, query.Page, query.PageSize)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	return dto.ToEmployeeResponseList(employees, total, query.Page, query.PageSize), nil
}
