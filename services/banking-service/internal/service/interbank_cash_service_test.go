package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
)

// ── Fakes scoped to interbank cash posting record-keeping ──────────────────

type fakeCashPostingRepo struct {
	postings  map[string]*model.InterbankCashPosting
	saveCount int
}

func newFakeCashPostingRepo() *fakeCashPostingRepo {
	return &fakeCashPostingRepo{postings: map[string]*model.InterbankCashPosting{}}
}

func (f *fakeCashPostingRepo) Create(_ context.Context, p *model.InterbankCashPosting) error {
	f.postings[p.PostingID] = p
	return nil
}

func (f *fakeCashPostingRepo) FindByID(_ context.Context, id string) (*model.InterbankCashPosting, error) {
	return f.postings[id], nil
}

func (f *fakeCashPostingRepo) Save(_ context.Context, p *model.InterbankCashPosting) error {
	f.postings[p.PostingID] = p
	f.saveCount++
	return nil
}

// recTxRepo / recPayRepo count created records so tests can assert exactly-once.

type recTxRepo struct {
	created  []*model.Transaction
	existing *model.Transaction // returned by GetByID for the dedup gate
}

func (r *recTxRepo) Create(_ context.Context, t *model.Transaction) error {
	t.TransactionID = uint(len(r.created) + 1)
	r.created = append(r.created, t)
	return nil
}
func (r *recTxRepo) Update(_ context.Context, _ *model.Transaction) error { return nil }
func (r *recTxRepo) GetByID(_ context.Context, _ uint) (*model.Transaction, error) {
	return r.existing, nil
}
func (r *recTxRepo) GetByPayerAccountNumber(_ context.Context, _ string) ([]*model.Transaction, error) {
	return nil, nil
}
func (r *recTxRepo) GetByRecipientAccountNumber(_ context.Context, _ string) ([]*model.Transaction, error) {
	return nil, nil
}

type recPayRepo struct {
	created []*model.Payment
}

func (r *recPayRepo) Create(_ context.Context, p *model.Payment) error {
	r.created = append(r.created, p)
	return nil
}
func (r *recPayRepo) GetByID(_ context.Context, _ uint) (*model.Payment, error) { return nil, nil }
func (r *recPayRepo) Update(_ context.Context, _ *model.Payment) error          { return nil }
func (r *recPayRepo) FindByAccount(_ context.Context, _ string, _ *dto.PaymentFilters) ([]model.Payment, int64, error) {
	return nil, 0, nil
}
func (r *recPayRepo) FindByClient(_ context.Context, _ uint, _ *dto.PaymentFilters) ([]model.Payment, int64, error) {
	return nil, 0, nil
}

func newCashService(acc *fakePaymentAccountRepo, posts *fakeCashPostingRepo, txRepo *recTxRepo, payRepo *recPayRepo) *InterbankCashService {
	return NewInterbankCashService(acc, posts, &fakeBankingTxManager{}, &fakeCurrencyConverter{}, txRepo, payRepo)
}

func localAccount(number string, balance float64) *model.Account {
	return &model.Account{
		AccountNumber:    number,
		Balance:          balance,
		AvailableBalance: balance,
		Currency:         model.Currency{Code: model.RSD},
	}
}

// ── Commit creates Transaction+Payment history records ─────────────────────

