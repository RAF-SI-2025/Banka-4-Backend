package dto

import (
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
)

func TestAccountAndCardResponses(t *testing.T) {
	t.Parallel()

	companyID := uint(9)
	account := &model.Account{
		AccountNumber:    "444000100000000001",
		Name:             "Main",
		ClientID:         1,
		CompanyID:        &companyID,
		EmployeeID:       2,
		Balance:          150,
		AvailableBalance: 125,
		Currency:         model.Currency{Code: model.EUR},
		Status:           "Active",
		AccountType:      model.AccountTypeBusiness,
		AccountKind:      model.AccountKindCurrent,
		Subtype:          model.SubtypeLLC,
		MaintenanceFee:   10,
		DailyLimit:       1000,
		MonthlyLimit:     5000,
		DailySpending:    100,
		MonthlySpending:  200,
		CreatedAt:        time.Now(),
		ExpiresAt:        time.Now().Add(24 * time.Hour),
	}

	accountResp := ToAccountResponse(account)
	if accountResp.ReservedFunds != 25 || accountResp.CurrencyCode != model.EUR || accountResp.CompanyID == nil {
		t.Fatalf("unexpected account response %#v", accountResp)
	}

	authorizedPersonID := uint(3)
	cards := []model.Card{{
		CardID:             7,
		AccountNumber:      account.AccountNumber,
		CardNumber:         "1234567812345678",
		CardType:           model.CardTypeDebit,
		CardBrand:          model.CardBrandVisa,
		Name:               "Primary",
		Limit:              500,
		Status:             model.CardStatusActive,
		AuthorizedPersonID: &authorizedPersonID,
	}}

	cardResp := ToCardResponse(&cards[0], account.Name)
	if cardResp.MaskedCardNumber != "1234********5678" || cardResp.AccountName != "Main" {
		t.Fatalf("unexpected card response %#v", cardResp)
	}
	if maskCardNumber("short") != "short" {
		t.Fatal("short card number should not be masked")
	}

	accountCards := ToAccountCardsResponse(account, cards)
	if len(accountCards.Cards) != 1 || accountCards.Cards[0].ID != 7 {
		t.Fatalf("unexpected account cards response %#v", accountCards)
	}
}

func TestBankingCollectionResponses(t *testing.T) {
	t.Parallel()

	companies := ToCompanyResponses([]model.Company{{
		CompanyID:          1,
		Name:               "Acme",
		RegistrationNumber: "12345678",
		TaxNumber:          "123456789",
		WorkCodeID:         4,
		Address:            "Main Street",
		OwnerID:            5,
	}})
	if len(companies) != 1 || companies[0].Name != "Acme" {
		t.Fatalf("unexpected companies %#v", companies)
	}

	workCodes := ToWorkCodeResponses([]model.WorkCode{{WorkCodeID: 1, Code: "6201", Description: "Software"}})
	if len(workCodes) != 1 || workCodes[0].Code != "6201" {
		t.Fatalf("unexpected work codes %#v", workCodes)
	}
}

func TestExchangeAndPaymentResponses(t *testing.T) {
	t.Parallel()

	now := time.Now()
	rates := ToExchangeRatesResponse([]model.ExchangeRate{{
		BaseCurrency:         model.RSD,
		CurrencyCode:         model.EUR,
		BuyRate:              117.123,
		MiddleRate:           117.456,
		SellRate:             118.789,
		ProviderUpdatedAt:    now,
		ProviderNextUpdateAt: now.Add(24 * time.Hour),
	}})
	if rates.BaseCurrency != string(model.RSD) || rates.Rates[0].BuyRate != 117.12 {
		t.Fatalf("unexpected exchange rates %#v", rates)
	}

	converted := ToConvertResponse(10, model.EUR, model.RSD, 1174.567)
	if converted.Total != 1174.57 {
		t.Fatalf("unexpected convert response %#v", converted)
	}

	payment := ToPaymentResponse(&model.Payment{
		PaymentID:       6,
		RecipientName:   "Recipient",
		ReferenceNumber: "97",
		PaymentCode:     "289",
		Purpose:         "Invoice",
		Transaction: model.Transaction{
			PayerAccountNumber:     "payer",
			RecipientAccountNumber: "recipient",
			StartAmount:            100,
			Commission:             1.5,
			StartCurrencyCode:      model.RSD,
			Status:                 model.TransactionCompleted,
			CreatedAt:              now,
		},
	})
	if payment.Amount != 98.5 || payment.CurrencyCode != string(model.RSD) {
		t.Fatalf("unexpected payment response %#v", payment)
	}
}
