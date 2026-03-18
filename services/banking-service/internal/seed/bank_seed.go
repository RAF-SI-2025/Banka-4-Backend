package seed

import (
	"banking-service/internal/model"
	"errors"
	"log"
	"time"

	"gorm.io/gorm"
)

var currencies = []model.Currency{
	{Name: "Serbian Dinar", Code: "RSD", Symbol: "din", Country: "Serbia", Description: "Official currency of Serbia", Status: "Active"},
	{Name: "Euro", Code: "EUR", Symbol: "€", Country: "European Union", Description: "Official currency of the European Union", Status: "Active"},
	{Name: "Swiss Franc", Code: "CHF", Symbol: "Fr", Country: "Switzerland", Description: "Official currency of Switzerland", Status: "Active"},
	{Name: "US Dollar", Code: "USD", Symbol: "$", Country: "United States", Description: "Official currency of the United States", Status: "Active"},
	{Name: "British Pound", Code: "GBP", Symbol: "£", Country: "United Kingdom", Description: "Official currency of the United Kingdom", Status: "Active"},
	{Name: "Japanese Yen", Code: "JPY", Symbol: "¥", Country: "Japan", Description: "Official currency of Japan", Status: "Active"},
	{Name: "Canadian Dollar", Code: "CAD", Symbol: "CA$", Country: "Canada", Description: "Official currency of Canada", Status: "Active"},
	{Name: "Australian Dollar", Code: "AUD", Symbol: "A$", Country: "Australia", Description: "Official currency of Australia", Status: "Active"},
}

var workCodes = []model.WorkCode{
	{Code: "10.1", Description: "Production of food"},
	{Code: "10.2", Description: "Processing of fish"},
	{Code: "26.1", Description: "Manufacturing of electronic components"},
	{Code: "47.1", Description: "Retail trade"},
	{Code: "62.0", Description: "Computer programming and IT"},
	{Code: "64.1", Description: "Banking and financial services"},
	{Code: "69.1", Description: "Legal activities"},
	{Code: "70.2", Description: "Management consulting"},
	{Code: "85.1", Description: "Primary education"},
	{Code: "86.1", Description: "Hospital activities"},
}

var companies = []struct {
	Name               string
	RegistrationNumber string
	TaxNumber          string
	Address            string
	OwnerID            uint
	WorkCodeCode       string
}{
	{
		Name:               "Tech DOO",
		RegistrationNumber: "12345678",
		TaxNumber:          "123456789",
		Address:            "Trg Republike 5, Beograd, Srbija",
		OwnerID:            1,
		WorkCodeCode:       "62.0",
	},
	{
		Name:               "Global AD",
		RegistrationNumber: "87654321",
		TaxNumber:          "987654321",
		Address:            "Knez Mihailova 10, Beograd, Srbija",
		OwnerID:            1,
		WorkCodeCode:       "64.1",
	},
	{
		Name:               "Humanitarian Fondacija",
		RegistrationNumber: "11223344",
		TaxNumber:          "112233445",
		Address:            "Bulevar Oslobodjenja 20, Novi Sad, Srbija",
		OwnerID:            1,
		WorkCodeCode:       "85.1",
	},
}