func TestInterbankCashService_Commit_CreatesHistory(t *testing.T) {
	const local = "444000000000000011"
	const counterparty = "111000000000000022"

	cases := []struct {
		name          string
		amount        float64 // resolved amount; sign drives payer/recipient
		bankingTxID   uint64
		existingTx    *model.Transaction
		wantRecords   bool
		wantPayer     string
		wantRecipient string
	}{
		{
			name:          "incoming credit records local as recipient",
			amount:        100,
			wantRecords:   true,
			wantPayer:     counterparty,
			wantRecipient: local,
		},
		{
			name:          "outgoing debit records local as payer",
			amount:        -100,
			wantRecords:   true,
			wantPayer:     local,
			wantRecipient: counterparty,
		},
		{
			name:        "initiating payment leg with existing transaction is skipped",
			amount:      -100,
			bankingTxID: 7,
			existingTx:  &model.Transaction{TransactionID: 7},
			wantRecords: false,
		},
		{
			name:          "initiating leg whose transaction is missing still records",
			amount:        -100,
			bankingTxID:   7,
			existingTx:    nil,
			wantRecords:   true,
			wantPayer:     local,
			wantRecipient: counterparty,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			acc := newFakePaymentAccountRepo(localAccount(local, 1000))
			posts := newFakeCashPostingRepo()
			txRepo := &recTxRepo{existing: tc.existingTx}
			payRepo := &recPayRepo{}
			svc := newCashService(acc, posts, txRepo, payRepo)

			posts.postings["pid-1"] = &model.InterbankCashPosting{
				PostingID:                 "pid-1",
				AccountNumber:             local,
				CurrencyCode:              model.RSD,
				Amount:                    tc.amount,
				RequestedCurrencyCode:     model.RSD,
				RequestedAmount:           tc.amount,
				Status:                    model.InterbankCashPostingPrepared,
				BankingTxID:               tc.bankingTxID,
				CounterpartyAccountNumber: counterparty,
				PaymentCode:               "289",
				Purpose:                   "interbank transfer",
			}

			_, err := svc.Commit(context.Background(), "pid-1")
			require.NoError(t, err)

			if !tc.wantRecords {
				require.Empty(t, txRepo.created, "expected no transaction record")
				require.Empty(t, payRepo.created, "expected no payment record")
				return
			}

			require.Len(t, txRepo.created, 1)
			require.Len(t, payRepo.created, 1)
			rec := txRepo.created[0]
			require.Equal(t, model.TransactionCompleted, rec.Status)
			require.Equal(t, 100.0, rec.StartAmount)
			require.Equal(t, 100.0, rec.EndAmount)
			require.Equal(t, tc.wantPayer, rec.PayerAccountNumber)
			require.Equal(t, tc.wantRecipient, rec.RecipientAccountNumber)
			require.Equal(t, "289", payRepo.created[0].PaymentCode)
			require.Equal(t, "interbank transfer", payRepo.created[0].Purpose)
			require.Equal(t, rec.TransactionID, payRepo.created[0].TransactionID)
		})
	}
}

func TestInterbankCashService_Commit_Idempotent(t *testing.T) {
	const local = "444000000000000011"
	acc := newFakePaymentAccountRepo(localAccount(local, 1000))
	posts := newFakeCashPostingRepo()
	txRepo := &recTxRepo{}
	payRepo := &recPayRepo{}
	svc := newCashService(acc, posts, txRepo, payRepo)

	posts.postings["pid-1"] = &model.InterbankCashPosting{
		PostingID:             "pid-1",
		AccountNumber:         local,
		CurrencyCode:          model.RSD,
		Amount:                100,
		RequestedCurrencyCode: model.RSD,
		RequestedAmount:       100,
		Status:                model.InterbankCashPostingPrepared,
	}

	_, err := svc.Commit(context.Background(), "pid-1")
	require.NoError(t, err)
	// A retransmitted COMMIT_TX must not create a second record.
	_, err = svc.Commit(context.Background(), "pid-1")
	require.NoError(t, err)

	require.Len(t, txRepo.created, 1, "record must be created exactly once")
	require.Len(t, payRepo.created, 1)
}

type fakeInterbankAccountRepo struct {
	accounts       map[string]*model.Account
	accountsByUser map[uint][]model.Account
	findErr        error
	updateErr      error
}

func newFakeInterbankAccountRepo(accounts ...*model.Account) *fakeInterbankAccountRepo {
	repo := &fakeInterbankAccountRepo{
		accounts:       map[string]*model.Account{},
		accountsByUser: map[uint][]model.Account{},
	}
	for _, account := range accounts {
		repo.accounts[account.AccountNumber] = account
		repo.accountsByUser[account.ClientID] = append(repo.accountsByUser[account.ClientID], *account)
	}
	return repo
}

