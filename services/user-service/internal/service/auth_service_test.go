package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	commonjwt "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/jwt"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/model"
)

func newAuthService(
	identityRepo *fakeIdentityRepo,
	employeeRepo *fakeEmployeeRepo,
	clientRepo *fakeClientRepo,
	activationTokenRepo *fakeActivationTokenRepo,
	resetTokenRepo *fakeResetTokenRepo,
	refreshTokenRepo *fakeRefreshTokenRepo,
	mailer *fakeMailer,
) *AuthService {
	return NewAuthService(
		identityRepo,
		employeeRepo,
		clientRepo,
		activationTokenRepo,
		resetTokenRepo,
		refreshTokenRepo,
		mailer,
		testConfig(),
		&fakeTxManager{},
	)
}

func TestLogin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		identityRepo *fakeIdentityRepo
		employeeRepo *fakeEmployeeRepo
		clientRepo   *fakeClientRepo
		req          *dto.LoginRequest
		expectErr    bool
		errMsg       string
		expectType   auth.IdentityType
		clientID     *uint
		employeeID   *uint
		userID       uint
	}{
		{
			name:         "successful login",
			identityRepo: &fakeIdentityRepo{byEmail: activeIdentity()},
			employeeRepo: &fakeEmployeeRepo{byIdentityID: activeEmployee()},
			clientRepo:   &fakeClientRepo{},
			req:          &dto.LoginRequest{Email: "john@example.com", Password: "Password12"},
			expectType:   auth.IdentityEmployee,
			employeeID:   testUintPtr(1),
			userID:       1,
		},
		{
			name:         "successful client login",
			identityRepo: &fakeIdentityRepo{byEmail: activeClientIdentity()},
			employeeRepo: &fakeEmployeeRepo{},
			clientRepo:   &fakeClientRepo{byIdentityID: activeClient()},
			req:          &dto.LoginRequest{Email: "client@example.com", Password: "Password12"},
			expectType:   auth.IdentityClient,
			clientID:     testUintPtr(1),
			userID:       1,
		},
		{
			name:         "identity not found",
			identityRepo: &fakeIdentityRepo{byEmail: nil},
			employeeRepo: &fakeEmployeeRepo{},
			clientRepo:   &fakeClientRepo{},
			req:          &dto.LoginRequest{Email: "nobody@example.com", Password: "Password12"},
			expectErr:    true,
			errMsg:       "invalid credentials",
		},
		{
			name:         "wrong password",
			identityRepo: &fakeIdentityRepo{byEmail: activeIdentity()},
			employeeRepo: &fakeEmployeeRepo{},
			clientRepo:   &fakeClientRepo{},
			req:          &dto.LoginRequest{Email: "john@example.com", Password: "WrongPass99"},
			expectErr:    true,
			errMsg:       "invalid credentials",
		},
		{
			name: "inactive account",
			identityRepo: &fakeIdentityRepo{byEmail: func() *model.Identity {
				i := activeIdentity()
				i.Active = false
				return i
			}()},
			employeeRepo: &fakeEmployeeRepo{},
			clientRepo:   &fakeClientRepo{},
			req:          &dto.LoginRequest{Email: "john@example.com", Password: "Password12"},
			expectErr:    true,
			errMsg:       "account is disabled",
		},
		{
			name:         "repo error",
			identityRepo: &fakeIdentityRepo{findErr: fmt.Errorf("db down")},
			employeeRepo: &fakeEmployeeRepo{},
			clientRepo:   &fakeClientRepo{},
			req:          &dto.LoginRequest{Email: "john@example.com", Password: "Password12"},
			expectErr:    true,
		},
		{
			name:         "employee profile missing",
			identityRepo: &fakeIdentityRepo{byEmail: activeIdentity()},
			employeeRepo: &fakeEmployeeRepo{byIdentityID: nil},
			clientRepo:   &fakeClientRepo{},
			req:          &dto.LoginRequest{Email: "john@example.com", Password: "Password12"},
			expectErr:    true,
			errMsg:       "employee profile not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newAuthService(tt.identityRepo, tt.employeeRepo, tt.clientRepo, &fakeActivationTokenRepo{}, &fakeResetTokenRepo{}, &fakeRefreshTokenRepo{}, &fakeMailer{})

			res, err := svc.Login(context.Background(), tt.req)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				require.Nil(t, res)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, res.Token)
				require.NotEmpty(t, res.RefreshToken)
				require.NotNil(t, res.User)
				require.Equal(t, tt.expectType, res.User.IdentityType)
				require.Equal(t, tt.userID, res.User.ID)

				claims := verifyTokenClaims(t, res.Token)
				require.Equal(t, tt.identityRepo.byEmail.ID, claims.IdentityID)
				require.Equal(t, string(tt.expectType), claims.IdentityType)
				requireUintPtrEqual(t, tt.employeeID, claims.EmployeeID)
				requireUintPtrEqual(t, tt.clientID, claims.ClientID)
			}
		})
	}
}

