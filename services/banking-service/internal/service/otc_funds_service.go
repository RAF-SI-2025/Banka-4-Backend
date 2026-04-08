package service

import (
	"context"
	"strings"

	commonerrors "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/repository"
)

type OtcFundsService struct {
	accountRepo     repository.AccountRepository
	reservationRepo repository.OtcFundsReservationRepository
	txManager       repository.TransactionManager
	exchangeService *ExchangeService
}

func NewOtcFundsService(
	accountRepo repository.AccountRepository,
	reservationRepo repository.OtcFundsReservationRepository,
	txManager repository.TransactionManager,
	exchangeService *ExchangeService,
) *OtcFundsService {
	return &OtcFundsService{
		accountRepo:     accountRepo,
		reservationRepo: reservationRepo,
		txManager:       txManager,
		exchangeService: exchangeService,
	}
}

func (s *OtcFundsService) Reserve(
	ctx context.Context,
	executionID, buyerAccountNumber, sellerAccountNumber string,
	amount float64,
	tradeCurrencyCode model.CurrencyCode,
) (*model.OtcFundsReservation, error) {
	executionID = strings.TrimSpace(executionID)
	buyerAccountNumber = strings.TrimSpace(buyerAccountNumber)
	sellerAccountNumber = strings.TrimSpace(sellerAccountNumber)
	if executionID == "" {
		return nil, commonerrors.BadRequestErr("execution id is required")
	}
	if buyerAccountNumber == "" || sellerAccountNumber == "" {
		return nil, commonerrors.BadRequestErr("buyer and seller account numbers are required")
	}
	if buyerAccountNumber == sellerAccountNumber {
		return nil, commonerrors.BadRequestErr("buyer and seller accounts must be different")
	}
	if amount <= 0 {
		return nil, commonerrors.BadRequestErr("amount must be greater than zero")
	}
	if !model.AllowedCurrencies[tradeCurrencyCode] {
		return nil, commonerrors.BadRequestErr("unsupported trade currency")
	}

	var result *model.OtcFundsReservation
	err := s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		existing, err := s.reservationRepo.FindByExecutionID(ctx, executionID)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if existing != nil {
			if existing.BuyerAccountNumber != buyerAccountNumber ||
				existing.SellerAccountNumber != sellerAccountNumber ||
				existing.TradeAmount != amount ||
				existing.TradeCurrencyCode != tradeCurrencyCode {
				return commonerrors.ConflictErr("execution id already exists with different reservation parameters")
			}

			result = existing
			return nil
		}

		buyerAccount, err := s.accountRepo.FindByAccountNumber(ctx, buyerAccountNumber)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if buyerAccount == nil {
			return commonerrors.NotFoundErr("buyer account not found")
		}

		sellerAccount, err := s.accountRepo.FindByAccountNumber(ctx, sellerAccountNumber)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if sellerAccount == nil {
			return commonerrors.NotFoundErr("seller account not found")
		}

		sourceAmount, err := s.convertTradeAmount(ctx, amount, tradeCurrencyCode, buyerAccount.Currency.Code)
		if err != nil {
			return err
		}
		destinationAmount, err := s.convertTradeAmount(ctx, amount, tradeCurrencyCode, sellerAccount.Currency.Code)
		if err != nil {
			return err
		}

		if buyerAccount.AvailableBalance < sourceAmount {
			return commonerrors.BadRequestErr("insufficient buyer funds")
		}

		buyerAccount.AvailableBalance -= sourceAmount
		if err := s.accountRepo.UpdateBalance(ctx, buyerAccount); err != nil {
			return commonerrors.InternalErr(err)
		}

		reservation := &model.OtcFundsReservation{
			ExecutionID:             executionID,
			BuyerAccountNumber:      buyerAccount.AccountNumber,
			SellerAccountNumber:     sellerAccount.AccountNumber,
			TradeAmount:             amount,
			TradeCurrencyCode:       tradeCurrencyCode,
			SourceAmount:            sourceAmount,
			SourceCurrencyCode:      buyerAccount.Currency.Code,
			DestinationAmount:       destinationAmount,
			DestinationCurrencyCode: sellerAccount.Currency.Code,
			Status:                  model.OtcFundsReservationStatusReserved,
		}
		if err := s.reservationRepo.Create(ctx, reservation); err != nil {
			return commonerrors.InternalErr(err)
		}

		result = reservation
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *OtcFundsService) Release(ctx context.Context, executionID string) (*model.OtcFundsReservation, error) {
	return s.transition(ctx, executionID, func(ctx context.Context, reservation *model.OtcFundsReservation) error {
		switch reservation.Status {
		case model.OtcFundsReservationStatusReleased:
			return nil
		case model.OtcFundsReservationStatusCommitted:
			return commonerrors.BadRequestErr("cannot release committed OTC funds")
		case model.OtcFundsReservationStatusRefunded:
			return commonerrors.BadRequestErr("cannot release refunded OTC funds")
		case model.OtcFundsReservationStatusReserved:
		default:
			return commonerrors.BadRequestErr("cannot release OTC funds in current status")
		}

		buyerAccount, err := s.accountRepo.FindByAccountNumber(ctx, reservation.BuyerAccountNumber)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if buyerAccount == nil {
			return commonerrors.NotFoundErr("buyer account not found")
		}

		buyerAccount.AvailableBalance += reservation.SourceAmount
		if err := s.accountRepo.UpdateBalance(ctx, buyerAccount); err != nil {
			return commonerrors.InternalErr(err)
		}

		reservation.Status = model.OtcFundsReservationStatusReleased
		return nil
	})
}

