package service

import (
	"context"
	"time"

	commonauth "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/audit"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/config"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
)

const taxRate = 0.15

type TaxService struct {
	taxRepo          repository.TaxRepository
	bankingClient    client.BankingClient
	taxAccountNumber string
	auditRepo        audit.Repository
}

func NewTaxService(
	taxRepo repository.TaxRepository,
	bankingClient client.BankingClient,
	cfg *config.Configuration,
	auditRepo audit.Repository,
) *TaxService {
	return &TaxService{
		taxRepo:          taxRepo,
		bankingClient:    bankingClient,
		taxAccountNumber: cfg.TaxAccountNumber,
		auditRepo:        auditRepo,
	}
}

func (s *TaxService) RecordTax(ctx context.Context, accountNumber string, employeeID *uint, profit float64, currencyCode string) error {
	if profit <= 0 {
		return nil
	}

	taxAmount := profit * taxRate

	if err := s.taxRepo.AddTaxOwed(ctx, accountNumber, employeeID, taxAmount, currencyCode); err != nil {
		return errors.InternalErr(err)
	}

	return nil
}

func (s *TaxService) CollectTaxes(ctx context.Context) error {
	taxes, err := s.taxRepo.FindAllPositiveAccumulatedTax(ctx)
	if err != nil {
		return errors.InternalErr(err)
	}
	now := time.Now()

	for _, tax := range taxes {
		amountToCollect := tax.TaxOwed

		collectionErr := s.collectSingleTax(ctx, tax.AccountNumber, amountToCollect)

		var status model.TaxStatus
		var failureReason *string
		if collectionErr != nil {
			status = model.TaxStatusFailed
			reason := collectionErr.Error()
			failureReason = &reason
		} else {
			status = model.TaxStatusCollected
		}

		collection := &model.TaxCollection{
			AccountNumber:     tax.AccountNumber,
			EmployeeID:        tax.EmployeeID,
			TaxOwed:           amountToCollect,
			CurrencyCode:      tax.CurrencyCode,
			Status:            status,
			FailureReason:     failureReason,
			TaxingPeriodStart: tax.LastUpdatedAt,
			TaxingPeriodEnd:   &now,
		}

		err = s.taxRepo.RecordCollectionResult(ctx, collection, collectionErr == nil, amountToCollect, now)
		if err != nil {
			return errors.InternalErr(err)
		}
	}

	if authCtx := commonauth.GetAuthFromContext(ctx); authCtx != nil && authCtx.EmployeeID != nil {
		if err := s.auditRepo.Save(ctx, &audit.AuditLog{
			ActionType:    audit.ActionTaxCollectionTriggered,
			PerformedByID: *authCtx.EmployeeID,
		}); err != nil {
			return errors.InternalErr(err)
		}
	}

	return nil
}

func (s *TaxService) collectSingleTax(ctx context.Context, accountNumber string, amount float64) error {
	_, err := s.bankingClient.CreatePaymentWithoutVerification(ctx, &pb.CreatePaymentRequest{
		PayerAccountNumber:     accountNumber,
		RecipientAccountNumber: s.taxAccountNumber,
		RecipientName:          "Republika Srbija",
		Amount:                 amount,
		PaymentCode:            "253",
		Purpose:                "Porez na kapitalnu dobit",
	})
	return err
}

func (s *TaxService) GetAccumulatedTax(ctx context.Context, accountNumber string) (*model.AccumulatedTax, error) {
	tax, err := s.taxRepo.FindAccumulatedTaxByAccountNumber(ctx, accountNumber)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	return tax, nil
}

func (s *TaxService) GetTaxCollections(ctx context.Context, accountNumber string) ([]model.TaxCollection, error) {
	collections, err := s.taxRepo.FindTaxCollectionsByAccountNumber(ctx, accountNumber)
	if err != nil {
		return nil, errors.InternalErr(err)
	}
	return collections, nil
}

func (s *TaxService) GetEmployeeTotalTax(ctx context.Context, employeeID uint) (float64, error) {
	taxes, err := s.taxRepo.FindAccumulatedTaxByEmployeeID(ctx, employeeID)
	if err != nil {
		return 0, errors.InternalErr(err)
	}

	totals := map[string]float64{}
	for _, t := range taxes {
		totals[t.CurrencyCode] += t.TaxOwed
	}

	return s.sumToRSD(ctx, totals)
}

func (s *TaxService) GetClientTotalTax(ctx context.Context, clientID uint64) (float64, error) {
	accountsResp, err := s.bankingClient.GetAccountsByClientID(ctx, clientID)
	if err != nil {
		return 0, errors.InternalErr(err)
	}

	accountNumbers := make([]string, 0, len(accountsResp.Accounts))
	for _, acc := range accountsResp.Accounts {
		accountNumbers = append(accountNumbers, acc.AccountNumber)
	}

	taxes, err := s.taxRepo.FindAccumulatedTaxByClientAccountNumbers(ctx, accountNumbers)
	if err != nil {
		return 0, errors.InternalErr(err)
	}

	totals := map[string]float64{}
	for _, t := range taxes {
		totals[t.CurrencyCode] += t.TaxOwed
	}

	return s.sumToRSD(ctx, totals)
}

func (s *TaxService) sumToRSD(ctx context.Context, totals map[string]float64) (float64, error) {
	total := 0.0
	for currency, amount := range totals {
		if amount <= 0 {
			continue
		}
		if currency == "RSD" {
			total += amount
			continue
		}
		converted, err := s.bankingClient.ConvertCurrency(ctx, amount, currency, "RSD")
		if err != nil {
			return 0, errors.InternalErr(err)
		}
		total += converted
	}
	return total, nil
}
