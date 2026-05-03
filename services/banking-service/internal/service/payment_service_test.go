package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/repository"
)

// ── Fake Payment Repo ──────────────────────────────────────────────

type fakePaymentRepo struct {
	createErr   error
	getErr      error
	findAllErr  error
	payment     *model.Payment
	payments    []model.Payment
	allPayments []model.Payment
	findAccErr  error
	total       int64
	//capturedFilter repository.PaymentFilter	UNUSED
}

func (f *fakePaymentRepo) Create(ctx context.Context, p *model.Payment) error {
	if f.createErr != nil {
		return f.createErr
	}
	p.PaymentID = 1
	f.payment = p
	return nil
}

func (f *fakePaymentRepo) GetByID(ctx context.Context, id uint) (*model.Payment, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.payment, nil
}

func (f *fakePaymentRepo) Update(ctx context.Context, p *model.Payment) error {
	f.payment = p
	return nil
}

func (f *fakePaymentRepo) FindByAccount(_ context.Context, _ string, _ *dto.PaymentFilters) ([]model.Payment, int64, error) {
	return f.payments, f.total, f.findAccErr
}

func (r *fakePaymentAccountRepo) FindByClientID(_ context.Context, _ uint) ([]model.Account, error) {
	return nil, nil
}

func (f *fakePaymentRepo) FindByClient(_ context.Context, _ uint, _ *dto.PaymentFilters) ([]model.Payment, int64, error) {
	return f.allPayments, int64(len(f.allPayments)), f.findAllErr
}

type fakeTransactionRepo struct {
	createErr error
	//getErr        error	UNUSED
	//updateErr     error	UNUSED
	transaction   *model.Transaction
	returnedTx    *model.Transaction
	returnedTxErr error
}

func (f *fakeTransactionRepo) Create(_ context.Context, t *model.Transaction) error {
	if f.createErr != nil {
		return f.createErr
	}
	t.TransactionID = 1 // simulate ID assignment
	f.transaction = t
	return nil
}

func (f *fakeTransactionRepo) Update(ctx context.Context, t *model.Transaction) error {
	f.transaction = t
	return nil
}

func (f *fakeTransactionRepo) GetByID(_ context.Context, _ uint) (*model.Transaction, error) {
	// return preset transaction or error
	return f.returnedTx, f.returnedTxErr
}

func (f *fakeTransactionRepo) GetByPayerAccountNumber(_ context.Context, _ string) ([]*model.Transaction, error) {
	return nil, nil
}

func (f *fakeTransactionRepo) GetByRecipientAccountNumber(_ context.Context, _ string) ([]*model.Transaction, error) {
	return nil, nil
}

// ── Fake Payment Account Repo ──────────────────────────────────────

type fakePaymentAccountRepo struct {
	accounts   map[string]*model.Account
	accountArr []model.Account
	account    *model.Account
	findErr    error
	nameExists bool
}

func newFakePaymentAccountRepo(accounts ...*model.Account) *fakePaymentAccountRepo {
	m := make(map[string]*model.Account)
	for _, a := range accounts {
		m[a.AccountNumber] = a
	}
	return &fakePaymentAccountRepo{accounts: m}
}

func (f *fakePaymentAccountRepo) Create(ctx context.Context, account *model.Account) error {
	return nil
}

func (f *fakePaymentAccountRepo) AccountNumberExists(ctx context.Context, accountNumber string) (bool, error) {
	_, exists := f.accounts[accountNumber]
	return exists, nil
}

func (f *fakePaymentAccountRepo) FindByAccountNumber(ctx context.Context, accountNumber string) (*model.Account, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	acc, exists := f.accounts[accountNumber]
	if !exists {
		return nil, errors.New("account not found")
	}
	return acc, nil
}

func (f *fakePaymentAccountRepo) FindAllByClientID(_ context.Context, _ uint) ([]model.Account, error) {
	return f.accountArr, nil
}

func (f *fakePaymentAccountRepo) FindByAccountNumberAndClientID(_ context.Context, _ string, _ uint) (*model.Account, error) {
	return f.account, nil
}

func (f *fakePaymentAccountRepo) NameExistsForClient(_ context.Context, _ uint, _ string, _ string) (bool, error) {
	return f.nameExists, nil
}

func (f *fakePaymentAccountRepo) UpdateName(_ context.Context, _ string, _ string) error {
	return nil
}

func (f *fakePaymentAccountRepo) UpdateLimits(_ context.Context, _ string, _ float64, _ float64) error {
	return nil
}

