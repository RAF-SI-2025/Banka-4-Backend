package dto

import "banking-service/internal/model"

// LoanResponse za listu svih kredita (osnovne informacije)
type LoanResponse struct {
	ID                 uint                    `json:"id"`
	LoanType           string                  `json:"loan_type"`
	Amount             float64                 `json:"amount"`
	Currency           string                  `json:"currency"`
	MonthlyInstallment float64                 `json:"monthly_installment"`
	Status             model.LoanRequestStatus `json:"status"`
}

// Rata za plan otplate
type Installment struct {
	Number int     `json:"number"`
	Amount float64 `json:"amount"`
	Status string  `json:"status"` // npr. "PAID", "UPCOMING"
}

// LoanDetailsResponse za specifičan kredit sa planom otplate
type LoanDetailsResponse struct {
	LoanResponse
	RepaymentPeriod int           `json:"repayment_period"`
	InterestRate    float64       `json:"interest_rate"`
	Installments    []Installment `json:"installments"`
}
