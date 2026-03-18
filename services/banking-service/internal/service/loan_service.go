package service

import (
	"context"
	"math"

	"banking-service/internal/dto"
	"banking-service/internal/model"
	"banking-service/internal/repository"
	"common/pkg/errors"
)

type LoanService struct {
	accountRepo  repository.AccountRepository
	loanTypeRepo repository.LoanTypeRepository
	loanRepo     repository.LoanRepository
}

func NewLoanService(
	accountRepo repository.AccountRepository,
	loanTypeRepo repository.LoanTypeRepository,
	loanRepo repository.LoanRepository,
) *LoanService {
	return &LoanService{
		accountRepo:  accountRepo,
		loanTypeRepo: loanTypeRepo,
		loanRepo:     loanRepo,
	}
}

func (s *LoanService) CalculateMonthlyInstallment(amount float64, annualRatePercent float64, months int) float64 {
	if annualRatePercent <= 0 {
		if months == 0 {
			return 0
		}
		return amount / float64(months)
	}

	monthlyRate := (annualRatePercent / 100.0) / 12.0
	compoundFactor := math.Pow(1.0+monthlyRate, float64(months))
	installment := amount * (monthlyRate * compoundFactor) / (compoundFactor - 1.0)

	return math.Round(installment*100) / 100
}

func (s *LoanService) SubmitLoanRequest(ctx context.Context, req *dto.CreateLoanRequest, clientID uint) (*dto.CreateLoanResponse, error) {
	// 1. DOHVATANJE RAČUNA (pretpostavljam da imate metodu sličnu ovoj u AccountRepository)
	account, err := s.accountRepo.GetByAccountNumber(ctx, req.AccountNumber)
	if err != nil || account == nil {
		return nil, errors.BadRequestErr("račun nije pronađen")
	}

	// 2. DOHVATANJE TIPA KREDITA
	loanType, err := s.loanTypeRepo.FindByID(ctx, req.LoanTypeID)
	if err != nil || loanType == nil {
		return nil, errors.BadRequestErr("tip kredita nije pronađen")
	}

	// VALIDACIJA 1: Vlasništvo računa
	if account.ClientID != clientID {
		return nil, errors.ForbiddenErr("ovaj račun ne pripada vama")
	}

	// VALIDACIJA 2: Valuta (Ovo zavisi kako je tačno Currency struktura definisana,
	// ali logika je da se uporedi string kod valute, npr. "RSD" == "RSD")
	// Ako ne možete direktno preko account.Currency, možda ćete morati da dohvatite valutu preko CurrencyID
	if string(account.Currency.Code) != string(req.CurrencyCode) {
		return nil, errors.BadRequestErr("valuta kredita se ne poklapa sa valutom računa")
	}

	// VALIDACIJA 3: Period otplate
	if req.RepaymentPeriod < loanType.MinRepaymentPeriod || req.RepaymentPeriod > loanType.MaxRepaymentPeriod {
		return nil, errors.BadRequestErr("period otplate nije u dozvoljenom opsegu za izabrani tip kredita")
	}

	// 3. RAČUNANJE KAMATE I RATE
	totalInterestRate := loanType.BaseInterestRate + loanType.BankMargin
	monthlyInstallment := s.CalculateMonthlyInstallment(req.Amount, totalInterestRate, req.RepaymentPeriod)

	// 4. KREIRANJE ZAHTEVA
	newRequest := &model.LoanRequest{
		ClientID:           clientID,
		AccountNumber:      req.AccountNumber,
		LoanTypeID:         req.LoanTypeID,
		CurrencyCode:       req.CurrencyCode,
		Amount:             req.Amount,
		RepaymentPeriod:    req.RepaymentPeriod,
		CalculatedRate:     totalInterestRate,
		MonthlyInstallment: monthlyInstallment,
		Status:             model.LoanRequestPending, // Kreira se sa statusom PENDING, kako piše u tasku
	}

	// Upis u bazu
	if err := s.loanRepo.CreateRequest(ctx, newRequest); err != nil {
		return nil, errors.InternalErr(err)
	}

	// 5. VRAĆANJE ODGOVORA
	return &dto.CreateLoanResponse{
		RequestID:          newRequest.ID,
		Status:             newRequest.Status,
		MonthlyInstallment: monthlyInstallment,
	}, nil
}
func (s *LoanService) GetClientLoans(ctx context.Context, clientID uint, sortByAmountDesc bool) ([]dto.LoanResponse, error) {
	loans, err := s.loanRepo.FindByClientID(ctx, clientID, sortByAmountDesc)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	var response []dto.LoanResponse
	for _, l := range loans {
		response = append(response, dto.LoanResponse{
			ID:                 l.ID,
			LoanType:           l.LoanType.Name,
			Amount:             l.Amount,
			Currency:           string(l.CurrencyCode),
			MonthlyInstallment: l.MonthlyInstallment,
			Status:             l.Status,
		})
	}
	return response, nil
}

func (s *LoanService) GetLoanDetails(ctx context.Context, clientID uint, loanID uint) (*dto.LoanDetailsResponse, error) {
	loan, err := s.loanRepo.FindByIDAndClientID(ctx, loanID, clientID)
	if err != nil {
		return nil, errors.NotFoundErr("kredit nije pronađen")
	}

	// Generišemo plan otplate (Installments)
	var installments []dto.Installment
	for i := 1; i <= loan.RepaymentPeriod; i++ {
		installments = append(installments, dto.Installment{
			Number: i,
			Amount: loan.MonthlyInstallment,
			Status: "UPCOMING", // Svi su upcoming dok se ne napravi payment sistem
		})
	}

	return &dto.LoanDetailsResponse{
		LoanResponse: dto.LoanResponse{
			ID:                 loan.ID,
			LoanType:           loan.LoanType.Name,
			Amount:             loan.Amount,
			Currency:           string(loan.CurrencyCode),
			MonthlyInstallment: loan.MonthlyInstallment,
			Status:             loan.Status,
		},
		RepaymentPeriod: loan.RepaymentPeriod,
		InterestRate:    loan.CalculatedRate,
		Installments:    installments,
	}, nil
}
