package service

import (
	"common/pkg/errors"
	"common/pkg/jwt"
	"common/pkg/permission"

	"context"
	"fmt"
	"time"

	"crypto/rand"
	"encoding/hex"

	"user-service/internal/config"
	"user-service/internal/dto"
	"user-service/internal/model"
	"user-service/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

type EmployeeService struct {
	repo           repository.EmployeeRepository // <-- no pointer
	tokenRepo      repository.ActivationTokenRepository
	resetTokenRepo repository.ResetTokenRepository
	positionRepo   repository.PositionRepository
	emailService   *EmailService
	cfg            *config.Configuration
}

func NewEmployeeService(
	repo repository.EmployeeRepository, tokenRepo repository.ActivationTokenRepository, resetTokenRepo repository.ResetTokenRepository, positionRepo repository.PositionRepository, emailService *EmailService, cfg *config.Configuration) *EmployeeService {
	return &EmployeeService{
		repo:           repo,
		tokenRepo:      tokenRepo,
		resetTokenRepo: resetTokenRepo,
		positionRepo:   positionRepo,
		emailService:   emailService,
		cfg:            cfg,
	}
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
		Permissions: mapPermissions(0, req.Permissions),
	}

	if err := s.repo.Create(ctx, employee); err != nil {
		return nil, errors.InternalErr(err)
	}

	// slanje emaila
	tokenBytes := make([]byte, 16) // 128-bit token
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, errors.InternalErr(err)
	}
	tokenStr := hex.EncodeToString(tokenBytes)

	activationToken := &model.ActivationToken{
		EmployeeID: employee.EmployeeID,
		Token:      tokenStr,
		ExpiresAt:  time.Now().Add(24 * time.Hour), // token važi 24h
	}

	if err := s.tokenRepo.Create(ctx, activationToken); err != nil {
		return nil, errors.InternalErr(err)
	}

	link := fmt.Sprintf("http://localhost:8080/activate?token=%s", tokenStr)

	s.emailService.Send(
		employee.Email,
		"Welcome!",
		fmt.Sprintf("Kliknite ovde da postavite lozinku: %s", link),
	)

	return employee, nil
}

func (s *EmployeeService) ActivateAccount(ctx context.Context, tokenStr, password string) error {
	// Pronađi token u bazi
	activationToken, err := s.tokenRepo.FindByToken(ctx, tokenStr)
	if err != nil || activationToken == nil {
		return errors.BadRequestErr("invalid or expired token")
	}

	// Provera da li je token istekao
	if activationToken.ExpiresAt.Before(time.Now()) {
		return errors.BadRequestErr("token expired")
	}

	// Nađi zaposlenog preko EmployeeID iz tokena
	employee, err := s.repo.FindByID(ctx, activationToken.EmployeeID)
	if err != nil {
		return errors.InternalErr(err)
	}
	if employee == nil {
		return errors.ConflictErr("employee not found")
	}

	// Hash lozinke
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return errors.InternalErr(err)
	}

	employee.Password = string(hashedPassword)
	if err := s.repo.Update(ctx, employee); err != nil {
		return errors.InternalErr(err)
	}

	// Obriši token jer je iskorišćen
	_ = s.tokenRepo.Delete(ctx, activationToken)

	// Pošalji mejl
	s.emailService.Send(employee.Email, "Account activated", "Vaš nalog je uspešno aktiviran.")

	return nil
}