func (s *OtcFundsService) Commit(ctx context.Context, executionID string) (*model.OtcFundsReservation, error) {
	return s.transition(ctx, executionID, func(ctx context.Context, reservation *model.OtcFundsReservation) error {
		switch reservation.Status {
		case model.OtcFundsReservationStatusCommitted:
			return nil
		case model.OtcFundsReservationStatusReleased:
			return commonerrors.BadRequestErr("cannot commit released OTC funds")
		case model.OtcFundsReservationStatusRefunded:
			return commonerrors.BadRequestErr("cannot commit refunded OTC funds")
		case model.OtcFundsReservationStatusReserved:
		default:
			return commonerrors.BadRequestErr("cannot commit OTC funds in current status")
		}

		buyerAccount, err := s.accountRepo.FindByAccountNumber(ctx, reservation.BuyerAccountNumber)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if buyerAccount == nil {
			return commonerrors.NotFoundErr("buyer account not found")
		}

		sellerAccount, err := s.accountRepo.FindByAccountNumber(ctx, reservation.SellerAccountNumber)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if sellerAccount == nil {
			return commonerrors.NotFoundErr("seller account not found")
		}

		if buyerAccount.Balance < reservation.SourceAmount {
			return commonerrors.BadRequestErr("insufficient buyer balance for commit")
		}

		buyerAccount.Balance -= reservation.SourceAmount
		sellerAccount.Balance += reservation.DestinationAmount
		sellerAccount.AvailableBalance += reservation.DestinationAmount

		if err := s.accountRepo.UpdateBalance(ctx, buyerAccount); err != nil {
			return commonerrors.InternalErr(err)
		}
		if err := s.accountRepo.UpdateBalance(ctx, sellerAccount); err != nil {
			return commonerrors.InternalErr(err)
		}

		reservation.Status = model.OtcFundsReservationStatusCommitted
		return nil
	})
}

func (s *OtcFundsService) Refund(ctx context.Context, executionID string) (*model.OtcFundsReservation, error) {
	return s.transition(ctx, executionID, func(ctx context.Context, reservation *model.OtcFundsReservation) error {
		switch reservation.Status {
		case model.OtcFundsReservationStatusRefunded:
			return nil
		case model.OtcFundsReservationStatusReleased:
			return commonerrors.BadRequestErr("cannot refund released OTC funds")
		case model.OtcFundsReservationStatusReserved:
			return commonerrors.BadRequestErr("cannot refund uncommitted OTC funds")
		case model.OtcFundsReservationStatusCommitted:
		default:
			return commonerrors.BadRequestErr("cannot refund OTC funds in current status")
		}

		buyerAccount, err := s.accountRepo.FindByAccountNumber(ctx, reservation.BuyerAccountNumber)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if buyerAccount == nil {
			return commonerrors.NotFoundErr("buyer account not found")
		}

		sellerAccount, err := s.accountRepo.FindByAccountNumber(ctx, reservation.SellerAccountNumber)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if sellerAccount == nil {
			return commonerrors.NotFoundErr("seller account not found")
		}

		if sellerAccount.Balance < reservation.DestinationAmount || sellerAccount.AvailableBalance < reservation.DestinationAmount {
			return commonerrors.BadRequestErr("insufficient seller funds for refund")
		}

		sellerAccount.Balance -= reservation.DestinationAmount
		sellerAccount.AvailableBalance -= reservation.DestinationAmount
		buyerAccount.Balance += reservation.SourceAmount
		buyerAccount.AvailableBalance += reservation.SourceAmount

		if err := s.accountRepo.UpdateBalance(ctx, sellerAccount); err != nil {
			return commonerrors.InternalErr(err)
		}
		if err := s.accountRepo.UpdateBalance(ctx, buyerAccount); err != nil {
			return commonerrors.InternalErr(err)
		}

		reservation.Status = model.OtcFundsReservationStatusRefunded
		return nil
	})
}

func (s *OtcFundsService) GetByExecutionID(ctx context.Context, executionID string) (*model.OtcFundsReservation, error) {
	reservation, err := s.reservationRepo.FindByExecutionID(ctx, strings.TrimSpace(executionID))
	if err != nil {
		return nil, commonerrors.InternalErr(err)
	}
	if reservation == nil {
		return nil, commonerrors.NotFoundErr("OTC funds reservation not found")
	}
	return reservation, nil
}

func (s *OtcFundsService) transition(
	ctx context.Context,
	executionID string,
	fn func(ctx context.Context, reservation *model.OtcFundsReservation) error,
) (*model.OtcFundsReservation, error) {
	executionID = strings.TrimSpace(executionID)
	if executionID == "" {
		return nil, commonerrors.BadRequestErr("execution id is required")
	}

	var result *model.OtcFundsReservation
	err := s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		reservation, err := s.reservationRepo.FindByExecutionID(ctx, executionID)
		if err != nil {
			return commonerrors.InternalErr(err)
		}
		if reservation == nil {
			return commonerrors.NotFoundErr("OTC funds reservation not found")
		}

		if err := fn(ctx, reservation); err != nil {
			return err
		}

		if err := s.reservationRepo.Save(ctx, reservation); err != nil {
			return commonerrors.InternalErr(err)
		}

		result = reservation
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *OtcFundsService) convertTradeAmount(
	ctx context.Context,
	amount float64,
	fromCode, toCode model.CurrencyCode,
) (float64, error) {
	if fromCode == toCode {
		return amount, nil
	}

	converted, err := s.exchangeService.Convert(ctx, amount, fromCode, toCode)
	if err != nil {
		return 0, commonerrors.InternalErr(err)
	}

	return converted, nil
}
