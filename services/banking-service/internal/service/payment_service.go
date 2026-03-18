package service

import (
	"banking-service/internal/dto"
	"banking-service/internal/model"
	"banking-service/internal/repository"
	"common/pkg/errors"
	"context"
)

type PaymentService struct {
	paymentRepo     repository.PaymentRepository
	transactionRepo repository.TransactionRepository
}

func NewPaymentService(paymentRepo repository.PaymentRepository, transactionRepo repository.TransactionRepository) *PaymentService {
	return &PaymentService{paymentRepo: paymentRepo, transactionRepo: transactionRepo}
}

func (s *PaymentService) CreatePayment(ctx context.Context, req dto.CreatePaymentRequest) (*model.Payment, error) {

	// TODO: proveriti sredstva (#45)
	// TODO: proveriti limit
	// TODO: proveriti postojanje računa (#45)

	// TODO: currency conversion and provision (#44)
	var endAmount = req.Amount;
	// TODO: get the right end currency code
	var endCurrencyCode = req.CurrencyCode;

	transaction := &model.Transaction{	
		PayerAccountNumber:     req.PayerAccountNumber,
		RecipientAccountNumber: req.RecipientAccountNumber,
		StartAmount:            req.Amount,
		StartCurrencyCode:      req.CurrencyCode,
		EndAmount:         			endAmount,
		EndCurrencyCode:        endCurrencyCode,
		Status:                 model.TransactionProcessing,
	}

	err := s.transactionRepo.Create(ctx, transaction)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	payment := &model.Payment{
		TransactionID:    transaction.TransactionID,
		RecipientName:    req.RecipientName,
		ReferenceNumber:  req.ReferenceNumber,
		PaymentCode:      req.PaymentCode,
		Purpose:          req.Purpose,
	}

	err = s.paymentRepo.Create(ctx, payment)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	return payment, nil
}

func (s *PaymentService) VerifyPayment(ctx context.Context, id uint, code string) (*model.Payment, error) {

	payment, err := s.paymentRepo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	// TODO: mobile verification, update transaction status

	err = s.paymentRepo.Update(ctx, payment)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	return payment, nil
}
