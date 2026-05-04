package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
)

// ── helpers ────────────────────────────────────────────────────────────────────

func newPaymentSvcWithProcessor(
	paymentRepo *fakePaymentRepo,
	transactionRepo *fakeTransactionRepo,
	accountRepo *fakePaymentAccountRepo,
	exchangeSvc CurrencyConverter,
	processor paymentTransactionProcessor,
) *PaymentService {
	svc := newTestPaymentService(paymentRepo, transactionRepo, accountRepo, exchangeSvc)
	svc.transactionProcessor = processor
	return svc
}

func richPayerAccount(number string, clientID uint) *model.Account {
	return &model.Account{
		AccountNumber:    number,
		ClientID:         clientID,
		Balance:          100_000,
		AvailableBalance: 100_000,
		DailyLimit:       500_000,
		MonthlyLimit:     2_000_000,
		DailySpending:    0,
		MonthlySpending:  0,
		Currency:         model.Currency{Code: model.RSD},
	}
}

func basicRecipientAccount(number string, clientID uint) *model.Account {
	return &model.Account{
		AccountNumber: number,
		ClientID:      clientID,
		Currency:      model.Currency{Code: model.RSD},
	}
}

// ── CreatePaymentWithoutVerification ─────────────────────────────────────────

func TestCreatePaymentWithoutVerification_Success(t *testing.T) {
	payer := richPayerAccount("PAYER-1", 1)
	recipient := basicRecipientAccount("RECIP-1", 2)

	processor := &fakeVerifyTransactionProcessor{}
	svc := newPaymentSvcWithProcessor(
		&fakePaymentRepo{},
		&fakeTransactionRepo{},
		newFakePaymentAccountRepo(payer, recipient),
		&fakeExchangeService{rate: 1.0},
		processor,
	)

	req := dto.CreatePaymentRequest{
		RecipientName:          "Recipient",
		RecipientAccountNumber: "RECIP-1",
		Amount:                 500,
		PayerAccountNumber:     "PAYER-1",
	}

	payment, err := svc.CreatePaymentWithoutVerification(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, payment)
	require.Len(t, processor.processed, 1)
}

func TestCreatePaymentWithoutVerification_CreatePaymentFails(t *testing.T) {
	// No payer account => CreatePayment will fail
	processor := &fakeVerifyTransactionProcessor{}
	svc := newPaymentSvcWithProcessor(
		&fakePaymentRepo{},
		&fakeTransactionRepo{},
		newFakePaymentAccountRepo(), // empty
		&fakeExchangeService{rate: 1.0},
		processor,
	)

	req := dto.CreatePaymentRequest{
		RecipientName:          "R",
		RecipientAccountNumber: "RECIP-1",
		Amount:                 100,
		PayerAccountNumber:     "PAYER-1",
	}

	_, err := svc.CreatePaymentWithoutVerification(context.Background(), req)
	require.Error(t, err)
	require.Empty(t, processor.processed)
}

func TestCreatePaymentWithoutVerification_ProcessorFails(t *testing.T) {
	payer := richPayerAccount("PAYER-1", 1)
	recipient := basicRecipientAccount("RECIP-1", 2)

	processor := &fakeVerifyTransactionProcessor{err: errors.New("processor error")}
	svc := newPaymentSvcWithProcessor(
		&fakePaymentRepo{},
		&fakeTransactionRepo{},
		newFakePaymentAccountRepo(payer, recipient),
		&fakeExchangeService{rate: 1.0},
		processor,
	)

	req := dto.CreatePaymentRequest{
		RecipientName:          "Recipient",
		RecipientAccountNumber: "RECIP-1",
		Amount:                 500,
		PayerAccountNumber:     "PAYER-1",
	}

	_, err := svc.CreatePaymentWithoutVerification(context.Background(), req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "processor error")
}

// ── GenerateReceipt ───────────────────────────────────────────────────────────

