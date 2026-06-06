package model

import "time"

// OutboundMessageStatus tracks the lifecycle of an outbox row.
type OutboundMessageStatus string

const (
	OutboundPending  OutboundMessageStatus = "PENDING"  // ready to be sent / re-sent
	OutboundSent     OutboundMessageStatus = "SENT"     // peer confirmed (200/204)
	OutboundCanceled OutboundMessageStatus = "CANCELED" // marked not-to-send before any send attempt (e.g., on rollback)
	OutboundFailed   OutboundMessageStatus = "FAILED"   // gave up after exceeding max attempts
)

// FlowType labels which higher-level flow an outbound message belongs to. Both
// flows are driven identically through the MessageProcessor 2PC; the label is
// kept for observability only.
const (
	FlowTypeOTC     = "OTC"
	FlowTypePayment = "PAYMENT"
)

// OutboundMessage is a row in the message outbox. The sender records every
// outgoing message in the same local transaction as the preparation it
// belongs to, and a background worker drains the queue.
type OutboundMessage struct {
	ID                  uint   `gorm:"primaryKey"`
	PeerRoutingNumber   int    `gorm:"not null;index;column:peer_routing_number"`
	MessageType         string `gorm:"not null;size:32;column:message_type"`
	IdempotenceKeyLocal string `gorm:"not null;size:64;column:idempotence_key_local;uniqueIndex"`
	Payload             []byte `gorm:"type:jsonb;not null;column:payload"`
	FlowType            string `gorm:"not null;size:16;default:'PAYMENT';column:flow_type"`
	// BankingTxID links a PAYMENT NEW_TX row back to the banking-service
	// transaction that initiated it, so the outbox can report the final 2PC
	// outcome to banking. 0 for OTC and follow-up (COMMIT/ROLLBACK) rows.
	BankingTxID        uint64                `gorm:"not null;default:0;column:banking_tx_id"`
	Status             OutboundMessageStatus `gorm:"not null;size:16;index;column:status"`
	Attempts           int                   `gorm:"not null;default:0;column:attempts"`
	NextRetryAt        time.Time             `gorm:"not null;index;column:next_retry_at"`
	LastResponseStatus int                   `gorm:"column:last_response_status"`
	LastResponseBody   []byte                `gorm:"type:jsonb;column:last_response_body"`
	LastError          string                `gorm:"column:last_error"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (OutboundMessage) TableName() string { return "interbank_outbound_messages" }
