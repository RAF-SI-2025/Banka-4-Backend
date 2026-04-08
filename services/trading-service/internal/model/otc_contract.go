package model

import "time"

type OtcContractStatus string
type OtcShareReservationStatus string
type OtcExecutionStep string
type OtcExecutionStatus string

const (
	OtcContractStatusActive    OtcContractStatus = "ACTIVE"
	OtcContractStatusExercised OtcContractStatus = "EXERCISED"
	OtcContractStatusExpired   OtcContractStatus = "EXPIRED"
	OtcContractStatusCancelled OtcContractStatus = "CANCELLED"
)

const (
	OtcShareReservationStatusActive   OtcShareReservationStatus = "ACTIVE"
	OtcShareReservationStatusConsumed OtcShareReservationStatus = "CONSUMED"
	OtcShareReservationStatusReleased OtcShareReservationStatus = "RELEASED"
)

const (
	OtcExecutionStepInit                 OtcExecutionStep = "INIT"
	OtcExecutionStepFundsReserved        OtcExecutionStep = "FUNDS_RESERVED"
	OtcExecutionStepSharesConfirmed      OtcExecutionStep = "SHARES_CONFIRMED"
	OtcExecutionStepFundsCommitted       OtcExecutionStep = "FUNDS_COMMITTED"
	OtcExecutionStepOwnershipTransferred OtcExecutionStep = "OWNERSHIP_TRANSFERRED"
	OtcExecutionStepCompleted            OtcExecutionStep = "COMPLETED"
)

const (
	OtcExecutionStatusInProgress   OtcExecutionStatus = "IN_PROGRESS"
	OtcExecutionStatusCompensating OtcExecutionStatus = "COMPENSATING"
	OtcExecutionStatusCompleted    OtcExecutionStatus = "COMPLETED"
	OtcExecutionStatusFailed       OtcExecutionStatus = "FAILED"
)

type OtcContract struct {
	OtcContractID       uint              `gorm:"primaryKey;autoIncrement"`
	BuyerIdentityID     uint              `gorm:"not null;index"`
	BuyerOwnerType      OwnerType         `gorm:"not null;size:10"`
	SellerIdentityID    uint              `gorm:"not null;index"`
	SellerOwnerType     OwnerType         `gorm:"not null;size:10"`
	SellerAccountNumber string            `gorm:"not null;size:18"`
	AssetID             uint              `gorm:"not null;index"`
	Asset               Asset             `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	Quantity            uint              `gorm:"not null"`
	StrikePrice         float64           `gorm:"not null"`
	Premium             float64           `gorm:"not null"`
	TradeCurrencyCode   string            `gorm:"not null;size:4"`
	SettlementDate      time.Time         `gorm:"not null"`
	Status              OtcContractStatus `gorm:"not null;size:20"`
	ExercisedAt         *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type OtcShareReservation struct {
	OtcShareReservationID uint                      `gorm:"primaryKey;autoIncrement"`
	ContractID            uint                      `gorm:"not null;uniqueIndex"`
	Contract              OtcContract               `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	SellerIdentityID      uint                      `gorm:"not null;index"`
	SellerOwnerType       OwnerType                 `gorm:"not null;size:10"`
	AssetID               uint                      `gorm:"not null;index"`
	ReservedAmount        float64                   `gorm:"not null"`
	Status                OtcShareReservationStatus `gorm:"not null;size:20"`
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type OtcExecutionSaga struct {
	OtcExecutionSagaID   uint               `gorm:"primaryKey;autoIncrement"`
	ContractID           uint               `gorm:"not null;uniqueIndex"`
	Contract             OtcContract        `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	ExecutionKey         string             `gorm:"not null;uniqueIndex;size:100"`
	BuyerAccountNumber   string             `gorm:"not null;size:18"`
	CurrentStep          OtcExecutionStep   `gorm:"not null;size:40"`
	Status               OtcExecutionStatus `gorm:"not null;size:20"`
	RetryCount           int                `gorm:"not null;default:0"`
	NextRetryAt          *time.Time
	LastError            string
	BankingReservationID string `gorm:"size:100"`
	CompletedAt          *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}
