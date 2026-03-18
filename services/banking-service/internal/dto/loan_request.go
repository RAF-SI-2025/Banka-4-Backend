package dto

import "banking-service/internal/model"

// CreateLoanRequest predstavlja podatke koje klijent salje kada trazi kredit
type CreateLoanRequest struct {
	AccountNumber   string             `json:"account_number" binding:"required"`
	LoanTypeID      uint               `json:"loan_type_id" binding:"required"`
	Amount          float64            `json:"amount" binding:"required,gt=0"`
	RepaymentPeriod int                `json:"repayment_period" binding:"required,gt=0"` // Broj meseci
	CurrencyCode    model.CurrencyCode `json:"currency" binding:"required"`
}

// CreateLoanResponse je odgovor koji se vraca kada je zahtev uspesno prosledjen
type CreateLoanResponse struct {
	RequestID          uint                    `json:"id"`
	Status             model.LoanRequestStatus `json:"status"`
	MonthlyInstallment float64                 `json:"monthly_installment"`
}
