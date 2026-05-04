package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
)

type fakeAccountRepo struct {
	//accNumExists        bool	UNUSED
	accountNumberExists bool
	nameExists          bool
	nameExistsErr       error
	//accNumExistsErr     error UNUSED
	createErr         error
	accounts          []model.Account
	account           *model.Account
	findErr           error
	updateNameErr     error
	updateLimitsErr   error
	getByAccNumber    *model.Account
	getByAccNumberErr error
	updateErr         error
	allAccounts       []*model.Account
	allTotal          int64
	getAllErr         error
}

func (f *fakeAccountRepo) Create(_ context.Context, _ *model.Account) error {
	return f.createErr
}

func (f *fakeAccountRepo) AccountNumberExists(_ context.Context, _ string) (bool, error) {
	return f.accountNumberExists, nil
}

func (r *fakeAccountRepo) FindByClientID(_ context.Context, _ uint) ([]model.Account, error) {
	return r.accounts, r.findErr
}

func (f *fakeAccountRepo) NameExistsForClient(_ context.Context, _ uint, _ string, _ string) (bool, error) {
	if f.nameExistsErr != nil {
		return false, f.nameExistsErr
	}
	return f.nameExists, nil
}

func (r *fakeAccountRepo) FindAllByClientID(_ context.Context, _ uint) ([]model.Account, error) {
	return r.accounts, r.findErr
}

func (r *fakeAccountRepo) FindByAccountNumber(_ context.Context, _ string) (*model.Account, error) {
	return r.account, r.findErr
}

func (r *fakeAccountRepo) FindByAccountNumberAndClientID(_ context.Context, _ string, _ uint) (*model.Account, error) {
	return r.account, r.findErr
}

func (r *fakeAccountRepo) UpdateName(_ context.Context, _ string, _ string) error {
	return r.updateNameErr
}

func (r *fakeAccountRepo) UpdateLimits(_ context.Context, _ string, _ float64, _ float64) error {
	return r.updateLimitsErr
}

func (r *fakeAccountRepo) GetByAccountNumber(_ context.Context, _ string) (*model.Account, error) {
	return r.getByAccNumber, r.getByAccNumberErr
}

func (f *fakeAccountRepo) Update(_ context.Context, _ *model.Account) error {
	return f.updateErr
}

func (f *fakeAccountRepo) UpdateBalance(_ context.Context, _ *model.Account) error {
	return nil
}

func (r *fakeAccountRepo) FindAll(_ context.Context, _ *dto.ListAccountsQuery) ([]*model.Account, int64, error) {
	return r.allAccounts, r.allTotal, r.getAllErr
}

type fakeVerificationTokenRepo struct {
	token     *model.VerificationToken
	findErr   error
	createErr error
	deleteErr error
}

func (r *fakeVerificationTokenRepo) Create(_ context.Context, _ *model.VerificationToken) error {
	return r.createErr
}

func (r *fakeVerificationTokenRepo) FindByAccountAndClient(_ context.Context, _ string, _ uint) (*model.VerificationToken, error) {
	return r.token, r.findErr
}

func (r *fakeVerificationTokenRepo) DeleteByAccountAndClient(_ context.Context, _ string, _ uint) error {
	return r.deleteErr
}

type fakeCurrencyRepo struct {
	currency *model.Currency
	findErr  error
}

func (f *fakeCurrencyRepo) FindByCode(_ context.Context, _ model.CurrencyCode) (*model.Currency, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	return f.currency, nil
}

type fakeUserClient struct {
	clientErr   error
	employeeErr error
}

func (f *fakeUserClient) GetClientByID(_ context.Context, _ uint) (*pb.GetClientByIdResponse, error) {
	if f.clientErr != nil {
		return nil, f.clientErr
	}
	return &pb.GetClientByIdResponse{}, nil
}

type fakeBankingTxManager struct{}