func (r *fakeInterbankAccountRepo) Create(context.Context, *model.Account) error { return nil }
func (r *fakeInterbankAccountRepo) AccountNumberExists(context.Context, string) (bool, error) {
	return false, nil
}
func (r *fakeInterbankAccountRepo) GetByAccountNumber(ctx context.Context, accountNumber string) (*model.Account, error) {
	return r.FindByAccountNumber(ctx, accountNumber)
}
func (r *fakeInterbankAccountRepo) Update(context.Context, *model.Account) error { return nil }
func (r *fakeInterbankAccountRepo) FindAllByClientID(context.Context, uint) ([]model.Account, error) {
	return nil, nil
}
func (r *fakeInterbankAccountRepo) FindByAccountNumberAndClientID(context.Context, string, uint) (*model.Account, error) {
	return nil, nil
}
func (r *fakeInterbankAccountRepo) UpdateName(context.Context, string, string) error { return nil }
func (r *fakeInterbankAccountRepo) UpdateLimits(context.Context, string, float64, float64) error {
	return nil
}
func (r *fakeInterbankAccountRepo) NameExistsForClient(context.Context, uint, string, string) (bool, error) {
	return false, nil
}
func (r *fakeInterbankAccountRepo) FindByAccountNumber(_ context.Context, accountNumber string) (*model.Account, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.accounts[accountNumber], nil
}
func (r *fakeInterbankAccountRepo) UpdateBalance(_ context.Context, account *model.Account) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.accounts[account.AccountNumber] = account
	return nil
}
func (r *fakeInterbankAccountRepo) FindAll(context.Context, *dto.ListAccountsQuery) ([]*model.Account, int64, error) {
	return nil, 0, nil
}
func (r *fakeInterbankAccountRepo) FindByClientID(_ context.Context, clientID uint) ([]model.Account, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.accountsByUser[clientID], nil
}
func (r *fakeInterbankAccountRepo) FindByAccountType(context.Context, model.AccountType) (*model.Account, error) {
	return nil, nil
}

type fakeInterbankPostingRepo struct {
	rows    map[string]*model.InterbankCashPosting
	findErr error
	saveErr error
}

func newFakeInterbankPostingRepo(rows ...*model.InterbankCashPosting) *fakeInterbankPostingRepo {
	repo := &fakeInterbankPostingRepo{rows: map[string]*model.InterbankCashPosting{}}
	for _, row := range rows {
		cp := *row
		repo.rows[row.PostingID] = &cp
	}
	return repo
}

func (r *fakeInterbankPostingRepo) Create(_ context.Context, posting *model.InterbankCashPosting) error {
	cp := *posting
	r.rows[posting.PostingID] = &cp
	return nil
}

func (r *fakeInterbankPostingRepo) FindByID(_ context.Context, postingID string) (*model.InterbankCashPosting, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	row := r.rows[postingID]
	if row == nil {
		return nil, nil
	}
	cp := *row
	return &cp, nil
}

func (r *fakeInterbankPostingRepo) Save(_ context.Context, posting *model.InterbankCashPosting) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	cp := *posting
	r.rows[posting.PostingID] = &cp
	return nil
}

type fakeInterbankConverter struct {
	result float64
	err    error
}

func (c fakeInterbankConverter) Convert(context.Context, float64, model.CurrencyCode, model.CurrencyCode) (float64, error) {
	if c.err != nil {
		return 0, c.err
	}
	return c.result, nil
}

func (c fakeInterbankConverter) CalculateFee(amount float64) float64 { return amount }

func interbankAccount(number string, clientID uint, code model.CurrencyCode, balance, available float64) *model.Account {
	return &model.Account{
		AccountNumber:    number,
		ClientID:         clientID,
		Currency:         model.Currency{Code: code},
		Balance:          balance,
		AvailableBalance: available,
		Status:           "Active",
	}
}

func TestInterbankCashPrepareExplicitAccountAndIdempotency(t *testing.T) {
	t.Parallel()

	account := interbankAccount("444000000000000011", 1, model.RSD, 1000, 1000)
	accounts := newFakeInterbankAccountRepo(account)
	postings := newFakeInterbankPostingRepo()
	svc := NewInterbankCashService(accounts, postings, &fakeBankingTxManager{}, fakeInterbankConverter{}, &recTxRepo{}, &recPayRepo{})

	posting, err := svc.Prepare(context.Background(), "posting-1", account.AccountNumber, 0, model.RSD, -250, "CLIENT", PostingMetadata{})
	if err != nil {
		t.Fatalf("prepare explicit account: %v", err)
	}
	if posting.Status != model.InterbankCashPostingPrepared || posting.AccountNumber != account.AccountNumber {
		t.Fatalf("unexpected posting %#v", posting)
	}
	if accounts.accounts[account.AccountNumber].AvailableBalance != 750 {
		t.Fatalf("available balance = %.2f, want 750", accounts.accounts[account.AccountNumber].AvailableBalance)
	}

	again, err := svc.Prepare(context.Background(), "posting-1", account.AccountNumber, 0, model.RSD, -250, "CLIENT", PostingMetadata{})
	if err != nil {
		t.Fatalf("idempotent prepare: %v", err)
	}
	if again.PostingID != "posting-1" || accounts.accounts[account.AccountNumber].AvailableBalance != 750 {
		t.Fatalf("unexpected idempotent prepare result %#v", again)
	}

	_, err = svc.Prepare(context.Background(), "posting-1", account.AccountNumber, 0, model.RSD, -251, "CLIENT", PostingMetadata{})
	if err == nil {
		t.Fatal("expected conflict for reused posting id with different amount")
	}
}

