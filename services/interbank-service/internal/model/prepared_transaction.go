package model

import "time"

type PreparedTransactionStatus string

const (
	PreparedTransactionPreparing  PreparedTransactionStatus = "PREPARING"
	PreparedTransactionPrepared   PreparedTransactionStatus = "PREPARED"
	PreparedTransactionCommitted  PreparedTransactionStatus = "COMMITTED"
	PreparedTransactionRolledBack PreparedTransactionStatus = "ROLLED_BACK"
)

type PreparedTransaction struct {
	RoutingNumber int    `gorm:"primaryKey;column:routing_number"`
	ID            string `gorm:"primaryKey;size:64;column:id"`

	Status      PreparedTransactionStatus `gorm:"not null;size:20;column:status"`
	RequestBody []byte                    `gorm:"type:jsonb;not null;column:request_body"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (PreparedTransaction) TableName() string { return "interbank_prepared_transactions" }
