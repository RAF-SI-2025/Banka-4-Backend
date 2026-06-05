package service

import (
	"context"
	"fmt"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/repository"
)

type InterbankPaymentService struct {
	accountRepo     repository.AccountRepository
	transactionRepo repository.TransactionRepository
	reservationRepo repository.InterbankReservationRepository
	txManager       repository.TransactionManager
}

func NewInterbankPaymentService(
	accountRepo repository.AccountRepository,
	transactionRepo repository.TransactionRepository,
	reservationRepo repository.InterbankReservationRepository,
	txManager repository.TransactionManager,
) *InterbankPaymentService {
	return &InterbankPaymentService{
		accountRepo:     accountRepo,
		transactionRepo: transactionRepo,
		reservationRepo: reservationRepo,
		txManager:       txManager,
	}
}

// ValidateInterbankPosting checks whether an account exists, is active, and
// can receive the given currency. Used by the peer bank's participant role.
func (s *InterbankPaymentService) ValidateInterbankPosting(ctx context.Context, accountNumber, currency string) (bool, string) {
	account, err := s.accountRepo.FindByAccountNumber(ctx, accountNumber)
	if err != nil {
		return false, "internal error"
	}
	if account == nil {
		return false, fmt.Sprintf("account %s not found", accountNumber)
	}
	if account.Status != "Active" {
		return false, fmt.Sprintf("account %s is not active", accountNumber)
	}
	if string(account.Currency.Code) != currency {
		return false, fmt.Sprintf("account %s currency %s does not match posting currency %s",
			accountNumber, account.Currency.Code, currency)
	}
	return true, ""
}

// ReserveInterbankFunds reserves funds on the payer's account for an outbound
// inter-bank payment. AvailableBalance is reduced; Balance is unchanged until
// CommitInterbankPayment.
func (s *InterbankPaymentService) ReserveInterbankFunds(ctx context.Context, accountNumber string, amount float64, currency string, bankingTxID uint) error {
	return s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		account, err := s.accountRepo.FindByAccountNumber(ctx, accountNumber)
		if err != nil {
			return errors.InternalErr(err)
		}
		if account == nil {
			return errors.NotFoundErr("payer account not found")
		}
		if account.AvailableBalance < amount {
			return errors.BadRequestErr("insufficient funds for inter-bank payment")
		}

		account.AvailableBalance -= amount
		if err := s.accountRepo.UpdateBalance(ctx, account); err != nil {
			return errors.InternalErr(err)
		}

		reservation := &model.InterbankReservation{
			PendingBankingTxID: bankingTxID,
			Status:             model.InterbankReservationStatusReserved,
		}
		if err := s.reservationRepo.Create(ctx, reservation); err != nil {
			return errors.InternalErr(err)
		}
		return nil
	})
}

// CommitInterbankPayment finalises an outbound inter-bank payment by deducting
// Balance. Idempotent: if reservation is already COMMITTED it is a no-op.
func (s *InterbankPaymentService) CommitInterbankPayment(ctx context.Context, bankingTxID uint) error {
	return s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		reservation, err := s.reservationRepo.FindByPendingBankingTxID(ctx, bankingTxID)
		if err != nil {
			return errors.InternalErr(err)
		}
		if reservation == nil {
			return errors.NotFoundErr("interbank reservation not found")
		}

		if reservation.Status == model.InterbankReservationStatusCommitted {
			return nil
		}
		if reservation.Status == model.InterbankReservationStatusRolledBack {
			return errors.BadRequestErr("cannot commit a rolled-back reservation")
		}

		tx, err := s.transactionRepo.GetByID(ctx, bankingTxID)
		if err != nil {
			return errors.InternalErr(err)
		}
		if tx == nil {
			return errors.NotFoundErr("transaction not found")
		}

		account, err := s.accountRepo.FindByAccountNumber(ctx, tx.PayerAccountNumber)
		if err != nil {
			return errors.InternalErr(err)
		}
		if account == nil {
			return errors.NotFoundErr("payer account not found")
		}

		account.Balance -= tx.StartAmount
		if err := s.accountRepo.UpdateBalance(ctx, account); err != nil {
			return errors.InternalErr(err)
		}

		reservation.Status = model.InterbankReservationStatusCommitted
		if err := s.reservationRepo.Save(ctx, reservation); err != nil {
			return errors.InternalErr(err)
		}

		if tx.Status == model.TransactionProcessing {
			tx.Status = model.TransactionCompleted
			if err := s.transactionRepo.Update(ctx, tx); err != nil {
				return errors.InternalErr(err)
			}
		}
		return nil
	})
}

