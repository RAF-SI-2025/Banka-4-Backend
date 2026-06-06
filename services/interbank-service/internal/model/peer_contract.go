package model

import "time"

type PeerContractStatus string

const (
	PeerContractActive    PeerContractStatus = "ACTIVE"
	PeerContractExercised PeerContractStatus = "EXERCISED"
	PeerContractExpired   PeerContractStatus = "EXPIRED"
	PeerContractCancelled PeerContractStatus = "CANCELLED"
)

// PeerContract is the accepted cross-bank OTC option contract. The contract
// id is owned by the seller's bank and currently matches the authoritative
// negotiation id from which the contract was created.
type PeerContract struct {
	AuthorityRoutingNumber int    `gorm:"primaryKey;column:authority_routing_number"`
	ID                     string `gorm:"primaryKey;size:64;column:id"`

	NegotiationID string `gorm:"not null;size:64;index;column:negotiation_id"`

	BuyerRoutingNumber  int    `gorm:"not null;index;column:buyer_routing_number"`
	BuyerID             string `gorm:"not null;size:64;column:buyer_id"`
	SellerRoutingNumber int    `gorm:"not null;index;column:seller_routing_number"`
	SellerID            string `gorm:"not null;size:64;column:seller_id"`

	Ticker          string  `gorm:"not null;size:16;column:ticker"`
	Amount          int     `gorm:"not null;column:amount"`
	StrikePrice     float64 `gorm:"not null;column:strike_price"`
	StrikeCurrency  string  `gorm:"not null;size:8;column:strike_currency"`
	Premium         float64 `gorm:"not null;column:premium"`
	PremiumCurrency string  `gorm:"not null;size:8;column:premium_currency"`
	SettlementDate  string  `gorm:"not null;column:settlement_date"`

	// Account numbers are local-only execution hints. A bank should only rely on
	// the account number owned by its own routing number.
	BuyerAccountNumber  *string `gorm:"size:64;column:buyer_account_number"`
	SellerAccountNumber *string `gorm:"size:64;column:seller_account_number"`

	Status          PeerContractStatus `gorm:"not null;size:20;column:status"`
	IsAuthoritative bool               `gorm:"not null;column:is_authoritative"`
	ExercisedAt     *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (PeerContract) TableName() string { return "interbank_peer_contracts" }
