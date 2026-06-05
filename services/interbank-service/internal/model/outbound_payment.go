package model

import "time"

// OutboundPayment links a banking-service transaction to its inter-bank
// outbound 2PC flow. It is a simple join table — state is inferred from
// the associated outbound_messages rows.
type OutboundPayment struct {
	ID                uint   `gorm:"primaryKey;autoIncrement"`
	TransactionIDKey  string `gorm:"not null;size:64;uniqueIndex;column:transaction_id_key"`
	BankingTxID       uint64 `gorm:"not null;column:banking_tx_id"`
	PeerRoutingNumber int    `gorm:"not null;column:peer_routing_number"`
	CreatedAt         time.Time
}

func (OutboundPayment) TableName() string { return "interbank_outbound_payments" }
