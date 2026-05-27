package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/model"
)

func newClientService(
	clientRepo *fakeClientRepo,
	identityRepo *fakeIdentityRepo,
	activationTokenRepo *fakeActivationTokenRepo,
	mailer *fakeMailer,
) *ClientService {
	return NewClientService(
		clientRepo,
		identityRepo,
		activationTokenRepo,
		mailer,
		testConfig(),
		&fakeTxManager{},
	)
}

func TestGetMobileVerificationSecret(t *testing.T) {
	t.Parallel()

	t.Run("returns secret for authenticated client", func(t *testing.T) {
		svc := newClientService(
			&fakeClientRepo{byID: &model.Client{ClientID: 2, MobileVerificationSecret: "JBSWY3DPEHPK3PXP"}},
			&fakeIdentityRepo{},
			&fakeActivationTokenRepo{},
			&fakeMailer{},
		)

		secret, err := svc.GetMobileVerificationSecret(context.Background(), 2)
		require.NoError(t, err)
		require.Equal(t, "JBSWY3DPEHPK3PXP", secret)
	})

	t.Run("not found when secret is empty", func(t *testing.T) {
		svc := newClientService(
			&fakeClientRepo{byID: &model.Client{ClientID: 2}},
			&fakeIdentityRepo{},
			&fakeActivationTokenRepo{},
			&fakeMailer{},
		)

		secret, err := svc.GetMobileVerificationSecret(context.Background(), 2)
		require.Error(t, err)
		require.Empty(t, secret)
		require.Contains(t, err.Error(), "mobile verification secret not found")
	})
}

func TestClientRegister(t *testing.T) {
	t.Parallel()

	req := &dto.CreateClientRequest{
		FirstName:   "Jane",
		LastName:    "Client",
		DateOfBirth: time.Now().AddDate(-25, 0, 0),
		Gender:      "female",
		Email:       "client@example.com",
		Username:    "clientuser",
		PhoneNumber: "0603333333",
		Address:     "Client Address 1",
	}

	tests := []struct {
		name         string
		clientRepo   *fakeClientRepo
		identityRepo *fakeIdentityRepo
		tokenRepo    *fakeActivationTokenRepo
		mailer       *fakeMailer
		expectErr    bool
		expectSent   bool
		errMsg       string
	}{
		{
			name:         "successful registration",
			clientRepo:   &fakeClientRepo{},
			identityRepo: &fakeIdentityRepo{},
			tokenRepo:    &fakeActivationTokenRepo{},
			mailer:       &fakeMailer{},
			expectSent:   true,
		},
		{
			name:         "email already in use",
			clientRepo:   &fakeClientRepo{},
			identityRepo: &fakeIdentityRepo{emailExists: true},
			tokenRepo:    &fakeActivationTokenRepo{},
			mailer:       &fakeMailer{},
			expectErr:    true,
			errMsg:       "email already in use",
		},
		{
			name:         "username already in use",
			clientRepo:   &fakeClientRepo{},
			identityRepo: &fakeIdentityRepo{usernameExists: true},
			tokenRepo:    &fakeActivationTokenRepo{},
			mailer:       &fakeMailer{},
			expectErr:    true,
			errMsg:       "username already in use",
		},
		{
			name:         "identity create fails",
			clientRepo:   &fakeClientRepo{},
			identityRepo: &fakeIdentityRepo{createErr: fmt.Errorf("db error")},
			tokenRepo:    &fakeActivationTokenRepo{},
			mailer:       &fakeMailer{},
			expectErr:    true,
		},
		{
			name:         "client create fails",
			clientRepo:   &fakeClientRepo{createErr: fmt.Errorf("db error")},
			identityRepo: &fakeIdentityRepo{},
			tokenRepo:    &fakeActivationTokenRepo{},
			mailer:       &fakeMailer{},
			expectErr:    true,
		},
		{
			name:         "activation token create fails",
			clientRepo:   &fakeClientRepo{},
			identityRepo: &fakeIdentityRepo{},
			tokenRepo:    &fakeActivationTokenRepo{createErr: fmt.Errorf("db error")},
			mailer:       &fakeMailer{},
			expectErr:    true,
		},
		{
			name:         "email send fails",
			clientRepo:   &fakeClientRepo{},
			identityRepo: &fakeIdentityRepo{},
			tokenRepo:    &fakeActivationTokenRepo{},
			mailer:       &fakeMailer{sendErr: fmt.Errorf("smtp down")},
			expectErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newClientService(tt.clientRepo, tt.identityRepo, tt.tokenRepo, tt.mailer)

			client, err := svc.Register(context.Background(), req)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, client)
			require.Equal(t, "Jane", client.FirstName)
			require.Equal(t, auth.IdentityClient, client.Identity.Type)
			require.False(t, client.Identity.Active)
			require.Equal(t, uint(1), client.IdentityID)
			require.NotEmpty(t, client.MobileVerificationSecret)
			if tt.expectSent {
				require.True(t, tt.mailer.sent)
			}
		})
	}
}
func TestClientGetAll(t *testing.T) {
	t.Parallel()

	query := &dto.ListClientsQuery{
		Page:     1,
		PageSize: 10,
	}

	tests := []struct {
		name       string
		clientRepo *fakeClientRepo
		expectErr  bool
		wantTotal  int64
		wantCount  int
	}{
		{
			name: "successful list",
			clientRepo: &fakeClientRepo{
				allClients: []model.Client{*activeClient(), *activeClient()},
				allTotal:   2,
			},
			wantTotal: 2,
			wantCount: 2,
		},
		{
			name:       "empty list",
			clientRepo: &fakeClientRepo{},
			wantTotal:  0,
			wantCount:  0,
		},
		{
			name:       "repo error",
			clientRepo: &fakeClientRepo{getAllErr: fmt.Errorf("db error")},
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newClientService(tt.clientRepo, &fakeIdentityRepo{}, &fakeActivationTokenRepo{}, &fakeMailer{})

			clients, err := svc.GetAllClients(context.Background(), query)

			if tt.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Len(t, clients.Data, tt.wantCount)
		})
	}
}

func TestClientUpdate(t *testing.T) {
	t.Parallel()

	newFirstName := "Updated"
	newPhone := "0609999999"

	tests := []struct {
		name       string
		clientRepo *fakeClientRepo
		req        *dto.UpdateClientRequest
		expectErr  bool
		errMsg     string
	}{
		{
			name:       "successful update",
			clientRepo: &fakeClientRepo{byID: activeClient()},
			req: &dto.UpdateClientRequest{
				FirstName:   &newFirstName,
				PhoneNumber: &newPhone,
			},
		},
		{
			name:       "client not found",
			clientRepo: &fakeClientRepo{},
			req:        &dto.UpdateClientRequest{FirstName: &newFirstName},
			expectErr:  true,
			errMsg:     "client not found",
		},
		{
			name:       "find error",
			clientRepo: &fakeClientRepo{findErr: fmt.Errorf("db error")},
			req:        &dto.UpdateClientRequest{FirstName: &newFirstName},
			expectErr:  true,
		},
		{
			name:       "update error",
			clientRepo: &fakeClientRepo{byID: activeClient(), updateErr: fmt.Errorf("db error")},
			req:        &dto.UpdateClientRequest{FirstName: &newFirstName},
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newClientService(tt.clientRepo, &fakeIdentityRepo{}, &fakeActivationTokenRepo{}, &fakeMailer{})

			client, err := svc.UpdateClient(context.Background(), 1, tt.req)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, client)
			require.Equal(t, newFirstName, client.FirstName)
			require.Equal(t, newPhone, client.PhoneNumber)
		})
	}
}
