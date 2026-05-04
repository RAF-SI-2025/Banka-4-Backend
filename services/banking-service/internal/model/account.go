package model

import (
	"time"
)

type AccountType string
type AccountKind string
type Subtype string

const (
	AccountTypePersonal AccountType = "Personal"
	AccountTypeBusiness AccountType = "Business"
	AccountTypeBank     AccountType = "Bank"
	AccountTypeFund     AccountType = "Fund"
)

const (
	AccountKindCurrent  AccountKind = "Current"
	AccountKindForeign  AccountKind = "Foreign"
	AccountKindInternal AccountKind = "Internal"
)

const (
	SubtypeStandard   Subtype = "Standard"
	SubtypeSavings    Subtype = "Savings"
	SubtypePension    Subtype = "Pension"
	SubtypeYouth      Subtype = "Youth"
	SubtypeStudent    Subtype = "Student"
	SubtypeUnemployed Subtype = "Unemployed"
	SubtypeLLC        Subtype = "LLC"
	SubtypeJointStock Subtype = "JointStock"
	SubtypeFoundation Subtype = "Foundation"
)

const (
	BankCode   = "444"
	BranchCode = "0001"
)

const (
	DefaultDailyLimitRSD   = 250000.0
	DefaultMonthlyLimitRSD = 1000000.0
)

var AccountKindCodes = map[AccountKind]string{
	AccountKindInternal: "0",
	AccountKindCurrent:  "1",
	AccountKindForeign:  "2",
}

var SubtypeTypeCodes = map[Subtype]string{
	SubtypeStandard:   "1",
	SubtypeLLC:        "2",
	SubtypeJointStock: "2",
	SubtypeFoundation: "2",
	SubtypeSavings:    "3",
	SubtypePension:    "4",
	SubtypeYouth:      "5",
	SubtypeStudent:    "6",
	SubtypeUnemployed: "7",
}

var ValidPersonalSubtypes = map[Subtype]bool{
	SubtypeStandard:   true,
	SubtypeSavings:    true,
	SubtypePension:    true,
	SubtypeYouth:      true,
	SubtypeStudent:    true,
	SubtypeUnemployed: true,
}

var ValidBusinessSubtypes = map[Subtype]bool{
	SubtypeLLC:        true,
	SubtypeJointStock: true,
	SubtypeFoundation: true,
}

type Account struct {
	AccountNumber string `gorm:"primaryKey;size:18"`
	Name          string
	ClientID      uint `gorm:"not null;index"`

	CompanyID *uint `gorm:"index"`
	Company   *Company

	EmployeeID uint `gorm:"not null"`

	CurrencyID uint `gorm:"index;not null"`
	Currency   Currency

	Balance          float64 `gorm:"not null;default:0"`
	AvailableBalance float64 `gorm:"not null;default:0"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
	ExpiresAt time.Time `gorm:"not null"`

	Status string `gorm:"not null;default:Active"`

	AccountType AccountType `gorm:"not null;size:20"`
	AccountKind AccountKind `gorm:"not null;size:20"`
	Subtype     Subtype     `gorm:"size:20"`

	MaintenanceFee  float64 `gorm:"not null;default:0"`
	DailyLimit      float64 `gorm:"not null;default:0"`
	MonthlyLimit    float64 `gorm:"not null;default:0"`
	DailySpending   float64 `gorm:"not null;default:0"`
	MonthlySpending float64 `gorm:"not null;default:0"`

	Payees                []Payee             `gorm:"foreignKey:AccountNumber"`
	VerificationTokens    []VerificationToken `gorm:"foreignKey:AccountNumber"`
	CardRequests          []CardRequest       `gorm:"foreignKey:AccountNumber"`
	AuthorizedPersons     []AuthorizedPerson  `gorm:"foreignKey:AccountNumber"`
	LoanRequests          []LoanRequest       `gorm:"foreignKey:AccountNumber"`
	TransactionsRecipient []Transaction       `gorm:"foreignKey:RecipientAccountNumber"`
	TransactionsPayer     []Transaction       `gorm:"foreignKey:PayerAccountNumber"`
	Cards                 []Card              `gorm:"foreignKey:AccountNumber"`
}

func GetSubtypeCode(s Subtype) string {
	if val, ok := SubtypeTypeCodes[s]; ok {
		return val
	}
	return "0"
}

func GetTypeCode(accountKind AccountKind, accountType AccountType, subtype Subtype) string {
	return AccountKindCodes[accountKind] + GetSubtypeCode(subtype)
}

func UpdateBalances(account *Account, amount float64) {
	account.Balance += amount
	account.AvailableBalance += amount
}
