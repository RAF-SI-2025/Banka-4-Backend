package service

import (
	"context"
	stderrors "errors"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
)

type fakeTransferAccountRepo struct {
	accounts          map[string]model.Account
	updateErrByNumber map[string]error
}

func (r *fakeTransferAccountRepo) Create(_ context.Context, _ *model.Account) error {
	return nil
}

func (r *fakeTransferAccountRepo) AccountNumberExists(_ context.Context, accountNumber string) (bool, error) {
	_, exists := r.accounts[accountNumber]
	return exists, nil
}

func (r *fakeTransferAccountRepo) FindByAccountNumber(_ context.Context, accountNumber string) (*model.Account, error) {
	account, exists := r.accounts[accountNumber]
	if !exists {
		return nil, nil
	}
	copy := account
	return &copy, nil
}

func (f *fakeTransferAccountRepo) FindByClientID(ctx context.Context, clientID uint) ([]model.Account, error) {
	return nil, nil
}

func (r *fakeTransferAccountRepo) GetByAccountNumber(ctx context.Context, accountNumber string) (*model.Account, error) {
	return r.FindByAccountNumber(ctx, accountNumber)
}

func (r *fakeTransferAccountRepo) Update(_ context.Context, account *model.Account) error {
	r.accounts[account.AccountNumber] = *account
	return nil
}
func (r *fakeTransferAccountRepo) FindByAccountType(ctx context.Context, accountType model.AccountType) (*model.Account, error) {
	return nil, nil
}

func (r *fakeTransferAccountRepo) FindAllByClientID(_ context.Context, clientID uint) ([]model.Account, error) {
	result := make([]model.Account, 0)
	for _, account := range r.accounts {
		if account.ClientID == clientID {
			result = append(result, account)
		}
	}
	return result, nil
}

func (r *fakeTransferAccountRepo) FindByAccountNumberAndClientID(_ context.Context, accountNumber string, clientID uint) (*model.Account, error) {
	account, err := r.FindByAccountNumber(context.Background(), accountNumber)
	if err != nil || account == nil {
		return account, err
	}
	if account.ClientID != clientID {
		return nil, nil
	}
	return account, nil
}

func (r *fakeTransferAccountRepo) UpdateName(_ context.Context, accountNumber string, name string) error {
	account, exists := r.accounts[accountNumber]
	if !exists {
		return nil
	}
	account.Name = name
	r.accounts[accountNumber] = account
	return nil
}

func (r *fakeTransferAccountRepo) UpdateLimits(_ context.Context, accountNumber string, daily float64, monthly float64) error {
	account, exists := r.accounts[accountNumber]
	if !exists {
		return nil
	}
	account.DailyLimit = daily
	account.MonthlyLimit = monthly
	r.accounts[accountNumber] = account
	return nil
}

