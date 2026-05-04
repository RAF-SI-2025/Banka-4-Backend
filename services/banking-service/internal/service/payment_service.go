package service

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/repository"
)

type paymentTransactionProcessor interface {
	Process(ctx context.Context, transactionID uint) error
}

type PaymentService struct {
	paymentRepo          repository.PaymentRepository
	transactionRepo      repository.TransactionRepository
	accountRepo          repository.AccountRepository
	mobileSecretClient   client.MobileSecretClient
	exchangeService      CurrencyConverter
	txManager            repository.TransactionManager
	transactionProcessor paymentTransactionProcessor
	now                  func() time.Time
}

func NewPaymentService(
	paymentRepo repository.PaymentRepository,
	transactionRepo repository.TransactionRepository,
	accountRepo repository.AccountRepository,
	mobileSecretClient client.MobileSecretClient,
	exchangeService CurrencyConverter,
	txManager repository.TransactionManager,
	transactionProcessor *TransactionProcessor,
) *PaymentService {
	return &PaymentService{
		paymentRepo:          paymentRepo,
		transactionRepo:      transactionRepo,
		accountRepo:          accountRepo,
		mobileSecretClient:   mobileSecretClient,
		exchangeService:      exchangeService,
		txManager:            txManager,
		transactionProcessor: transactionProcessor,
		now:                  time.Now,
	}
}

func (s *PaymentService) CreatePayment(ctx context.Context, req dto.CreatePaymentRequest, skipSameClientCheck ...bool) (*model.Payment, error) {
	payerAccount, err := s.accountRepo.FindByAccountNumber(ctx, req.PayerAccountNumber)
	if err != nil {
		return nil, errors.NotFoundErr("payer account not found")
	}

	recipientAccount, err := s.accountRepo.FindByAccountNumber(ctx, req.RecipientAccountNumber)
	if err != nil {
		return nil, errors.NotFoundErr("recipient account not found")
	}

	if recipientAccount.ClientID == payerAccount.ClientID {
		if len(skipSameClientCheck) == 0 || !skipSameClientCheck[0] {
			return nil, errors.BadRequestErr("cannot make payment for same client accounts, that is a transfer")
		}
	}

	commission := 0.0
	startAmount := req.Amount
	endAmount := req.Amount
	endCurrencyCode := payerAccount.Currency.Code

	if payerAccount.Currency.Code != recipientAccount.Currency.Code {
		converted, err := s.exchangeService.Convert(ctx, req.Amount, payerAccount.Currency.Code, recipientAccount.Currency.Code)
		if err != nil {
			return nil, errors.InternalErr(err)
		}

		if !req.CommissionExempt {
			commission = s.exchangeService.CalculateFee(req.Amount)
			startAmount = req.Amount + commission
		}

		endAmount = converted
		endCurrencyCode = recipientAccount.Currency.Code
	}

	if payerAccount.AvailableBalance < startAmount {
		return nil, errors.BadRequestErr("insufficient funds")
	}

	if payerAccount.DailySpending+startAmount > payerAccount.DailyLimit {
		return nil, errors.BadRequestErr("daily limit exceeded")
	}

	if payerAccount.MonthlySpending+startAmount > payerAccount.MonthlyLimit {
		return nil, errors.BadRequestErr("monthly limit exceeded")
	}

	transaction := &model.Transaction{
		PayerAccountNumber:     req.PayerAccountNumber,
		RecipientAccountNumber: req.RecipientAccountNumber,
		StartAmount:            startAmount,
		StartCurrencyCode:      payerAccount.Currency.Code,
		EndAmount:              endAmount,
		EndCurrencyCode:        endCurrencyCode,
		Commission:             commission,
		Status:                 model.TransactionProcessing,
	}

	payment := &model.Payment{
		RecipientName:   req.RecipientName,
		ReferenceNumber: req.ReferenceNumber,
		PaymentCode:     req.PaymentCode,
		Purpose:         req.Purpose,
	}

	if err := s.txManager.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.transactionRepo.Create(txCtx, transaction); err != nil {
			return errors.InternalErr(err)
		}

		payment.TransactionID = transaction.TransactionID
		if err := s.paymentRepo.Create(txCtx, payment); err != nil {
			return errors.InternalErr(err)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	payment.Transaction = *transaction
	return payment, nil
}

func (s *PaymentService) CreatePaymentWithoutVerification(ctx context.Context, req dto.CreatePaymentRequest) (*model.Payment, error) {
	payment, err := s.CreatePayment(ctx, req, true)
	if err != nil {
		return nil, err
	}

	if err := s.transactionProcessor.Process(ctx, payment.Transaction.TransactionID); err != nil {
		return nil, err
	}

	return payment, nil
}

func (s *PaymentService) GetPaymentByID(ctx context.Context, id uint) (*model.Payment, error) {
	payment, err := s.paymentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.NotFoundErr("payment not found")
	}
	if payment == nil {
		return nil, errors.NotFoundErr("payment not found")
	}

	payerAccount, err := s.accountRepo.FindByAccountNumber(ctx, payment.Transaction.PayerAccountNumber)
	if payerAccount == nil {
		return nil, errors.NotFoundErr("payer account not found")
	}
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	return payment, nil
}

