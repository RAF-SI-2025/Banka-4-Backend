package seed

import (
	"errors"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
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
	{Code: "84.1", Description: "Government and public administration"},
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
		Name:               "Banka 4",
		RegistrationNumber: "44444444",
		TaxNumber:          "444444444",
		Address:            "Bankarska ulica bb, Beograd, Srbija",
		OwnerID:            1,
		WorkCodeCode:       "64.1",
	},
	{
		Name:               "Tech DOO",
		RegistrationNumber: "12345678",
		TaxNumber:          "123456789",
		Address:            "Trg Republike 5, Beograd, Srbija",
		OwnerID:            2,
		WorkCodeCode:       "62.0",
	},
	{
		Name:               "Global AD",
		RegistrationNumber: "87654321",
		TaxNumber:          "987654321",
		Address:            "Knez Mihailova 10, Beograd, Srbija",
		OwnerID:            2,
		WorkCodeCode:       "64.1",
	},
	{
		Name:               "Humanitarian Fondacija",
		RegistrationNumber: "11223344",
		TaxNumber:          "112233445",
		Address:            "Bulevar Oslobodjenja 20, Novi Sad, Srbija",
		OwnerID:            2,
		WorkCodeCode:       "85.1",
	},
	{
		Name:               "Republika Srbija",
		RegistrationNumber: "00000001",
		TaxNumber:          "100000001",
		Address:            "Nemanjina 11, Beograd, Srbija",
		OwnerID:            1,
		WorkCodeCode:       "84.1",
	},
}

var loanTypes = []model.LoanType{
	{
		Name:               "Cash Loan",
		Description:        "Unsecured personal loan for general purposes",
		BankMargin:         1.75,
		BaseInterestRate:   5.0,
		MinRepaymentPeriod: 6,
		MaxRepaymentPeriod: 84,
	},
	{
		Name:               "Mortgage Loan",
		Description:        "Loan for purchasing or refinancing real estate",
		BankMargin:         1.50,
		BaseInterestRate:   3.5,
		MinRepaymentPeriod: 60,
		MaxRepaymentPeriod: 360,
	},
	{
		Name:               "Car Loan",
		Description:        "Loan for purchasing a vehicle",
		BankMargin:         1.25,
		BaseInterestRate:   4.0,
		MinRepaymentPeriod: 12,
		MaxRepaymentPeriod: 84,
	},
	{
		Name:               "Refinancing Loan",
		Description:        "Loan used to refinance existing debts under better terms",
		BankMargin:         1.00,
		BaseInterestRate:   4.5,
		MinRepaymentPeriod: 12,
		MaxRepaymentPeriod: 120,
	},
	{
		Name:               "Student Loan",
		Description:        "Loan intended for education-related expenses",
		BankMargin:         0.75,
		BaseInterestRate:   2.5,
		MinRepaymentPeriod: 12,
		MaxRepaymentPeriod: 120,
	},
}

func getUintPointer(value uint) *uint {
	return &value
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
		ClientID:      5,
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
		ClientID:      3,
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
		ClientID:      2,
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
		ClientID:      3,
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
		ClientID:      1,
		CompanyID:     getUintPointer(1),
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
		ClientID:      1,
		CompanyID:     getUintPointer(1),
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
		ClientID:      1,
		CompanyID:     getUintPointer(1),
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
		ClientID:      1,
		CompanyID:     getUintPointer(1),
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
		ClientID:      1,
		CompanyID:     getUintPointer(1),
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
		ClientID:      1,
		CompanyID:     getUintPointer(1),
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
		ClientID:      1,
		CompanyID:     getUintPointer(1),
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
		ClientID:      1,
		CompanyID:     getUintPointer(1),
		EmployeeID:    3,
		Balance:       1_000_000_000.00,
		ExpiresAt:     time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrencyCode:  model.AUD,
		AccountType:   model.AccountTypeBank,
		AccountKind:   model.AccountKindInternal,
		DailyLimit:    1e12,
		MonthlyLimit:  1e13,
	},
	{
		AccountNumber: "444000000000000008",
		Name:          "Republika Srbija Tax Account",
		ClientID:      6,
		CompanyID:     getUintPointer(5),
		EmployeeID:    3,
		Balance:       0,
		ExpiresAt:     time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrencyCode:  model.RSD,
		AccountType:   model.AccountTypeBank,
		AccountKind:   model.AccountKindInternal,
		DailyLimit:    1e12,
		MonthlyLimit:  1e13,
	},
	{
		AccountNumber: "444000000000000010",
		Name:          "Alpha Growth Fund Account",
		ClientID:      1,
		CompanyID:     getUintPointer(1),
		EmployeeID:    3,
		Balance:       1500000.00,
		ExpiresAt:     time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrencyCode:  model.RSD,
		AccountType:   model.AccountTypeFund,
		AccountKind:   model.AccountKindInternal,
		DailyLimit:    1e12,
		MonthlyLimit:  1e13,
	},
	{
		AccountNumber: "444000000000000011",
		Name:          "Beta Stable Fund Account",
		ClientID:      1,
		CompanyID:     getUintPointer(1),
		EmployeeID:    3,
		Balance:       3000000.00,
		ExpiresAt:     time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrencyCode:  model.RSD,
		AccountType:   model.AccountTypeFund,
		AccountKind:   model.AccountKindInternal,
		DailyLimit:    1e12,
		MonthlyLimit:  1e13,
	},
}

