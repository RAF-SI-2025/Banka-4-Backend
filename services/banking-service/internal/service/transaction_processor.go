package service

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/repository"
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
	interbankClient client.InterbankClient
}

func NewTransactionProcessor(accountRepo repository.AccountRepository, transactionRepo repository.TransactionRepository, txManager repository.TransactionManager, interbankClient client.InterbankClient) *TransactionProcessor {
	return &TransactionProcessor{accountRepo: accountRepo, transactionRepo: transactionRepo, txManager: txManager, interbankClient: interbankClient}
}

func (tp *TransactionProcessor) Process(ctx context.Context, transactionID uint) error {
	// A foreign recipient is settled through interbank-service (2PC), not the
	// local ledger. Branch here, before opening a DB transaction: interbank calls
	// back into banking to reserve/commit the payer, so holding a tx on the payer
	// row here would deadlock with that callback.
	transaction, err := tp.transactionRepo.GetByID(ctx, transactionID)
	if err != nil {
		return errors.InternalErr(err)
	}
	if transaction == nil {
		return errors.NotFoundErr("transaction not found")
	}
	if model.IsForeignAccountNumber(transaction.RecipientAccountNumber) {
		return tp.processInterbank(ctx, transaction)
	}

	return tp.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		transaction, err := tp.transactionRepo.GetByID(ctx, transactionID)
		if err != nil {
			return errors.InternalErr(err)
		}

		if transaction.Status != model.TransactionProcessing {
			return errors.BadRequestErr("transaction already processed")
		}

		payer, err := tp.accountRepo.FindByAccountNumber(ctx, transaction.PayerAccountNumber)
		if err != nil {
			return errors.InternalErr(err)
		}

		// Check funds
		if payer.AvailableBalance < transaction.StartAmount {
			return errors.BadRequestErr("insufficient payer funds")
		}

		// Check limits
		if payer.DailySpending+transaction.StartAmount > payer.DailyLimit {
			return errors.BadRequestErr("daily limit exceeded")
		}

		if payer.MonthlySpending+transaction.StartAmount > payer.MonthlyLimit {
			return errors.BadRequestErr("monthly limit exceeded")
		}

		recipient, err := tp.accountRepo.FindByAccountNumber(ctx, transaction.RecipientAccountNumber)
		if err != nil {
			return errors.InternalErr(err)
		}

		if recipient.AccountNumber == payer.AccountNumber {
			return errors.BadRequestErr("cannot make payment to the same account")
		}

		if transaction.StartCurrencyCode != transaction.EndCurrencyCode {
			banksAccountTo, err := tp.accountRepo.FindByAccountNumber(ctx, BankAccounts[transaction.StartCurrencyCode])
			if err != nil {
				return errors.InternalErr(err)
			}

			banksAccountFrom, err := tp.accountRepo.FindByAccountNumber(ctx, BankAccounts[transaction.EndCurrencyCode])
			if err != nil {
				return errors.InternalErr(err)
			}

			if banksAccountFrom.AvailableBalance < transaction.EndAmount {
				return errors.BadRequestErr("insufficient banks funds")
			}

			updates := map[string]float64{}

			updates[payer.AccountNumber] += -transaction.StartAmount
			updates[banksAccountTo.AccountNumber] += transaction.StartAmount
			updates[banksAccountFrom.AccountNumber] += -transaction.EndAmount
			updates[recipient.AccountNumber] += transaction.EndAmount

			for accNum, delta := range updates {
				acc, err := tp.accountRepo.FindByAccountNumber(ctx, accNum)
				if err != nil {
					return errors.InternalErr(err)
				}

				model.UpdateBalances(acc, delta)

				if err := tp.accountRepo.UpdateBalance(ctx, acc); err != nil {
					return errors.InternalErr(err)
				}
			}
		} else {
			model.UpdateBalances(payer, -transaction.StartAmount)
			model.UpdateBalances(recipient, transaction.EndAmount)

			if err := tp.accountRepo.UpdateBalance(ctx, payer); err != nil {
				return errors.InternalErr(err)
			}
			if err := tp.accountRepo.UpdateBalance(ctx, recipient); err != nil {
				return errors.InternalErr(err)
			}
		}

		transaction.Status = model.TransactionCompleted
		return tp.transactionRepo.Update(ctx, transaction)
	})
}