// RollbackInterbankPayment releases a reserved amount back to AvailableBalance.
// Idempotent: if already ROLLED_BACK it is a no-op.
func (s *InterbankPaymentService) RollbackInterbankPayment(ctx context.Context, bankingTxID uint) error {
	return s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		reservation, err := s.reservationRepo.FindByPendingBankingTxID(ctx, bankingTxID)
		if err != nil {
			return errors.InternalErr(err)
		}
		if reservation == nil {
			return errors.NotFoundErr("interbank reservation not found")
		}

		if reservation.Status == model.InterbankReservationStatusRolledBack {
			return nil
		}
		if reservation.Status == model.InterbankReservationStatusCommitted {
			return errors.BadRequestErr("cannot rollback a committed reservation")
		}

		tx, err := s.transactionRepo.GetByID(ctx, bankingTxID)
		if err != nil {
			return errors.InternalErr(err)
		}
		if tx == nil {
			return errors.NotFoundErr("transaction not found")
		}

		account, err := s.accountRepo.FindByAccountNumber(ctx, tx.PayerAccountNumber)
		if err != nil {
			return errors.InternalErr(err)
		}
		if account == nil {
			return errors.NotFoundErr("payer account not found")
		}

		account.AvailableBalance += tx.StartAmount
		if err := s.accountRepo.UpdateBalance(ctx, account); err != nil {
			return errors.InternalErr(err)
		}

		reservation.Status = model.InterbankReservationStatusRolledBack
		if err := s.reservationRepo.Save(ctx, reservation); err != nil {
			return errors.InternalErr(err)
		}

		if tx.Status == model.TransactionProcessing {
			tx.Status = model.TransactionRejected
			if err := s.transactionRepo.Update(ctx, tx); err != nil {
				return errors.InternalErr(err)
			}
		}
		return nil
	})
}

// CreditInterbankPayment credits an incoming inter-bank payment to the
// recipient's account. Idempotent by interbankTxID.
func (s *InterbankPaymentService) CreditInterbankPayment(
	ctx context.Context,
	accountNumber string,
	amount float64,
	currency string,
	interbankTxID string,
	message string,
	paymentCode string,
	paymentPurpose string,
) error {
	return s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		existing, err := s.transactionRepo.FindByInterbankTxID(ctx, interbankTxID)
		if err != nil {
			return errors.InternalErr(err)
		}
		if existing != nil {
			return nil
		}

		account, err := s.accountRepo.FindByAccountNumber(ctx, accountNumber)
		if err != nil {
			return errors.InternalErr(err)
		}
		if account == nil {
			return errors.NotFoundErr("recipient account not found")
		}

		account.Balance += amount
		account.AvailableBalance += amount
		if err := s.accountRepo.UpdateBalance(ctx, account); err != nil {
			return errors.InternalErr(err)
		}

		txID := interbankTxID
		tx := &model.Transaction{
			PayerAccountNumber:     "INTERBANK",
			RecipientAccountNumber: accountNumber,
			StartAmount:            amount,
			StartCurrencyCode:      model.CurrencyCode(currency),
			EndAmount:              amount,
			EndCurrencyCode:        model.CurrencyCode(currency),
			Status:                 model.TransactionCompleted,
			InterbankTxID:          &txID,
			CreatedAt:              time.Now(),
		}
		if err := s.transactionRepo.Create(ctx, tx); err != nil {
			return errors.InternalErr(err)
		}
		return nil
	})
}
