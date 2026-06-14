package service

import (
	"context"
	"math"
	"strings"

	commonerrors "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/repository"
)

// PostingMetadata carries the payment-history context for an interbank cash
// posting. It is captured at prepare time and used to build a Transaction+Payment
// record when the posting commits. BankingTxID is non-zero only for the initiating
// payment leg (which already has a Transaction), gating record creation off for it.
type PostingMetadata struct {
	BankingTxID               uint64
	CounterpartyAccountNumber string
	PaymentCode               string
	Purpose                   string
}

type InterbankCashService struct {
	accountRepo     repository.AccountRepository
	postingRepo     repository.InterbankCashPostingRepository
	txManager       repository.TransactionManager
	converter       CurrencyConverter
	transactionRepo repository.TransactionRepository
	paymentRepo     repository.PaymentRepository
}

func NewInterbankCashService(
	accountRepo repository.AccountRepository,
	postingRepo repository.InterbankCashPostingRepository,
	txManager repository.TransactionManager,
	converter CurrencyConverter,
	transactionRepo repository.TransactionRepository,
	paymentRepo repository.PaymentRepository,
) *InterbankCashService {
	return &InterbankCashService{
		accountRepo:     accountRepo,
		postingRepo:     postingRepo,
		txManager:       txManager,
		converter:       converter,
		transactionRepo: transactionRepo,
		paymentRepo:     paymentRepo,
	}
}

func (s *InterbankCashService) Prepare(ctx context.Context, postingID, accountNumber string, clientID uint, currencyCode model.CurrencyCode, amount float64, userType string, meta PostingMetadata) (*model.InterbankCashPosting, error) {
	postingID = strings.TrimSpace(postingID)
	accountNumber = strings.TrimSpace(accountNumber)
	if postingID == "" {
		return nil, commonerrors.BadRequestErr("posting id is required")
	}
	if amount == 0 {
		return nil, commonerrors.BadRequestErr("amount must not be zero")
	}
	if !model.AllowedCurrencies[currencyCode] {
		return nil, commonerrors.BadRequestErr("unsupported currency")
	}

	var result *model.InterbankCashPosting
	err := s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		existing, err := s.postingRepo.FindByID(ctx, postingID)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if existing != nil {
			if existing.RequestedAmount != amount || existing.RequestedCurrencyCode != currencyCode {
				return commonerrors.ConflictErr("posting id already exists with different parameters")
			}
			result = existing
			return nil
		}

		account, err := s.resolveAccount(ctx, accountNumber, clientID, currencyCode, userType)
		if err != nil {
			return err
		}

		// Freeze the amount converted into the resolved account's currency so the
		// reservation, commit and rollback all operate on a consistent value even
		// if exchange rates move between phases.
		resolvedAmount := amount
		if account.Currency.Code != currencyCode {
			resolvedAmount, err = s.converter.Convert(ctx, amount, currencyCode, account.Currency.Code)
			if err != nil {
				return err
			}
		}

		if resolvedAmount < 0 {
			required := -resolvedAmount
			if account.AvailableBalance < required {
				return commonerrors.BadRequestErr("insufficient funds")
			}
			account.AvailableBalance -= required
			if err := s.accountRepo.UpdateBalance(ctx, account); err != nil {
				return commonerrors.InternalErr(err)
			}
		}

		posting := &model.InterbankCashPosting{
			PostingID:                 postingID,
			AccountNumber:             account.AccountNumber,
			CurrencyCode:              account.Currency.Code,
			Amount:                    resolvedAmount,
			RequestedCurrencyCode:     currencyCode,
			RequestedAmount:           amount,
			Status:                    model.InterbankCashPostingPrepared,
			BankingTxID:               meta.BankingTxID,
			CounterpartyAccountNumber: meta.CounterpartyAccountNumber,
			PaymentCode:               meta.PaymentCode,
			Purpose:                   meta.Purpose,
		}
		if err := s.postingRepo.Create(ctx, posting); err != nil {
			return commonerrors.InternalErr(err)
		}
		result = posting
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *InterbankCashService) Commit(ctx context.Context, postingID string) (*model.InterbankCashPosting, error) {
	return s.transition(ctx, postingID, func(ctx context.Context, posting *model.InterbankCashPosting, account *model.Account) error {
		switch posting.Status {
		case model.InterbankCashPostingCommitted:
			return nil
		case model.InterbankCashPostingRolledBack:
			return commonerrors.BadRequestErr("cannot commit rolled back posting")
		case model.InterbankCashPostingPrepared:
		default:
			return commonerrors.BadRequestErr("posting is not prepared")
		}

		if posting.Amount < 0 {
			required := -posting.Amount
			if account.Balance < required {
				return commonerrors.BadRequestErr("insufficient balance")
			}
			account.Balance -= required
		} else {
			account.Balance += posting.Amount
			account.AvailableBalance += posting.Amount
		}

		if err := s.accountRepo.UpdateBalance(ctx, account); err != nil {
			return commonerrors.InternalErr(err)
		}
		posting.Status = model.InterbankCashPostingCommitted

		// This branch only runs on the PREPARED→COMMITTED transition (an
		// already-committed posting short-circuits above), so the history record
		// is created exactly once even if COMMIT_TX is retransmitted.
		return s.recordPaymentHistory(ctx, posting)
	})
}

