package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
)

// ── Fake repos for TransactionProcessor tests ──────────────────────────────

type fakeTpAccountRepo struct {
	accounts  map[string]*model.Account
	findErr   map[string]error
	updateErr map[string]error
}

func newFakeTpAccountRepo(accounts ...*model.Account) *fakeTpAccountRepo {
	m := make(map[string]*model.Account)
	for _, a := range accounts {
		copy := *a
		m[a.AccountNumber] = &copy
	}
	return &fakeTpAccountRepo{
		accounts:  m,
		findErr:   map[string]error{},
		updateErr: map[string]error{},
	}
}

func (f *fakeTpAccountRepo) Create(_ context.Context, _ *model.Account) error { return nil }
func (f *fakeTpAccountRepo) AccountNumberExists(_ context.Context, num string) (bool, error) {
	_, ok := f.accounts[num]
	return ok, nil
}
func (f *fakeTpAccountRepo) FindByAccountNumber(_ context.Context, num string) (*model.Account, error) {
	if err, ok := f.findErr[num]; ok {
		return nil, err
	}
	acc, ok := f.accounts[num]
	if !ok {
		return nil, errors.New("account not found")
	}
	return acc, nil
}
func (f *fakeTpAccountRepo) GetByAccountNumber(ctx context.Context, num string) (*model.Account, error) {
	return f.FindByAccountNumber(ctx, num)
}
func (f *fakeTpAccountRepo) Update(_ context.Context, a *model.Account) error {
	f.accounts[a.AccountNumber] = a
	return nil
}
func (f *fakeTpAccountRepo) FindAllByClientID(_ context.Context, _ uint) ([]model.Account, error) {
	return nil, nil
}
func (f *fakeTpAccountRepo) FindByClientID(_ context.Context, _ uint) ([]model.Account, error) {
	return nil, nil
}
func (f *fakeTpAccountRepo) FindByAccountNumberAndClientID(_ context.Context, _ string, _ uint) (*model.Account, error) {
	return nil, nil
}
func (f *fakeTpAccountRepo) UpdateName(_ context.Context, _ string, _ string) error { return nil }
func (f *fakeTpAccountRepo) UpdateLimits(_ context.Context, _ string, _ float64, _ float64) error {
	return nil
}
func (f *fakeTpAccountRepo) NameExistsForClient(_ context.Context, _ uint, _ string, _ string) (bool, error) {
	return false, nil
}
func (f *fakeTpAccountRepo) UpdateBalance(_ context.Context, a *model.Account) error {
	if err, ok := f.updateErr[a.AccountNumber]; ok {
		return err
	}
	f.accounts[a.AccountNumber] = a
	return nil
}
func (f *fakeTpAccountRepo) FindAll(_ context.Context, _ *dto.ListAccountsQuery) ([]*model.Account, int64, error) {
	return nil, 0, nil
}

func (r *fakeTpAccountRepo) FindByAccountType(ctx context.Context, accountType model.AccountType) (*model.Account, error) {
	return nil, nil
}

type fakeTpTransactionRepo struct {
	tx        *model.Transaction
	getErr    error
	updateErr error
}

func (f *fakeTpTransactionRepo) Create(_ context.Context, t *model.Transaction) error {
	t.TransactionID = 1
	f.tx = t
	return nil
}
func (f *fakeTpTransactionRepo) GetByID(_ context.Context, _ uint) (*model.Transaction, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.tx, nil
}
func (f *fakeTpTransactionRepo) Update(_ context.Context, t *model.Transaction) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.tx = t
	return nil
}
func (f *fakeTpTransactionRepo) GetByPayerAccountNumber(_ context.Context, _ string) ([]*model.Transaction, error) {
	return nil, nil
}
func (f *fakeTpTransactionRepo) GetByRecipientAccountNumber(_ context.Context, _ string) ([]*model.Transaction, error) {
	return nil, nil
}