func (r *fakeTransferAccountRepo) NameExistsForClient(_ context.Context, clientID uint, name string, excludeNumber string) (bool, error) {
	for accountNumber, account := range r.accounts {
		if account.ClientID != clientID || accountNumber == excludeNumber {
			continue
		}
		if account.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeTransferAccountRepo) FindAll(_ context.Context, _ *dto.ListAccountsQuery) ([]*model.Account, int64, error) {
	return nil, 0, nil
}

func (r *fakeTransferAccountRepo) UpdateBalance(_ context.Context, account *model.Account) error {
	if err := r.updateErrByNumber[account.AccountNumber]; err != nil {
		return err
	}
	r.accounts[account.AccountNumber] = *account
	return nil
}

func (r *fakeTransferAccountRepo) clone() map[string]model.Account {
	cloned := make(map[string]model.Account, len(r.accounts))
	for key, value := range r.accounts {
		cloned[key] = value
	}
	return cloned
}

func (r *fakeTransferAccountRepo) restore(snapshot map[string]model.Account) {
	r.accounts = snapshot
}

type fakeTransferTransactionRepo struct {
	createErr    error
	transactions []model.Transaction
	nextID       uint
}

func (r *fakeTransferTransactionRepo) Create(_ context.Context, transaction *model.Transaction) error {
	if r.createErr != nil {
		return r.createErr
	}

	if r.nextID == 0 {
		r.nextID = 1
	}

	transaction.TransactionID = r.nextID
	r.nextID++
	transaction.CreatedAt = time.Now().UTC()

	cloned := cloneTransaction(*transaction)
	r.transactions = append(r.transactions, cloned)

	return nil
}

func (r *fakeTransferTransactionRepo) GetByPayerAccountNumber(_ context.Context, _ string) ([]*model.Transaction, error) {
	return nil, nil
}

func (r *fakeTransferTransactionRepo) GetByRecipientAccountNumber(_ context.Context, _ string) ([]*model.Transaction, error) {
	return nil, nil
}

func (r *fakeTransferTransactionRepo) GetByID(_ context.Context, transactionID uint) (*model.Transaction, error) {
	for _, transaction := range r.transactions {
		if transaction.TransactionID == transactionID {
			copy := transaction
			return &copy, nil
		}
	}
	return nil, nil
}

func (r *fakeTransferTransactionRepo) Update(_ context.Context, _ *model.Transaction) error {
	return nil
}

func (r *fakeTransferTransactionRepo) clone() []model.Transaction {
	cloned := make([]model.Transaction, 0, len(r.transactions))
	for _, transaction := range r.transactions {
		cloned = append(cloned, cloneTransaction(transaction))
	}
	return cloned
}

func (r *fakeTransferTransactionRepo) restore(snapshot []model.Transaction) {
	r.transactions = snapshot
	var maxID uint
	for _, transaction := range snapshot {
		if transaction.TransactionID > maxID {
			maxID = transaction.TransactionID
		}
	}
	r.nextID = maxID + 1
}

type fakeTransferRepo struct {
	createErr error
	history   []model.Transfer
	created   []model.Transfer
}

func (r *fakeTransferRepo) Create(_ context.Context, transfer *model.Transfer) error {
	if r.createErr != nil {
		return r.createErr
	}

	if transfer.TransferID == 0 {
		transfer.TransferID = uint(len(r.created) + 1)
	}

	cloned := cloneTransfer(*transfer)
	r.created = append(r.created, cloned)
	r.history = append(r.history, cloned)
	return nil
}

func (r *fakeTransferRepo) ListByClientID(_ context.Context, clientID uint, page, pageSize int) ([]model.Transfer, int64, error) {
	filtered := make([]model.Transfer, 0)
	for _, transfer := range r.history {
		_ = clientID
		filtered = append(filtered, cloneTransfer(transfer))
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Transaction.CreatedAt.Equal(filtered[j].Transaction.CreatedAt) {
			return filtered[i].Transaction.TransactionID > filtered[j].Transaction.TransactionID
		}
		return filtered[i].Transaction.CreatedAt.After(filtered[j].Transaction.CreatedAt)
	})

	total := int64(len(filtered))
	offset := (page - 1) * pageSize
	if offset >= len(filtered) {
		return []model.Transfer{}, total, nil
	}

	end := offset + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}

	return filtered[offset:end], total, nil
}

func (r *fakeTransferRepo) clone() []model.Transfer {
	cloned := make([]model.Transfer, 0, len(r.history))
	for _, transfer := range r.history {
		cloned = append(cloned, cloneTransfer(transfer))
	}
	return cloned
}

func (r *fakeTransferRepo) restore(snapshot []model.Transfer) {
	r.history = snapshot
	r.created = make([]model.Transfer, 0)
	for _, transfer := range snapshot {
		r.created = append(r.created, cloneTransfer(transfer))
	}
}

type fakeTransferExchangeConverter struct {
	converted float64
	err       error
}