func TestInterbankCashPrepareChoosesClientAccountAndConverts(t *testing.T) {
	t.Parallel()

	eur := interbankAccount("444000000000000021", 7, model.EUR, 500, 500)
	rsd := interbankAccount("444000000000000022", 7, model.RSD, 20000, 20000)
	accounts := newFakeInterbankAccountRepo(eur, rsd)
	postings := newFakeInterbankPostingRepo()
	svc := NewInterbankCashService(accounts, postings, &fakeBankingTxManager{}, fakeInterbankConverter{result: -11700}, &recTxRepo{}, &recPayRepo{})

	posting, err := svc.Prepare(context.Background(), "posting-2", "", 7, model.USD, -100, "CLIENT", PostingMetadata{})
	if err != nil {
		t.Fatalf("prepare client account: %v", err)
	}
	if posting.AccountNumber != rsd.AccountNumber || posting.CurrencyCode != model.RSD || posting.Amount != -11700 {
		t.Fatalf("unexpected converted posting %#v", posting)
	}
	if accounts.accounts[rsd.AccountNumber].AvailableBalance != 8300 {
		t.Fatalf("available balance = %.2f, want 8300 after converted reserve", accounts.accounts[rsd.AccountNumber].AvailableBalance)
	}
}

func TestInterbankCashPrepareEmployeeUsesBankAccount(t *testing.T) {
	t.Parallel()

	bankAccount := interbankAccount(BankAccounts[model.RSD], 0, model.RSD, 100000, 100000)
	accounts := newFakeInterbankAccountRepo(bankAccount)
	svc := NewInterbankCashService(accounts, newFakeInterbankPostingRepo(), &fakeBankingTxManager{}, fakeInterbankConverter{}, &recTxRepo{}, &recPayRepo{})

	posting, err := svc.Prepare(context.Background(), "posting-employee", "", 0, model.RSD, 500, "EMPLOYEE", PostingMetadata{})
	if err != nil {
		t.Fatalf("prepare employee posting: %v", err)
	}
	if posting.AccountNumber != BankAccounts[model.RSD] || posting.Amount != 500 {
		t.Fatalf("unexpected employee posting %#v", posting)
	}
}

func TestInterbankCashCommitAndRollbackTransitions(t *testing.T) {
	t.Parallel()

	account := interbankAccount("444000000000000031", 1, model.RSD, 1000, 800)
	posting := &model.InterbankCashPosting{
		PostingID:             "posting-commit",
		AccountNumber:         account.AccountNumber,
		CurrencyCode:          model.RSD,
		Amount:                -200,
		RequestedCurrencyCode: model.RSD,
		RequestedAmount:       -200,
		Status:                model.InterbankCashPostingPrepared,
	}
	accounts := newFakeInterbankAccountRepo(account)
	postings := newFakeInterbankPostingRepo(posting)
	svc := NewInterbankCashService(accounts, postings, &fakeBankingTxManager{}, fakeInterbankConverter{}, &recTxRepo{}, &recPayRepo{})

	committed, err := svc.Commit(context.Background(), "posting-commit")
	if err != nil {
		t.Fatalf("commit posting: %v", err)
	}
	if committed.Status != model.InterbankCashPostingCommitted || accounts.accounts[account.AccountNumber].Balance != 800 {
		t.Fatalf("unexpected committed state posting=%#v account=%#v", committed, accounts.accounts[account.AccountNumber])
	}

	again, err := svc.Commit(context.Background(), "posting-commit")
	if err != nil {
		t.Fatalf("idempotent commit: %v", err)
	}
	if again.Status != model.InterbankCashPostingCommitted || accounts.accounts[account.AccountNumber].Balance != 800 {
		t.Fatalf("unexpected second commit state posting=%#v account=%#v", again, accounts.accounts[account.AccountNumber])
	}

	creditAccount := interbankAccount("444000000000000032", 2, model.RSD, 1000, 1000)
	creditPosting := &model.InterbankCashPosting{
		PostingID:             "posting-credit",
		AccountNumber:         creditAccount.AccountNumber,
		CurrencyCode:          model.RSD,
		Amount:                300,
		RequestedCurrencyCode: model.RSD,
		RequestedAmount:       300,
		Status:                model.InterbankCashPostingPrepared,
	}
	accounts.accounts[creditAccount.AccountNumber] = creditAccount
	postings.rows[creditPosting.PostingID] = creditPosting
	credited, err := svc.Commit(context.Background(), "posting-credit")
	if err != nil {
		t.Fatalf("commit credit posting: %v", err)
	}
	if credited.Status != model.InterbankCashPostingCommitted || accounts.accounts[creditAccount.AccountNumber].Balance != 1300 || accounts.accounts[creditAccount.AccountNumber].AvailableBalance != 1300 {
		t.Fatalf("unexpected credited state posting=%#v account=%#v", credited, accounts.accounts[creditAccount.AccountNumber])
	}
}