func (r *fakeTpTransactionRepo) FindByAccountType(ctx context.Context, accountType model.AccountType) ([]model.Account, error) {
	return nil, nil
}

// ── Helpers ────────────────────────────────────────────────────────────────

func tpAccount(number string, balance float64, currency model.CurrencyCode) *model.Account {
	return &model.Account{
		AccountNumber:    number,
		Balance:          balance,
		AvailableBalance: balance,
		DailyLimit:       1_000_000,
		MonthlyLimit:     10_000_000,
		Currency:         model.Currency{Code: currency},
	}
}

func newTpProcessor(accRepo *fakeTpAccountRepo, txRepo *fakeTpTransactionRepo) *TransactionProcessor {
	return NewTransactionProcessor(accRepo, txRepo, &fakeBankingTxManager{})
}

// ── Process Tests ──────────────────────────────────────────────────────────

func TestProcess_AlreadyProcessed(t *testing.T) {
	txRepo := &fakeTpTransactionRepo{
		tx: &model.Transaction{
			TransactionID:          1,
			PayerAccountNumber:     "PAYER",
			RecipientAccountNumber: "RECIP",
			StartAmount:            100,
			StartCurrencyCode:      model.RSD,
			EndAmount:              100,
			EndCurrencyCode:        model.RSD,
			Status:                 model.TransactionCompleted,
		},
	}
	accRepo := newFakeTpAccountRepo()
	tp := newTpProcessor(accRepo, txRepo)

	err := tp.Process(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "transaction already processed")
}

func TestProcess_PayerNotFound(t *testing.T) {
	txRepo := &fakeTpTransactionRepo{
		tx: &model.Transaction{
			TransactionID:          1,
			PayerAccountNumber:     "MISSING-PAYER",
			RecipientAccountNumber: "RECIP",
			StartAmount:            100,
			StartCurrencyCode:      model.RSD,
			EndAmount:              100,
			EndCurrencyCode:        model.RSD,
			Status:                 model.TransactionProcessing,
		},
	}
	accRepo := newFakeTpAccountRepo()
	tp := newTpProcessor(accRepo, txRepo)

	err := tp.Process(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "account not found")
}