func TestActivateAccount(t *testing.T) {
	t.Parallel()

	validToken := &model.ActivationToken{
		IdentityID: 1,
		Token:      "valid-token",
		ExpiresAt:  time.Now().Add(1 * time.Hour),
	}

	expiredToken := &model.ActivationToken{
		IdentityID: 1,
		Token:      "expired-token",
		ExpiresAt:  time.Now().Add(-1 * time.Hour),
	}

	tests := []struct {
		name         string
		tokenRepo    *fakeActivationTokenRepo
		identityRepo *fakeIdentityRepo
		token        string
		password     string
		expectErr    bool
		errMsg       string
	}{
		{
			name:         "successful activation",
			tokenRepo:    &fakeActivationTokenRepo{token: validToken},
			identityRepo: &fakeIdentityRepo{byID: activeIdentity()},
			token:        "valid-token",
			password:     "NewPass12",
		},
		{
			name:         "invalid token",
			tokenRepo:    &fakeActivationTokenRepo{token: nil},
			identityRepo: &fakeIdentityRepo{},
			token:        "bad-token",
			expectErr:    true,
			errMsg:       "invalid or expired token",
		},
		{
			name:         "expired token",
			tokenRepo:    &fakeActivationTokenRepo{token: expiredToken},
			identityRepo: &fakeIdentityRepo{},
			token:        "expired-token",
			expectErr:    true,
			errMsg:       "token expired",
		},
		{
			name:         "identity not found",
			tokenRepo:    &fakeActivationTokenRepo{token: validToken},
			identityRepo: &fakeIdentityRepo{byID: nil},
			token:        "valid-token",
			expectErr:    true,
			errMsg:       "identity not found",
		},
		{
			name:         "repo update fails",
			tokenRepo:    &fakeActivationTokenRepo{token: validToken},
			identityRepo: &fakeIdentityRepo{byID: activeIdentity(), updateErr: fmt.Errorf("db error")},
			token:        "valid-token",
			password:     "NewPass12",
			expectErr:    true,
		},
		{
			name:         "token repo find error",
			tokenRepo:    &fakeActivationTokenRepo{findErr: fmt.Errorf("db error")},
			identityRepo: &fakeIdentityRepo{},
			token:        "some-token",
			expectErr:    true,
		},
		{
			name: "test-token prefix path successful activation - token pre-exists",
			tokenRepo: &fakeActivationTokenRepo{
				// pre-populate so both FindByToken calls return the token
				token: &model.ActivationToken{
					IdentityID: 1,
					Token:      "test-token-1",
					ExpiresAt:  time.Now().Add(24 * time.Hour),
				},
			},
			identityRepo: &fakeIdentityRepo{byID: func() *model.Identity {
				i := activeIdentity()
				i.Active = false
				return i
			}()},
			token:    "test-token-1",
			password: "NewPass12",
		},
		{
			name: "test-token prefix with invalid id format",
			tokenRepo: &fakeActivationTokenRepo{
				token: nil,
			},
			identityRepo: &fakeIdentityRepo{},
			token:        "test-token-notanumber",
			expectErr:    true,
			errMsg:       "invalid test token format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newAuthService(tt.identityRepo, &fakeEmployeeRepo{}, &fakeClientRepo{}, tt.tokenRepo, &fakeResetTokenRepo{}, &fakeRefreshTokenRepo{}, &fakeMailer{})

			err := svc.ActivateAccount(context.Background(), tt.token, tt.password)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, tt.identityRepo.updatedIdentity)
				require.NotEqual(t, tt.password, tt.identityRepo.updatedIdentity.PasswordHash)
				require.True(t, tt.identityRepo.updatedIdentity.Active)
			}
		})
	}
}

