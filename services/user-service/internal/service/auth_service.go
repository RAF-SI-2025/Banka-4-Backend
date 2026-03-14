package service

import (
	"common/pkg/auth"
	"common/pkg/errors"
	"common/pkg/jwt"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"user-service/internal/config"
	"user-service/internal/dto"
	"user-service/internal/model"
	"user-service/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	identityRepo        repository.IdentityRepository
	employeeRepo        repository.EmployeeRepository
	clientRepo          repository.ClientRepository
	activationTokenRepo repository.ActivationTokenRepository
	resetTokenRepo      repository.ResetTokenRepository
	refreshTokenRepo    repository.RefreshTokenRepository
	emailService        Mailer
	cfg                 *config.Configuration
}

func NewAuthService(
	identityRepo repository.IdentityRepository,
	employeeRepo repository.EmployeeRepository,
	clientRepo repository.ClientRepository,
	activationTokenRepo repository.ActivationTokenRepository,
	resetTokenRepo repository.ResetTokenRepository,
	refreshTokenRepo repository.RefreshTokenRepository,
	emailService Mailer,
	cfg *config.Configuration,
) *AuthService {
	return &AuthService{
		identityRepo:        identityRepo,
		employeeRepo:        employeeRepo,
		clientRepo:          clientRepo,
		activationTokenRepo: activationTokenRepo,
		resetTokenRepo:      resetTokenRepo,
		refreshTokenRepo:    refreshTokenRepo,
		emailService:        emailService,
		cfg:                 cfg,
	}
}

func (s *AuthService) Login(ctx context.Context, req *dto.LoginRequest) (*dto.LoginResponse, error) {
	identity, err := s.identityRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	if identity == nil {
		return nil, errors.UnauthorizedErr("invalid credentials")
	}

	if !identity.Active {
		return nil, errors.ForbiddenErr("account is disabled")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(identity.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.UnauthorizedErr("invalid credentials")
	}

	session, err := s.buildSession(ctx, identity)
	if err != nil {
		return nil, err
	}

	token, err := jwt.GenerateToken(session.Claims, s.cfg.JWTSecret, s.cfg.JWTExpiry)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	_ = s.refreshTokenRepo.DeleteByIdentityID(ctx, identity.ID)
	rawRefresh, err := generateSecureToken(32)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	refreshToken := &model.RefreshToken{
		IdentityID: identity.ID,
		Token:      rawRefresh,
		ExpiresAt:  time.Now().Add(time.Duration(s.cfg.RefreshExpiry) * time.Minute),
	}

	if err := s.refreshTokenRepo.Create(ctx, refreshToken); err != nil {
		return nil, errors.InternalErr(err)
	}

	return &dto.LoginResponse{
		Token:        token,
		RefreshToken: rawRefresh,
		User:         session.User,
	}, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshTokenStr string) (*dto.RefreshResponse, error) {
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

	identity, err := s.identityRepo.FindByID(ctx, storedToken.IdentityID)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	if identity == nil {
		return nil, errors.UnauthorizedErr("identity not found")
	}

	if !identity.Active {
		return nil, errors.ForbiddenErr("account is disabled")
	}

	session, err := s.buildSession(ctx, identity)
	if err != nil {
		return nil, err
	}

	_ = s.refreshTokenRepo.DeleteByIdentityID(ctx, identity.ID)

	newAccessToken, err := jwt.GenerateToken(session.Claims, s.cfg.JWTSecret, s.cfg.JWTExpiry)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	newRawRefresh, err := generateSecureToken(32)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	newRefreshToken := &model.RefreshToken{
		IdentityID: identity.ID,
		Token:      newRawRefresh,
		ExpiresAt:  time.Now().Add(time.Duration(s.cfg.RefreshExpiry) * time.Minute),
	}

	if err := s.refreshTokenRepo.Create(ctx, newRefreshToken); err != nil {
		return nil, errors.InternalErr(err)
	}

	return &dto.RefreshResponse{
		Token:        newAccessToken,
		RefreshToken: newRawRefresh,
		User:         session.User,
	}, nil
}

func (s *AuthService) ActivateAccount(ctx context.Context, tokenStr, password string) error {
	activationToken, err := s.activationTokenRepo.FindByToken(ctx, tokenStr)
	if err != nil {
		return errors.InternalErr(err)
	}

	if activationToken == nil {
		return errors.BadRequestErr("invalid or expired token")
	}

	if activationToken.ExpiresAt.Before(time.Now()) {
		return errors.BadRequestErr("token expired")
	}

	identity, err := s.identityRepo.FindByID(ctx, activationToken.IdentityID)
	if err != nil {
		return errors.InternalErr(err)
	}

	if identity == nil {
		return errors.NotFoundErr("identity not found")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return errors.InternalErr(err)
	}

	identity.PasswordHash = string(hashedPassword)
	identity.Active = true
	if err := s.identityRepo.Update(ctx, identity); err != nil {
		return errors.InternalErr(err)
	}

	_ = s.activationTokenRepo.Delete(ctx, activationToken)

	if err := s.emailService.Send(identity.Email, "Account activated", "Vas nalog je uspesno aktiviran."); err != nil {
		log.Printf("failed to send account activation confirmation email to identity_id=%d: %v", identity.ID, err)
	}
	return nil
}

func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) error {
	identity, err := s.identityRepo.FindByEmail(ctx, email)
	if err != nil {
		return errors.InternalErr(err)
	}

	if identity == nil {
		return nil
	}

	if err := s.resetTokenRepo.DeleteByIdentityID(ctx, identity.ID); err != nil {
		return errors.InternalErr(err)
	}

	tokenStr, err := generateSecureToken(16)
	if err != nil {
		return errors.InternalErr(err)
	}

	resetToken := &model.ResetToken{
		IdentityID: identity.ID,
		Token:      tokenStr,
		ExpiresAt:  time.Now().Add(15 * time.Minute),
	}

	if err := s.resetTokenRepo.Create(ctx, resetToken); err != nil {
		return errors.InternalErr(err)
	}

	resetBase := strings.TrimRight(s.cfg.URLs.FrontendBaseURL, "/")
	link := fmt.Sprintf("%s/reset-password?token=%s", resetBase, url.QueryEscape(tokenStr))
	if err := s.emailService.Send(
		identity.Email,
		"Password reset",
		fmt.Sprintf("Kliknite ovde da resetujete lozinku: %s", link),
	); err != nil {
		log.Printf("failed to send password reset email to identity_id=%d: %v", identity.ID, err)
	}

	return nil
}