// processInterbank hands a foreign-recipient payment to interbank-service. The
// payer is reserved/settled by interbank calling back into banking's
// PrepareInterbankCashPosting/Commit/Rollback, so we move no balances here. The
// transaction stays Processing until interbank reports the 2PC outcome via the
// FinalizeInterbankPayment callback — it is only Rejected up-front if the
// initiation itself fails (e.g. insufficient funds at reservation).
func (tp *TransactionProcessor) processInterbank(ctx context.Context, transaction *model.Transaction) error {
	if transaction.Status != model.TransactionProcessing {
		return errors.BadRequestErr("transaction already processed")
	}

	req := &pb.InitiateInterbankPaymentRequest{
		PayerAccountNumber: transaction.PayerAccountNumber,
		PayeeAccountNumber: transaction.RecipientAccountNumber,
		Amount:             transaction.StartAmount,
		Currency:           string(transaction.StartCurrencyCode),
		BankingTxId:        uint64(transaction.TransactionID),
	}

	if err := tp.interbankClient.InitiatePayment(ctx, req); err != nil {
		transaction.Status = model.TransactionRejected
		if updateErr := tp.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
			return tp.transactionRepo.Update(ctx, transaction)
		}); updateErr != nil {
			return errors.InternalErr(updateErr)
		}
		return errors.MapGrpcToHttpError(err)
	}

	return nil
}

func (tp *TransactionProcessor) ProcessTradeSettlement(ctx context.Context, transactionID uint) error {
	return tp.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		transaction, err := tp.transactionRepo.GetByID(ctx, transactionID)
		if err != nil {
			return errors.InternalErr(err)
		}

		if transaction.Status != model.TransactionProcessing {
			return errors.BadRequestErr("transaction already processed")
		}

		payer, err := tp.accountRepo.FindByAccountNumber(ctx, transaction.PayerAccountNumber)
		if err != nil {
			return errors.InternalErr(err)
		}

		recipient, err := tp.accountRepo.FindByAccountNumber(ctx, transaction.RecipientAccountNumber)
		if err != nil {
			return errors.InternalErr(err)
		}

		if payer.AccountNumber == recipient.AccountNumber {
			return errors.BadRequestErr("cannot settle trade to the same account")
		}

		if payer.AvailableBalance < transaction.StartAmount {
			return errors.BadRequestErr("insufficient payer funds")
		}

		model.UpdateBalances(payer, -transaction.StartAmount)
		model.UpdateBalances(recipient, transaction.EndAmount)

		if err := tp.accountRepo.UpdateBalance(ctx, payer); err != nil {
			return errors.InternalErr(err)
		}
		if err := tp.accountRepo.UpdateBalance(ctx, recipient); err != nil {
			return errors.InternalErr(err)
		}

		transaction.Status = model.TransactionCompleted
		return tp.transactionRepo.Update(ctx, transaction)
	})
}

func (tp *TransactionProcessor) ProcessLoanInstallment(ctx context.Context, transactionID uint) error {
	return tp.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		transaction, err := tp.transactionRepo.GetByID(ctx, transactionID)
		if err != nil {
			return errors.InternalErr(err)
		}

		if transaction.Status != model.TransactionProcessing {
			return errors.BadRequestErr("transaction already processed")
		}

		payer, err := tp.accountRepo.FindByAccountNumber(ctx, transaction.PayerAccountNumber)
		if err != nil {
			return errors.InternalErr(err)
		}

		if payer.AvailableBalance < transaction.StartAmount {
			return errors.BadRequestErr("insufficient payer funds")
		}

		recipient, err := tp.accountRepo.FindByAccountNumber(ctx, transaction.RecipientAccountNumber)
		if err != nil {
			return errors.InternalErr(err)
		}

		model.UpdateBalances(payer, -transaction.StartAmount)
		model.UpdateBalances(recipient, transaction.EndAmount)

		if err := tp.accountRepo.UpdateBalance(ctx, payer); err != nil {
			return errors.InternalErr(err)
		}
		if err := tp.accountRepo.UpdateBalance(ctx, recipient); err != nil {
			return errors.InternalErr(err)
		}

		transaction.Status = model.TransactionCompleted
		return tp.transactionRepo.Update(ctx, transaction)
	})
}
