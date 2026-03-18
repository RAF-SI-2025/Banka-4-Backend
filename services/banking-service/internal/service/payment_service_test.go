package service

import (
	"banking-service/internal/dto"
	"banking-service/internal/model"
	"banking-service/internal/repository"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// ── Fake Repo ────────────────────────────────────────────────────────

type fakePaymentRepo struct {
	createErr error
	getErr    error
	payment   *model.Payment
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

type fakeTransactionRepo struct {
	createErr       error
	getErr          error
	updateErr       error
	transaction     *model.Transaction
	returnedTx      *model.Transaction
	returnedTxErr   error
}

func (f *fakeTransactionRepo) Create(_ context.Context, t *model.Transaction) error {
	if f.createErr != nil {
		return f.createErr
	}
	t.TransactionID = 1 // simulate ID assignment
	f.transaction = t
	return nil
}

func (f *fakeTransactionRepo) GetByID(_ context.Context, _ uint) (*model.Transaction, error) {
	// return preset transaction or error
	return f.returnedTx, f.returnedTxErr
}

func (f *fakeTransactionRepo) Update(_ context.Context, _ *model.Transaction) error {
	return f.updateErr
}

// ── Constructor ────────────────────────────────────────────────────────

func newPaymentService(paymentRepo repository.PaymentRepository, transactionRepo repository.TransactionRepository) *PaymentService {
	return &PaymentService{paymentRepo: paymentRepo, transactionRepo: transactionRepo}
}

// ── Tests ──────────────────────────────────────────────────────────────

func TestCreatePayment(t *testing.T) {
	paymentRepo := &fakePaymentRepo{}
	transactionRepo := &fakeTransactionRepo{}
	svc := newPaymentService(paymentRepo, transactionRepo)

	req := dto.CreatePaymentRequest{
		RecipientName:          "John Doe",
		RecipientAccountNumber: "12345678",
		Amount:                 100,
		PayerAccountNumber:     "87654321",
		CurrencyCode:           model.CurrencyCode("RSD"),
	}

	payment, err := svc.CreatePayment(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, "John Doe", payment.RecipientName)
}

func TestCreatePayment_Error(t *testing.T) {
	paymentRepo := &fakePaymentRepo{createErr: errors.New("db error")}
	transactionRepo := &fakeTransactionRepo{}
	svc := newPaymentService(paymentRepo, transactionRepo)

	req := dto.CreatePaymentRequest{
		RecipientName:          "John Doe",
		RecipientAccountNumber: "12345678",
		Amount:                 100,
		PayerAccountNumber:     "87654321",
		CurrencyCode:           model.CurrencyCode("RSD"),
	}

	p, err := svc.CreatePayment(context.Background(), req)
	require.Nil(t, p)
	require.Error(t, err)
	require.Equal(t, "db error", err.Error())
}