func (f *fakePaymentAccountRepo) GetByAccountNumber(ctx context.Context, accountNumber string) (*model.Account, error) {
	return f.FindByAccountNumber(ctx, accountNumber)
}

func (f *fakePaymentAccountRepo) Update(_ context.Context, _ *model.Account) error {
	return nil
}

func (f *fakePaymentAccountRepo) FindAll(_ context.Context, _ *dto.ListAccountsQuery) ([]*model.Account, int64, error) {
	return nil, 0, nil
}

func (f *fakePaymentAccountRepo) UpdateBalance(ctx context.Context, account *model.Account) error {
	f.accounts[account.AccountNumber] = account
	return nil
}
func (r *fakePaymentAccountRepo) FindByAccountType(ctx context.Context, accountType model.AccountType) (*model.Account, error) {
	return nil, nil
}

// ── Fake Exchange Service ──────────────────────────────────────────

type fakeExchangeService struct {
	rate float64
	err  error
}

func (f *fakeExchangeService) Convert(ctx context.Context, amount float64, from, to model.CurrencyCode) (float64, error) {
	if f.err != nil {
		return 0, f.err
	}
	return amount * f.rate, nil
}

func (f *fakeExchangeService) CalculateFee(amount float64) float64 {
	return amount * model.BankCommission
}

type fakeMobileSecretClient struct {
	secret string
	err    error
}

func (f *fakeMobileSecretClient) GetMobileSecret(_ context.Context, _ string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	if f.secret != "" {
		return f.secret, nil
	}
	return "JBSWY3DPEHPK3PXP", nil
}

type fakeVerifyTransactionProcessor struct {
	err       error
	processed []uint
}

func (f *fakeVerifyTransactionProcessor) Process(_ context.Context, transactionID uint) error {
	f.processed = append(f.processed, transactionID)
	return f.err
}

// ── Constructor ────────────────────────────────────────────────────────

func newTestPaymentService(
	paymentRepo repository.PaymentRepository,
	transactionRepo repository.TransactionRepository,
	accountRepo repository.AccountRepository,
	exchangeSvc CurrencyConverter,
) *PaymentService {
	return &PaymentService{
		paymentRepo:          paymentRepo,
		transactionRepo:      transactionRepo,
		accountRepo:          accountRepo,
		exchangeService:      exchangeSvc,
		mobileSecretClient:   &fakeMobileSecretClient{},
		txManager:            &fakeBankingTxManager{},
		transactionProcessor: &fakeVerifyTransactionProcessor{},
		now:                  time.Now,
	}
}

func uintPtr(v uint) *uint {
	return &v
}

// ── Tests ──────────────────────────────────────────────────────────

func TestCreatePayment_Success(t *testing.T) {
	payerAccount := &model.Account{
		AccountNumber:    "87654321",
		ClientID:         1,
		Balance:          10000,
		AvailableBalance: 10000,
		DailyLimit:       250000,
		MonthlyLimit:     1000000,
		DailySpending:    0,
		MonthlySpending:  0,
		Currency:         model.Currency{Code: model.RSD},
	}
	recipientAccount := &model.Account{
		AccountNumber: "12345678",
		ClientID:      2,
		Currency:      model.Currency{Code: model.RSD},
	}

	svc := newTestPaymentService(
		&fakePaymentRepo{},
		&fakeTransactionRepo{},
		newFakePaymentAccountRepo(payerAccount, recipientAccount),
		&fakeExchangeService{rate: 1.0},
	)

	req := dto.CreatePaymentRequest{
		RecipientName:          "John Doe",
		RecipientAccountNumber: "12345678",
		Amount:                 100,
		PayerAccountNumber:     "87654321",
	}

	payment, err := svc.CreatePayment(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, "John Doe", payment.RecipientName)
}

func TestCreatePayment_InsufficientFunds(t *testing.T) {
	payerAccount := &model.Account{
		AccountNumber:    "87654321",
		ClientID:         1,
		Balance:          50,
		AvailableBalance: 50,
		DailyLimit:       250000,
		MonthlyLimit:     1000000,
		Currency:         model.Currency{Code: model.RSD},
	}
	recipientAccount := &model.Account{
		AccountNumber: "12345678",
		ClientID:      2,
		Currency:      model.Currency{Code: model.RSD},
	}

	svc := newTestPaymentService(
		&fakePaymentRepo{},
		&fakeTransactionRepo{},
		newFakePaymentAccountRepo(payerAccount, recipientAccount),
		&fakeExchangeService{rate: 1.0},
	)

	req := dto.CreatePaymentRequest{
		RecipientName:          "John Doe",
		RecipientAccountNumber: "12345678",
		Amount:                 100,
		PayerAccountNumber:     "87654321",
	}

	payment, err := svc.CreatePayment(context.Background(), req)
	require.Nil(t, payment)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient funds")
}