// recordPaymentHistory writes a completed Transaction+Payment for a committed
// MONAS cash posting so it appears in the local account's payment history. It is a
// no-op for the initiating payment leg, which already has a Transaction created by
// CreatePayment (identified by BankingTxID).
func (s *InterbankCashService) recordPaymentHistory(ctx context.Context, posting *model.InterbankCashPosting) error {
	if posting.BankingTxID != 0 {
		existing, err := s.transactionRepo.GetByID(ctx, uint(posting.BankingTxID))
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if existing != nil {
			return nil
		}
	}

	transaction := &model.Transaction{
		StartAmount:       math.Abs(posting.RequestedAmount),
		StartCurrencyCode: posting.RequestedCurrencyCode,
		EndAmount:         math.Abs(posting.Amount),
		EndCurrencyCode:   posting.CurrencyCode,
		Commission:        0,
		Status:            model.TransactionCompleted,
	}
	// Sign convention (§2.8): a negative amount is a CREDIT (money leaves the local
	// account → local is the payer); a positive amount is a DEBIT (money enters →
	// local is the recipient). The counterparty may be empty when unknown.
	if posting.Amount < 0 {
		transaction.PayerAccountNumber = posting.AccountNumber
		transaction.RecipientAccountNumber = posting.CounterpartyAccountNumber
	} else {
		transaction.PayerAccountNumber = posting.CounterpartyAccountNumber
		transaction.RecipientAccountNumber = posting.AccountNumber
	}
	if err := s.transactionRepo.Create(ctx, transaction); err != nil {
		return commonerrors.InternalErr(err)
	}

	payment := &model.Payment{
		TransactionID: transaction.TransactionID,
		PaymentCode:   posting.PaymentCode,
		Purpose:       posting.Purpose,
	}
	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		return commonerrors.InternalErr(err)
	}
	return nil
}

func (s *InterbankCashService) Rollback(ctx context.Context, postingID string) (*model.InterbankCashPosting, error) {
	return s.transition(ctx, postingID, func(ctx context.Context, posting *model.InterbankCashPosting, account *model.Account) error {
		switch posting.Status {
		case model.InterbankCashPostingRolledBack:
			return nil
		case model.InterbankCashPostingCommitted:
			return commonerrors.BadRequestErr("cannot rollback committed posting")
		case model.InterbankCashPostingPrepared:
		default:
			return commonerrors.BadRequestErr("posting is not prepared")
		}

		if posting.Amount < 0 {
			account.AvailableBalance += -posting.Amount
			if err := s.accountRepo.UpdateBalance(ctx, account); err != nil {
				return commonerrors.InternalErr(err)
			}
		}
		posting.Status = model.InterbankCashPostingRolledBack
		return nil
	})
}

func (s *InterbankCashService) transition(
	ctx context.Context,
	postingID string,
	fn func(context.Context, *model.InterbankCashPosting, *model.Account) error,
) (*model.InterbankCashPosting, error) {
	postingID = strings.TrimSpace(postingID)
	if postingID == "" {
		return nil, commonerrors.BadRequestErr("posting id is required")
	}

	var result *model.InterbankCashPosting
	err := s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		posting, err := s.postingRepo.FindByID(ctx, postingID)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if posting == nil {
			return commonerrors.NotFoundErr("interbank cash posting not found")
		}

		account, err := s.accountRepo.FindByAccountNumber(ctx, posting.AccountNumber)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if account == nil {
			return commonerrors.NotFoundErr("account not found")
		}

		if err := fn(ctx, posting, account); err != nil {
			return err
		}
		if err := s.postingRepo.Save(ctx, posting); err != nil {
			return commonerrors.InternalErr(err)
		}
		result = posting
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// resolveAccount selects the local account a posting applies to. The returned
// account may have a currency different from the posting currency; the caller is
// responsible for converting the amount into the account's currency.
//
// For an explicit account number the named account is used as-is. For a client
// (PERSON / OTC postings) an active account is chosen by tier: first one whose
// currency matches the posting, else the client's active RSD account, else any
// active account.
func (s *InterbankCashService) resolveAccount(ctx context.Context, accountNumber string, clientID uint, currencyCode model.CurrencyCode, userType string) (*model.Account, error) {
	if accountNumber != "" {
		account, err := s.accountRepo.FindByAccountNumber(ctx, accountNumber)
		if err != nil {
			return nil, commonerrors.InternalErr(err)
		}
		if account == nil {
			return nil, commonerrors.NotFoundErr("account not found")
		}
		return account, nil
	}

	// EMPLOYEE party means the bank itself (acting through an employee). The bank
	// has no per-client account — money moves through its Firma account for the
	// currency, the same account used for commissions/fees.
	if userType == "EMPLOYEE" {
		bankAccountNumber, ok := BankAccounts[currencyCode]
		if !ok {
			return nil, commonerrors.BadRequestErr("no bank account for this currency")
		}
		account, err := s.accountRepo.FindByAccountNumber(ctx, bankAccountNumber)
		if err != nil {
			return nil, commonerrors.InternalErr(err)
		}
		if account == nil {
			return nil, commonerrors.NotFoundErr("bank account not found")
		}
		return account, nil
	}

	if clientID == 0 {
		return nil, commonerrors.BadRequestErr("client id or account number is required")
	}
	accounts, err := s.accountRepo.FindByClientID(ctx, clientID)
	if err != nil {
		return nil, commonerrors.InternalErr(err)
	}

	var rsdAccount, anyAccount *model.Account
	for i := range accounts {
		if accounts[i].Status != "Active" {
			continue
		}
		if accounts[i].Currency.Code == currencyCode {
			return &accounts[i], nil
		}
		if accounts[i].Currency.Code == model.RSD && rsdAccount == nil {
			rsdAccount = &accounts[i]
		}
		if anyAccount == nil {
			anyAccount = &accounts[i]
		}
	}
	if rsdAccount != nil {
		return rsdAccount, nil
	}
	if anyAccount != nil {
		return anyAccount, nil
	}
	return nil, commonerrors.NotFoundErr("no active account for client")
}