func (s *AuthService) ConfirmPasswordReset(ctx context.Context, token, newPassword string) error {
	resetToken, err := s.resetTokenRepo.FindByToken(ctx, token)
	if err != nil {
		return errors.InternalErr(err)
	}

	if resetToken == nil {
		return errors.BadRequestErr("invalid or expired token")
	}

	if resetToken.ExpiresAt.Before(time.Now()) {
		_ = s.resetTokenRepo.DeleteByIdentityID(ctx, resetToken.IdentityID)
		return errors.BadRequestErr("token has expired")
	}

	identity, err := s.identityRepo.FindByID(ctx, resetToken.IdentityID)
	if err != nil {
		return errors.InternalErr(err)
	}

	if identity == nil {
		return errors.NotFoundErr("identity not found")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.InternalErr(err)
	}

	identity.PasswordHash = string(hashedPassword)
	if err := s.identityRepo.Update(ctx, identity); err != nil {
		return errors.InternalErr(err)
	}

	_ = s.resetTokenRepo.DeleteByIdentityID(ctx, identity.ID)

	if err := s.emailService.Send(
		identity.Email,
		"Password changed",
		"Vasa lozinka je uspesno promenjena.",
	); err != nil {
		log.Printf("failed to send password changed confirmation email to identity_id=%d: %v", identity.ID, err)
	}

	return nil
}

func (s *AuthService) ChangePassword(ctx context.Context, oldPassword, newPassword string) error {
	authCtx := auth.GetAuthFromContext(ctx)
	if authCtx == nil {
		return errors.UnauthorizedErr("not authenticated")
	}

	if oldPassword == newPassword {
		return errors.BadRequestErr("new password cannot be the same as the old one")
	}

	identity, err := s.identityRepo.FindByID(ctx, authCtx.IdentityID)
	if err != nil {
		return errors.InternalErr(err)
	}

	if identity == nil {
		return errors.UnauthorizedErr("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(identity.PasswordHash), []byte(oldPassword)); err != nil {
		return errors.UnauthorizedErr("invalid credentials")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return errors.InternalErr(err)
	}

	identity.PasswordHash = string(hashedPassword)
	return s.identityRepo.Update(ctx, identity)
}

type authSession struct {
	Claims *jwt.Claims
	User   *dto.AuthUser
}

func (s *AuthService) buildSession(ctx context.Context, identity *model.Identity) (*authSession, error) {
	session := &authSession{
		Claims: &jwt.Claims{
			IdentityID:   identity.ID,
			IdentityType: string(identity.Type),
		},
		User: &dto.AuthUser{
			ID:           identity.ID,
			IdentityType: identity.Type,
			Email:        identity.Email,
			Username:     identity.Username,
		},
	}

	switch identity.Type {
	case auth.IdentityEmployee:
		emp, err := s.employeeRepo.FindByIdentityID(ctx, identity.ID)
		if err != nil {
			return nil, errors.InternalErr(err)
		}
		if emp == nil {
			return nil, errors.InternalErr(fmt.Errorf("employee profile not found for identity_id=%d", identity.ID))
		}

		session.Claims.EmployeeID = uintPtr(emp.EmployeeID)
		session.User = dto.NewAuthUserFromEmployee(identity, emp)
	case auth.IdentityClient:
		cli, err := s.clientRepo.FindByIdentityID(ctx, identity.ID)
		if err != nil {
			return nil, errors.InternalErr(err)
		}

		if cli == nil {
			return nil, errors.InternalErr(fmt.Errorf("client profile not found for identity_id=%d", identity.ID))
		}

		session.Claims.ClientID = uintPtr(cli.ClientID)
		session.User = dto.NewAuthUserFromClient(identity, cli)
	default:
		return nil, errors.InternalErr(fmt.Errorf("unsupported identity type: %s", identity.Type))
	}

	return session, nil
}

func uintPtr(v uint) *uint {
	return &v
}

func generateSecureToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}