func TestInterbankCashRollbackAndErrors(t *testing.T) {
	t.Parallel()

	account := interbankAccount("444000000000000041", 1, model.RSD, 1000, 700)
	prepared := &model.InterbankCashPosting{
		PostingID:             "posting-rollback",
		AccountNumber:         account.AccountNumber,
		CurrencyCode:          model.RSD,
		Amount:                -300,
		RequestedCurrencyCode: model.RSD,
		RequestedAmount:       -300,
		Status:                model.InterbankCashPostingPrepared,
	}
	committed := &model.InterbankCashPosting{
		PostingID:             "posting-committed",
		AccountNumber:         account.AccountNumber,
		CurrencyCode:          model.RSD,
		Amount:                -100,
		RequestedCurrencyCode: model.RSD,
		RequestedAmount:       -100,
		Status:                model.InterbankCashPostingCommitted,
	}
	accounts := newFakeInterbankAccountRepo(account)
	postings := newFakeInterbankPostingRepo(prepared, committed)
	svc := NewInterbankCashService(accounts, postings, &fakeBankingTxManager{}, fakeInterbankConverter{}, &recTxRepo{}, &recPayRepo{})

	rolledBack, err := svc.Rollback(context.Background(), "posting-rollback")
	if err != nil {
		t.Fatalf("rollback posting: %v", err)
	}
	if rolledBack.Status != model.InterbankCashPostingRolledBack || accounts.accounts[account.AccountNumber].AvailableBalance != 1000 {
		t.Fatalf("unexpected rollback state posting=%#v account=%#v", rolledBack, accounts.accounts[account.AccountNumber])
	}

	again, err := svc.Rollback(context.Background(), "posting-rollback")
	if err != nil {
		t.Fatalf("idempotent rollback: %v", err)
	}
	if again.Status != model.InterbankCashPostingRolledBack || accounts.accounts[account.AccountNumber].AvailableBalance != 1000 {
		t.Fatalf("unexpected second rollback state posting=%#v account=%#v", again, accounts.accounts[account.AccountNumber])
	}

	if _, err := svc.Rollback(context.Background(), "posting-committed"); err == nil {
		t.Fatal("expected error when rolling back committed posting")
	}
	if _, err := svc.Commit(context.Background(), "missing"); err == nil {
		t.Fatal("expected error for missing posting")
	}
	if _, err := svc.Prepare(context.Background(), "", account.AccountNumber, 0, model.RSD, -1, "CLIENT", PostingMetadata{}); err == nil {
		t.Fatal("expected error for empty posting id")
	}
	if _, err := svc.Prepare(context.Background(), "bad-zero", account.AccountNumber, 0, model.RSD, 0, "CLIENT", PostingMetadata{}); err == nil {
		t.Fatal("expected error for zero amount")
	}
	if _, err := svc.Prepare(context.Background(), "bad-currency", account.AccountNumber, 0, model.CurrencyCode("BAD"), -1, "CLIENT", PostingMetadata{}); err == nil {
		t.Fatal("expected error for unsupported currency")
	}

	converterErr := NewInterbankCashService(accounts, newFakeInterbankPostingRepo(), &fakeBankingTxManager{}, fakeInterbankConverter{err: errors.New("rates unavailable")}, &recTxRepo{}, &recPayRepo{})
	if _, err := converterErr.Prepare(context.Background(), "bad-convert", "", 1, model.USD, -1, "CLIENT", PostingMetadata{}); err == nil {
		t.Fatal("expected converter error")
	}
}