func (m *fakeBankingTxManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
func (m *fakeBankingTxManager) FindByAccountType(ctx context.Context, accountType model.AccountType) (*model.Account, error) {
	return nil, nil
}

func (f *fakeUserClient) GetEmployeeByID(_ context.Context, _ uint) (*pb.GetEmployeeByIdResponse, error) {
	if f.employeeErr != nil {
		return nil, f.employeeErr
	}
	return &pb.GetEmployeeByIdResponse{}, nil
}

type fakeCurrencyConverter struct {
	result     float64
	convertErr error
}

type fakeAccountMobileSecretClient struct {
	secret string
	err    error
}

func (f *fakeAccountMobileSecretClient) GetMobileSecret(_ context.Context, _ string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.secret, nil
}

func (f *fakeCurrencyConverter) CalculateFee(amount float64) float64 {
	return amount * model.BankCommission
}

func (r *fakeAccountRepo) FindByAccountType(ctx context.Context, accountType model.AccountType) (*model.Account, error) {
	return nil, nil
}
func (f *fakeCurrencyConverter) Convert(_ context.Context, amount float64, _ model.CurrencyCode, _ model.CurrencyCode) (float64, error) {
	if f.convertErr != nil {
		return 0, f.convertErr
	}
	if f.result != 0 {
		return f.result, nil
	}
	return amount, nil
}

func rsdCurrency() *model.Currency {
	return &model.Currency{
		CurrencyID: 1,
		Name:       "Serbian Dinar",
		Code:       model.RSD,
		Symbol:     "RSD",
		Country:    "Serbia",
		Status:     "Active",
	}
}

func eurCurrency() *model.Currency {
	return &model.Currency{
		CurrencyID: 2,
		Name:       "Euro",
		Code:       model.EUR,
		Symbol:     "€",
		Country:    "EU",
		Status:     "Active",
	}
}

func ptrUint(v uint) *uint { return &v }

func baseExpiresAt() time.Time {
	return time.Now().AddDate(5, 0, 0)
}

type fakeAccountServiceMailer struct {
	sendErr error
	sent    []sentEmail
}

func (f *fakeAccountServiceMailer) Send(to, subject, body string) error {
	if f.sendErr != nil {
		return f.sendErr
	}

	f.sent = append(f.sent, sentEmail{
		to:      to,
		subject: subject,
		body:    body,
	})
	return nil
}

func newAccountService(
	accountRepo *fakeAccountRepo,
	vr *fakeVerificationTokenRepo,
	currencyRepo *fakeCurrencyRepo,
	userClient *fakeUserClient,
	mobileSecretClient client.MobileSecretClient,
	exchangeConverter *fakeCurrencyConverter,
	mailer Mailer,
) *AccountService {
	if mobileSecretClient == nil {
		mobileSecretClient = &fakeAccountMobileSecretClient{}
	}
	if mailer == nil {
		mailer = &fakeAccountServiceMailer{}
	}
	return NewAccountService(accountRepo, currencyRepo, vr, userClient, nil, mobileSecretClient, exchangeConverter, &fakeBankingTxManager{}, mailer)
}

func TestCreateAccount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		accountRepo       *fakeAccountRepo
		currencyRepo      *fakeCurrencyRepo
		userClient        *fakeUserClient
		exchangeConverter *fakeCurrencyConverter
		req               dto.CreateAccountRequest
		expectErr         bool
		errMsg            string
	}{
		{
			name:              "successful personal current account",
			accountRepo:       &fakeAccountRepo{},
			currencyRepo:      &fakeCurrencyRepo{currency: rsdCurrency()},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "My Account",
				ClientID:    1,
				EmployeeID:  1,
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeStandard,
				ExpiresAt:   baseExpiresAt(),
			},
		},
		{
			name:              "successful business current account",
			accountRepo:       &fakeAccountRepo{},
			currencyRepo:      &fakeCurrencyRepo{currency: rsdCurrency()},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "Business Account",
				ClientID:    1,
				EmployeeID:  1,
				CompanyID:   ptrUint(10),
				AccountType: model.AccountTypeBusiness,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeLLC,
				ExpiresAt:   baseExpiresAt(),
			},
		},
		{
			name:              "successful foreign account with converted limits",
			accountRepo:       &fakeAccountRepo{},
			currencyRepo:      &fakeCurrencyRepo{currency: eurCurrency()},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{result: 2500.0},
			req: dto.CreateAccountRequest{
				Name:         "EUR Account",
				ClientID:     1,
				EmployeeID:   1,
				AccountType:  model.AccountTypePersonal,
				AccountKind:  model.AccountKindForeign,
				CurrencyCode: model.EUR,
				ExpiresAt:    baseExpiresAt(),
			},
		},
		{
			name:              "client not found",
			accountRepo:       &fakeAccountRepo{},
			currencyRepo:      &fakeCurrencyRepo{},
			userClient:        &fakeUserClient{clientErr: fmt.Errorf("not found")},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "My Account",
				ClientID:    999,
				EmployeeID:  1,
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeStandard,
				ExpiresAt:   baseExpiresAt(),
			},
			expectErr: true,
			errMsg:    "client not found",
		},
		{
			name:              "employee not found",
			accountRepo:       &fakeAccountRepo{},
			currencyRepo:      &fakeCurrencyRepo{},
			userClient:        &fakeUserClient{employeeErr: fmt.Errorf("not found")},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "My Account",
				ClientID:    1,
				EmployeeID:  999,
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeStandard,
				ExpiresAt:   baseExpiresAt(),
			},
			expectErr: true,
			errMsg:    "employee not found",
		},
		{
			name:              "business account without company",
			accountRepo:       &fakeAccountRepo{},
			currencyRepo:      &fakeCurrencyRepo{},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "Business No Company",
				ClientID:    1,
				EmployeeID:  1,
				AccountType: model.AccountTypeBusiness,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeLLC,
				ExpiresAt:   baseExpiresAt(),
			},
			expectErr: true,
			errMsg:    "business account requires a company",
		},
		{
			name:              "personal account with company",
			accountRepo:       &fakeAccountRepo{},
			currencyRepo:      &fakeCurrencyRepo{},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "Personal With Company",
				ClientID:    1,
				EmployeeID:  1,
				CompanyID:   ptrUint(10),
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeStandard,
				ExpiresAt:   baseExpiresAt(),
			},
			expectErr: true,
			errMsg:    "personal account cannot have a company",
		},
		{
			name:              "foreign account without currency code",
			accountRepo:       &fakeAccountRepo{},
			currencyRepo:      &fakeCurrencyRepo{},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "Foreign No Currency",
				ClientID:    1,
				EmployeeID:  1,
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindForeign,
				ExpiresAt:   baseExpiresAt(),
			},
			expectErr: true,
			errMsg:    "currency code is required for foreign accounts",
		},
		{
			name:              "current account without subtype",
			accountRepo:       &fakeAccountRepo{},
			currencyRepo:      &fakeCurrencyRepo{},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "No Subtype",
				ClientID:    1,
				EmployeeID:  1,
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindCurrent,
				ExpiresAt:   baseExpiresAt(),
			},
			expectErr: true,
			errMsg:    "subtype is required for current accounts",
		},
		{
			name:              "account name already exists for client",
			accountRepo:       &fakeAccountRepo{nameExists: true},
			currencyRepo:      &fakeCurrencyRepo{},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "Duplicate Name",
				ClientID:    1,
				EmployeeID:  1,
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeStandard,
				ExpiresAt:   baseExpiresAt(),
			},
			expectErr: true,
			errMsg:    "account with this name already exists",
		},
		{
			name:              "name exists repo error",
			accountRepo:       &fakeAccountRepo{nameExistsErr: fmt.Errorf("db error")},
			currencyRepo:      &fakeCurrencyRepo{},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "My Account",
				ClientID:    1,
				EmployeeID:  1,
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeStandard,
				ExpiresAt:   baseExpiresAt(),
			},
			expectErr: true,
		},
		{
			name:              "currency not found",
			accountRepo:       &fakeAccountRepo{},
			currencyRepo:      &fakeCurrencyRepo{findErr: fmt.Errorf("currency not found: RSD")},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "My Account",
				ClientID:    1,
				EmployeeID:  1,
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeStandard,
				ExpiresAt:   baseExpiresAt(),
			},
			expectErr: true,
			errMsg:    "currency not found",
		},
		{
			name:              "exchange conversion fails",
			accountRepo:       &fakeAccountRepo{},
			currencyRepo:      &fakeCurrencyRepo{currency: eurCurrency()},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{convertErr: fmt.Errorf("exchange service unavailable")},
			req: dto.CreateAccountRequest{
				Name:         "EUR Account",
				ClientID:     1,
				EmployeeID:   1,
				AccountType:  model.AccountTypePersonal,
				AccountKind:  model.AccountKindForeign,
				CurrencyCode: model.EUR,
				ExpiresAt:    baseExpiresAt(),
			},
			expectErr: true,
		},
		{
			name:              "repo create fails",
			accountRepo:       &fakeAccountRepo{createErr: fmt.Errorf("db error")},
			currencyRepo:      &fakeCurrencyRepo{currency: rsdCurrency()},
			userClient:        &fakeUserClient{},
			exchangeConverter: &fakeCurrencyConverter{},
			req: dto.CreateAccountRequest{
				Name:        "My Account",
				ClientID:    1,
				EmployeeID:  1,
				AccountType: model.AccountTypePersonal,
				AccountKind: model.AccountKindCurrent,
				Subtype:     model.SubtypeStandard,
				ExpiresAt:   baseExpiresAt(),
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newAccountService(tt.accountRepo, &fakeVerificationTokenRepo{}, tt.currencyRepo, tt.userClient, nil, tt.exchangeConverter, nil)
			account, err := svc.Create(context.Background(), tt.req)

			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, account)
				require.NotEmpty(t, account.AccountNumber)
				require.Equal(t, tt.req.ClientID, account.ClientID)
				require.Equal(t, tt.req.EmployeeID, account.EmployeeID)
				require.Equal(t, tt.req.AccountType, account.AccountType)
				require.Equal(t, tt.req.AccountKind, account.AccountKind)
				require.Equal(t, tt.currencyRepo.currency.CurrencyID, account.CurrencyID)
			}
		})
	}
}

