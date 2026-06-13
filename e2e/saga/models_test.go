//go:build saga_e2e

package saga_e2e

// Local row mappings for the tables the chaos tests touch. The services keep
// their GORM models in internal packages, which cannot be imported across
// module boundaries, so the suite mirrors just the fields it needs. Field
// names match the service models exactly so GORM's naming strategy produces
// the same column names; explicit column tags mirror the services' overrides.

import "time"

const (
	sagaStatusInProgress   = "IN_PROGRESS"
	sagaStatusCompensating = "COMPENSATING"
	sagaStatusCompleted    = "COMPLETED"
	sagaStatusFailed       = "FAILED"

	sagaStepInit            = "INIT"
	sagaStepFundsReserved   = "FUNDS_RESERVED"
	sagaStepSharesConfirmed = "SHARES_CONFIRMED"

	logOutcomeOK  = "OK"
	logOutcomeErr = "ERR"

	contractStatusActive    = "ACTIVE"
	contractStatusExercised = "EXERCISED"

	ownerTypeClient = "CLIENT"

	shareReservationActive = "ACTIVE"

	fundsReservationReserved = "RESERVED"

	// Contract economics shared by every fixture: qty 10 x strike 300 RSD.
	sagaTradeAmount = 3000.0
)

type tradeAsset struct {
	AssetID   uint `gorm:"primaryKey;autoIncrement"`
	Ticker    string
	Name      string
	AssetType string
}

func (tradeAsset) TableName() string { return "assets" }

type tradeStock struct {
	StockID           uint `gorm:"primaryKey;autoIncrement"`
	AssetID           uint
	OutstandingShares float64
	DividendYield     float64
}

func (tradeStock) TableName() string { return "stocks" }

type tradeAssetOwnership struct {
	AssetOwnershipID uint `gorm:"primaryKey;autoIncrement"`
	UserId           uint
	OwnerType        string
	AssetID          uint
	Amount           float64
	AvgBuyPriceRSD   float64
	PublicAmount     float64
	ReservedAmount   float64
	UpdatedAt        time.Time
}

func (tradeAssetOwnership) TableName() string { return "asset_ownerships" }

type tradeOtcOptionContract struct {
	OtcOptionContractID uint `gorm:"primaryKey;autoIncrement"`
	OtcOfferID          uint
	BuyerID             uint
	SellerID            uint
	StockAssetID        uint
	Amount              int
	StrikePriceRSD      float64 `gorm:"column:strike_price"`
	PremiumRSD          float64 `gorm:"column:premium"`
	SettlementDate      time.Time
	BuyerAccountNumber  string
	SellerAccountNumber string
	Status              string
	ExercisedAt         *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

func (tradeOtcOptionContract) TableName() string { return "otc_option_contracts" }

type tradeOtcShareReservation struct {
	OtcShareReservationID uint `gorm:"primaryKey;autoIncrement"`
	ContractID            uint
	SellerID              uint
	OwnerType             string
	StockAssetID          uint
	ReservedAmount        float64
	Status                string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func (tradeOtcShareReservation) TableName() string { return "otc_share_reservations" }

type tradeOtcExecutionSaga struct {
	OtcExecutionSagaID uint `gorm:"primaryKey;autoIncrement"`
	ContractID         uint
	ExecutionKey       string
	CurrentStep        string
	Status             string
	RetryCount         int
	NextRetryAt        *time.Time
	LastError          string
	FaultSpec          string
	CompletedAt        *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (tradeOtcExecutionSaga) TableName() string { return "otc_execution_sagas" }

type tradeOtcExecutionSagaLogEntry struct {
	OtcExecutionSagaLogEntryID uint `gorm:"primaryKey;autoIncrement"`
	OtcExecutionSagaID         uint
	Step                       string
	Outcome                    string
	Error                      string
	CreatedAt                  time.Time
}

func (tradeOtcExecutionSagaLogEntry) TableName() string { return "otc_execution_saga_log_entries" }

type bankCurrency struct {
	CurrencyID  uint `gorm:"primaryKey"`
	Name        string
	Code        string
	Symbol      string
	Country     string
	Description string
	Status      string
}

func (bankCurrency) TableName() string { return "currencies" }

type bankAccount struct {
	AccountNumber    string `gorm:"primaryKey;size:18"`
	Name             string
	ClientID         uint
	EmployeeID       uint
	CurrencyID       uint
	Balance          float64
	AvailableBalance float64
	CreatedAt        time.Time
	ExpiresAt        time.Time
	Status           string
	AccountType      string
	AccountKind      string
	Subtype          string
	MaintenanceFee   float64
	DailyLimit       float64
	MonthlyLimit     float64
	DailySpending    float64
	MonthlySpending  float64
}

func (bankAccount) TableName() string { return "accounts" }

type bankOtcFundsReservation struct {
	OtcFundsReservationID uint `gorm:"primaryKey;autoIncrement"`
	ExecutionID           string
	Status                string
}

func (bankOtcFundsReservation) TableName() string { return "otc_funds_reservations" }