func TestGenerateReceipt_Success(t *testing.T) {
	payer := richPayerAccount("PAYER-1", 1)
	paymentRepo := &fakePaymentRepo{
		payment: &model.Payment{
			PaymentID:       1,
			RecipientName:   "Ana Jovanovic",
			ReferenceNumber: "REF-001",
			PaymentCode:     "289",
			Purpose:         "Invoice payment",
			Transaction: model.Transaction{
				TransactionID:          1,
				PayerAccountNumber:     "PAYER-1",
				RecipientAccountNumber: "RECIP-1",
				StartAmount:            1234.56,
				StartCurrencyCode:      model.RSD,
				Status:                 model.TransactionCompleted,
			},
		},
	}

	svc := newTestPaymentService(paymentRepo, &fakeTransactionRepo{}, newFakePaymentAccountRepo(payer), &fakeExchangeService{})

	pdfBytes, err := svc.GenerateReceipt(context.Background(), 1)
	require.NoError(t, err)
	require.NotEmpty(t, pdfBytes)
	// PDF files start with %PDF
	require.True(t, len(pdfBytes) > 4)
	require.Equal(t, "%PDF", string(pdfBytes[:4]))
}

func TestGenerateReceipt_PaymentNotFound(t *testing.T) {
	svc := newTestPaymentService(
		&fakePaymentRepo{getErr: errors.New("not found")},
		&fakeTransactionRepo{},
		newFakePaymentAccountRepo(),
		&fakeExchangeService{},
	)

	_, err := svc.GenerateReceipt(context.Background(), 99)
	require.Error(t, err)
	require.Contains(t, err.Error(), "payment not found")
}

func TestGenerateReceipt_NilPayment(t *testing.T) {
	svc := newTestPaymentService(
		&fakePaymentRepo{}, // GetByID returns nil payment
		&fakeTransactionRepo{},
		newFakePaymentAccountRepo(),
		&fakeExchangeService{},
	)

	_, err := svc.GenerateReceipt(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "payment not found")
}

// ── GetClientPayments ──────────────────────────────────────────────────────────

func TestGetClientPayments_Success(t *testing.T) {
	paymentRepo := &fakePaymentRepo{
		allPayments: []model.Payment{
			{PaymentID: 1, RecipientName: "Ana"},
			{PaymentID: 2, RecipientName: "Stefan"},
		},
	}
	svc := newTestPaymentService(paymentRepo, &fakeTransactionRepo{}, newFakePaymentAccountRepo(), &fakeExchangeService{})

	payments, total, err := svc.GetClientPayments(context.Background(), 1, &dto.PaymentFilters{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, payments, 2)
	require.Equal(t, int64(2), total)
}

func TestGetClientPayments_Empty(t *testing.T) {
	svc := newTestPaymentService(
		&fakePaymentRepo{allPayments: []model.Payment{}},
		&fakeTransactionRepo{},
		newFakePaymentAccountRepo(),
		&fakeExchangeService{},
	)

	payments, total, err := svc.GetClientPayments(context.Background(), 1, &dto.PaymentFilters{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Empty(t, payments)
	require.Equal(t, int64(0), total)
}

func TestGetClientPayments_RepoError(t *testing.T) {
	paymentRepo := &fakePaymentRepo{findAllErr: errors.New("db failure")}
	svc := newTestPaymentService(paymentRepo, &fakeTransactionRepo{}, newFakePaymentAccountRepo(), &fakeExchangeService{})

	_, _, err := svc.GetClientPayments(context.Background(), 1, &dto.PaymentFilters{Page: 1, PageSize: 10})
	require.Error(t, err)
	require.Contains(t, err.Error(), "db failure")
}

// ── VerifyPayment edge cases ───────────────────────────────────────────────────

func TestVerifyPayment_PaymentAlreadyProcessed(t *testing.T) {
	paymentRepo := &fakePaymentRepo{
		payment: &model.Payment{
			PaymentID: 1,
			Transaction: model.Transaction{
				TransactionID:      1,
				PayerAccountNumber: "PAYER-1",
				Status:             model.TransactionCompleted, // already processed
			},
		},
	}
	clientID := uint(1)
	ctx := auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityType: auth.IdentityClient,
		ClientID:     &clientID,
	})
	svc := newTestPaymentService(paymentRepo, &fakeTransactionRepo{},
		newFakePaymentAccountRepo(&model.Account{AccountNumber: "PAYER-1", ClientID: 1}),
		&fakeExchangeService{},
	)

	_, err := svc.VerifyPayment(ctx, 1, "123456", "Bearer token")
	require.Error(t, err)
	require.Contains(t, err.Error(), "payment already processed")
}

func TestVerifyPayment_PaymentNotFound(t *testing.T) {
	paymentRepo := &fakePaymentRepo{getErr: errors.New("not found")}
	svc := newTestPaymentService(paymentRepo, &fakeTransactionRepo{}, newFakePaymentAccountRepo(), &fakeExchangeService{})

	clientID := uint(1)
	ctx := auth.SetAuthOnContext(context.Background(), &auth.AuthContext{ClientID: &clientID})

	_, err := svc.VerifyPayment(ctx, 1, "123456", "Bearer token")
	require.Error(t, err)
	require.Contains(t, err.Error(), "payment not found")
}

func TestVerifyPayment_NilPayment(t *testing.T) {
	paymentRepo := &fakePaymentRepo{} // GetByID returns nil
	svc := newTestPaymentService(paymentRepo, &fakeTransactionRepo{}, newFakePaymentAccountRepo(), &fakeExchangeService{})

	clientID := uint(1)
	ctx := auth.SetAuthOnContext(context.Background(), &auth.AuthContext{ClientID: &clientID})

	_, err := svc.VerifyPayment(ctx, 1, "123456", "Bearer token")
	require.Error(t, err)
	require.Contains(t, err.Error(), "payment not found")
}

func TestVerifyPayment_WrongClient(t *testing.T) {
	paymentRepo := &fakePaymentRepo{
		payment: &model.Payment{
			PaymentID: 1,
			Transaction: model.Transaction{
				TransactionID:      1,
				PayerAccountNumber: "PAYER-1",
				Status:             model.TransactionProcessing,
			},
		},
	}
	// Account belongs to client 2, but JWT says client 1
	clientID := uint(1)
	ctx := auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityType: auth.IdentityClient,
		ClientID:     &clientID,
	})
	svc := newTestPaymentService(paymentRepo, &fakeTransactionRepo{},
		newFakePaymentAccountRepo(&model.Account{AccountNumber: "PAYER-1", ClientID: 2}),
		&fakeExchangeService{},
	)

	_, err := svc.VerifyPayment(ctx, 1, "123456", "Bearer token")
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot verify payment for another client")
}

