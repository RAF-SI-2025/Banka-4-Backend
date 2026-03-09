package service

import (
	"context"
	"fmt"
	"unicode"

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

	// slanje emaila
	es := NewEmailService()
	link := GenerateActivationLink(employee.Email)
	es.Send(employee.Email, "Welcome!", fmt.Sprintf("Kliknite ovde da postavite lozinku: %s", link))

	return employee, nil
}
func isValidPassword(password string) bool {

	if len(password) < 8 || len(password) > 32 {
		return false
	}

	var upper, lower, digits int

	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			upper++
		case unicode.IsLower(c):
			lower++
		case unicode.IsDigit(c):
			digits++
		}
	}

	return upper >= 1 && lower >= 1 && digits >= 2
}
func (s *EmployeeService) SetPassword(email, password string) error {
	// validacija password-a
	if !isValidPassword(password) {
		return errors.ConflictErr("password must contain 8-32 characters, 1 uppercase, 1 lowercase and 2 numbers")
	}

	employee, err := s.repo.FindByEmail(context.Background(), email)
	if err != nil || employee == nil {
		return errors.ConflictErr("employee not found")
	}

	employee.Password = password // kasnije hashovati
	return s.repo.Update(context.Background(), employee)
}