func (c *fakeTransferExchangeConverter) Convert(_ context.Context, amount float64, _, _ model.CurrencyCode) (float64, error) {
	if c.err != nil {
		return 0, c.err
	}
	if c.converted > 0 {
		return c.converted, nil
	}
	return amount, nil
}

func (c *fakeTransferExchangeConverter) CalculateFee(amount float64) float64 {
	return amount * model.BankCommission
}

type fakeTransferTxManager struct {
	accountRepo     *fakeTransferAccountRepo
	transactionRepo *fakeTransferTransactionRepo
	transferRepo    *fakeTransferRepo
}

func (m *fakeTransferTxManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	accountSnapshot := m.accountRepo.clone()
	transactionSnapshot := m.transactionRepo.clone()
	transferSnapshot := m.transferRepo.clone()

	err := fn(ctx)
	if err != nil {
		m.accountRepo.restore(accountSnapshot)
		m.transactionRepo.restore(transactionSnapshot)
		m.transferRepo.restore(transferSnapshot)
	}
	return err
}

func TestExecuteTransferSameCurrency(t *testing.T) {
	env := newTransferServiceForTests(
		[]model.Account{
			testAccount("FROM-RSD", 1, model.RSD, 1000, "Active"),
			testAccount("TO-RSD", 1, model.RSD, 100, "Active"),
		},
		&fakeTransferExchangeConverter{},
	)

	response, err := env.service.ExecuteTransfer(
		transferClientContext(1),
		dto.TransferRequest{
			FromAccountNumber: "FROM-RSD",
			ToAccountNumber:   "TO-RSD",
			Amount:            200,
		},
	)

	require.NoError(t, err)
	require.Equal(t, 200.0, response.InitialAmount)
	require.Equal(t, 200.0, response.FinalAmount)
	require.Equal(t, 0.0, response.Commission)
	require.Nil(t, response.ExchangeRate)
}

func TestExecuteTransferCrossCurrency(t *testing.T) {
	env := newTransferServiceForTests(
		[]model.Account{
			testAccount("FROM-RSD", 1, model.RSD, 1000, "Active"),
			testAccount("TO-EUR", 1, model.EUR, 100, "Active"),
		},
		&fakeTransferExchangeConverter{converted: 120},
	)

	response, err := env.service.ExecuteTransfer(
		transferClientContext(1),
		dto.TransferRequest{
			FromAccountNumber: "FROM-RSD",
			ToAccountNumber:   "TO-EUR",
			Amount:            100,
		},
	)

	require.NoError(t, err)
	require.InDelta(t, 1.2, *response.ExchangeRate, 0.000001)
	require.InDelta(t, 1.5, response.Commission, 0.000001)
	require.InDelta(t, 120.0, response.FinalAmount, 0.000001)
	require.InDelta(t, 898.5, env.accountRepo.accounts["FROM-RSD"].AvailableBalance, 0.000001)
	require.InDelta(t, 220.0, env.accountRepo.accounts["TO-EUR"].AvailableBalance, 0.000001)
}

func TestTransferRejectsSameSourceAndDestination(t *testing.T) {
	service := newTransferServiceForTests(
		[]model.Account{
			testAccount("ACC-1", 1, model.RSD, 1000, "Active"),
		},
		&fakeTransferExchangeConverter{},
	).service

	_, err := service.ExecuteTransfer(
		transferClientContext(1),
		dto.TransferRequest{
			FromAccountNumber: "ACC-1",
			ToAccountNumber:   "ACC-1",
			Amount:            100,
		},
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "source and destination account must be different")
}

func TestTransferRejectsWhenAccountDoesNotBelongToClient(t *testing.T) {
	service := newTransferServiceForTests(
		[]model.Account{
			testAccount("FROM-OWN", 1, model.RSD, 1000, "Active"),
			testAccount("TO-OTHER", 2, model.RSD, 100, "Active"),
		},
		&fakeTransferExchangeConverter{},
	).service

	_, err := service.ExecuteTransfer(
		transferClientContext(1),
		dto.TransferRequest{
			FromAccountNumber: "FROM-OWN",
			ToAccountNumber:   "TO-OTHER",
			Amount:            100,
		},
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "accounts do not belong to the provided client")
}

