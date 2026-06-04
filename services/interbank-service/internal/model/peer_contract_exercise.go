package model

import "time"

type PeerExerciseStep string
type PeerExerciseStatus string

const (
	PeerExerciseStepInit                 PeerExerciseStep = "INIT"
	PeerExerciseStepFundsReserved        PeerExerciseStep = "FUNDS_RESERVED"
	PeerExerciseStepSharesConfirmed      PeerExerciseStep = "SHARES_CONFIRMED"
	PeerExerciseStepFundsCommitted       PeerExerciseStep = "FUNDS_COMMITTED"
	PeerExerciseStepOwnershipTransferred PeerExerciseStep = "OWNERSHIP_TRANSFERRED"
	PeerExerciseStepCompleted            PeerExerciseStep = "COMPLETED"
)

const (
	PeerExerciseInProgress   PeerExerciseStatus = "IN_PROGRESS"
	PeerExerciseCompensating PeerExerciseStatus = "COMPENSATING"
	PeerExerciseCompleted    PeerExerciseStatus = "COMPLETED"
	PeerExerciseFailed       PeerExerciseStatus = "FAILED"
)

type PeerContractExercise struct {
	ID uint `gorm:"primaryKey;autoIncrement"`

	ContractAuthorityRoutingNumber int    `gorm:"not null;uniqueIndex:idx_peer_contract_exercise_contract;column:contract_authority_routing_number"`
	ContractID                     string `gorm:"not null;size:64;uniqueIndex:idx_peer_contract_exercise_contract;column:contract_id"`
	ExecutionKey                   string `gorm:"not null;uniqueIndex;size:128;column:execution_key"`

	CurrentStep PeerExerciseStep   `gorm:"not null;size:40;column:current_step"`
	Status      PeerExerciseStatus `gorm:"not null;size:20;column:status"`
	RetryCount  int                `gorm:"not null;default:0;column:retry_count"`
	LastError   string             `gorm:"column:last_error"`
	CompletedAt *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (PeerContractExercise) TableName() string { return "interbank_peer_contract_exercises" }
