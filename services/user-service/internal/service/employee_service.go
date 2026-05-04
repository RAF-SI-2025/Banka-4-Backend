package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/permission"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/config"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/repository"
)

type EmployeeService struct {
	employeeRepo        repository.EmployeeRepository
	identityRepo        repository.IdentityRepository
	activationTokenRepo repository.ActivationTokenRepository
	positionRepo        repository.PositionRepository
	emailService        Mailer
	cfg                 *config.Configuration
	txManager           repository.TransactionManager
	tradingClient       client.TradingClient
}

func NewEmployeeService(
	employeeRepo repository.EmployeeRepository,
	identityRepo repository.IdentityRepository,
	activationTokenRepo repository.ActivationTokenRepository,
	positionRepo repository.PositionRepository,
	emailService Mailer,
	cfg *config.Configuration,
	txManager repository.TransactionManager,
	tradingClient client.TradingClient,
) *EmployeeService {
	return &EmployeeService{
		employeeRepo:        employeeRepo,
		identityRepo:        identityRepo,
		activationTokenRepo: activationTokenRepo,
		positionRepo:        positionRepo,
		emailService:        emailService,
		cfg:                 cfg,
		txManager:           txManager,
		tradingClient:       tradingClient,
	}
}

func (s *EmployeeService) Register(ctx context.Context, req *dto.CreateEmployeeRequest) (*model.Employee, error) {
	actor, err := s.currentActor(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.ensureAdminControls(actor, len(req.Permissions) > 0, req.IsAgent || req.IsSupervisor); err != nil {
		return nil, err
	}
	if !req.IsAgent && !req.IsSupervisor && (req.Limit > 0 || req.NeedApproval) {
		return nil, errors.BadRequestErr("actuary settings require agent or supervisor role")
	}

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

	employee := &model.Employee{
		IdentityID:  0,
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

	actuaryInfo, err := buildActuaryInfo(nil, employee.IsAdmin(), req.IsAgent, req.IsSupervisor, req.Limit, req.NeedApproval)
	if err != nil {
		return nil, err
	}
	employee.ActuaryInfo = actuaryInfo

	tokenStr, err := generateSecureToken(16)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	activationToken := &model.ActivationToken{
		IdentityID: 0,
		Token:      tokenStr,
		ExpiresAt:  time.Now().Add(24 * time.Hour),
	}

	if err := s.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.identityRepo.Create(txCtx, identity); err != nil {
			return errors.InternalErr(err)
		}

		employee.IdentityID = identity.ID
		if err := s.employeeRepo.Create(txCtx, employee); err != nil {
			return errors.InternalErr(err)
		}

		activationToken.IdentityID = identity.ID
		if err := s.activationTokenRepo.Create(txCtx, activationToken); err != nil {
			return errors.InternalErr(err)
		}

		return nil
	}); err != nil {
		return nil, err
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
	actor, err := s.currentActor(ctx)
	if err != nil {
		return nil, err
	}

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

	if employee.IsAdmin() && actor.EmployeeID != employee.EmployeeID {
		return nil, errors.ForbiddenErr("cannot modify admin")
	}

	if err := s.ensureAdminControls(actor, req.Permissions != nil, req.IsAgent != nil || req.IsSupervisor != nil); err != nil {
		return nil, err
	}

	if employee.IsAdmin() && (req.Permissions != nil || req.IsAgent != nil || req.IsSupervisor != nil) {
		return nil, errors.ForbiddenErr("cannot modify admin permissions")
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

	desiredIsAgent := employee.IsAgent()
	desiredIsSupervisor := employee.ActuaryInfo != nil && employee.ActuaryInfo.IsSupervisor
	if req.IsAgent != nil {
		desiredIsAgent = *req.IsAgent
	}
	if req.IsSupervisor != nil {
		desiredIsSupervisor = *req.IsSupervisor
	}

	// Snimiti pre update-a
	wasSupervisor := employee.IsSupervisor()

	actuaryInfo, err := buildActuaryInfo(employee.ActuaryInfo, employee.IsAdmin(), desiredIsAgent, desiredIsSupervisor, currentActuaryLimit(employee.ActuaryInfo), currentActuaryNeedApproval(employee.ActuaryInfo))
	if err != nil {
		return nil, err
	}
	employee.ActuaryInfo = actuaryInfo

	if err := s.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		if identityChanged {
			if err := s.identityRepo.Update(txCtx, identity); err != nil {
				return errors.InternalErr(err)
			}
		}

		if err := s.employeeRepo.Update(txCtx, employee); err != nil {
			return errors.InternalErr(err)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	if wasSupervisor && !employee.IsSupervisor() {
		if err := s.tradingClient.TransferManagerFunds(ctx, employee.EmployeeID, actor.EmployeeID); err != nil {
			fmt.Printf("WARN: failed to transfer funds from supervisor %d to admin %d: %v\n",
				employee.EmployeeID, actor.EmployeeID, err)
		}
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

func (s *EmployeeService) currentActor(ctx context.Context) (*model.Employee, error) {
	authCtx := auth.GetAuthFromContext(ctx)
	if authCtx == nil {
		return nil, errors.UnauthorizedErr("not authenticated")
	}

	var (
		employee *model.Employee
		err      error
	)

	if authCtx.EmployeeID != nil {
		employee, err = s.employeeRepo.FindByID(ctx, *authCtx.EmployeeID)
	} else {
		employee, err = s.employeeRepo.FindByIdentityID(ctx, authCtx.IdentityID)
	}
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if employee == nil {
		return nil, errors.NotFoundErr("employee not found")
	}

	return employee, nil
}

func (s *EmployeeService) ensureAdminControls(actor *model.Employee, permissionChange, roleChange bool) error {
	if (permissionChange || roleChange) && !actor.IsAdmin() {
		return errors.ForbiddenErr("only admins can modify employee permissions")
	}

	return nil
}

func buildActuaryInfo(existing *model.ActuaryInfo, isAdmin, isAgent, isSupervisor bool, limit float64, needApproval bool) (*model.ActuaryInfo, error) {
	if isAdmin {
		isAgent = false
		isSupervisor = true
	}

	if isAgent && isSupervisor {
		return nil, errors.BadRequestErr("employee cannot be both agent and supervisor")
	}

	if !isAgent && !isSupervisor {
		return nil, nil
	}

	actuary := &model.ActuaryInfo{}
	if existing != nil {
		*actuary = *existing
	}

	actuary.IsAgent = isAgent
	actuary.IsSupervisor = isSupervisor

	if isSupervisor {
		actuary.Limit = 0
		actuary.UsedLimit = 0
		actuary.NeedApproval = false
		return actuary, nil
	}

	actuary.Limit = limit
	actuary.NeedApproval = needApproval
	return actuary, nil
}

func currentActuaryLimit(actuary *model.ActuaryInfo) float64 {
	if actuary == nil {
		return 0
	}

	return actuary.Limit
}

func currentActuaryNeedApproval(actuary *model.ActuaryInfo) bool {
	if actuary == nil {
		return false
	}

	return actuary.NeedApproval
}

func (s *EmployeeService) DeactivateEmployee(ctx context.Context, id uint) error {
	employee, err := s.employeeRepo.FindByID(ctx, id)
	if err != nil {
		return errors.InternalErr(err)
	}
	if employee == nil {
		return errors.NotFoundErr("employee not found")
	}

	if employee.IsAdmin() {
		return errors.ForbiddenErr("cannot deactivate admin")
	}

	identity, err := s.identityRepo.FindByID(ctx, employee.IdentityID)
	if err != nil {
		return errors.InternalErr(err)
	}
	if identity == nil {
		return errors.NotFoundErr("identity not found")
	}

	identity.Active = false

	if err := s.identityRepo.Update(ctx, identity); err != nil {
		return errors.InternalErr(err)
	}

	return nil
}