func TestVerifyPayment_MobileSecretError(t *testing.T) {
	paymentRepo := &fakePaymentRepo{
		payment: &model.Payment{
			PaymentID: 1,
			Transaction: model.Transaction{
				TransactionID:      1,
				PayerAccountNumber: "PAYER-1",
				Status:             model.TransactionProcessing,
			},
		},
	}
	clientID := uint(1)
	ctx := auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityType: auth.IdentityClient,
		ClientID:     &clientID,
	})
	svc := &PaymentService{
		paymentRepo:          paymentRepo,
		accountRepo:          newFakePaymentAccountRepo(&model.Account{AccountNumber: "PAYER-1", ClientID: 1}),
		mobileSecretClient:   &fakeMobileSecretClient{err: errors.New("secret service down")},
		txManager:            &fakeBankingTxManager{},
		transactionProcessor: &fakeVerifyTransactionProcessor{},
	}

	_, err := svc.VerifyPayment(ctx, 1, "123456", "Bearer token")
	require.Error(t, err)
	require.Contains(t, err.Error(), "secret service down")
}

func TestVerifyPayment_MagicCode123456Success(t *testing.T) {
	// code "123456" is always accepted (bypass)
	processor := &fakeVerifyTransactionProcessor{}
	paymentRepo := &fakePaymentRepo{
		payment: &model.Payment{
			PaymentID: 1,
			Transaction: model.Transaction{
				TransactionID:      42,
				PayerAccountNumber: "PAYER-1",
				Status:             model.TransactionProcessing,
			},
		},
	}
	clientID := uint(5)
	ctx := auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityType: auth.IdentityClient,
		ClientID:     &clientID,
	})
	svc := &PaymentService{
		paymentRepo:          paymentRepo,
		accountRepo:          newFakePaymentAccountRepo(&model.Account{AccountNumber: "PAYER-1", ClientID: 5}),
		mobileSecretClient:   &fakeMobileSecretClient{secret: "JBSWY3DPEHPK3PXP"},
		transactionProcessor: processor,
		txManager:            &fakeBankingTxManager{},
	}

	payment, err := svc.VerifyPayment(ctx, 1, "123456", "Bearer token")
	require.NoError(t, err)
	require.NotNil(t, payment)
	require.Len(t, processor.processed, 1)
	require.Equal(t, uint(42), processor.processed[0])
}

