package service

import (
	"common/pkg/auth"
	"common/pkg/errors"
	"common/pkg/jwt"
	"common/pkg/permission"

	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
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
	repo             repository.EmployeeRepository // <-- no pointer
	tokenRepo        repository.ActivationTokenRepository
	resetTokenRepo   repository.ResetTokenRepository
  refreshTokenRepo repository.RefreshTokenRepository
	positionRepo     repository.PositionRepository
	emailService     Mailer
	cfg              *config.Configuration
}

func NewEmployeeService(
	repo repository.EmployeeRepository, tokenRepo repository.ActivationTokenRepository, resetTokenRepo repository.ResetTokenRepository, refreshTokenRepo repository.RefreshTokenRepository, positionRepo repository.PositionRepository, emailService Mailer, cfg *config.Configuration) *EmployeeService {
	return &EmployeeService{
		repo:             repo,
		tokenRepo:        tokenRepo,
		resetTokenRepo:   resetTokenRepo,
		refreshTokenRepo: refreshTokenRepo,
		positionRepo:     positionRepo,
		emailService:     emailService,
		cfg:              cfg,
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

	activationBase := strings.TrimRight(s.cfg.URLs.FrontendBaseURL, "/")
	link := fmt.Sprintf("%s/activate?token=%s", activationBase, url.QueryEscape(tokenStr))

	if err := s.emailService.Send(
		employee.Email,
		"Welcome!",
		fmt.Sprintf("Kliknite ovde da postavite lozinku: %s", link),
	); err != nil {
		return nil, errors.ServiceUnavailableErr(err)
	}

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
		return errors.NotFoundErr("employee not found")
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
	if err := s.emailService.Send(employee.Email, "Account activated", "Vaš nalog je uspešno aktiviran."); err != nil {
		log.Printf("failed to send account activation confirmation email to employee_id=%d: %v", employee.EmployeeID, err)
	}

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
	resetBase := strings.TrimRight(s.cfg.URLs.FrontendBaseURL, "/")
	link := fmt.Sprintf("%s/reset-password?token=%s", resetBase, url.QueryEscape(token))
	if err := s.emailService.Send(
		employee.Email,
		"Password reset",
		fmt.Sprintf("Kliknite ovde da resetujete lozinku: %s", link),
	); err != nil {
		log.Printf("failed to send password reset email to employee_id=%d: %v", employee.EmployeeID, err)
	}

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
	if err := s.emailService.Send(
		employee.Email,
		"Password changed",
		"Vaša lozinka je uspešno promenjena.",
	); err != nil {
		log.Printf("failed to send password changed confirmation email to employee_id=%d: %v", employee.EmployeeID, err)
	}

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

	_ = s.refreshTokenRepo.DeleteByEmployeeID(ctx, employee.EmployeeID)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, errors.InternalErr(err)
	}
	rawRefreshToken := hex.EncodeToString(tokenBytes)

	refreshToken := &model.RefreshToken{
		EmployeeID: employee.EmployeeID,
		Token:      rawRefreshToken,
		ExpiresAt:  time.Now().Add(time.Duration(s.cfg.RefreshExpiry) * time.Minute),
	}

	if err := s.refreshTokenRepo.Create(ctx, refreshToken); err != nil {
		return nil, errors.InternalErr(err)
	}

	//Vrati generisani token
	return &dto.LoginResponse{
		Token:        token,
		RefreshToken: rawRefreshToken,
	}, nil
}

// dodata funkcija za rotaciju tokena, kad se refresh token iskoristi vraca se novi refresh jer ce stari isteci pre roka (vec je akttivan timer za taj token)
func (s *EmployeeService) RefreshToken(ctx context.Context, refreshTokenStr string) (*dto.RefreshResponse, error) {
	storedToken, err := s.refreshTokenRepo.FindByToken(ctx, refreshTokenStr)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if storedToken == nil {
		return nil, errors.UnauthorizedErr("invalid or expired refresh token")
	}
	if storedToken.ExpiresAt.Before(time.Now()) {
		return nil, errors.UnauthorizedErr("refresh token expired")
	}

	employee, err := s.repo.FindByID(ctx, storedToken.EmployeeID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	if employee == nil {
		return nil, errors.UnauthorizedErr("user not found")
	}
	if !employee.Active {
		return nil, errors.ForbiddenErr("account is disabled")
	}

	_ = s.refreshTokenRepo.DeleteByEmployeeID(ctx, employee.EmployeeID)

	newAccessToken, err := jwt.GenerateToken(employee.EmployeeID, s.cfg.JWTSecret, s.cfg.JWTExpiry)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, errors.InternalErr(err)
	}
	newRawRefresh := hex.EncodeToString(tokenBytes)

	newRefreshToken := &model.RefreshToken{
		EmployeeID: employee.EmployeeID,
		Token:      newRawRefresh,
		ExpiresAt:  time.Now().Add(time.Duration(s.cfg.RefreshExpiry) * time.Minute),
	}

	if err := s.refreshTokenRepo.Create(ctx, newRefreshToken); err != nil {
		return nil, errors.InternalErr(err)
	}

	return &dto.RefreshResponse{
		Token:        newAccessToken,
		RefreshToken: newRawRefresh,
	}, nil
}

func (s *EmployeeService) ConfirmChangePassword(ctx context.Context, oldPassword string, newPassword string) error {
	authCtx := auth.GetAuthFromContext(ctx)

	if authCtx == nil {
		return errors.UnauthorizedErr("invalid credentials")
	}

	userID := authCtx.UserID
	// ako je nova ista kao stara
	if oldPassword == newPassword {
		return errors.BadRequestErr("new password cannot be the same as the old one")
	}
	//ako korisnik postoji
	employee, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return errors.InternalErr(err)
	}
	if employee == nil {
		return errors.UnauthorizedErr("invalid credentials")
	}
	// Provaravamo staru lozinku
	if err = bcrypt.CompareHashAndPassword([]byte(employee.Password), []byte(oldPassword)); err != nil {
		return errors.UnauthorizedErr("invalid credentials")
	}
	// Postavljamo novu lozninku
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.InternalErr(err)
	}

	employee.Password = string(hashedPassword)
	
	if err := s.repo.Update(ctx, employee); err != nil {
		return errors.InternalErr(err)
	}

	return nil
}