var accounts = []struct {
	AccountNumber string
	Name          string
	ClientID      uint
	CompanyID     *uint
	EmployeeID    uint
	Balance       float64
	ExpiresAt     time.Time
	CurrencyCode  model.CurrencyCode
	AccountType   model.AccountType
	AccountKind   model.AccountKind
	Subtype       model.Subtype
	DailyLimit    float64
	MonthlyLimit  float64
}{
	// personal current accounts
	{
		AccountNumber: "444000112345678911",
		Name:          "Standard Personal Account",
		ClientID:      1,
		EmployeeID:    1,
		Balance:       50000.00,
		ExpiresAt:     time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrencyCode:  "RSD",
		AccountType:   model.AccountTypePersonal,
		AccountKind:   model.AccountKindCurrent,
		Subtype:       model.SubtypeStandard,
		DailyLimit:    250000.00,
		MonthlyLimit:  1000000.00,
	},
	{
		AccountNumber: "444000112345678913",
		Name:          "Savings Account",
		ClientID:      1,
		EmployeeID:    1,
		Balance:       100000.00,
		ExpiresAt:     time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrencyCode:  "RSD",
		AccountType:   model.AccountTypePersonal,
		AccountKind:   model.AccountKindCurrent,
		Subtype:       model.SubtypeSavings,
		DailyLimit:    250000.00,
		MonthlyLimit:  1000000.00,
	},

	{
		AccountNumber: "444000112345678921",
		Name:          "Personal EUR Account",
		ClientID:      1,
		EmployeeID:    1,
		Balance:       2000.00,
		ExpiresAt:     time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrencyCode:  "EUR",
		AccountType:   model.AccountTypePersonal,
		AccountKind:   model.AccountKindForeign,
		DailyLimit:    5000.00,
		MonthlyLimit:  20000.00,
	},
	{
		AccountNumber: "444000112345678922",
		Name:          "Personal USD Account",
		ClientID:      1,
		EmployeeID:    1,
		Balance:       1500.00,
		ExpiresAt:     time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrencyCode:  "USD",
		AccountType:   model.AccountTypePersonal,
		AccountKind:   model.AccountKindForeign,
		DailyLimit:    5000.00,
		MonthlyLimit:  20000.00,
	},
	{
		AccountNumber: "444000000000000000",
		Name:          "Bank RSD Account",
		ClientID:      2,
		EmployeeID:    3,
		Balance:       1_000_000_000.00,
		ExpiresAt:     time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrencyCode:  model.RSD,
		AccountType:   model.AccountTypeBank,
		AccountKind:   model.AccountKindInternal,
		DailyLimit:    1e12,
		MonthlyLimit:  1e13,
	},
	{
		AccountNumber: "444000000000000001",
		Name:          "Bank EUR Account",
		ClientID:      2,
		EmployeeID:    3,
		Balance:       1_000_000_000.00,
		ExpiresAt:     time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrencyCode:  model.EUR,
		AccountType:   model.AccountTypeBank,
		AccountKind:   model.AccountKindInternal,
		DailyLimit:    1e12,
		MonthlyLimit:  1e13,
	},
	{
		AccountNumber: "444000000000000002",
		Name:          "Bank USD Account",
		ClientID:      2,
		EmployeeID:    3,
		Balance:       1_000_000_000.00,
		ExpiresAt:     time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrencyCode:  model.USD,
		AccountType:   model.AccountTypeBank,
		AccountKind:   model.AccountKindInternal,
		DailyLimit:    1e12,
		MonthlyLimit:  1e13,
	},
	{
		AccountNumber: "444000000000000003",
		Name:          "Bank CHF Account",
		ClientID:      2,
		EmployeeID:    3,
		Balance:       1_000_000_000.00,
		ExpiresAt:     time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrencyCode:  model.CHF,
		AccountType:   model.AccountTypeBank,
		AccountKind:   model.AccountKindInternal,
		DailyLimit:    1e12,
		MonthlyLimit:  1e13,
	},
	{
		AccountNumber: "444000000000000004",
		Name:          "Bank GBP Account",
		ClientID:      2,
		EmployeeID:    3,
		Balance:       1_000_000_000.00,
		ExpiresAt:     time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrencyCode:  model.GBP,
		AccountType:   model.AccountTypeBank,
		AccountKind:   model.AccountKindInternal,
		DailyLimit:    1e12,
		MonthlyLimit:  1e13,
	},
	{
		AccountNumber: "444000000000000005",
		Name:          "Bank JPY Account",
		ClientID:      2,
		EmployeeID:    3,
		Balance:       1_000_000_000.00,
		ExpiresAt:     time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrencyCode:  model.JPY,
		AccountType:   model.AccountTypeBank,
		AccountKind:   model.AccountKindInternal,
		DailyLimit:    1e12,
		MonthlyLimit:  1e13,
	},
	{
		AccountNumber: "444000000000000006",
		Name:          "Bank CAD Account",
		ClientID:      2,
		EmployeeID:    3,
		Balance:       1_000_000_000.00,
		ExpiresAt:     time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrencyCode:  model.CAD,
		AccountType:   model.AccountTypeBank,
		AccountKind:   model.AccountKindInternal,
		DailyLimit:    1e12,
		MonthlyLimit:  1e13,
	},
	{
		AccountNumber: "444000000000000007",
		Name:          "Bank AUD Account",
		ClientID:      2,
		EmployeeID:    3,
		Balance:       1_000_000_000.00,
		ExpiresAt:     time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrencyCode:  model.AUD,
		AccountType:   model.AccountTypeBank,
		AccountKind:   model.AccountKindInternal,
		DailyLimit:    1e12,
		MonthlyLimit:  1e13,
	},
}