func (s *EmployeeService) UpdateEmployee(ctx context.Context, id uint, req *dto.UpdateEmployeeRequest) (*model.Employee, error) {
	employee, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if employee == nil {
		return nil, errors.NotFoundErr("employee not found")
	}

	// we make sure unique fields stay unique

	if req.Email != employee.Email {
		existing, err := s.repo.FindByEmail(ctx, req.Email)
		if err != nil {
			return nil, errors.InternalErr(err)
		}
		if existing != nil {
			return nil, errors.ConflictErr("email already in use")
		}
	}

	if req.Username != employee.Username {
		existing, err := s.repo.FindByUserName(ctx, req.Username)
		if err != nil {
			return nil, errors.InternalErr(err)
		}
		if existing != nil {
			return nil, errors.ConflictErr("username already in use")
		}
	}

	if req.PositionID != employee.PositionID {
		exists, err := s.positionRepo.Exists(ctx, req.PositionID)
		if err != nil {
			return nil, errors.InternalErr(err)
		}
		if !exists {
			return nil, errors.BadRequestErr("invalid position_id")
		}
	}

	employee.FirstName = req.FirstName
	employee.LastName = req.LastName
	employee.Gender = req.Gender
	employee.DateOfBirth = req.DateOfBirth
	employee.Email = req.Email
	employee.PhoneNumber = req.PhoneNumber
	employee.Address = req.Address
	employee.Username = req.Username
	employee.Department = req.Department
	employee.PositionID = req.PositionID
	employee.Active = req.Active
	employee.Permissions = mapPermissions(employee.EmployeeID, req.Permissions)

	if err := s.repo.Update(ctx, employee); err != nil {
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

func (s *EmployeeService) RequestPasswordReset(ctx context.Context, email string) error {
	// Pronađi zaposlenog po emailu
	employee, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		return errors.InternalErr(err)
	}
	// Provera da li zaposleni postoji
	if employee == nil {
		return nil
	}

	// Obriši stari reset token ako postoji, zaposleni može imati samo jedan aktivan token
	if err := s.resetTokenRepo.DeleteByEmployeeID(ctx, employee.EmployeeID); err != nil {
		return errors.InternalErr(err)
	}

	// Generišemo kriptografski siguran hex token
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return errors.InternalErr(err)
	}
	token := hex.EncodeToString(tokenBytes)

	// Čuvamo token u bazi sa rokom važenja od 15 minuta
	resetToken := &model.ResetToken{
		EmployeeID: employee.EmployeeID,
		Token:      token,
		ExpiresAt:  time.Now().Add(15 * time.Minute),
	}

	if err := s.resetTokenRepo.Create(ctx, resetToken); err != nil {
		return errors.InternalErr(err)
	}

	// Šaljemo link sa tokenom na email
	link := fmt.Sprintf("http://localhost:8080/reset-password?token=%s", token)
	s.emailService.Send(
		employee.Email,
		"Password reset",
		fmt.Sprintf("Kliknite ovde da resetujete lozinku: %s", link),
	)

	return nil
}

func (s *EmployeeService) ConfirmPasswordReset(ctx context.Context, token, newPassword string) error {
	// Pronađi token po kodu iz linka
	resetToken, err := s.resetTokenRepo.FindByToken(ctx, token)
	if err != nil {
		return errors.InternalErr(err)
	}
	if resetToken == nil {
		return errors.BadRequestErr("invalid or expired token")
	}

	// Provera da li je token istekao
	if resetToken.ExpiresAt.Before(time.Now()) {
		// Čistimo istekli token iz baze
		_ = s.resetTokenRepo.DeleteByEmployeeID(ctx, resetToken.EmployeeID)
		return errors.BadRequestErr("token has expired")
	}

	// Nađi zaposlenog preko EmployeeID iz tokena
	employee, err := s.repo.FindByID(ctx, resetToken.EmployeeID)
	if err != nil {
		return errors.InternalErr(err)
	}
	if employee == nil {
		return errors.NotFoundErr("employee not found")
	}

	// Hash nove lozinke
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.InternalErr(err)
	}

	employee.Password = string(hashedPassword)
	if err := s.repo.Update(ctx, employee); err != nil {
		return errors.InternalErr(err)
	}

	// Obriši token jer je iskorišćen, kod je jednokratan
	_ = s.resetTokenRepo.DeleteByEmployeeID(ctx, employee.EmployeeID)

	// Pošalji potvrdu na email
	s.emailService.Send(
		employee.Email,
		"Password changed",
		"Vaša lozinka je uspešno promenjena.",
	)

	return nil
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

func (s *EmployeeService) Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error) {
	//Pronadji zaposlenog po email-u
	employee, err := s.repo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if employee == nil {
		return nil, errors.UnauthorizedErr("invalid credentials")
	}

	//Proveri da li je zaposleni aktivan
	if !employee.Active {
		return nil, errors.ForbiddenErr("account is disabled")
	}

	err = bcrypt.CompareHashAndPassword([]byte(employee.Password), []byte(req.Password))
	if err != nil {
		return nil, errors.UnauthorizedErr("invalid credentials")
	}

	//Generisi token
	token, err := jwt.GenerateToken(employee.EmployeeID, s.cfg.JWTSecret, s.cfg.JWTExpiry)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	//Vrati generisani token
	return &dto.LoginResponse{Token: token}, nil
}