func TestRefreshToken(t *testing.T) {
	t.Parallel()

	validRefresh := &model.RefreshToken{
		IdentityID: 1,
		Token:      "valid-refresh",
		ExpiresAt:  time.Now().Add(1 * time.Hour),
	}

	expiredRefresh := &model.RefreshToken{
		IdentityID: 1,
		Token:      "expired-refresh",
		ExpiresAt:  time.Now().Add(-1 * time.Hour),
	}

	tests := []struct {
		name         string
		refreshRepo  *fakeRefreshTokenRepo
		identityRepo *fakeIdentityRepo
		employeeRepo *fakeEmployeeRepo
		clientRepo   *fakeClientRepo
		token        string
		expectErr    bool
		errMsg       string
		expectType   auth.IdentityType
		clientID     *uint
		employeeID   *uint
		userID       uint
	}{
		{
			name:         "successful refresh",
			refreshRepo:  &fakeRefreshTokenRepo{token: validRefresh},
			identityRepo: &fakeIdentityRepo{byID: activeIdentity()},
			employeeRepo: &fakeEmployeeRepo{byIdentityID: activeEmployee()},
			clientRepo:   &fakeClientRepo{},
			token:        "valid-refresh",
			expectType:   auth.IdentityEmployee,
			employeeID:   testUintPtr(1),
			userID:       1,
		},
		{
			name:         "successful client refresh",
			refreshRepo:  &fakeRefreshTokenRepo{token: &model.RefreshToken{IdentityID: 2, Token: "client-refresh", ExpiresAt: time.Now().Add(1 * time.Hour)}},
			identityRepo: &fakeIdentityRepo{byID: activeClientIdentity()},
			employeeRepo: &fakeEmployeeRepo{},
			clientRepo:   &fakeClientRepo{byIdentityID: activeClient()},
			token:        "client-refresh",
			expectType:   auth.IdentityClient,
			clientID:     testUintPtr(1),
			userID:       1,
		},
		{
			name:         "token not found",
			refreshRepo:  &fakeRefreshTokenRepo{token: nil},
			identityRepo: &fakeIdentityRepo{},
			employeeRepo: &fakeEmployeeRepo{},
			clientRepo:   &fakeClientRepo{},
			token:        "bad-token",
			expectErr:    true,
			errMsg:       "invalid or expired refresh token",
		},
		{
			name:         "expired refresh token",
			refreshRepo:  &fakeRefreshTokenRepo{token: expiredRefresh},
			identityRepo: &fakeIdentityRepo{},
			employeeRepo: &fakeEmployeeRepo{},
			clientRepo:   &fakeClientRepo{},
			token:        "expired-refresh",
			expectErr:    true,
			errMsg:       "refresh token expired",
		},
		{
			name:         "identity not found",
			refreshRepo:  &fakeRefreshTokenRepo{token: validRefresh},
			identityRepo: &fakeIdentityRepo{byID: nil},
			employeeRepo: &fakeEmployeeRepo{},
			clientRepo:   &fakeClientRepo{},
			token:        "valid-refresh",
			expectErr:    true,
			errMsg:       "identity not found",
		},
		{
			name:        "inactive account",
			refreshRepo: &fakeRefreshTokenRepo{token: validRefresh},
			identityRepo: &fakeIdentityRepo{byID: func() *model.Identity {
				i := activeIdentity()
				i.Active = false
				return i
			}()},
			employeeRepo: &fakeEmployeeRepo{},
			clientRepo:   &fakeClientRepo{},
			token:        "valid-refresh",
			expectErr:    true,
			errMsg:       "account is disabled",
		},
		{
			name:         "repo error on find",
			refreshRepo:  &fakeRefreshTokenRepo{findErr: fmt.Errorf("db down")},
			identityRepo: &fakeIdentityRepo{},
			employeeRepo: &fakeEmployeeRepo{},
			clientRepo:   &fakeClientRepo{},
			token:        "any",
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newAuthService(tt.identityRepo, tt.employeeRepo, tt.clientRepo, &fakeActivationTokenRepo{}, &fakeResetTokenRepo{}, tt.refreshRepo, &fakeMailer{})

			res, err := svc.RefreshToken(context.Background(), tt.token)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, res.Token)
				require.NotEmpty(t, res.RefreshToken)
				require.NotNil(t, res.User)
				require.Equal(t, tt.userID, res.User.ID)

				claims := verifyTokenClaims(t, res.Token)
				require.Equal(t, tt.identityRepo.byID.ID, claims.IdentityID)
				require.Equal(t, string(tt.expectType), claims.IdentityType)
				requireUintPtrEqual(t, tt.employeeID, claims.EmployeeID)
				requireUintPtrEqual(t, tt.clientID, claims.ClientID)
			}
		})
	}
}