// ── CreatePayment currency conversion ─────────────────────────────────────────

func TestCreatePayment_SameClientAccounts(t *testing.T) {
	payer := richPayerAccount("PAYER-1", 1)
	recipient := basicRecipientAccount("RECIP-1", 1) // same client

	svc := newTestPaymentService(
		&fakePaymentRepo{},
		&fakeTransactionRepo{},
		newFakePaymentAccountRepo(payer, recipient),
		&fakeExchangeService{rate: 1.0},
	)

	req := dto.CreatePaymentRequest{
		RecipientName:          "Self",
		RecipientAccountNumber: "RECIP-1",
		Amount:                 100,
		PayerAccountNumber:     "PAYER-1",
	}

	_, err := svc.CreatePayment(context.Background(), req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot make payment for same client accounts")
}

func TestCreatePayment_CurrencyConversionError(t *testing.T) {
	payer := &model.Account{
		AccountNumber:    "PAYER-1",
		ClientID:         1,
		Balance:          100_000,
		AvailableBalance: 100_000,
		DailyLimit:       500_000,
		MonthlyLimit:     2_000_000,
		Currency:         model.Currency{Code: model.RSD},
	}
	recipient := &model.Account{
		AccountNumber: "RECIP-1",
		ClientID:      2,
		Currency:      model.Currency{Code: model.EUR},
	}

	svc := newTestPaymentService(
		&fakePaymentRepo{},
		&fakeTransactionRepo{},
		newFakePaymentAccountRepo(payer, recipient),
		&fakeExchangeService{err: errors.New("exchange unavailable")},
	)

	req := dto.CreatePaymentRequest{
		RecipientName:          "R",
		RecipientAccountNumber: "RECIP-1",
		Amount:                 1000,
		PayerAccountNumber:     "PAYER-1",
	}

	_, err := svc.CreatePayment(context.Background(), req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "exchange unavailable")
}

func TestCreatePayment_CrossCurrencySuccess(t *testing.T) {
	payer := &model.Account{
		AccountNumber:    "PAYER-1",
		ClientID:         1,
		Balance:          100_000,
		AvailableBalance: 100_000,
		DailyLimit:       500_000,
		MonthlyLimit:     2_000_000,
		Currency:         model.Currency{Code: model.RSD},
	}
	recipient := &model.Account{
		AccountNumber: "RECIP-1",
		ClientID:      2,
		Currency:      model.Currency{Code: model.EUR},
	}

	svc := newTestPaymentService(
		&fakePaymentRepo{},
		&fakeTransactionRepo{},
		newFakePaymentAccountRepo(payer, recipient),
		&fakeExchangeService{rate: 0.0085}, // 1000 RSD * 0.0085 = 8.5 EUR
	)

	req := dto.CreatePaymentRequest{
		RecipientName:          "R",
		RecipientAccountNumber: "RECIP-1",
		Amount:                 1000,
		PayerAccountNumber:     "PAYER-1",
	}

	payment, err := svc.CreatePayment(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, payment)
	// commission applied
	require.Greater(t, payment.Transaction.StartAmount, 1000.0)
	require.Equal(t, model.EUR, payment.Transaction.EndCurrencyCode)
}

func TestCreatePayment_PaymentRepoError(t *testing.T) {
	payer := richPayerAccount("PAYER-1", 1)
	recipient := basicRecipientAccount("RECIP-1", 2)

	svc := newTestPaymentService(
		&fakePaymentRepo{createErr: errors.New("payment repo error")},
		&fakeTransactionRepo{},
		newFakePaymentAccountRepo(payer, recipient),
		&fakeExchangeService{rate: 1.0},
	)

	req := dto.CreatePaymentRequest{
		RecipientName:          "R",
		RecipientAccountNumber: "RECIP-1",
		Amount:                 100,
		PayerAccountNumber:     "PAYER-1",
	}

	_, err := svc.CreatePayment(context.Background(), req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "payment repo error")
}