func TestCreatePayment_DailyLimitExceeded(t *testing.T) {
	payerAccount := &model.Account{
		AccountNumber:    "87654321",
		ClientID:         1,
		Balance:          500000,
		AvailableBalance: 500000,
		DailyLimit:       1000,
		MonthlyLimit:     1000000,
		DailySpending:    900,
		MonthlySpending:  0,
		Currency:         model.Currency{Code: model.RSD},
	}
	recipientAccount := &model.Account{
		AccountNumber: "12345678",
		ClientID:      2,
		Currency:      model.Currency{Code: model.RSD},
	}

	svc := newTestPaymentService(
		&fakePaymentRepo{},
		&fakeTransactionRepo{},
		newFakePaymentAccountRepo(payerAccount, recipientAccount),
		&fakeExchangeService{rate: 1.0},
	)

	req := dto.CreatePaymentRequest{
		RecipientName:          "John Doe",
		RecipientAccountNumber: "12345678",
		Amount:                 200,
		PayerAccountNumber:     "87654321",
	}

	payment, err := svc.CreatePayment(context.Background(), req)
	require.Nil(t, payment)
	require.Error(t, err)
	require.Contains(t, err.Error(), "daily limit exceeded")
}

func TestCreatePayment_MonthlyLimitExceeded(t *testing.T) {
	payerAccount := &model.Account{
		AccountNumber:    "87654321",
		ClientID:         1,
		Balance:          500000,
		AvailableBalance: 500000,
		DailyLimit:       1000000,
		MonthlyLimit:     1000,
		DailySpending:    0,
		MonthlySpending:  900,
		Currency:         model.Currency{Code: model.RSD},
	}
	recipientAccount := &model.Account{
		AccountNumber: "12345678",
		ClientID:      2,
		Currency:      model.Currency{Code: model.RSD},
	}

	svc := newTestPaymentService(
		&fakePaymentRepo{},
		&fakeTransactionRepo{},
		newFakePaymentAccountRepo(payerAccount, recipientAccount),
		&fakeExchangeService{rate: 1.0},
	)

	req := dto.CreatePaymentRequest{
		RecipientName:          "John Doe",
		RecipientAccountNumber: "12345678",
		Amount:                 200,
		PayerAccountNumber:     "87654321",
	}

	payment, err := svc.CreatePayment(context.Background(), req)
	require.Nil(t, payment)
	require.Error(t, err)
	require.Contains(t, err.Error(), "monthly limit exceeded")
}

func TestCreatePayment_RecipientNotFound(t *testing.T) {
	payerAccount := &model.Account{
		AccountNumber:    "87654321",
		ClientID:         1,
		Balance:          10000,
		AvailableBalance: 10000,
		DailyLimit:       250000,
		MonthlyLimit:     1000000,
		Currency:         model.Currency{Code: model.RSD},
	}

	svc := newTestPaymentService(
		&fakePaymentRepo{},
		&fakeTransactionRepo{},
		newFakePaymentAccountRepo(payerAccount),
		&fakeExchangeService{rate: 1.0},
	)

	req := dto.CreatePaymentRequest{
		RecipientName:          "John Doe",
		RecipientAccountNumber: "99999999",
		Amount:                 100,
		PayerAccountNumber:     "87654321",
	}

	payment, err := svc.CreatePayment(context.Background(), req)
	require.Nil(t, payment)
	require.Error(t, err)
}

func TestGetPaymentByID_Success(t *testing.T) {
	payerAccount := &model.Account{
		AccountNumber: "111",
		ClientID:      1,
	}
	repo := &fakePaymentRepo{
		payment: &model.Payment{
			PaymentID:     1,
			RecipientName: "Stefan",
			Transaction: model.Transaction{
				PayerAccountNumber: "111",
			},
		},
	}

	svc := newTestPaymentService(repo, &fakeTransactionRepo{}, newFakePaymentAccountRepo(payerAccount), &fakeExchangeService{})

	payment, err := svc.GetPaymentByID(ctxWithClient(1), 1)
	require.NoError(t, err)
	require.Equal(t, "Stefan", payment.RecipientName)
}

