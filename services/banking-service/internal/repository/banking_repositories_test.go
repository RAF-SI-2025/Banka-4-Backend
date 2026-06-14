package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
)

func setupBankingRepoDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	database, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.AutoMigrate(
		&model.Currency{},
		&model.Account{},
		&model.Transaction{},
		&model.Payment{},
		&model.Transfer{},
		&model.InterbankCashPosting{},
		&model.Card{},
		&model.Company{},
		&model.WorkCode{},
		&model.ExchangeRate{},
		&model.VerificationToken{},
		&model.LoanType{},
		&model.LoanRequest{},
		&model.Loan{},
		&model.LoanInstallment{},
		&model.Payee{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return database
}

func seedRepoCurrency(t *testing.T, database *gorm.DB, code model.CurrencyCode) model.Currency {
	t.Helper()
	currency := model.Currency{Name: string(code), Code: code, Status: "Active"}
	if err := database.Create(&currency).Error; err != nil {
		t.Fatalf("create currency: %v", err)
	}
	return currency
}

func seedRepoAccount(t *testing.T, database *gorm.DB, number string, clientID uint, currency model.Currency, balance float64) model.Account {
	t.Helper()
	account := model.Account{
		AccountNumber:    number,
		Name:             "Main",
		ClientID:         clientID,
		EmployeeID:       1,
		CurrencyID:       currency.CurrencyID,
		Currency:         currency,
		Balance:          balance,
		AvailableBalance: balance,
		Status:           "Active",
		AccountType:      model.AccountTypePersonal,
		AccountKind:      model.AccountKindCurrent,
		Subtype:          model.SubtypeStandard,
		ExpiresAt:        time.Now().Add(24 * time.Hour),
		DailyLimit:       1000,
		MonthlyLimit:     10000,
	}
	if err := database.Create(&account).Error; err != nil {
		t.Fatalf("create account: %v", err)
	}
	return account
}

func TestAccountRepositoryQueriesAndUpdates(t *testing.T) {
	t.Parallel()

	database := setupBankingRepoDB(t)
	repo := NewAccountRepository(database)
	ctx := context.Background()
	rsd := seedRepoCurrency(t, database, model.RSD)
	eur := seedRepoCurrency(t, database, model.EUR)
	account := seedRepoAccount(t, database, "444000000000000011", 7, rsd, 1000)
	other := seedRepoAccount(t, database, "444000000000000022", 8, eur, 2000)

	exists, err := repo.AccountNumberExists(ctx, account.AccountNumber)
	if err != nil {
		t.Fatalf("account exists: %v", err)
	}
	if !exists {
		t.Fatal("expected account to exist")
	}
	byNumber, err := repo.FindByAccountNumber(ctx, account.AccountNumber)
	if err != nil {
		t.Fatalf("find by account number: %v", err)
	}
	if byNumber == nil || byNumber.Currency.Code != model.RSD {
		t.Fatalf("unexpected account by number %#v", byNumber)
	}
	byClient, err := repo.FindByClientID(ctx, 7)
	if err != nil {
		t.Fatalf("find by client: %v", err)
	}
	if len(byClient) != 1 || byClient[0].AccountNumber != account.AccountNumber {
		t.Fatalf("unexpected client accounts %#v", byClient)
	}
	activeByClient, err := repo.FindAllByClientID(ctx, 8)
	if err != nil {
		t.Fatalf("find active by client: %v", err)
	}
	if len(activeByClient) != 1 || activeByClient[0].AccountNumber != other.AccountNumber {
		t.Fatalf("unexpected active client accounts %#v", activeByClient)
	}

	if err := repo.UpdateName(ctx, account.AccountNumber, "Savings"); err != nil {
		t.Fatalf("update name: %v", err)
	}
	if err := repo.UpdateLimits(ctx, account.AccountNumber, 500, 5000); err != nil {
		t.Fatalf("update limits: %v", err)
	}
	byNumber.Balance = 900
	byNumber.AvailableBalance = 850
	byNumber.DailySpending = 25
	if err := repo.UpdateBalance(ctx, byNumber); err != nil {
		t.Fatalf("update balance: %v", err)
	}
	updated, err := repo.GetByAccountNumber(ctx, account.AccountNumber)
	if err != nil {
		t.Fatalf("get updated account: %v", err)
	}
	if updated.Name != "Savings" || updated.DailyLimit != 500 || updated.Balance != 900 || updated.DailySpending != 25 {
		t.Fatalf("unexpected updated account %#v", updated)
	}

	clientID := uint(7)
	rows, total, err := repo.FindAll(ctx, &dto.ListAccountsQuery{ClientID: &clientID, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("find all accounts: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].AccountNumber != account.AccountNumber {
		t.Fatalf("unexpected account list total=%d rows=%#v", total, rows)
	}

	bankAccount, err := repo.FindByAccountType(ctx, model.AccountTypePersonal)
	if err != nil {
		t.Fatalf("find by type: %v", err)
	}
	if bankAccount == nil {
		t.Fatal("expected account by type")
	}
}

func TestPaymentAndTransferRepositories(t *testing.T) {
	t.Parallel()

	database := setupBankingRepoDB(t)
	ctx := context.Background()
	rsd := seedRepoCurrency(t, database, model.RSD)
	payer := seedRepoAccount(t, database, "444000000000000031", 11, rsd, 1000)
	recipient := seedRepoAccount(t, database, "444000000000000032", 12, rsd, 1000)
	tx := model.Transaction{
		PayerAccountNumber:     payer.AccountNumber,
		RecipientAccountNumber: recipient.AccountNumber,
		StartAmount:            150,
		StartCurrencyCode:      model.RSD,
		EndAmount:              150,
		EndCurrencyCode:        model.RSD,
		Status:                 model.TransactionCompleted,
		CreatedAt:              time.Now(),
	}
	if err := database.Create(&tx).Error; err != nil {
		t.Fatalf("create transaction: %v", err)
	}

	paymentRepo := NewPaymentRepository(database)
	payment := &model.Payment{TransactionID: tx.TransactionID, RecipientName: "Recipient", PaymentCode: "289"}
	if err := paymentRepo.Create(ctx, payment); err != nil {
		t.Fatalf("create payment: %v", err)
	}
	foundPayment, err := paymentRepo.GetByID(ctx, payment.PaymentID)
	if err != nil {
		t.Fatalf("get payment: %v", err)
	}
	if foundPayment == nil || foundPayment.Transaction.TransactionID != tx.TransactionID {
		t.Fatalf("unexpected payment %#v", foundPayment)
	}
	foundPayment.FailedAttempts = 2
	if err := paymentRepo.Update(ctx, foundPayment); err != nil {
		t.Fatalf("update payment: %v", err)
	}

	byAccount, total, err := paymentRepo.FindByAccount(ctx, payer.AccountNumber, &dto.PaymentFilters{Page: 1, PageSize: 10, Status: string(model.TransactionCompleted)})
	if err != nil {
		t.Fatalf("find payment by account: %v", err)
	}
	if total != 1 || len(byAccount) != 1 {
		t.Fatalf("unexpected account payments total=%d rows=%#v", total, byAccount)
	}
	byClient, total, err := paymentRepo.FindByClient(ctx, 11, &dto.PaymentFilters{Page: 1, PageSize: 10, MinAmount: 100, MaxAmount: 200})
	if err != nil {
		t.Fatalf("find payment by client: %v", err)
	}
	if total != 1 || len(byClient) != 1 {
		t.Fatalf("unexpected client payments total=%d rows=%#v", total, byClient)
	}

	transferRepo := NewTransferRepository(database)
	rate := 1.0
	transfer := &model.Transfer{TransactionID: tx.TransactionID, ExchangeRate: &rate}
	if err := transferRepo.Create(ctx, transfer); err != nil {
		t.Fatalf("create transfer: %v", err)
	}
	transfers, transferTotal, err := transferRepo.ListByClientID(ctx, 11, 1, 10)
	if err != nil {
		t.Fatalf("list transfers: %v", err)
	}
	if transferTotal != 1 || len(transfers) != 1 || transfers[0].Transaction.PayerAccountNumber != payer.AccountNumber {
		t.Fatalf("unexpected transfers total=%d rows=%#v", transferTotal, transfers)
	}
}

func TestInterbankCashPostingRepositoryCreateFindAndSave(t *testing.T) {
	t.Parallel()

	repo := NewInterbankCashPostingRepository(setupBankingRepoDB(t))
	ctx := context.Background()
	posting := &model.InterbankCashPosting{
		PostingID:             "posting-1",
		AccountNumber:         "444000000000000041",
		CurrencyCode:          model.RSD,
		Amount:                -100,
		RequestedCurrencyCode: model.RSD,
		RequestedAmount:       -100,
		Status:                model.InterbankCashPostingPrepared,
	}
	if err := repo.Create(ctx, posting); err != nil {
		t.Fatalf("create posting: %v", err)
	}
	found, err := repo.FindByID(ctx, "posting-1")
	if err != nil {
		t.Fatalf("find posting: %v", err)
	}
	if found == nil || found.Status != model.InterbankCashPostingPrepared {
		t.Fatalf("unexpected posting %#v", found)
	}
	found.Status = model.InterbankCashPostingCommitted
	if err := repo.Save(ctx, found); err != nil {
		t.Fatalf("save posting: %v", err)
	}
	updated, err := repo.FindByID(ctx, "posting-1")
	if err != nil {
		t.Fatalf("find updated posting: %v", err)
	}
	if updated.Status != model.InterbankCashPostingCommitted {
		t.Fatalf("status = %q, want committed", updated.Status)
	}
	missing, err := repo.FindByID(ctx, "missing")
	if err != nil {
		t.Fatalf("find missing posting: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected missing posting nil, got %#v", missing)
	}
}
