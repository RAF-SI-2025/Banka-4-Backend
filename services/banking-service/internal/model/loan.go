package model

import "time"

// Status zahteva za kredit (enum)
type LoanRequestStatus string

const (
	LoanRequestPending  LoanRequestStatus = "PENDING"
	LoanRequestApproved LoanRequestStatus = "APPROVED"
	LoanRequestRejected LoanRequestStatus = "REJECTED"
)

// Tip kredita koji banka nudi (npr. Kes kredit, Stambeni kredit)
type LoanType struct {
	ID                 uint   `gorm:"primaryKey"`
	Name               string `gorm:"size:100;not null"`
	Description        string
	BankMargin         float64 // Marza banke u procentima (npr. 2.5%)
	BaseInterestRate   float64 // Bazna kamatna stopa (npr. 3.0%)
	MinRepaymentPeriod int     // Minimalni broj meseci
	MaxRepaymentPeriod int     // Maksimalni broj meseci
}

// Zahtev klijenta za kredit
type LoanRequest struct {
	ID                 uint   `gorm:"primaryKey"`
	ClientID           uint   `gorm:"not null"`         // Veza ka klijentu (User)
	AccountNumber      string `gorm:"size:20;not null"` // Racun na koji se prebacuju pare
	LoanTypeID         uint   `gorm:"not null"`         // Veza ka tipu kredita
	LoanType           LoanType
	CurrencyCode       CurrencyCode      `gorm:"size:3;not null"` // RSD, EUR
	Amount             float64           `gorm:"not null"`        // Iznos koji klijent trazi
	RepaymentPeriod    int               `gorm:"not null"`        // Na koliko meseci (n)
	CalculatedRate     float64           // Kamata (Marza + Osnovna kamata)
	MonthlyInstallment float64           // Izracunata mesecna rata (A)
	Status             LoanRequestStatus `gorm:"size:20;default:'PENDING'"`
	CreatedAt          time.Time
}