func TestGetClientAccounts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		repo      *fakeAccountRepo
		expectErr bool
		check     func(t *testing.T, accounts []model.Account)
	}{
		{
			name: "success returns accounts",
			repo: &fakeAccountRepo{
				accounts: []model.Account{
					{AccountNumber: "444000112345678911", Name: "Standard Personal Account"},
					{AccountNumber: "444000112345678921", Name: "Personal EUR Account"},
				},
			},
			check: func(t *testing.T, accounts []model.Account) {
				require.Len(t, accounts, 2)
				require.Equal(t, "444000112345678911", accounts[0].AccountNumber)
				require.Equal(t, "444000112345678921", accounts[1].AccountNumber)
			},
		},
		{
			name:      "repo error returns internal error",
			repo:      &fakeAccountRepo{findErr: fmt.Errorf("db failure")},
			expectErr: true,
		},
		{
			name: "returns empty slice when client has no accounts",
			repo: &fakeAccountRepo{accounts: []model.Account{}},
			check: func(t *testing.T, accounts []model.Account) {
				require.Empty(t, accounts)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := newAccountService(tt.repo, &fakeVerificationTokenRepo{}, &fakeCurrencyRepo{}, &fakeUserClient{}, nil, &fakeCurrencyConverter{}, nil)
			accounts, err := svc.GetClientAccounts(context.Background(), 1)
			if tt.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.check != nil {
				tt.check(t, accounts)
			}
		})
	}
}