func TestTransferRejectsInsufficientFunds(t *testing.T) {
	service := newTransferServiceForTests(
		[]model.Account{
			testAccount("FROM-LOW", 1, model.RSD, 50, "Active"),
			testAccount("TO-RSD", 1, model.RSD, 100, "Active"),
		},
		&fakeTransferExchangeConverter{},
	).service

	_, err := service.ExecuteTransfer(
		transferClientContext(1),
		dto.TransferRequest{
			FromAccountNumber: "FROM-LOW",
			ToAccountNumber:   "TO-RSD",
			Amount:            100,
		},
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient funds")
}

func TestTransferRejectsInsufficientFundsWhenCommissionApplies(t *testing.T) {
	service := newTransferServiceForTests(
		[]model.Account{
			testAccount("FROM-RSD", 1, model.RSD, 100, "Active"),
			testAccount("TO-EUR", 1, model.EUR, 100, "Active"),
		},
		&fakeTransferExchangeConverter{converted: 120},
	).service

	_, err := service.ExecuteTransfer(
		transferClientContext(1),
		dto.TransferRequest{
			FromAccountNumber: "FROM-RSD",
			ToAccountNumber:   "TO-EUR",
			Amount:            100,
		},
	)

	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient funds")
}

func TestExecuteTransferPersistsAndUpdatesBalances(t *testing.T) {
	env := newTransferServiceForTests(
		[]model.Account{
			testAccount("FROM-RSD", 1, model.RSD, 1000, "Active"),
			testAccount("TO-RSD", 1, model.RSD, 500, "Active"),
		},
		&fakeTransferExchangeConverter{},
	)

	response, err := env.service.ExecuteTransfer(
		transferClientContext(1),
		dto.TransferRequest{
			FromAccountNumber: "FROM-RSD",
			ToAccountNumber:   "TO-RSD",
			Amount:            250,
		},
	)

	require.NoError(t, err)
	require.NotZero(t, response.TransferID)
	require.Equal(t, 1, len(env.transactionRepo.transactions))
	require.Equal(t, 1, len(env.transferRepo.created))
	require.InDelta(t, 750.0, env.accountRepo.accounts["FROM-RSD"].AvailableBalance, 0.000001)
	require.InDelta(t, 750.0, env.accountRepo.accounts["TO-RSD"].AvailableBalance, 0.000001)
}

func TestExecuteTransferRollsBackWhenPersistenceFails(t *testing.T) {
	env := newTransferServiceForTests(
		[]model.Account{
			testAccount("FROM-RSD", 1, model.RSD, 1000, "Active"),
			testAccount("TO-RSD", 1, model.RSD, 500, "Active"),
		},
		&fakeTransferExchangeConverter{},
	)
	env.transferRepo.createErr = stderrors.New("insert failed")

	_, err := env.service.ExecuteTransfer(
		transferClientContext(1),
		dto.TransferRequest{
			FromAccountNumber: "FROM-RSD",
			ToAccountNumber:   "TO-RSD",
			Amount:            250,
		},
	)

	require.Error(t, err)
	require.Equal(t, 0, len(env.transferRepo.created))
	require.Equal(t, 0, len(env.transactionRepo.transactions))
	require.InDelta(t, 1000.0, env.accountRepo.accounts["FROM-RSD"].AvailableBalance, 0.000001)
	require.InDelta(t, 500.0, env.accountRepo.accounts["TO-RSD"].AvailableBalance, 0.000001)
}

func TestTransferHistoryReturnsNewestFirst(t *testing.T) {
	env := newTransferServiceForTests(
		[]model.Account{
			testAccount("FROM", 1, model.RSD, 1000, "Active"),
			testAccount("TO", 1, model.RSD, 1000, "Active"),
		},
		&fakeTransferExchangeConverter{},
	)

	now := time.Now().UTC()

	env.transferRepo.history = []model.Transfer{
		{
			TransferID:    1,
			TransactionID: 7,
			Transaction: model.Transaction{
				TransactionID:          7,
				PayerAccountNumber:     "FROM",
				RecipientAccountNumber: "TO",
				StartAmount:            100,
				EndAmount:              100,
				CreatedAt:              now.Add(-10 * time.Minute),
			},
		},
		{
			TransferID:    2,
			TransactionID: 9,
			Transaction: model.Transaction{
				TransactionID:          9,
				PayerAccountNumber:     "FROM",
				RecipientAccountNumber: "TO",
				StartAmount:            100,
				EndAmount:              100,
				CreatedAt:              now,
			},
		},
		{
			TransferID:    3,
			TransactionID: 8,
			Transaction: model.Transaction{
				TransactionID:          8,
				PayerAccountNumber:     "FROM",
				RecipientAccountNumber: "TO",
				StartAmount:            100,
				EndAmount:              100,
				CreatedAt:              now.Add(-5 * time.Minute),
			},
		},
	}

	response, err := env.service.GetTransferHistory(transferClientContext(1), 1, 1, 10)
	require.NoError(t, err)
	require.Len(t, response.Data, 3)
	require.Equal(t, uint(9), response.Data[0].TransactionID)
	require.Equal(t, uint(8), response.Data[1].TransactionID)
	require.Equal(t, uint(7), response.Data[2].TransactionID)
}

type transferTestEnv struct {
	service         *TransferService
	accountRepo     *fakeTransferAccountRepo
	transactionRepo *fakeTransferTransactionRepo
	transferRepo    *fakeTransferRepo
}

func newTransferServiceForTests(accounts []model.Account, converter *fakeTransferExchangeConverter) *transferTestEnv {
	accountRepo := &fakeTransferAccountRepo{
		accounts:          map[string]model.Account{},
		updateErrByNumber: map[string]error{},
	}

	for currency, accountNumber := range BankAccounts {
		accountRepo.accounts[accountNumber] = testAccount(accountNumber, 0, currency, 1_000_000_000, "Active")
	}

	for _, account := range accounts {
		accountRepo.accounts[account.AccountNumber] = account
	}

	transactionRepo := &fakeTransferTransactionRepo{}
	transferRepo := &fakeTransferRepo{
		history: []model.Transfer{},
		created: []model.Transfer{},
	}

	txManager := &fakeTransferTxManager{
		accountRepo:     accountRepo,
		transactionRepo: transactionRepo,
		transferRepo:    transferRepo,
	}

	transactionProcessor := NewTransactionProcessor(accountRepo, transactionRepo, txManager)
	service := NewTransferService(transferRepo, transactionRepo, accountRepo, converter, txManager, transactionProcessor)
	return &transferTestEnv{
		service:         service,
		accountRepo:     accountRepo,
		transactionRepo: transactionRepo,
		transferRepo:    transferRepo,
	}
}

func testAccount(number string, clientID uint, currency model.CurrencyCode, balance float64, status string) model.Account {
	return model.Account{
		AccountNumber:    number,
		ClientID:         clientID,
		Balance:          balance,
		AvailableBalance: balance,
		DailyLimit:       1_000_000_000,
		MonthlyLimit:     1_000_000_000,
		Status:           status,
		Currency:         model.Currency{Code: currency},
	}
}

func transferClientContext(clientID uint) context.Context {
	return auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityType: auth.IdentityClient,
		ClientID:     &clientID,
	})
}

func cloneTransaction(transaction model.Transaction) model.Transaction { return transaction }

func cloneTransfer(transfer model.Transfer) model.Transfer {
	cloned := transfer
	if transfer.ExchangeRate != nil {
		value := *transfer.ExchangeRate
		cloned.ExchangeRate = &value
	}
	cloned.Transaction = cloneTransaction(transfer.Transaction)
	return cloned
}
