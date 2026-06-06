package service

import (
	"context"
	"strings"

	commonerrors "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/repository"
)

type InterbankCashService struct {
	accountRepo repository.AccountRepository
	postingRepo repository.InterbankCashPostingRepository
	txManager   repository.TransactionManager
}

func NewInterbankCashService(
	accountRepo repository.AccountRepository,
	postingRepo repository.InterbankCashPostingRepository,
	txManager repository.TransactionManager,
) *InterbankCashService {
	return &InterbankCashService{
		accountRepo: accountRepo,
		postingRepo: postingRepo,
		txManager:   txManager,
	}
}

func (s *InterbankCashService) Prepare(ctx context.Context, postingID, accountNumber string, clientID uint, currencyCode model.CurrencyCode, amount float64) (*model.InterbankCashPosting, error) {
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
			if existing.Amount != amount || existing.CurrencyCode != currencyCode {
				return commonerrors.ConflictErr("posting id already exists with different parameters")
			}
			result = existing
			return nil
		}

		account, err := s.resolveAccount(ctx, accountNumber, clientID, currencyCode)
		if err != nil {
			return err
		}

		if amount < 0 {
			required := -amount
			if account.AvailableBalance < required {
				return commonerrors.BadRequestErr("insufficient funds")
			}
			account.AvailableBalance -= required
			if err := s.accountRepo.UpdateBalance(ctx, account); err != nil {
				return commonerrors.InternalErr(err)
			}
		}

		posting := &model.InterbankCashPosting{
			PostingID:     postingID,
			AccountNumber: account.AccountNumber,
			CurrencyCode:  currencyCode,
			Amount:        amount,
			Status:        model.InterbankCashPostingPrepared,
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
		return nil
	})
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

func (s *InterbankCashService) resolveAccount(ctx context.Context, accountNumber string, clientID uint, currencyCode model.CurrencyCode) (*model.Account, error) {
	if accountNumber != "" {
		account, err := s.accountRepo.FindByAccountNumber(ctx, accountNumber)
		if err != nil {
			return nil, commonerrors.InternalErr(err)
		}
		if account == nil {
			return nil, commonerrors.NotFoundErr("account not found")
		}
		if account.Currency.Code != currencyCode {
			return nil, commonerrors.BadRequestErr("account currency does not match posting currency")
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
	for i := range accounts {
		if accounts[i].Currency.Code == currencyCode && accounts[i].Status == "Active" {
			return &accounts[i], nil
		}
	}
	return nil, commonerrors.NotFoundErr("client account for currency not found")
}
