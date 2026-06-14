package repository

import (
	"context"
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
)

func TestCardRepositoryLifecycleAndCounts(t *testing.T) {
	t.Parallel()

	database := setupBankingRepoDB(t)
	repo := NewCardRepository(database)
	ctx := context.Background()
	authorizedPersonID := uint(55)
	accountNumber := "444000000000000051"
	card := &model.Card{
		CardNumber:         "4111111111111111",
		CardType:           model.CardTypeDebit,
		CardBrand:          model.CardBrandVisa,
		Name:               "Primary",
		AccountNumber:      accountNumber,
		CVV:                "123",
		Limit:              1000,
		Status:             model.CardStatusActive,
		AuthorizedPersonID: &authorizedPersonID,
		ExpiresAt:          time.Now().AddDate(3, 0, 0),
	}
	if err := repo.Create(ctx, card); err != nil {
		t.Fatalf("create card: %v", err)
	}
	if err := repo.Create(ctx, &model.Card{
		CardNumber:    "5555555555554444",
		CardType:      model.CardTypeDebit,
		CardBrand:     model.CardBrandMasterCard,
		Name:          "Inactive",
		AccountNumber: accountNumber,
		CVV:           "456",
		Limit:         500,
		Status:        model.CardStatusDeactivated,
		ExpiresAt:     time.Now().AddDate(2, 0, 0),
	}); err != nil {
		t.Fatalf("create deactivated card: %v", err)
	}

	found, err := repo.FindByID(ctx, card.CardID)
	if err != nil {
		t.Fatalf("find card: %v", err)
	}
	if found == nil || found.CardNumber != card.CardNumber {
		t.Fatalf("unexpected card %#v", found)
	}

	list, err := repo.ListByAccountNumber(ctx, accountNumber)
	if err != nil {
		t.Fatalf("list cards: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len(list) = %d, want 2", len(list))
	}

	total, err := repo.CountByAccountNumber(ctx, accountNumber)
	if err != nil {
		t.Fatalf("count by account: %v", err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	total, err = repo.CountByAccountNumberAndAuthorizedPersonID(ctx, accountNumber, &authorizedPersonID)
	if err != nil {
		t.Fatalf("count by authorized person: %v", err)
	}
	if total != 1 {
		t.Fatalf("authorized total = %d, want 1", total)
	}
	total, err = repo.CountNonDeactivatedByAccountNumber(ctx, accountNumber)
	if err != nil {
		t.Fatalf("count non-deactivated: %v", err)
	}
	if total != 1 {
		t.Fatalf("non-deactivated total = %d, want 1", total)
	}
	total, err = repo.CountNonDeactivatedByAccountNumberAndAuthorizedPersonID(ctx, accountNumber, nil)
	if err != nil {
		t.Fatalf("count non-deactivated without authorized person: %v", err)
	}
	if total != 0 {
		t.Fatalf("nil authorized non-deactivated total = %d, want 0", total)
	}

	exists, err := repo.CardNumberExists(ctx, card.CardNumber)
	if err != nil {
		t.Fatalf("card number exists: %v", err)
	}
	if !exists {
		t.Fatal("expected card number to exist")
	}

	found.Status = model.CardStatusBlocked
	if err := repo.Update(ctx, found); err != nil {
		t.Fatalf("update card: %v", err)
	}
	updated, err := repo.FindByID(ctx, card.CardID)
	if err != nil {
		t.Fatalf("find updated card: %v", err)
	}
	if updated.Status != model.CardStatusBlocked {
		t.Fatalf("status = %q, want blocked", updated.Status)
	}

	missing, err := repo.FindByID(ctx, 9999)
	if err != nil {
		t.Fatalf("find missing card: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected missing card nil, got %#v", missing)
	}
}

func TestCompanyCurrencyExchangeAndVerificationRepositories(t *testing.T) {
	t.Parallel()

	database := setupBankingRepoDB(t)
	ctx := context.Background()

	companyRepo := NewCompanyRepository(database)
	workCode := model.WorkCode{Code: "62.01", Description: "Software"}
	if err := database.Create(&workCode).Error; err != nil {
		t.Fatalf("create work code: %v", err)
	}
	company := &model.Company{Name: "Acme", RegistrationNumber: "12345678", TaxNumber: "123456789", WorkCodeID: workCode.WorkCodeID, Address: "Main", OwnerID: 9}
	if err := companyRepo.Create(ctx, company); err != nil {
		t.Fatalf("create company: %v", err)
	}
	companies, err := companyRepo.GetCompanies(ctx)
	if err != nil {
		t.Fatalf("get companies: %v", err)
	}
	if len(companies) != 1 || companies[0].Name != "Acme" {
		t.Fatalf("unexpected companies %#v", companies)
	}
	workCodes, err := companyRepo.GetWorkCodes(ctx)
	if err != nil {
		t.Fatalf("get work codes: %v", err)
	}
	if len(workCodes) != 1 || workCodes[0].Code != "62.01" {
		t.Fatalf("unexpected work codes %#v", workCodes)
	}
	workCodeExists, err := companyRepo.WorkCodeExists(ctx, workCode.WorkCodeID)
	if err != nil {
		t.Fatalf("work code exists: %v", err)
	}
	regExists, err := companyRepo.RegistrationNumberExists(ctx, "12345678")
	if err != nil {
		t.Fatalf("registration exists: %v", err)
	}
	taxExists, err := companyRepo.TaxNumberExists(ctx, "123456789")
	if err != nil {
		t.Fatalf("tax exists: %v", err)
	}
	if !workCodeExists || !regExists || !taxExists {
		t.Fatalf("expected company lookup booleans true, got work=%t reg=%t tax=%t", workCodeExists, regExists, taxExists)
	}

	currencyRepo := NewCurrencyRepository(database)
	rsd := seedRepoCurrency(t, database, model.RSD)
	currency, err := currencyRepo.FindByCode(ctx, model.RSD)
	if err != nil {
		t.Fatalf("find currency: %v", err)
	}
	if currency.CurrencyID != rsd.CurrencyID {
		t.Fatalf("currency id = %d, want %d", currency.CurrencyID, rsd.CurrencyID)
	}

	exchangeRepo := NewExchangeRateRepository(database)
	now := time.Now().UTC()
	if err := exchangeRepo.UpsertAll(ctx, []model.ExchangeRate{{
		CurrencyCode:         model.EUR,
		BaseCurrency:         model.RSD,
		BuyRate:              117,
		MiddleRate:           118,
		SellRate:             119,
		ProviderUpdatedAt:    now,
		ProviderNextUpdateAt: now.Add(time.Hour),
	}, {
		CurrencyCode:         model.USD,
		BaseCurrency:         model.RSD,
		BuyRate:              105,
		MiddleRate:           106,
		SellRate:             107,
		ProviderUpdatedAt:    now,
		ProviderNextUpdateAt: now.Add(time.Hour),
	}}); err != nil {
		t.Fatalf("upsert exchange rates: %v", err)
	}
	if err := exchangeRepo.UpsertAll(ctx, []model.ExchangeRate{{
		CurrencyCode:         model.EUR,
		BaseCurrency:         model.RSD,
		BuyRate:              120,
		MiddleRate:           121,
		SellRate:             122,
		ProviderUpdatedAt:    now,
		ProviderNextUpdateAt: now.Add(time.Hour),
	}}); err != nil {
		t.Fatalf("upsert replacement exchange rate: %v", err)
	}
	rates, err := exchangeRepo.GetAll(ctx)
	if err != nil {
		t.Fatalf("get exchange rates: %v", err)
	}
	if len(rates) != 2 {
		t.Fatalf("len(rates) = %d, want 2", len(rates))
	}

	tokenRepo := NewVerificationTokenRepository(database)
	token := &model.VerificationToken{ClientID: 44, AccountNumber: "444000000000000061", NewDailyLimit: 2000, NewMonthlyLimit: 20000}
	if err := tokenRepo.Create(ctx, token); err != nil {
		t.Fatalf("create token: %v", err)
	}
	foundToken, err := tokenRepo.FindByAccountAndClient(ctx, token.AccountNumber, token.ClientID)
	if err != nil {
		t.Fatalf("find token: %v", err)
	}
	if foundToken == nil || foundToken.NewDailyLimit != 2000 {
		t.Fatalf("unexpected token %#v", foundToken)
	}
	if err := tokenRepo.DeleteByAccountAndClient(ctx, token.AccountNumber, token.ClientID); err != nil {
		t.Fatalf("delete token: %v", err)
	}
	missingToken, err := tokenRepo.FindByAccountAndClient(ctx, token.AccountNumber, token.ClientID)
	if err != nil {
		t.Fatalf("find deleted token: %v", err)
	}
	if missingToken != nil {
		t.Fatalf("expected missing token nil, got %#v", missingToken)
	}
}

func TestLoanRepositoriesLifecycleAndQueries(t *testing.T) {
	t.Parallel()

	database := setupBankingRepoDB(t)
	ctx := context.Background()
	loanTypeRepo := NewLoanTypeRepository(database)
	requestRepo := NewLoanRequestRepository(database)
	loanRepo := NewLoanRepository(database)
	now := time.Now().UTC()

	loanType := model.LoanType{Name: "Cash", BankMargin: 2, BaseInterestRate: 4, MinRepaymentPeriod: 6, MaxRepaymentPeriod: 72}
	if err := database.Create(&loanType).Error; err != nil {
		t.Fatalf("create loan type: %v", err)
	}
	foundType, err := loanTypeRepo.FindByID(ctx, loanType.LoanTypeID)
	if err != nil {
		t.Fatalf("find loan type: %v", err)
	}
	if foundType == nil || foundType.Name != "Cash" {
		t.Fatalf("unexpected loan type %#v", foundType)
	}

	request := &model.LoanRequest{
		ClientID:           77,
		AccountNumber:      "444000000000000071",
		LoanTypeID:         loanType.LoanTypeID,
		Amount:             10000,
		RepaymentPeriod:    12,
		CalculatedRate:     6,
		MonthlyInstallment: 860,
		Status:             model.LoanRequestPending,
		CreatedAt:          now,
	}
	if err := requestRepo.CreateRequest(ctx, request); err != nil {
		t.Fatalf("create loan request: %v", err)
	}
	requests, total, err := requestRepo.FindAll(ctx, &dto.ListLoanRequestsQuery{ClientID: 77, Status: model.LoanRequestPending, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("find all loan requests: %v", err)
	}
	if total != 1 || len(requests) != 1 || requests[0].LoanType.Name != "Cash" {
		t.Fatalf("unexpected loan requests total=%d rows=%#v", total, requests)
	}
	foundRequest, err := requestRepo.FindByID(ctx, request.ID)
	if err != nil {
		t.Fatalf("find loan request: %v", err)
	}
	if foundRequest == nil || foundRequest.Amount != 10000 {
		t.Fatalf("unexpected loan request %#v", foundRequest)
	}
	request.Status = model.LoanRequestApproved
	if err := requestRepo.Update(ctx, request); err != nil {
		t.Fatalf("update loan request: %v", err)
	}

	loan := &model.Loan{
		LoanRequestID:       request.ID,
		MonthlyInstallment:  860,
		InterestRate:        6,
		IsVariableRate:      true,
		RemainingDebt:       10000,
		RepaymentPeriod:     12,
		StartDate:           now,
		NextInstallmentDate: now.AddDate(0, 1, 0),
		Status:              model.LoanStatusActive,
	}
	if err := loanRepo.CreateLoan(ctx, loan); err != nil {
		t.Fatalf("create loan: %v", err)
	}
	byClient, err := loanRepo.FindByClientID(ctx, 77, true)
	if err != nil {
		t.Fatalf("find loans by client: %v", err)
	}
	if len(byClient) != 1 || byClient[0].LoanRequest.ClientID != 77 {
		t.Fatalf("unexpected client loans %#v", byClient)
	}
	byID, err := loanRepo.FindByIDAndClientID(ctx, loan.ID, 77)
	if err != nil {
		t.Fatalf("find loan by id and client: %v", err)
	}
	if byID == nil || byID.ID != loan.ID {
		t.Fatalf("unexpected loan by id %#v", byID)
	}
	active, err := loanRepo.HasActiveByClientID(ctx, 77)
	if err != nil {
		t.Fatalf("has active loan: %v", err)
	}
	if !active {
		t.Fatal("expected active loan")
	}
	byRequest, err := loanRepo.FindLoanByRequestID(ctx, request.ID)
	if err != nil {
		t.Fatalf("find loan by request: %v", err)
	}
	if byRequest == nil || byRequest.ID != loan.ID {
		t.Fatalf("unexpected loan by request %#v", byRequest)
	}

	loan.RemainingDebt = 9500
	if err := loanRepo.UpdateLoan(ctx, loan); err != nil {
		t.Fatalf("update loan: %v", err)
	}

	retryAt := now.Add(-time.Hour)
	installments := []model.LoanInstallment{
		{LoanID: loan.ID, InstallmentNumber: 1, Amount: 860, InterestRate: 6, DueDate: now.Add(-24 * time.Hour), Status: model.InstallmentStatusPending},
		{LoanID: loan.ID, InstallmentNumber: 2, Amount: 860, InterestRate: 6, DueDate: now, RetryAt: &retryAt, Status: model.InstallmentStatusRetrying},
	}
	if err := loanRepo.CreateInstallments(ctx, installments); err != nil {
		t.Fatalf("create installments: %v", err)
	}
	due, err := loanRepo.FindDueInstallments(ctx, now)
	if err != nil {
		t.Fatalf("find due installments: %v", err)
	}
	if len(due) != 1 || due[0].InstallmentNumber != 1 {
		t.Fatalf("unexpected due installments %#v", due)
	}
	retry, err := loanRepo.FindRetryInstallments(ctx, now)
	if err != nil {
		t.Fatalf("find retry installments: %v", err)
	}
	if len(retry) != 1 || retry[0].InstallmentNumber != 2 {
		t.Fatalf("unexpected retry installments %#v", retry)
	}
	retry[0].Status = model.InstallmentStatusPaid
	if err := loanRepo.UpdateInstallment(ctx, &retry[0]); err != nil {
		t.Fatalf("update installment: %v", err)
	}
	variableLoans, err := loanRepo.FindActiveVariableRateLoans(ctx)
	if err != nil {
		t.Fatalf("find active variable loans: %v", err)
	}
	if len(variableLoans) != 1 || variableLoans[0].ID != loan.ID {
		t.Fatalf("unexpected variable loans %#v", variableLoans)
	}
}

func TestPayeeAndTransactionRepositories(t *testing.T) {
	t.Parallel()

	database := setupBankingRepoDB(t)
	ctx := context.Background()

	payeeRepo := NewPayeeRepository(database)
	payee := &model.Payee{ClientID: 88, Name: "Supplier", AccountNumber: "444000000000000081"}
	if err := payeeRepo.Create(ctx, payee); err != nil {
		t.Fatalf("create payee: %v", err)
	}
	payees, err := payeeRepo.FindAllByClientID(ctx, 88)
	if err != nil {
		t.Fatalf("find payees: %v", err)
	}
	if len(payees) != 1 || payees[0].Name != "Supplier" {
		t.Fatalf("unexpected payees %#v", payees)
	}
	foundPayee, err := payeeRepo.FindByID(ctx, payee.PayeeID)
	if err != nil {
		t.Fatalf("find payee: %v", err)
	}
	if foundPayee == nil || foundPayee.AccountNumber != payee.AccountNumber {
		t.Fatalf("unexpected payee %#v", foundPayee)
	}
	foundPayee.Name = "Updated"
	if err := payeeRepo.Update(ctx, foundPayee); err != nil {
		t.Fatalf("update payee: %v", err)
	}
	if err := payeeRepo.Delete(ctx, payee.PayeeID); err != nil {
		t.Fatalf("delete payee: %v", err)
	}
	missingPayee, err := payeeRepo.FindByID(ctx, payee.PayeeID)
	if err != nil {
		t.Fatalf("find deleted payee: %v", err)
	}
	if missingPayee != nil {
		t.Fatalf("expected deleted payee nil, got %#v", missingPayee)
	}

	txRepo := NewTransactionRepository(database)
	tx := &model.Transaction{
		PayerAccountNumber:     "444000000000000091",
		RecipientAccountNumber: "444000000000000092",
		StartAmount:            300,
		StartCurrencyCode:      model.RSD,
		EndAmount:              300,
		EndCurrencyCode:        model.RSD,
		Status:                 model.TransactionProcessing,
		CreatedAt:              time.Now().UTC(),
	}
	if err := txRepo.Create(ctx, tx); err != nil {
		t.Fatalf("create transaction: %v", err)
	}
	foundTx, err := txRepo.GetByID(ctx, tx.TransactionID)
	if err != nil {
		t.Fatalf("get transaction: %v", err)
	}
	if foundTx == nil || foundTx.StartAmount != 300 {
		t.Fatalf("unexpected transaction %#v", foundTx)
	}
	byPayer, err := txRepo.GetByPayerAccountNumber(ctx, tx.PayerAccountNumber)
	if err != nil {
		t.Fatalf("get by payer: %v", err)
	}
	byRecipient, err := txRepo.GetByRecipientAccountNumber(ctx, tx.RecipientAccountNumber)
	if err != nil {
		t.Fatalf("get by recipient: %v", err)
	}
	if len(byPayer) != 1 || len(byRecipient) != 1 {
		t.Fatalf("unexpected transaction lists payer=%#v recipient=%#v", byPayer, byRecipient)
	}
	tx.Status = model.TransactionCompleted
	tx.EndAmount = 310
	if err := txRepo.Update(ctx, tx); err != nil {
		t.Fatalf("update transaction: %v", err)
	}
	updatedTx, err := txRepo.GetByID(ctx, tx.TransactionID)
	if err != nil {
		t.Fatalf("get updated transaction: %v", err)
	}
	if updatedTx.Status != model.TransactionCompleted || updatedTx.EndAmount != 310 {
		t.Fatalf("unexpected updated transaction %#v", updatedTx)
	}
}