func TestGetPaymentByID_NotFound(t *testing.T) {
	repo := &fakePaymentRepo{getErr: errors.New("not found")}
	svc := newTestPaymentService(repo, &fakeTransactionRepo{}, newFakePaymentAccountRepo(), &fakeExchangeService{})

	payment, err := svc.GetPaymentByID(ctxWithClient(1), 99)
	require.Nil(t, payment)
	require.Error(t, err)
	require.Contains(t, err.Error(), "payment not found")
}

func TestGetPaymentByID_NilPayment(t *testing.T) {
	svc := newTestPaymentService(&fakePaymentRepo{}, &fakeTransactionRepo{}, newFakePaymentAccountRepo(), &fakeExchangeService{})

	payment, err := svc.GetPaymentByID(context.Background(), 1)
	require.Nil(t, payment)
	require.Error(t, err)
	require.Contains(t, err.Error(), "payment not found")
}

func TestCreatePayment_TransactionRepoError(t *testing.T) {
	payerAccount := &model.Account{
		AccountNumber:    "87654321",
		ClientID:         1,
		Balance:          10000,
		AvailableBalance: 10000,
		DailyLimit:       250000,
		MonthlyLimit:     1000000,
		Currency:         model.Currency{Code: model.RSD},
	}
	recipientAccount := &model.Account{
		AccountNumber: "12345678",
		ClientID:      2,
		Currency:      model.Currency{Code: model.RSD},
	}

	svc := newTestPaymentService(
		&fakePaymentRepo{},
		&fakeTransactionRepo{createErr: errors.New("db error")},
		newFakePaymentAccountRepo(payerAccount, recipientAccount),
		&fakeExchangeService{rate: 1.0},
	)

	req := dto.CreatePaymentRequest{
		RecipientName:          "John Doe",
		RecipientAccountNumber: "12345678",
		Amount:                 100,
		PayerAccountNumber:     "87654321",
	}

	payment, err := svc.CreatePayment(context.Background(), req)
	require.Nil(t, payment)
	require.Error(t, err)
}

func TestVerifyPayment_InvalidCode(t *testing.T) {
	paymentRepo := &fakePaymentRepo{
		payment: &model.Payment{
			PaymentID: 1,
			Transaction: model.Transaction{
				TransactionID:      42,
				PayerAccountNumber: "87654321",
				Status:             model.TransactionProcessing,
			},
		},
	}

	processor := &fakeVerifyTransactionProcessor{}
	authCtx := auth.SetAuthOnContext(context.Background(), &auth.AuthContext{ClientID: uintPtr(11)})
	svc := &PaymentService{
		paymentRepo:          paymentRepo,
		accountRepo:          newFakePaymentAccountRepo(&model.Account{AccountNumber: "87654321", ClientID: 11}),
		mobileSecretClient:   &fakeMobileSecretClient{secret: "JBSWY3DPEHPK3PXP"},
		transactionProcessor: processor,
		now:                  func() time.Time { return time.Unix(59, 0) },
	}

	payment, err := svc.VerifyPayment(authCtx, 1, "000000", "Bearer token")
	require.Nil(t, payment)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid verification code")
	require.Empty(t, processor.processed)
}

func TestVerifyPayment_CancelAfter3FailedAttempts(t *testing.T) {
	txRepo := &fakeTransactionRepo{}
	paymentRepo := &fakePaymentRepo{
		payment: &model.Payment{
			PaymentID:      1,
			FailedAttempts: 2,
			Transaction: model.Transaction{
				TransactionID:      42,
				PayerAccountNumber: "87654321",
				Status:             model.TransactionProcessing,
			},
		},
	}

	processor := &fakeVerifyTransactionProcessor{}
	authCtx := auth.SetAuthOnContext(context.Background(), &auth.AuthContext{ClientID: uintPtr(11)})
	svc := &PaymentService{
		paymentRepo:          paymentRepo,
		transactionRepo:      txRepo,
		accountRepo:          newFakePaymentAccountRepo(&model.Account{AccountNumber: "87654321", ClientID: 11}),
		mobileSecretClient:   &fakeMobileSecretClient{secret: "JBSWY3DPEHPK3PXP"},
		transactionProcessor: processor,
		now:                  func() time.Time { return time.Unix(59, 0) },
	}

	payment, err := svc.VerifyPayment(authCtx, 1, "000000", "Bearer token")
	require.Nil(t, payment)
	require.Error(t, err)
	require.Contains(t, err.Error(), "payment cancelled")
	require.Empty(t, processor.processed)
	require.Equal(t, model.TransactionRejected, txRepo.transaction.Status)
}

