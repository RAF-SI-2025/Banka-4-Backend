package service

import (
	"banking-service/internal/model"
	"banking-service/internal/repository"
	"common/pkg/errors"
	"context"
)

var BankAccounts = map[model.CurrencyCode]string{
	model.RSD: "444000000000000000",
	model.EUR: "444000000000000001",
	model.USD: "444000000000000002",
	model.CHF: "444000000000000003",
	model.GBP: "444000000000000004",
	model.JPY: "444000000000000005",
	model.CAD: "444000000000000006",
	model.AUD: "444000000000000007",
}

type TransactionProcessor struct {
	accountRepo     repository.AccountRepository
	transactionRepo repository.TransactionRepository
	txManager       repository.TransactionManager
}

func NewTransactionProcessor(accountRepo repository.AccountRepository, transactionRepo repository.TransactionRepository, txManager repository.TransactionManager) *TransactionProcessor {
	return &TransactionProcessor{accountRepo: accountRepo, transactionRepo: transactionRepo, txManager: txManager}
}

func (tp *TransactionProcessor) Process(ctx context.Context, transactionID uint) error {
	return tp.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		transaction, err := tp.transactionRepo.GetByID(ctx, transactionID)
		if err != nil {
				return errors.InternalErr(err)
		}

		if transaction.Status != model.TransactionProcessing {
			return errors.BadRequestErr("transaction already processed")
		}

		payer, err := tp.accountRepo.GetByAccountNumber(ctx, transaction.PayerAccountNumber)
		if err != nil {
				return errors.InternalErr(err)
		}

		recipient, err := tp.accountRepo.GetByAccountNumber(ctx, transaction.RecipientAccountNumber)
		if err != nil {
				return errors.InternalErr(err)
		}

		banksAccountTo, err := tp.accountRepo.GetByAccountNumber(ctx, BankAccounts[transaction.StartCurrencyCode])
		if err != nil {
				return errors.InternalErr(err)
		}

		banksAccountFrom, err := tp.accountRepo.GetByAccountNumber(ctx, BankAccounts[transaction.EndCurrencyCode])
		if err != nil {
				return errors.InternalErr(err)
		}

		// Check funds
		if payer.AvailableBalance < transaction.StartAmount {
				return errors.BadRequestErr("insufficient payer funds")
		}
		if banksAccountFrom.AvailableBalance < transaction.EndAmount {
				return errors.BadRequestErr("insufficient banks funds")
		}

		model.UpdateBalances(payer, -transaction.StartAmount)
		model.UpdateBalances(banksAccountTo, transaction.StartAmount)
		model.UpdateBalances(banksAccountFrom, -transaction.EndAmount)
		model.UpdateBalances(recipient, transaction.EndAmount)

		if err := tp.accountRepo.Update(ctx, payer); err != nil {
				return errors.InternalErr(err)
		}
		if err := tp.accountRepo.Update(ctx, recipient); err != nil {
				return errors.InternalErr(err)
		}
		if err := tp.accountRepo.Update(ctx, banksAccountTo); err != nil {
				return errors.InternalErr(err)
		}
		if err := tp.accountRepo.Update(ctx, banksAccountFrom); err != nil {
				return errors.InternalErr(err)
		}

		transaction.Status = model.TransactionCompleted
		return tp.transactionRepo.Update(ctx, transaction)
	})
}