func TestConfirmPasswordReset(t *testing.T) {
	t.Parallel()

	validReset := &model.ResetToken{
		IdentityID: 1,
		Token:      "valid-reset",
		ExpiresAt:  time.Now().Add(10 * time.Minute),
	}

	expiredReset := &model.ResetToken{
		IdentityID: 1,
		Token:      "expired-reset",
		ExpiresAt:  time.Now().Add(-10 * time.Minute),
	}

	tests := []struct {
		name         string
		resetRepo    *fakeResetTokenRepo
		identityRepo *fakeIdentityRepo
		token        string
		password     string
		expectErr    bool
		errMsg       string
	}{
		{
			name:         "successful reset",
			resetRepo:    &fakeResetTokenRepo{token: validReset},
			identityRepo: &fakeIdentityRepo{byID: activeIdentity()},
			token:        "valid-reset",
			password:     "NewPass12",
		},
		{
			name:         "token not found",
			resetRepo:    &fakeResetTokenRepo{token: nil},
			identityRepo: &fakeIdentityRepo{},
			token:        "bad",
			expectErr:    true,
			errMsg:       "invalid or expired token",
		},
		{
			name:         "expired token",
			resetRepo:    &fakeResetTokenRepo{token: expiredReset},
			identityRepo: &fakeIdentityRepo{},
			token:        "expired-reset",
			expectErr:    true,
			errMsg:       "token has expired",
		},
		{
			name:         "identity not found",
			resetRepo:    &fakeResetTokenRepo{token: validReset},
			identityRepo: &fakeIdentityRepo{byID: nil},
			token:        "valid-reset",
			expectErr:    true,
			errMsg:       "identity not found",
		},
		{
			name:         "reset token repo find error",
			resetRepo:    &fakeResetTokenRepo{findErr: fmt.Errorf("db error")},
			identityRepo: &fakeIdentityRepo{},
			token:        "any",
			expectErr:    true,
		},
		{
			name:         "identity update fails",
			resetRepo:    &fakeResetTokenRepo{token: validReset},
			identityRepo: &fakeIdentityRepo{byID: activeIdentity(), updateErr: fmt.Errorf("db error")},
			token:        "valid-reset",
			password:     "NewPass12",
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newAuthService(tt.identityRepo, &fakeEmployeeRepo{}, &fakeClientRepo{}, &fakeActivationTokenRepo{}, tt.resetRepo, &fakeRefreshTokenRepo{}, &fakeMailer{})

			err := svc.ConfirmPasswordReset(context.Background(), tt.token, tt.password)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRequestPasswordReset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		identityRepo *fakeIdentityRepo
		resetRepo    *fakeResetTokenRepo
		email        string
		expectErr    bool
	}{
		{
			name:         "identity found sends reset",
			identityRepo: &fakeIdentityRepo{byEmail: activeIdentity()},
			resetRepo:    &fakeResetTokenRepo{},
			email:        "john@example.com",
		},
		{
			name:         "identity not found returns nil",
			identityRepo: &fakeIdentityRepo{byEmail: nil},
			resetRepo:    &fakeResetTokenRepo{},
			email:        "nobody@example.com",
		},
		{
			name:         "repo error",
			identityRepo: &fakeIdentityRepo{findErr: fmt.Errorf("db down")},
			resetRepo:    &fakeResetTokenRepo{},
			email:        "john@example.com",
			expectErr:    true,
		},
		{
			name:         "delete old token fails",
			identityRepo: &fakeIdentityRepo{byEmail: activeIdentity()},
			resetRepo:    &fakeResetTokenRepo{deleteErr: fmt.Errorf("db error")},
			email:        "john@example.com",
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newAuthService(tt.identityRepo, &fakeEmployeeRepo{}, &fakeClientRepo{}, &fakeActivationTokenRepo{}, tt.resetRepo, &fakeRefreshTokenRepo{}, &fakeMailer{})

			err := svc.RequestPasswordReset(context.Background(), tt.email)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestChangePassword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		identityRepo *fakeIdentityRepo
		ctx          context.Context
		oldPassword  string
		newPassword  string
		expectErr    bool
		errMsg       string
	}{
		{
			name:         "successful change",
			identityRepo: &fakeIdentityRepo{byID: activeIdentity()},
			ctx:          withAuth(context.Background(), 1, auth.IdentityEmployee),
			oldPassword:  "Password12",
			newPassword:  "NewPass99",
		},
		{
			name:         "no auth context",
			identityRepo: &fakeIdentityRepo{},
			ctx:          context.Background(),
			oldPassword:  "Password12",
			newPassword:  "NewPass99",
			expectErr:    true,
			errMsg:       "not authenticated",
		},
		{
			name:         "same old and new password",
			identityRepo: &fakeIdentityRepo{byID: activeIdentity()},
			ctx:          withAuth(context.Background(), 1, auth.IdentityEmployee),
			oldPassword:  "Password12",
			newPassword:  "Password12",
			expectErr:    true,
			errMsg:       "new password cannot be the same as the old one",
		},
		{
			name:         "wrong old password",
			identityRepo: &fakeIdentityRepo{byID: activeIdentity()},
			ctx:          withAuth(context.Background(), 1, auth.IdentityEmployee),
			oldPassword:  "WrongPass99",
			newPassword:  "NewPass99",
			expectErr:    true,
			errMsg:       "invalid credentials",
		},
		{
			name:         "identity not found",
			identityRepo: &fakeIdentityRepo{byID: nil},
			ctx:          withAuth(context.Background(), 1, auth.IdentityEmployee),
			oldPassword:  "Password12",
			newPassword:  "NewPass99",
			expectErr:    true,
			errMsg:       "invalid credentials",
		},
		{
			name:         "repo update fails",
			identityRepo: &fakeIdentityRepo{byID: activeIdentity(), updateErr: fmt.Errorf("db error")},
			ctx:          withAuth(context.Background(), 1, auth.IdentityEmployee),
			oldPassword:  "Password12",
			newPassword:  "NewPass99",
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newAuthService(tt.identityRepo, &fakeEmployeeRepo{}, &fakeClientRepo{}, &fakeActivationTokenRepo{}, &fakeResetTokenRepo{}, &fakeRefreshTokenRepo{}, &fakeMailer{})

			err := svc.ChangePassword(tt.ctx, tt.oldPassword, tt.newPassword)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestResendActivation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		identityRepo *fakeIdentityRepo
		tokenRepo    *fakeActivationTokenRepo
		mailer       *fakeMailer
		email        string
		expectErr    bool
		expectSent   bool
	}{
		{
			name:         "identity not found returns nil",
			identityRepo: &fakeIdentityRepo{byEmail: nil},
			tokenRepo:    &fakeActivationTokenRepo{},
			mailer:       &fakeMailer{},
			email:        "nobody@example.com",
		},
		{
			name:         "identity already active returns nil",
			identityRepo: &fakeIdentityRepo{byEmail: activeIdentity()},
			tokenRepo:    &fakeActivationTokenRepo{},
			mailer:       &fakeMailer{},
			email:        "john@example.com",
		},
		{
			name: "success sends email",
			identityRepo: &fakeIdentityRepo{byEmail: func() *model.Identity {
				i := activeIdentity()
				i.Active = false
				return i
			}()},
			tokenRepo:  &fakeActivationTokenRepo{},
			mailer:     &fakeMailer{},
			email:      "john@example.com",
			expectSent: true,
		},
		{
			name: "email service failure returns error",
			identityRepo: &fakeIdentityRepo{byEmail: func() *model.Identity {
				i := activeIdentity()
				i.Active = false
				return i
			}()},
			tokenRepo: &fakeActivationTokenRepo{},
			mailer:    &fakeMailer{sendErr: fmt.Errorf("smtp down")},
			email:     "john@example.com",
			expectErr: true,
		},
		{
			name:         "repo error on find",
			identityRepo: &fakeIdentityRepo{findErr: fmt.Errorf("db down")},
			tokenRepo:    &fakeActivationTokenRepo{},
			mailer:       &fakeMailer{},
			email:        "john@example.com",
			expectErr:    true,
		},
		{
			name: "delete old token fails",
			identityRepo: &fakeIdentityRepo{byEmail: func() *model.Identity {
				i := activeIdentity()
				i.Active = false
				return i
			}()},
			tokenRepo: &fakeActivationTokenRepo{deleteErr: fmt.Errorf("db error")},
			mailer:    &fakeMailer{},
			email:     "john@example.com",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newAuthService(tt.identityRepo, &fakeEmployeeRepo{}, &fakeClientRepo{}, tt.tokenRepo, &fakeResetTokenRepo{}, &fakeRefreshTokenRepo{}, tt.mailer)

			err := svc.ResendActivation(context.Background(), tt.email)

			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if tt.expectSent {
				require.True(t, tt.mailer.sent)
			}
		})
	}
}

func verifyTokenClaims(t *testing.T, token string) *commonjwt.Claims {
	t.Helper()

	verifier := commonjwt.NewJWTVerifier(testConfig().JWTSecret)
	claims, err := verifier.VerifyToken(token)
	require.NoError(t, err)

	return claims
}

func requireUintPtrEqual(t *testing.T, expected, actual *uint) {
	t.Helper()

	if expected == nil {
		require.Nil(t, actual)
		return
	}

	require.NotNil(t, actual)
	require.Equal(t, *expected, *actual)
}

func testUintPtr(v uint) *uint {
	return &v
}