func TestVerifyPayment_FailedAttemptsBelow3DoNotCancel(t *testing.T) {
	txRepo := &fakeTransactionRepo{}
	paymentRepo := &fakePaymentRepo{
		payment: &model.Payment{
			PaymentID:      1,
			FailedAttempts: 1,
			Transaction: model.Transaction{
				TransactionID:      42,
				PayerAccountNumber: "87654321",
				Status:             model.TransactionProcessing,
			},
		},
	}

	processor := &fakeVerifyTransactionProcessor{}
	authCtx := auth.SetAuthOnContext(context.Background(), &auth.AuthContext{ClientID: uintPtr(11)})
	svc := &PaymentService{
		paymentRepo:          paymentRepo,
		transactionRepo:      txRepo,
		accountRepo:          newFakePaymentAccountRepo(&model.Account{AccountNumber: "87654321", ClientID: 11}),
		mobileSecretClient:   &fakeMobileSecretClient{secret: "JBSWY3DPEHPK3PXP"},
		transactionProcessor: processor,
		now:                  func() time.Time { return time.Unix(59, 0) },
	}

	payment, err := svc.VerifyPayment(authCtx, 1, "000000", "Bearer token")
	require.Nil(t, payment)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid verification code")
	require.Empty(t, processor.processed)
	// transaction not cancelled — txRepo.transaction is nil (Update not called for transaction)
	require.Nil(t, txRepo.transaction)
}

func TestVerifyPayment_Success(t *testing.T) {
	paymentRepo := &fakePaymentRepo{
		payment: &model.Payment{
			PaymentID: 1,
			Transaction: model.Transaction{
				TransactionID:      7,
				PayerAccountNumber: "87654321",
				Status:             model.TransactionProcessing,
			},
		},
	}

	processor := &fakeVerifyTransactionProcessor{}
	authCtx := auth.SetAuthOnContext(context.Background(), &auth.AuthContext{ClientID: uintPtr(5)})
	svc := &PaymentService{
		paymentRepo:          paymentRepo,
		accountRepo:          newFakePaymentAccountRepo(&model.Account{AccountNumber: "87654321", ClientID: 5}),
		mobileSecretClient:   &fakeMobileSecretClient{secret: "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"},
		transactionProcessor: processor,
		now:                  func() time.Time { return time.Unix(59, 0) },
	}

	payment, err := svc.VerifyPayment(authCtx, 1, "287082", "Bearer token")
	require.NoError(t, err)
	require.NotNil(t, payment)
	require.Equal(t, []uint{7}, processor.processed)
}

func TestGetAccountPayments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		repo      *fakePaymentRepo
		expectErr bool
		check     func(t *testing.T, payments []model.Payment, total int64)
	}{
		{
			name: "success returns payments",
			repo: &fakePaymentRepo{
				payments: []model.Payment{
					{PaymentID: 1, RecipientName: "Marko Markovic", Transaction: model.Transaction{StartAmount: 5000, StartCurrencyCode: "RSD", Status: model.TransactionCompleted}},
					{PaymentID: 2, RecipientName: "Ana Jovanovic", Transaction: model.Transaction{StartAmount: 1200, StartCurrencyCode: "RSD", Status: model.TransactionProcessing}},
				},
				total: 2,
			},
			check: func(t *testing.T, payments []model.Payment, total int64) {
				require.Len(t, payments, 2)
				require.Equal(t, int64(2), total)
				require.Equal(t, "Marko Markovic", payments[0].RecipientName)
				require.Equal(t, model.TransactionCompleted, payments[0].Transaction.Status)
				require.Equal(t, model.TransactionProcessing, payments[1].Transaction.Status)
			},
		},
		{
			name:      "repo error returns internal error",
			repo:      &fakePaymentRepo{findAccErr: errors.New("db failure")},
			expectErr: true,
		},
		{
			name: "returns empty list when no payments",
			repo: &fakePaymentRepo{payments: []model.Payment{}, total: 0},
			check: func(t *testing.T, payments []model.Payment, total int64) {
				require.Empty(t, payments)
				require.Equal(t, int64(0), total)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := newTestPaymentService(tt.repo, &fakeTransactionRepo{}, newFakePaymentAccountRepo(), &fakeExchangeService{})
			payments, total, err := svc.GetAccountPayments(context.Background(), "444000112345678911", &dto.PaymentFilters{Page: 1, PageSize: 10})
			if tt.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.check != nil {
				tt.check(t, payments, total)
			}
		})
	}
}