func uintPtr(v uint) *uint {
	return &v
}

func Run(db *gorm.DB) error {
	// seed currencies
	currencyMap := make(map[model.CurrencyCode]uint)
	for _, c := range currencies {
		var existing model.Currency
		err := db.Where("code = ?", c.Code).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := db.Create(&c).Error; err != nil {
				log.Printf("failed to create currency %s: %v", c.Code, err)
				return err
			}
			currencyMap[c.Code] = c.CurrencyID
			log.Printf("created currency: %s", c.Code)
		} else if err != nil {
			log.Printf("failed to query currency %s: %v", c.Code, err)
			return err
		} else {
			currencyMap[existing.Code] = existing.CurrencyID
		}
	}

	// seed work codes
	workCodeMap := make(map[string]uint)
	for _, w := range workCodes {
		var existing model.WorkCode
		err := db.Where("code = ?", w.Code).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := db.Create(&w).Error; err != nil {
				log.Printf("failed to create work code %s: %v", w.Code, err)
				return err
			}
			workCodeMap[w.Code] = w.WorkCodeID
			log.Printf("created work code: %s", w.Code)
		} else if err != nil {
			log.Printf("failed to query work code %s: %v", w.Code, err)
			return err
		} else {
			workCodeMap[existing.Code] = existing.WorkCodeID
			log.Printf("work code already exists: %s", w.Code)
		}
	}

	// seed companies
	for _, c := range companies {
		var existing model.Company
		err := db.Where("registration_number = ?", c.RegistrationNumber).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			company := model.Company{
				Name:               c.Name,
				RegistrationNumber: c.RegistrationNumber,
				TaxNumber:          c.TaxNumber,
				Address:            c.Address,
				OwnerID:            c.OwnerID,
				WorkCodeID:         workCodeMap[c.WorkCodeCode],
			}
			if err := db.Create(&company).Error; err != nil {
				log.Printf("failed to create company %s: %v", c.Name, err)
				return err
			}
			log.Printf("created company: %s", c.Name)
		} else if err != nil {
			log.Printf("failed to query company %s: %v", c.Name, err)
			return err
		} else {
			log.Printf("company already exists: %s", c.Name)
		}
	}

	// seed accounts
	for _, a := range accounts {
		var existing model.Account
		err := db.Where("account_number = ?", a.AccountNumber).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			account := model.Account{
				AccountNumber:    a.AccountNumber,
				Name:             a.Name,
				ClientID:         a.ClientID,
				CompanyID:        a.CompanyID,
				EmployeeID:       a.EmployeeID,
				Balance:          a.Balance,
				AvailableBalance: a.Balance,
				ExpiresAt:        a.ExpiresAt,
				CurrencyID:       currencyMap[a.CurrencyCode],
				AccountType:      a.AccountType,
				AccountKind:      a.AccountKind,
				Subtype:          a.Subtype,
				DailyLimit:       a.DailyLimit,
				MonthlyLimit:     a.MonthlyLimit,
			}
			if err := db.Create(&account).Error; err != nil {
				log.Printf("failed to create account %s: %v", a.AccountNumber, err)
				return err
			}
			log.Printf("created account: %s", a.AccountNumber)
		} else if err != nil {
			log.Printf("failed to query account %s: %v", a.AccountNumber, err)
			return err
		} else {
			log.Printf("account already exists: %s", a.AccountNumber)
		}
	}

	return nil
}