func TestProcess_InsufficientBalance(t *testing.T) {
	payer := tpAccount("PAYER", 50, model.RSD)
	recip := tpAccount("RECIP", 100, model.RSD)
	txRepo := &fakeTpTransactionRepo{
		tx: &model.Transaction{
			TransactionID:          1,
			PayerAccountNumber:     "PAYER",
			RecipientAccountNumber: "RECIP",
			StartAmount:            100,
			StartCurrencyCode:      model.RSD,
			EndAmount:              100,
			EndCurrencyCode:        model.RSD,
			Status:                 model.TransactionProcessing,
		},
	}
	accRepo := newFakeTpAccountRepo(payer, recip)
	tp := newTpProcessor(accRepo, txRepo)

	err := tp.Process(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient payer funds")
}

func TestProcess_DailyLimitExceeded(t *testing.T) {
	payer := tpAccount("PAYER", 10_000, model.RSD)
	payer.DailyLimit = 500
	payer.DailySpending = 450
	recip := tpAccount("RECIP", 100, model.RSD)

	txRepo := &fakeTpTransactionRepo{
		tx: &model.Transaction{
			TransactionID:          1,
			PayerAccountNumber:     "PAYER",
			RecipientAccountNumber: "RECIP",
			StartAmount:            100,
			StartCurrencyCode:      model.RSD,
			EndAmount:              100,
			EndCurrencyCode:        model.RSD,
			Status:                 model.TransactionProcessing,
		},
	}
	accRepo := newFakeTpAccountRepo(payer, recip)
	tp := newTpProcessor(accRepo, txRepo)

	err := tp.Process(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "daily limit exceeded")
}

func TestProcess_MonthlyLimitExceeded(t *testing.T) {
	payer := tpAccount("PAYER", 10_000, model.RSD)
	payer.MonthlyLimit = 500
	payer.MonthlySpending = 450
	recip := tpAccount("RECIP", 100, model.RSD)

	txRepo := &fakeTpTransactionRepo{
		tx: &model.Transaction{
			TransactionID:          1,
			PayerAccountNumber:     "PAYER",
			RecipientAccountNumber: "RECIP",
			StartAmount:            100,
			StartCurrencyCode:      model.RSD,
			EndAmount:              100,
			EndCurrencyCode:        model.RSD,
			Status:                 model.TransactionProcessing,
		},
	}
	accRepo := newFakeTpAccountRepo(payer, recip)
	tp := newTpProcessor(accRepo, txRepo)

	err := tp.Process(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "monthly limit exceeded")
}

func TestProcess_RecipientNotFound(t *testing.T) {
	payer := tpAccount("PAYER", 10_000, model.RSD)
	txRepo := &fakeTpTransactionRepo{
		tx: &model.Transaction{
			TransactionID:          1,
			PayerAccountNumber:     "PAYER",
			RecipientAccountNumber: "MISSING-RECIP",
			StartAmount:            100,
			StartCurrencyCode:      model.RSD,
			EndAmount:              100,
			EndCurrencyCode:        model.RSD,
			Status:                 model.TransactionProcessing,
		},
	}
	accRepo := newFakeTpAccountRepo(payer)
	tp := newTpProcessor(accRepo, txRepo)

	err := tp.Process(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "account not found")
}

func TestProcess_SelfPayment(t *testing.T) {
	payer := tpAccount("SAME-ACC", 10_000, model.RSD)
	txRepo := &fakeTpTransactionRepo{
		tx: &model.Transaction{
			TransactionID:          1,
			PayerAccountNumber:     "SAME-ACC",
			RecipientAccountNumber: "SAME-ACC",
			StartAmount:            100,
			StartCurrencyCode:      model.RSD,
			EndAmount:              100,
			EndCurrencyCode:        model.RSD,
			Status:                 model.TransactionProcessing,
		},
	}
	accRepo := newFakeTpAccountRepo(payer)
	tp := newTpProcessor(accRepo, txRepo)

	err := tp.Process(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot make payment to the same account")
}

func TestProcess_RecipientIsBankAccount(t *testing.T) {
	bankNum := BankAccounts[model.RSD]
	payer := tpAccount("PAYER", 10_000, model.RSD)
	bankAcc := tpAccount(bankNum, 1_000_000, model.RSD)

	txRepo := &fakeTpTransactionRepo{
		tx: &model.Transaction{
			TransactionID:          1,
			PayerAccountNumber:     "PAYER",
			RecipientAccountNumber: bankNum,
			StartAmount:            100,
			StartCurrencyCode:      model.RSD,
			EndAmount:              100,
			EndCurrencyCode:        model.RSD,
			Status:                 model.TransactionProcessing,
		},
	}
	accRepo := newFakeTpAccountRepo(payer, bankAcc)
	tp := newTpProcessor(accRepo, txRepo)

	err := tp.Process(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "recipient account cannot be one of the banks accounts")
}

func TestProcess_SameCurrencySuccess(t *testing.T) {
	payer := tpAccount("PAYER", 1000, model.RSD)
	recip := tpAccount("RECIP", 500, model.RSD)

	txRepo := &fakeTpTransactionRepo{
		tx: &model.Transaction{
			TransactionID:          1,
			PayerAccountNumber:     "PAYER",
			RecipientAccountNumber: "RECIP",
			StartAmount:            200,
			StartCurrencyCode:      model.RSD,
			EndAmount:              200,
			EndCurrencyCode:        model.RSD,
			Status:                 model.TransactionProcessing,
		},
	}
	accRepo := newFakeTpAccountRepo(payer, recip)
	tp := newTpProcessor(accRepo, txRepo)

	err := tp.Process(context.Background(), 1)
	require.NoError(t, err)

	require.InDelta(t, 800, accRepo.accounts["PAYER"].AvailableBalance, 0.01)
	require.InDelta(t, 700, accRepo.accounts["RECIP"].AvailableBalance, 0.01)
	require.Equal(t, model.TransactionCompleted, txRepo.tx.Status)
}

func TestProcess_CrossCurrencySuccess(t *testing.T) {
	payer := tpAccount("PAYER", 10_000, model.RSD)
	recip := tpAccount("RECIP", 100, model.EUR)
	bankRSD := tpAccount(BankAccounts[model.RSD], 1_000_000, model.RSD)
	bankEUR := tpAccount(BankAccounts[model.EUR], 1_000_000, model.EUR)

	// Also add all other bank accounts so FindByAccountNumber does not fail
	allAccounts := []*model.Account{payer, recip, bankRSD, bankEUR}
	for code, num := range BankAccounts {
		if code == model.RSD || code == model.EUR {
			continue
		}
		allAccounts = append(allAccounts, tpAccount(num, 1_000_000, code))
	}

	txRepo := &fakeTpTransactionRepo{
		tx: &model.Transaction{
			TransactionID:          1,
			PayerAccountNumber:     "PAYER",
			RecipientAccountNumber: "RECIP",
			StartAmount:            1000,
			StartCurrencyCode:      model.RSD,
			EndAmount:              8.5,
			EndCurrencyCode:        model.EUR,
			Status:                 model.TransactionProcessing,
		},
	}
	accRepo := newFakeTpAccountRepo(allAccounts...)
	tp := newTpProcessor(accRepo, txRepo)

	err := tp.Process(context.Background(), 1)
	require.NoError(t, err)

	// Payer lost StartAmount
	require.InDelta(t, 9000, accRepo.accounts["PAYER"].AvailableBalance, 0.01)
	// Bank RSD gained StartAmount
	require.InDelta(t, 1_001_000, accRepo.accounts[BankAccounts[model.RSD]].AvailableBalance, 0.01)
	// Bank EUR lost EndAmount
	require.InDelta(t, 999_991.5, accRepo.accounts[BankAccounts[model.EUR]].AvailableBalance, 0.01)
	// Recipient gained EndAmount
	require.InDelta(t, 108.5, accRepo.accounts["RECIP"].AvailableBalance, 0.01)
	require.Equal(t, model.TransactionCompleted, txRepo.tx.Status)
}

func TestProcess_CrossCurrency_BankFromNotFound(t *testing.T) {
	payer := tpAccount("PAYER", 10_000, model.RSD)
	recip := tpAccount("RECIP", 100, model.EUR)
	// Add bank RSD (the "to" bank for start currency) but NOT bank EUR (the "from" bank for end currency)
	bankRSD := tpAccount(BankAccounts[model.RSD], 1_000_000, model.RSD)

	txRepo := &fakeTpTransactionRepo{
		tx: &model.Transaction{
			TransactionID:          1,
			PayerAccountNumber:     "PAYER",
			RecipientAccountNumber: "RECIP",
			StartAmount:            1000,
			StartCurrencyCode:      model.RSD,
			EndAmount:              8.5,
			EndCurrencyCode:        model.EUR,
			Status:                 model.TransactionProcessing,
		},
	}
	// Only payer, recip, and bankRSD - no bankEUR
	accRepo := newFakeTpAccountRepo(payer, recip, bankRSD)
	tp := newTpProcessor(accRepo, txRepo)

	err := tp.Process(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "account not found")
}

func TestProcess_CrossCurrency_BankInsufficientFunds(t *testing.T) {
	payer := tpAccount("PAYER", 10_000, model.RSD)
	recip := tpAccount("RECIP", 100, model.EUR)
	bankRSD := tpAccount(BankAccounts[model.RSD], 1_000_000, model.RSD)
	bankEUR := tpAccount(BankAccounts[model.EUR], 5, model.EUR) // only 5 EUR, needs 8.5

	allAccounts := []*model.Account{payer, recip, bankRSD, bankEUR}
	for code, num := range BankAccounts {
		if code == model.RSD || code == model.EUR {
			continue
		}
		allAccounts = append(allAccounts, tpAccount(num, 1_000_000, code))
	}

	txRepo := &fakeTpTransactionRepo{
		tx: &model.Transaction{
			TransactionID:          1,
			PayerAccountNumber:     "PAYER",
			RecipientAccountNumber: "RECIP",
			StartAmount:            1000,
			StartCurrencyCode:      model.RSD,
			EndAmount:              8.5,
			EndCurrencyCode:        model.EUR,
			Status:                 model.TransactionProcessing,
		},
	}
	accRepo := newFakeTpAccountRepo(allAccounts...)
	tp := newTpProcessor(accRepo, txRepo)

	err := tp.Process(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient banks funds")
}

func TestProcess_GetByIDError(t *testing.T) {
	txRepo := &fakeTpTransactionRepo{
		getErr: errors.New("db error"),
	}
	accRepo := newFakeTpAccountRepo()
	tp := newTpProcessor(accRepo, txRepo)

	err := tp.Process(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "db error")
}

// ── ProcessTradeSettlement Tests ──────────────────────────────────────────

func TestProcessTradeSettlement_Success(t *testing.T) {
	payer := tpAccount("PAYER", 1000, model.RSD)
	recip := tpAccount("RECIP", 500, model.RSD)

	txRepo := &fakeTpTransactionRepo{
		tx: &model.Transaction{
			TransactionID:          1,
			PayerAccountNumber:     "PAYER",
			RecipientAccountNumber: "RECIP",
			StartAmount:            200,
			StartCurrencyCode:      model.RSD,
			EndAmount:              200,
			EndCurrencyCode:        model.RSD,
			Status:                 model.TransactionProcessing,
		},
	}
	accRepo := newFakeTpAccountRepo(payer, recip)
	tp := newTpProcessor(accRepo, txRepo)

	err := tp.ProcessTradeSettlement(context.Background(), 1)
	require.NoError(t, err)
	require.InDelta(t, 800, accRepo.accounts["PAYER"].AvailableBalance, 0.01)
	require.InDelta(t, 700, accRepo.accounts["RECIP"].AvailableBalance, 0.01)
	require.Equal(t, model.TransactionCompleted, txRepo.tx.Status)
}

func TestProcessTradeSettlement_AlreadyProcessed(t *testing.T) {
	txRepo := &fakeTpTransactionRepo{
		tx: &model.Transaction{
			TransactionID:          1,
			PayerAccountNumber:     "PAYER",
			RecipientAccountNumber: "RECIP",
			StartAmount:            100,
			Status:                 model.TransactionCompleted,
		},
	}
	accRepo := newFakeTpAccountRepo()
	tp := newTpProcessor(accRepo, txRepo)

	err := tp.ProcessTradeSettlement(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "transaction already processed")
}

func TestProcessTradeSettlement_PayerNotFound(t *testing.T) {
	txRepo := &fakeTpTransactionRepo{
		tx: &model.Transaction{
			TransactionID:          1,
			PayerAccountNumber:     "MISSING",
			RecipientAccountNumber: "RECIP",
			StartAmount:            100,
			Status:                 model.TransactionProcessing,
		},
	}
	accRepo := newFakeTpAccountRepo()
	tp := newTpProcessor(accRepo, txRepo)

	err := tp.ProcessTradeSettlement(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "account not found")
}

func TestProcessTradeSettlement_SelfSettlement(t *testing.T) {
	acc := tpAccount("SAME", 1000, model.RSD)
	txRepo := &fakeTpTransactionRepo{
		tx: &model.Transaction{
			TransactionID:          1,
			PayerAccountNumber:     "SAME",
			RecipientAccountNumber: "SAME",
			StartAmount:            100,
			Status:                 model.TransactionProcessing,
		},
	}
	accRepo := newFakeTpAccountRepo(acc)
	tp := newTpProcessor(accRepo, txRepo)

	err := tp.ProcessTradeSettlement(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot settle trade to the same account")
}

func TestProcessTradeSettlement_InsufficientFunds(t *testing.T) {
	payer := tpAccount("PAYER", 50, model.RSD)
	recip := tpAccount("RECIP", 500, model.RSD)

	txRepo := &fakeTpTransactionRepo{
		tx: &model.Transaction{
			TransactionID:          1,
			PayerAccountNumber:     "PAYER",
			RecipientAccountNumber: "RECIP",
			StartAmount:            100,
			Status:                 model.TransactionProcessing,
		},
	}
	accRepo := newFakeTpAccountRepo(payer, recip)
	tp := newTpProcessor(accRepo, txRepo)

	err := tp.ProcessTradeSettlement(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient payer funds")
}

// ── ProcessLoanInstallment Tests ─────────────────────────────────────────

func TestProcessLoanInstallment_Success(t *testing.T) {
	payer := tpAccount("PAYER", 1000, model.RSD)
	recip := tpAccount("RECIP", 500, model.RSD)

	txRepo := &fakeTpTransactionRepo{
		tx: &model.Transaction{
			TransactionID:          1,
			PayerAccountNumber:     "PAYER",
			RecipientAccountNumber: "RECIP",
			StartAmount:            200,
			StartCurrencyCode:      model.RSD,
			EndAmount:              200,
			EndCurrencyCode:        model.RSD,
			Status:                 model.TransactionProcessing,
		},
	}
	accRepo := newFakeTpAccountRepo(payer, recip)
	tp := newTpProcessor(accRepo, txRepo)

	err := tp.ProcessLoanInstallment(context.Background(), 1)
	require.NoError(t, err)
	require.InDelta(t, 800, accRepo.accounts["PAYER"].AvailableBalance, 0.01)
	require.InDelta(t, 700, accRepo.accounts["RECIP"].AvailableBalance, 0.01)
	require.Equal(t, model.TransactionCompleted, txRepo.tx.Status)
}

func TestProcessLoanInstallment_AlreadyProcessed(t *testing.T) {
	txRepo := &fakeTpTransactionRepo{
		tx: &model.Transaction{
			TransactionID:          1,
			PayerAccountNumber:     "PAYER",
			RecipientAccountNumber: "RECIP",
			StartAmount:            100,
			Status:                 model.TransactionCompleted,
		},
	}
	accRepo := newFakeTpAccountRepo()
	tp := newTpProcessor(accRepo, txRepo)

	err := tp.ProcessLoanInstallment(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "transaction already processed")
}

func TestProcessLoanInstallment_PayerNotFound(t *testing.T) {
	txRepo := &fakeTpTransactionRepo{
		tx: &model.Transaction{
			TransactionID:          1,
			PayerAccountNumber:     "MISSING",
			RecipientAccountNumber: "RECIP",
			StartAmount:            100,
			Status:                 model.TransactionProcessing,
		},
	}
	accRepo := newFakeTpAccountRepo()
	tp := newTpProcessor(accRepo, txRepo)

	err := tp.ProcessLoanInstallment(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "account not found")
}

func TestProcessLoanInstallment_InsufficientFunds(t *testing.T) {
	payer := tpAccount("PAYER", 50, model.RSD)
	recip := tpAccount("RECIP", 500, model.RSD)

	txRepo := &fakeTpTransactionRepo{
		tx: &model.Transaction{
			TransactionID:          1,
			PayerAccountNumber:     "PAYER",
			RecipientAccountNumber: "RECIP",
			StartAmount:            100,
			Status:                 model.TransactionProcessing,
		},
	}
	accRepo := newFakeTpAccountRepo(payer, recip)
	tp := newTpProcessor(accRepo, txRepo)

	err := tp.ProcessLoanInstallment(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient payer funds")
}