var seedTransactions = []struct {
	Transaction model.Transaction
	Payment     model.Payment
}{
	{
		Transaction: model.Transaction{
			PayerAccountNumber:     "444000112345678911",
			RecipientAccountNumber: "444000112345678913",
			StartAmount:            5000.00,
			StartCurrencyCode:      "RSD",
			EndAmount:              5000.00,
			EndCurrencyCode:        "RSD",
			Status:                 model.TransactionCompleted,
			CreatedAt:              time.Now().AddDate(0, 0, -10),
		},
		Payment: model.Payment{
			RecipientName:   "Petar Petrovic",
			ReferenceNumber: "97-1234567890",
			PaymentCode:     "289",
			Purpose:         "Prenos na stedni racun",
		},
	},
	{
		Transaction: model.Transaction{
			PayerAccountNumber:     "444000112345678911",
			RecipientAccountNumber: "444000112345678922",
			StartAmount:            12000.00,
			StartCurrencyCode:      "RSD",
			EndAmount:              12000.00,
			EndCurrencyCode:        "RSD",
			Status:                 model.TransactionCompleted,
			CreatedAt:              time.Now().AddDate(0, 0, -7),
		},
		Payment: model.Payment{
			RecipientName:   "Marko Markovic",
			ReferenceNumber: "97-9876543210",
			PaymentCode:     "289",
			Purpose:         "Uplata za kiriju",
		},
	},
	{
		Transaction: model.Transaction{
			PayerAccountNumber:     "444000112345678913",
			RecipientAccountNumber: "444000112345678911",
			StartAmount:            3000.00,
			StartCurrencyCode:      "RSD",
			EndAmount:              3000.00,
			EndCurrencyCode:        "RSD",
			Status:                 model.TransactionCompleted,
			CreatedAt:              time.Now().AddDate(0, 0, -3),
		},
		Payment: model.Payment{
			RecipientName:   "Petar Petrovic",
			ReferenceNumber: "",
			PaymentCode:     "289",
			Purpose:         "Povracaj novca",
		},
	},
	{
		Transaction: model.Transaction{
			PayerAccountNumber:     "444000112345678911",
			RecipientAccountNumber: "444000112345678922",
			StartAmount:            8500.00,
			StartCurrencyCode:      "RSD",
			EndAmount:              8500.00,
			EndCurrencyCode:        "RSD",
			Status:                 model.TransactionProcessing,
			CreatedAt:              time.Now().AddDate(0, 0, -1),
		},
		Payment: model.Payment{
			RecipientName:   "Ana Jovanovic",
			ReferenceNumber: "97-1111111111",
			PaymentCode:     "221",
			Purpose:         "Uplata za casove",
		},
	},
	{
		Transaction: model.Transaction{
			PayerAccountNumber:     "444000112345678911",
			RecipientAccountNumber: "444000112345678921",
			StartAmount:            50000.00,
			StartCurrencyCode:      "RSD",
			EndAmount:              50000.00,
			EndCurrencyCode:        "RSD",
			Status:                 model.TransactionRejected,
			CreatedAt:              time.Now().AddDate(0, 0, -5),
		},
		Payment: model.Payment{
			RecipientName:   "Nepoznat Primalac",
			ReferenceNumber: "",
			PaymentCode:     "289",
			Purpose:         "Sumnjiva transakcija",
		},
	},
}

var seedVerificationTokens = []model.VerificationToken{
	{
		ClientID:        7,
		AccountNumber:   "444000112345678911",
		NewDailyLimit:   500000.00,
		NewMonthlyLimit: 2000000.00,
	},
	{
		ClientID:        1,
		AccountNumber:   "444000112345678913",
		NewDailyLimit:   300000.00,
		NewMonthlyLimit: 1500000.00,
	},
	{
		ClientID:        1,
		AccountNumber:   "444000112345678911",
		NewDailyLimit:   100000.00,
		NewMonthlyLimit: 500000.00,
	},
}

// UNUSED
//func uintPtr(v uint) *uint {
//	return &v
//}

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

	// seed loan types
	for _, lt := range loanTypes {
		var existing model.LoanType

		err := db.Where("name = ?", lt.Name).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			loanType := model.LoanType{
				Name:               lt.Name,
				Description:        lt.Description,
				BankMargin:         lt.BankMargin,
				BaseInterestRate:   lt.BaseInterestRate,
				MinRepaymentPeriod: lt.MinRepaymentPeriod,
				MaxRepaymentPeriod: lt.MaxRepaymentPeriod,
			}

			if err := db.Create(&loanType).Error; err != nil {
				log.Printf("failed to create loan type %s: %v", lt.Name, err)
				return err
			}
			log.Printf("created loan type: %s", lt.Name)

		} else if err != nil {
			log.Printf("failed to query loan type %s: %v", lt.Name, err)
			return err
		} else {
			log.Printf("loan type already exists: %s", lt.Name)
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

	// seed transactions and payments
	var txCount int64
	if err := db.Model(&model.Transaction{}).Count(&txCount).Error; err != nil {
		return err
	}
	if txCount == 0 {
		for _, s := range seedTransactions {
			tx := s.Transaction
			if err := db.Create(&tx).Error; err != nil {
				log.Printf("failed to create transaction: %v", err)
				return err
			}
			payment := s.Payment
			payment.TransactionID = tx.TransactionID
			if err := db.Create(&payment).Error; err != nil {
				log.Printf("failed to create payment: %v", err)
				return err
			}
			log.Printf("created payment %d linked to transaction %d", payment.PaymentID, tx.TransactionID)
		}
	}

	// seed limit verification tokens
	var tokenCount int64
	if err := db.Model(&model.VerificationToken{}).Count(&tokenCount).Error; err != nil {
		return err
	}
	if tokenCount == 0 {
		for _, t := range seedVerificationTokens {
			token := t
			if err := db.Create(&token).Error; err != nil {
				log.Printf("failed to create verification token: %v", err)
				return err
			}
			log.Printf("created verification token: client=%d account=%s", token.ClientID, token.AccountNumber)
		}
	}

	return nil
}
