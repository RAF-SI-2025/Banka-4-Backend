package service

import (
	"banking-service/internal/dto"
	"banking-service/internal/model"
	"common/pkg/pb"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ── Fakes ─────────────────────────────────────────────────────────────────────

type fakeAccountRepo struct {
	accNumExists        bool
	accNumExistsErr     error
	createErr           error
	getByAccNumber      *model.Account
	getByAccNumberErr   error
	updateErr           error
}

func (r *fakeAccountRepo) Create(_ context.Context, _ *model.Account) error {
	return r.createErr
}

func (r *fakeAccountRepo) AccountNumberExists(_ context.Context, _ string) (bool, error) {
	return r.accNumExists, r.accNumExistsErr
}

func (r *fakeAccountRepo) GetByAccountNumber(_ context.Context, _ string) (*model.Account, error) {
	return r.getByAccNumber, r.getByAccNumberErr
}

func (r *fakeAccountRepo) Update(_ context.Context, _ *model.Account) error {
	return r.updateErr
}

type fakeAccountUserClient struct {
	clientErr   error
	employeeErr error
}

func (f *fakeAccountUserClient) GetClientByID(_ context.Context, _ uint) (*pb.GetClientByIdResponse, error) {
	return nil, f.clientErr
}

func (f *fakeAccountUserClient) GetEmployeeByID(_ context.Context, _ uint) (*pb.GetEmployeeByIdResponse, error) {
	return nil, f.employeeErr
}

// ── Constructor ───────────────────────────────────────────────────────────────

func newAccountService(
	repo *fakeAccountRepo,
	uc *fakeAccountUserClient,
	db *gorm.DB,
) *AccountService {
	return &AccountService{
		repo:       repo,
		userClient: uc,
		db:         db,
	}
}

// ── Fixtures ──────────────────────────────────────────────────────────────────

func validPersonalCurrentReq() dto.CreateAccountRequest {
	return dto.CreateAccountRequest{
		Name:           "Standard Personal Account",
		ClientID:       1,
		EmployeeID:     1,
		AccountType:    model.AccountTypePersonal,
		AccountKind:    model.AccountKindCurrent,
		Subtype:        model.SubtypeStandard,
		InitialBalance: 50000,
		ExpiresAt:      time.Now().AddDate(4, 0, 0),
	}
}

func validForeignReq() dto.CreateAccountRequest {
	return dto.CreateAccountRequest{
		Name:           "EUR Foreign Account",
		ClientID:       1,
		EmployeeID:     1,
		AccountType:    model.AccountTypePersonal,
		AccountKind:    model.AccountKindForeign,
		CurrencyCode:   "EUR",
		InitialBalance: 1000,
		ExpiresAt:      time.Now().AddDate(4, 0, 0),
	}
}

func ptrUint(v uint) *uint { return &v }

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestCreateAccount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		repo      *fakeAccountRepo
		uc        *fakeAccountUserClient
		setupMock func(mock sqlmock.Sqlmock)
		req       dto.CreateAccountRequest
		expectErr bool
		errMsg    string
		check     func(t *testing.T, account *model.Account)
	}{
		{
			name: "success personal current",
			repo: &fakeAccountRepo{},
			uc:   &fakeAccountUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT \* FROM "currencies"`).
					WillReturnRows(sqlmock.NewRows([]string{"currency_id", "code"}).AddRow(1, "RSD"))
			},
			req: validPersonalCurrentReq(),
			check: func(t *testing.T, a *model.Account) {
				require.Equal(t, model.AccountTypePersonal, a.AccountType)
				require.Equal(t, model.AccountKindCurrent, a.AccountKind)
				require.Equal(t, model.SubtypeStandard, a.Subtype)
				require.Equal(t, 250000.0, a.DailyLimit)
				require.Equal(t, 1000000.0, a.MonthlyLimit)
				require.NotEmpty(t, a.AccountNumber)
			},
		},
		{
			name: "success personal foreign EUR",
			repo: &fakeAccountRepo{},
			uc:   &fakeAccountUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT \* FROM "currencies"`).
					WillReturnRows(sqlmock.NewRows([]string{"currency_id", "code"}).AddRow(2, "EUR"))
			},
			req: validForeignReq(),
			check: func(t *testing.T, a *model.Account) {
				require.Equal(t, model.AccountKindForeign, a.AccountKind)
				require.Equal(t, 5000.0, a.DailyLimit)
				require.Equal(t, 20000.0, a.MonthlyLimit)
			},
		},
		{
			name: "success business current LLC",
			repo: &fakeAccountRepo{},
			uc:   &fakeAccountUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT \* FROM "currencies"`).
					WillReturnRows(sqlmock.NewRows([]string{"currency_id", "code"}).AddRow(1, "RSD"))
			},
			req: dto.CreateAccountRequest{
				Name:        "LLC Business Account",
				ClientID:    1,
				EmployeeID:  1,
				CompanyID:   ptrUint(1),
				AccountType: model.AccountTypeBusiness,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeLLC,
				ExpiresAt:   time.Now().AddDate(4, 0, 0),
			},
			check: func(t *testing.T, a *model.Account) {
				require.Equal(t, model.AccountTypeBusiness, a.AccountType)
				require.Equal(t, model.SubtypeLLC, a.Subtype)
			},
		},
		{
			name:      "client not found",
			repo:      &fakeAccountRepo{},
			uc:        &fakeAccountUserClient{clientErr: fmt.Errorf("not found")},
			setupMock: func(mock sqlmock.Sqlmock) {},
			req:       validPersonalCurrentReq(),
			expectErr: true,
			errMsg:    "client not found",
		},
		{
			name:      "employee not found",
			repo:      &fakeAccountRepo{},
			uc:        &fakeAccountUserClient{employeeErr: fmt.Errorf("not found")},
			setupMock: func(mock sqlmock.Sqlmock) {},
			req:       validPersonalCurrentReq(),
			expectErr: true,
			errMsg:    "employee not found",
		},
		{
			name:      "business account without company",
			repo:      &fakeAccountRepo{},
			uc:        &fakeAccountUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {},
			req: dto.CreateAccountRequest{
				Name:        "Test",
				ClientID:    1,
				EmployeeID:  1,
				AccountType: model.AccountTypeBusiness,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeLLC,
				ExpiresAt:   time.Now().AddDate(4, 0, 0),
			},
			expectErr: true,
			errMsg:    "business account requires a company",
		},
		{
			name:      "personal account with company",
			repo:      &fakeAccountRepo{},
			uc:        &fakeAccountUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {},
			req: dto.CreateAccountRequest{
				Name:        "Test",
				ClientID:    1,
				EmployeeID:  1,
				CompanyID:   ptrUint(1),
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeStandard,
				ExpiresAt:   time.Now().AddDate(4, 0, 0),
			},
			expectErr: true,
			errMsg:    "personal account cannot have a company",
		},
		{
			name:      "foreign account missing currency code",
			repo:      &fakeAccountRepo{},
			uc:        &fakeAccountUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {},
			req: dto.CreateAccountRequest{
				Name:        "Test",
				ClientID:    1,
				EmployeeID:  1,
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindForeign,
				ExpiresAt:   time.Now().AddDate(4, 0, 0),
			},
			expectErr: true,
			errMsg:    "currency code is required for foreign accounts",
		},
		{
			name:      "current account missing subtype",
			repo:      &fakeAccountRepo{},
			uc:        &fakeAccountUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {},
			req: dto.CreateAccountRequest{
				Name:        "Test",
				ClientID:    1,
				EmployeeID:  1,
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindCurrent,
				ExpiresAt:   time.Now().AddDate(4, 0, 0),
			},
			expectErr: true,
			errMsg:    "subtype is required for current accounts",
		},
		{
			name: "currency not found in db",
			repo: &fakeAccountRepo{},
			uc:   &fakeAccountUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT \* FROM "currencies"`).
					WillReturnError(fmt.Errorf("record not found"))
			},
			req:       validPersonalCurrentReq(),
			expectErr: true,
			errMsg:    "currency not found",
		},
		{
			name: "repo create fails",
			repo: &fakeAccountRepo{createErr: fmt.Errorf("insert failed")},
			uc:   &fakeAccountUserClient{},
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT \* FROM "currencies"`).
					WillReturnRows(sqlmock.NewRows([]string{"currency_id", "code"}).AddRow(1, "RSD"))
			},
			req:       validPersonalCurrentReq(),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db, mock := newMockDB(t)
			tt.setupMock(mock)

			svc := newAccountService(tt.repo, tt.uc, db)
			account, err := svc.Create(context.Background(), tt.req)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, account)
			if tt.check != nil {
				tt.check(t, account)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
