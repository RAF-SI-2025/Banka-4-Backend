package model

import "time"

type OtcShareReservationStatus string
type OtcExecutionStep string
type OtcExecutionStatus string

// OtcShareReservationStatus values track the lifecycle of seller shares that
// have been locked behind an OTC contract.
//
//   - ACTIVE   — shares are reserved for one specific contract and must not
//     be counted as available for new offers/orders.
//   - CONSUMED — the contract was exercised; ownership has moved to the buyer.
//     The reservation is kept for audit but no longer holds shares.
//   - RELEASED — the contract expired or the SAGA failed; shares are returned
//     to the seller's available pool.
//
// Only ACTIVE rows count toward seller capacity in
// SumActiveReservedBySellerAsset.
const (
	OtcShareReservationStatusActive   OtcShareReservationStatus = "ACTIVE"
	OtcShareReservationStatusConsumed OtcShareReservationStatus = "CONSUMED"
	OtcShareReservationStatusReleased OtcShareReservationStatus = "RELEASED"
)

// OtcExecutionStep records how far the SAGA has progressed. The five spec
// steps map to FUNDS_RESERVED through COMPLETED; INIT is the pre-run state
// before any remote call has been made.
//
//   - INIT                  — saga row created, no side effects yet.
//   - FUNDS_RESERVED        — banking-service has frozen buyer funds.
//   - SHARES_CONFIRMED      — re-validated seller still holds the reserved shares.
//   - FUNDS_COMMITTED       — banking-service has moved funds buyer→seller.
//   - OWNERSHIP_TRANSFERRED — trading-service has moved shares seller→buyer.
//   - COMPLETED             — final state-update step (spec "double check") done.
//
// On compensation, CurrentStep tells us what to undo: e.g. status=COMPENSATING
// with step=FUNDS_COMMITTED means "funds were moved, must call RefundOtcFunds".
const (
	OtcExecutionStepInit                 OtcExecutionStep = "INIT"
	OtcExecutionStepFundsReserved        OtcExecutionStep = "FUNDS_RESERVED"
	OtcExecutionStepSharesConfirmed      OtcExecutionStep = "SHARES_CONFIRMED"
	OtcExecutionStepFundsCommitted       OtcExecutionStep = "FUNDS_COMMITTED"
	OtcExecutionStepOwnershipTransferred OtcExecutionStep = "OWNERSHIP_TRANSFERRED"
	OtcExecutionStepCompleted            OtcExecutionStep = "COMPLETED"
)

// OtcExecutionStatus tracks the direction the SAGA is moving in. It is kept
// as a separate field from CurrentStep because the same step can be either
// "we just got here going forward" or "we are unwinding through here".
//
//   - IN_PROGRESS   — saga is moving forward, next step will be attempted.
//   - COMPENSATING  — a step failed; the saga is rolling back. The compensation
//     to run is determined by CurrentStep at the time the failure occurred.
//   - COMPLETED     — terminal success.
//   - FAILED        — terminal failure; compensations (if any) have been run.
//
// Transient errors (e.g. banking unavailable) keep status=IN_PROGRESS and set
// NextRetryAt; the worker re-picks the saga later. Only terminal banking
// errors or exhausted compensation flips status to FAILED.
const (
	OtcExecutionStatusInProgress   OtcExecutionStatus = "IN_PROGRESS"
	OtcExecutionStatusCompensating OtcExecutionStatus = "COMPENSATING"
	OtcExecutionStatusCompleted    OtcExecutionStatus = "COMPLETED"
	OtcExecutionStatusFailed       OtcExecutionStatus = "FAILED"
)

// OtcShareReservation locks a specific quantity of seller shares to one OTC
// contract. ContractID is unique because each contract has exactly one
// reservation. SellerID + StockAssetID are indexed because seller-capacity
// validation sums all ACTIVE rows for a (seller, stock) pair.
type OtcShareReservation struct {
	OtcShareReservationID uint                      `gorm:"primaryKey;autoIncrement"`
	ContractID            uint                      `gorm:"not null;uniqueIndex"`
	Contract              OtcOptionContract         `gorm:"foreignKey:ContractID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	SellerID              uint                      `gorm:"not null;index"`
	OwnerType             OwnerType                 `gorm:"not null;size:10"`
	StockAssetID          uint                      `gorm:"not null;index"`
	ReservedAmount        float64                   `gorm:"not null"`
	Status                OtcShareReservationStatus `gorm:"not null;size:20"`
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// OtcExecutionSaga is the durable state machine that drives one execution
// attempt of an OTC contract. The worker reads pending rows on each tick and
// resumes them — this is what makes the SAGA crash-safe across restarts.
//
// ContractID is unique: at most one saga per contract. After a FAILED state,
// a new ExerciseContract call resets this same row in place (new ExecutionKey,
// step=INIT, retries=0) rather than creating a second saga.
//
// ExecutionKey is the handle banking-service uses to look up its
// OtcFundsReservation. It is regenerated on each fresh attempt so that a
// retry after FAILED creates a brand new banking-side reservation rather than
// trying to reuse a dead one.
//
// RetryCount + NextRetryAt implement the retry mechanism the spec mandates
// ("sistem mora da ima retry mehanizam dok sredstva ne budu vraćena").
// LastError is the most recent error string, kept for ops debugging.
// FaultSpec carries a serialized fault-injection plan (see internal/faultinject)
// for this execution. It is only ever populated when the service runs with
// SAGA_FAULT_INJECTION enabled (test builds); in production it stays empty.
// Persisting it on the saga row lets injected faults survive worker restarts,
// which the chaos tests rely on.
type OtcExecutionSaga struct {
	OtcExecutionSagaID uint               `gorm:"primaryKey;autoIncrement"`
	ContractID         uint               `gorm:"not null;uniqueIndex"`
	Contract           OtcOptionContract  `gorm:"foreignKey:ContractID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	ExecutionKey       string             `gorm:"not null;uniqueIndex;size:100"`
	CurrentStep        OtcExecutionStep   `gorm:"not null;size:40"`
	Status             OtcExecutionStatus `gorm:"not null;size:20"`
	RetryCount         int                `gorm:"not null;default:0"`
	NextRetryAt        *time.Time
	LastError          string
	FaultSpec          string `gorm:"size:500"`
	CompletedAt        *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type OtcExecutionLogOutcome string

const (
	OtcExecutionLogOutcomeOK  OtcExecutionLogOutcome = "OK"
	OtcExecutionLogOutcomeErr OtcExecutionLogOutcome = "ERR"
)

// OtcExecutionSagaLogEntry is one record per attempted saga step, forward or
// compensating, in execution order (primary key order). The SAGA test spec
// labels steps F1..F5 for the forward phases and C1/C3 for the implemented
// compensators (release reserved funds, refund committed funds); F2/F4/F5
// apply their local effects transactionally and therefore have no separate
// compensator rows.
//
// Step label mapping to the execution flow:
//
//	F1 — reserve buyer funds in banking      C1 — release the funds reservation
//	F2 — re-validate seller share coverage
//	F3 — commit the funds transfer           C3 — refund the committed transfer
//	F4 — transfer share ownership locally
//	F5 — finalize the saga record
type OtcExecutionSagaLogEntry struct {
	OtcExecutionSagaLogEntryID uint                   `gorm:"primaryKey;autoIncrement"`
	OtcExecutionSagaID         uint                   `gorm:"not null;index"`
	Step                       string                 `gorm:"not null;size:8"`
	Outcome                    OtcExecutionLogOutcome `gorm:"not null;size:8"`
	Error                      string
	CreatedAt                  time.Time
}