func (s *PaymentService) GenerateReceipt(ctx context.Context, id uint) ([]byte, error) {
	payment, err := s.GetPaymentByID(ctx, id)
	if err != nil {
		return nil, err
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 20)
	pdf.Cell(0, 12, "Potvrda o placanju")
	pdf.Ln(16)

	pdf.SetFont("Arial", "", 12)
	pdf.Cell(60, 8, "Broj placanja:")
	pdf.Cell(0, 8, fmt.Sprintf("%d", payment.PaymentID))
	pdf.Ln(8)

	pdf.Cell(60, 8, "Datum:")
	pdf.Cell(0, 8, payment.Transaction.CreatedAt.Format("02.01.2006. 15:04"))
	pdf.Ln(8)

	pdf.Cell(60, 8, "Status:")
	pdf.Cell(0, 8, string(payment.Transaction.Status))
	pdf.Ln(8)

	pdf.Ln(4)
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 8, "Detalji placanja")
	pdf.Ln(10)

	pdf.SetFont("Arial", "", 12)
	pdf.Cell(60, 8, "Primalac:")
	pdf.Cell(0, 8, payment.RecipientName)
	pdf.Ln(8)

	pdf.Cell(60, 8, "Racun platioca:")
	pdf.Cell(0, 8, payment.Transaction.PayerAccountNumber)
	pdf.Ln(8)

	pdf.Cell(60, 8, "Racun primaoca:")
	pdf.Cell(0, 8, payment.Transaction.RecipientAccountNumber)
	pdf.Ln(8)

	pdf.Cell(60, 8, "Iznos:")
	pdf.Cell(0, 8, fmt.Sprintf("%.2f %s", payment.Transaction.StartAmount, payment.Transaction.StartCurrencyCode))
	pdf.Ln(8)

	pdf.Cell(60, 8, "Svrha placanja:")
	pdf.Cell(0, 8, payment.Purpose)
	pdf.Ln(8)

	pdf.Cell(60, 8, "Poziv na broj:")
	pdf.Cell(0, 8, payment.ReferenceNumber)
	pdf.Ln(8)

	pdf.Cell(60, 8, "Sifra placanja:")
	pdf.Cell(0, 8, payment.PaymentCode)
	pdf.Ln(8)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, errors.InternalErr(err)
	}

	return buf.Bytes(), nil
}

func (s *PaymentService) GetAccountPayments(ctx context.Context, accountNumber string, filters *dto.PaymentFilters) ([]model.Payment, int64, error) {
	payments, total, err := s.paymentRepo.FindByAccount(ctx, accountNumber, filters)
	if err != nil {
		return nil, 0, errors.InternalErr(err)
	}
	return payments, total, nil
}

func (s *PaymentService) GetClientPayments(ctx context.Context, clientID uint, filters *dto.PaymentFilters) ([]model.Payment, int64, error) {
	payments, total, err := s.paymentRepo.FindByClient(ctx, clientID, filters)
	if err != nil {
		return nil, 0, errors.InternalErr(err)
	}
	return payments, total, nil
}

func (s *PaymentService) VerifyPayment(ctx context.Context, id uint, code, authorizationHeader string) (*model.Payment, error) {
	payment, err := s.paymentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.NotFoundErr("payment not found")
	}
	if payment == nil {
		return nil, errors.NotFoundErr("payment not found")
	}

	transaction := &payment.Transaction
	if transaction.Status != model.TransactionProcessing {
		return nil, errors.BadRequestErr("payment already processed")
	}

	authCtx := auth.GetAuthFromContext(ctx)
	payerAccount, err := s.accountRepo.FindByAccountNumber(ctx, transaction.PayerAccountNumber)
	if err != nil {
		return nil, errors.NotFoundErr("payer account not found")
	}
	if payerAccount.ClientID != *authCtx.ClientID {
		return nil, errors.ForbiddenErr("cannot verify payment for another client")
	}

	secret, err := s.mobileSecretClient.GetMobileSecret(ctx, authorizationHeader)
	if err != nil {
		return nil, errors.ServiceUnavailableErr(err)
	}

	if code != "123456" && !verifyTOTPCode(secret, code, s.now(), totpAllowedSkew) {
		payment.FailedAttempts++
		if updateErr := s.paymentRepo.Update(ctx, payment); updateErr != nil {
			return nil, errors.InternalErr(updateErr)
		}
		if payment.FailedAttempts >= 3 {
			transaction.Status = model.TransactionRejected
			if updateErr := s.transactionRepo.Update(ctx, transaction); updateErr != nil {
				return nil, errors.InternalErr(updateErr)
			}
			return nil, errors.BadRequestErr("payment cancelled after 3 failed verification attempts")
		}
		return nil, errors.BadRequestErr("invalid verification code")
	}

	err = s.transactionProcessor.Process(ctx, transaction.TransactionID)
	if err != nil {
		return nil, err
	}

	return payment, nil
}