func TestGetAccountDetails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		repo      *fakeAccountRepo
		expectErr bool
		errMsg    string
	}{
		{
			name: "success",
			repo: &fakeAccountRepo{
				account: &model.Account{AccountNumber: "444000112345678911", Name: "My Account"},
			},
		},
		{
			name:      "account not found",
			repo:      &fakeAccountRepo{findErr: fmt.Errorf("record not found")},
			expectErr: true,
			errMsg:    "account not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := newAccountService(tt.repo, &fakeVerificationTokenRepo{}, &fakeCurrencyRepo{}, &fakeUserClient{}, nil, &fakeCurrencyConverter{}, nil)
			account, err := svc.GetAccountDetails(context.Background(), "444000112345678911", 1)
			if tt.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, account)
			require.Equal(t, "444000112345678911", account.AccountNumber)
		})
	}
}

func TestUpdateAccountName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		repo      *fakeAccountRepo
		newName   string
		expectErr bool
		errMsg    string
	}{
		{
			name: "success",
			repo: &fakeAccountRepo{
				account: &model.Account{AccountNumber: "444000112345678911", Name: "Old Name"},
			},
			newName: "New Name",
		},
		{
			name:      "account not found",
			repo:      &fakeAccountRepo{findErr: fmt.Errorf("record not found")},
			newName:   "New Name",
			expectErr: true,
			errMsg:    "account not found",
		},
		{
			name: "same name as current",
			repo: &fakeAccountRepo{
				account: &model.Account{AccountNumber: "444000112345678911", Name: "Same Name"},
			},
			newName:   "Same Name",
			expectErr: true,
			errMsg:    "same as the current name",
		},
		{
			name: "name already used by another account",
			repo: &fakeAccountRepo{
				account:    &model.Account{AccountNumber: "444000112345678911", Name: "Old Name"},
				nameExists: true,
			},
			newName:   "Taken Name",
			expectErr: true,
			errMsg:    "already exists",
		},
		{
			name: "update fails",
			repo: &fakeAccountRepo{
				account:       &model.Account{AccountNumber: "444000112345678911", Name: "Old Name"},
				updateNameErr: fmt.Errorf("db failure"),
			},
			newName:   "New Name",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := newAccountService(tt.repo, &fakeVerificationTokenRepo{}, &fakeCurrencyRepo{}, &fakeUserClient{}, nil, &fakeCurrencyConverter{}, nil)
			err := svc.UpdateAccountName(context.Background(), "444000112345678911", 1, tt.newName)
			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestRequestLimitsChange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		repo      *fakeAccountRepo
		vr        *fakeVerificationTokenRepo
		expectErr bool
		errMsg    string
	}{
		{
			name: "success",
			repo: &fakeAccountRepo{account: &model.Account{}},
			vr:   &fakeVerificationTokenRepo{},
		},
		{
			name:      "account not found",
			repo:      &fakeAccountRepo{findErr: fmt.Errorf("not found")},
			vr:        &fakeVerificationTokenRepo{},
			expectErr: true,
			errMsg:    "account not found",
		},
		{
			name:      "delete existing token fails",
			repo:      &fakeAccountRepo{account: &model.Account{}},
			vr:        &fakeVerificationTokenRepo{deleteErr: fmt.Errorf("db failure")},
			expectErr: true,
		},
		{
			name:      "create token fails",
			repo:      &fakeAccountRepo{account: &model.Account{}},
			vr:        &fakeVerificationTokenRepo{createErr: fmt.Errorf("db failure")},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := newAccountService(tt.repo, tt.vr, &fakeCurrencyRepo{}, &fakeUserClient{}, nil, &fakeCurrencyConverter{}, nil)
			err := svc.RequestLimitsChange(context.Background(), "444000112345678911", 1, 500000, 2000000)
			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func generateCurrentTOTPCode(t *testing.T, secret string) string {
	t.Helper()
	decoded, err := decodeBase32Secret(secret)
	require.NoError(t, err)
	counter := time.Now().Unix() / totpStepSeconds
	return generateTOTP(decoded, counter)
}

func TestConfirmLimitsChange(t *testing.T) {
	t.Parallel()

	const secret = "JBSWY3DPEHPK3PXP"
	validCode := generateCurrentTOTPCode(t, secret)
	validToken := &model.VerificationToken{
		ID:              1,
		ClientID:        1,
		AccountNumber:   "444000112345678911",
		NewDailyLimit:   500000,
		NewMonthlyLimit: 2000000,
	}

	tests := []struct {
		name      string
		repo      *fakeAccountRepo
		vr        *fakeVerificationTokenRepo
		secret    string
		secretErr error
		code      string
		expectErr bool
		errMsg    string
	}{
		{
			name:   "success with valid totp code",
			repo:   &fakeAccountRepo{},
			vr:     &fakeVerificationTokenRepo{token: validToken},
			secret: secret,
			code:   validCode,
		},
		{
			name:      "token not found",
			repo:      &fakeAccountRepo{},
			vr:        &fakeVerificationTokenRepo{findErr: fmt.Errorf("not found")},
			secret:    secret,
			code:      validCode,
			expectErr: true,
			errMsg:    "no pending limits change",
		},
		{
			name:      "invalid verification code",
			repo:      &fakeAccountRepo{},
			vr:        &fakeVerificationTokenRepo{token: validToken},
			secret:    secret,
			code:      "000000",
			expectErr: true,
			errMsg:    "invalid verification code",
		},
		{
			name:      "mobile secret unavailable",
			repo:      &fakeAccountRepo{},
			vr:        &fakeVerificationTokenRepo{token: validToken},
			secretErr: fmt.Errorf("secret service down"),
			code:      validCode,
			expectErr: true,
			errMsg:    "secret service down",
		},
		{
			name:      "update limits fails",
			repo:      &fakeAccountRepo{updateLimitsErr: fmt.Errorf("db failure")},
			vr:        &fakeVerificationTokenRepo{token: validToken},
			secret:    secret,
			code:      validCode,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mobileClient := &fakeAccountMobileSecretClient{secret: tt.secret, err: tt.secretErr}
			svc := newAccountService(tt.repo, tt.vr, &fakeCurrencyRepo{}, &fakeUserClient{}, mobileClient, &fakeCurrencyConverter{}, nil)
			err := svc.ConfirmLimitsChange(context.Background(), "444000112345678911", 1, tt.code, "Bearer test")
			if tt.expectErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestGetAllAccounts(t *testing.T) {
	t.Parallel()

	query := &dto.ListAccountsQuery{
		Page:     1,
		PageSize: 10,
	}

	tests := []struct {
		name      string
		repo      *fakeAccountRepo
		expectErr bool
		wantTotal int64
		wantCount int
	}{
		{
			name: "successful list",
			repo: &fakeAccountRepo{
				allAccounts: []*model.Account{
					{
						AccountNumber: "1234567890123456",
						ClientID:      1,
						AccountType:   model.AccountTypePersonal,
						AccountKind:   model.AccountKindCurrent,
						Status:        "Active",
					},
					{
						AccountNumber: "6543210987654321",
						ClientID:      2,
						AccountType:   model.AccountTypeBusiness,
						AccountKind:   model.AccountKindCurrent,
						Status:        "Active",
					},
				},
				allTotal: 2,
			},
			wantTotal: 2,
			wantCount: 2,
		},
		{
			name:      "empty list",
			repo:      &fakeAccountRepo{},
			wantTotal: 0,
			wantCount: 0,
		},
		{
			name:      "repo error",
			repo:      &fakeAccountRepo{getAllErr: fmt.Errorf("db error")},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := newAccountService(tt.repo, &fakeVerificationTokenRepo{}, &fakeCurrencyRepo{}, &fakeUserClient{}, nil, &fakeCurrencyConverter{}, nil)

			accounts, total, err := svc.GetAllAccounts(context.Background(), query)

			if tt.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantTotal, total)
			require.Len(t, accounts, tt.wantCount)
		})
	}
}
