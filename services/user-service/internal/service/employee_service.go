package service

import (
	"common/pkg/auth"
	"common/pkg/errors"
	"common/pkg/permission"
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"user-service/internal/config"
	"user-service/internal/dto"
	"user-service/internal/model"
	"user-service/internal/repository"
)

type EmployeeService struct {
	employeeRepo        repository.EmployeeRepository
	identityRepo        repository.IdentityRepository
	activationTokenRepo repository.ActivationTokenRepository
	positionRepo        repository.PositionRepository
	emailService        Mailer
	cfg                 *config.Configuration
}

func NewEmployeeService(
	employeeRepo repository.EmployeeRepository,
	identityRepo repository.IdentityRepository,
	activationTokenRepo repository.ActivationTokenRepository,
	positionRepo repository.PositionRepository,
	emailService Mailer,
	cfg *config.Configuration,
) *EmployeeService {
	return &EmployeeService{
		employeeRepo:        employeeRepo,
		identityRepo:        identityRepo,
		activationTokenRepo: activationTokenRepo,
		positionRepo:        positionRepo,
		emailService:        emailService,
		cfg:                 cfg,
	}
}

func (s *EmployeeService) Register(ctx context.Context, req *dto.CreateEmployeeRequest) (*model.Employee, error) {
	emailExists, err := s.identityRepo.EmailExists(ctx, req.Email)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	if emailExists {
		return nil, errors.ConflictErr("email already in use")
	}

	usernameExists, err := s.identityRepo.UsernameExists(ctx, req.Username)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	if usernameExists {
		return nil, errors.ConflictErr("username already in use")
	}

	positionValid, err := s.positionRepo.Exists(ctx, req.PositionID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	if !positionValid {
		return nil, errors.BadRequestErr("invalid position id")
	}

	identity := &model.Identity{
		Email:    req.Email,
		Username: req.Username,
		Type:     auth.IdentityEmployee,
		Active:   req.Active,
	}

	if err := s.identityRepo.Create(ctx, identity); err != nil {
		return nil, errors.InternalErr(err)
	}

	employee := &model.Employee{
		IdentityID:  identity.ID,
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		Gender:      req.Gender,
		DateOfBirth: req.DateOfBirth,
		PhoneNumber: req.PhoneNumber,
		Address:     req.Address,
		Department:  req.Department,
		PositionID:  req.PositionID,
		Permissions: mapPermissions(0, req.Permissions),
	}

	if err := s.employeeRepo.Create(ctx, employee); err != nil {
		return nil, errors.InternalErr(err)
	}

	tokenStr, err := generateSecureToken(16)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	activationToken := &model.ActivationToken{
		IdentityID: identity.ID,
		Token:      tokenStr,
		ExpiresAt:  time.Now().Add(24 * time.Hour),
	}

	if err := s.activationTokenRepo.Create(ctx, activationToken); err != nil {
		return nil, errors.InternalErr(err)
	}

	activationBase := strings.TrimRight(s.cfg.URLs.FrontendBaseURL, "/")
	link := fmt.Sprintf("%s/activate?token=%s", activationBase, url.QueryEscape(tokenStr))

	if err := s.emailService.Send(
		identity.Email,
		"Welcome!",
		fmt.Sprintf("Kliknite ovde da postavite lozinku: %s", link),
	); err != nil {
		return nil, errors.ServiceUnavailableErr(err)
	}

	employee.Identity = *identity
	return employee, nil
}

func (s *EmployeeService) UpdateEmployee(ctx context.Context, id uint, req *dto.UpdateEmployeeRequest) (*model.Employee, error) {
	employee, err := s.employeeRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	if employee == nil {
		return nil, errors.NotFoundErr("employee not found")
	}

	identity, err := s.identityRepo.FindByID(ctx, employee.IdentityID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	if identity == nil {
		return nil, errors.NotFoundErr("identity not found")
	}

	identityChanged := false

	if req.Email != nil && *req.Email != identity.Email {
		emailExists, err := s.identityRepo.EmailExists(ctx, *req.Email)
		if err != nil {
			return nil, errors.InternalErr(err)
		}

		if emailExists {
			return nil, errors.ConflictErr("email already in use")
		}

		identity.Email = *req.Email
		identityChanged = true
	}

	if req.Username != nil && *req.Username != identity.Username {
		usernameExists, err := s.identityRepo.UsernameExists(ctx, *req.Username)
		if err != nil {
			return nil, errors.InternalErr(err)
		}

		if usernameExists {
			return nil, errors.ConflictErr("username already in use")
		}

		identity.Username = *req.Username
		identityChanged = true
	}

	if req.Active != nil && *req.Active != identity.Active {
		identity.Active = *req.Active
		identityChanged = true
	}

	if req.PositionID != nil && *req.PositionID != employee.PositionID {
		exists, err := s.positionRepo.Exists(ctx, *req.PositionID)
		if err != nil {
			return nil, errors.InternalErr(err)
		}

		if !exists {
			return nil, errors.BadRequestErr("invalid position_id")
		}
	}

	setIfNotNil(&employee.FirstName, req.FirstName)
	setIfNotNil(&employee.LastName, req.LastName)
	setIfNotNil(&employee.Gender, req.Gender)
	setIfNotNil(&employee.DateOfBirth, req.DateOfBirth)
	setIfNotNil(&employee.PhoneNumber, req.PhoneNumber)
	setIfNotNil(&employee.Address, req.Address)
	setIfNotNil(&employee.Department, req.Department)
	setIfNotNil(&employee.PositionID, req.PositionID)

	if req.Permissions != nil {
		employee.Permissions = mapPermissions(employee.EmployeeID, *req.Permissions)
	}

	if identityChanged {
		if err := s.identityRepo.Update(ctx, identity); err != nil {
			return nil, errors.InternalErr(err)
		}
	}

	if err := s.employeeRepo.Update(ctx, employee); err != nil {
		return nil, errors.InternalErr(err)
	}

	employee.Identity = *identity
	return employee, nil
}

func (s *EmployeeService) GetEmployeeByID(ctx context.Context, id uint) (*dto.EmployeeResponse, error) {
	employee, err := s.employeeRepo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if employee == nil {
		return nil, errors.NotFoundErr("employee not found")
	}

	return dto.ToEmployeeResponse(employee), nil
}

func (s *EmployeeService) GetAllEmployees(ctx context.Context, query *dto.ListEmployeesQuery) (*dto.ListEmployeesResponse, error) {
	employees, total, err := s.employeeRepo.GetAll(ctx, query.Email, query.FirstName, query.LastName, query.Position, query.Page, query.PageSize)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	return dto.ToEmployeeResponseList(employees, total, query.Page, query.PageSize), nil
}

func setIfNotNil[T any](dst *T, src *T) {
	if src != nil {
		*dst = *src
	}
}

func mapPermissions(employeeID uint, permissions []permission.Permission) []model.EmployeePermission {
	result := make([]model.EmployeePermission, len(permissions))
	for i, p := range permissions {
		result[i] = model.EmployeePermission{
			EmployeeID: employeeID,
			Permission: p,
		}
	}
	return result
}
