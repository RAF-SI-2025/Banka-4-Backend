package service

import (
	"banking-service/internal/model"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateMonthlyInstallment(t *testing.T) {
	// Inicijalizujemo servis (repozitorijumi nam ovde ne trebaju jer testiramo samo matematiku)
	loanService := NewLoanService(nil, nil, nil)

	// Tabela sa testnim scenarijima (Table-Driven Tests)
	tests := []struct {
		name              string
		amount            float64
		annualRatePercent float64
		months            int
		expected          float64
	}{
		{
			name:              "Kredit bez kamate (0%)",
			amount:            1200.0,
			annualRatePercent: 0.0,
			months:            12,
			expected:          100.00, // 1200 / 12 = 100
		},
		{
			name:              "Standardni kredit (5% na 12 meseci)",
			amount:            10000.0,
			annualRatePercent: 5.0,
			months:            12,
			expected:          856.07, // Izračunato po formuli anuiteta
		},
		{
			name:              "Kredit sa većom kamatom (12% na 24 meseca)",
			amount:            50000.0,
			annualRatePercent: 12.0,
			months:            24,
			expected:          2353.67,
		},
		{
			name:              "Zaštita od deljenja sa nulom (0 meseci)",
			amount:            1000.0,
			annualRatePercent: 0.0,
			months:            0,
			expected:          0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := loanService.CalculateMonthlyInstallment(tt.amount, tt.annualRatePercent, tt.months)

			// Upoređujemo da li je funkcija vratila ono što smo peške izračunali
			assert.Equal(t, tt.expected, actual, "Rata za '%s' nije tačna", tt.name)
		})
	}
}

// 1. Pravimo Mock (lažni) repozitorijum koji simulira bazu
type MockLoanRepository struct {
	MockLoans []model.LoanRequest
	MockLoan  *model.LoanRequest
	MockErr   error
}

// Implementiramo sve metode iz interfejsa
func (m *MockLoanRepository) CreateRequest(ctx context.Context, request *model.LoanRequest) error {
	return m.MockErr
}

func (m *MockLoanRepository) FindByClientID(ctx context.Context, clientID uint, sortByAmountDesc bool) ([]model.LoanRequest, error) {
	return m.MockLoans, m.MockErr
}

func (m *MockLoanRepository) FindByIDAndClientID(ctx context.Context, id uint, clientID uint) (*model.LoanRequest, error) {
	if m.MockErr != nil {
		return nil, m.MockErr
	}
	return m.MockLoan, nil
}

// 2. Testiramo da li servis pravilno generiše rate za plan otplate
func TestGetLoanDetails(t *testing.T) {
	// Nameštamo naš lažni repozitorijum da vrati jedan specifičan kredit na 12 meseci
	mockRepo := &MockLoanRepository{
		MockLoan: &model.LoanRequest{
			ID:                 1,
			Amount:             10000.0,
			RepaymentPeriod:    12,    // Znači očekujemo 12 rata u odgovoru
			MonthlyInstallment: 856.0, // Rata je 856
			CurrencyCode:       "EUR",
			LoanType:           model.LoanType{Name: "Keš Kredit"},
		},
	}

	// Inicijalizujemo servis sa lažnim repozitorijumom
	loanService := NewLoanService(nil, nil, mockRepo)

	// Pozivamo metodu koju testiramo
	details, err := loanService.GetLoanDetails(context.Background(), 1, 1)

	// Proveravamo rezultate (Asserts)
	assert.NoError(t, err, "Ne bi trebalo da bude greške")
	assert.NotNil(t, details, "Detalji ne smeju biti prazni")

	// Ovo je ključno: Da li je for petlja iz servisa napravila tačno 12 rata?
	assert.Equal(t, 12, len(details.Installments), "Treba da generiše tačno 12 rata")

	// Da li prva rata ima tačan iznos i status?
	assert.Equal(t, 856.0, details.Installments[0].Amount, "Iznos prve rate se ne poklapa")
	assert.Equal(t, "UPCOMING", details.Installments[0].Status, "Status rate mora biti UPCOMING")

	// Da li poslednja rata ima ispravan redni broj?
	assert.Equal(t, 12, details.Installments[11].Number, "Poslednja rata mora biti pod rednim brojem 12")
}
