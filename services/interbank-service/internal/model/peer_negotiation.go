package model

import "time"

// PeerNegotiationStatus tracks the lifecycle of a cross-bank OTC negotiation.
type PeerNegotiationStatus string

const (
	// PeerNegotiationOngoing — back-and-forth between the two banks is in progress.
	PeerNegotiationOngoing PeerNegotiationStatus = "ONGOING"
	// PeerNegotiationAccepted — one side accepted; option contract has been formed.
	PeerNegotiationAccepted PeerNegotiationStatus = "ACCEPTED"
	// PeerNegotiationCancelled — one side withdrew (§3.5 DELETE).
	PeerNegotiationCancelled PeerNegotiationStatus = "CANCELLED"
	// PeerNegotiationExpired — settlement date passed without acceptance.
	PeerNegotiationExpired PeerNegotiationStatus = "EXPIRED"
)

// PeerNegotiation is a cross-bank OTC negotiation entity. It lives entirely
// in interbank-service and is intentionally separate from trading-service's
// same-bank OtcOffer
//
// Identification follows spec §3.2: the ID is an opaque string assigned by
// the seller's bank (the authoritative party). Both banks store the same ID —
// the buyer's mirror row uses the seller-assigned ID directly.
//
// Each party — buyer, seller, last-modifier — is encoded as the protocol's
// ForeignBankId pair (routing number + opaque id). A row may represent a
// negotiation where both parties live in our bank (router == ours for all),
// or any cross-bank combination.
type PeerNegotiation struct {
	// ID is the negotiation identifier assigned by the seller's bank (§3.2).
	// Both authoritative and mirror rows use this same value.
	ID string `gorm:"primaryKey;size:64;column:id"`

	// Parties — flat (routing, id) encoding of ForeignBankId.
	BuyerRoutingNumber  int    `gorm:"not null;index;column:buyer_routing_number"`
	BuyerID             string `gorm:"not null;size:64;column:buyer_id"`
	SellerRoutingNumber int    `gorm:"not null;index;column:seller_routing_number"`
	SellerID            string `gorm:"not null;size:64;column:seller_id"`

	// Offer contents (§3.2 OtcOffer body).
	Ticker             string  `gorm:"not null;size:16;column:ticker"`
	Amount             int     `gorm:"not null;column:amount"`
	PricePerStock      float64 `gorm:"not null;column:price_per_stock"`
	PriceCurrency      string  `gorm:"not null;size:8;column:price_currency"`
	Premium            float64 `gorm:"not null;column:premium"`
	PremiumCurrency    string  `gorm:"not null;size:8;column:premium_currency"`
	SettlementDate     string  `gorm:"not null;column:settlement_date"`
	BuyerAccountNumber string  `gorm:"not null;size:64;column:buyer_account_number"`

	// Tracking the last modifier — required for §3.3 turn enforcement
	// ("the party opposite to lastModifiedBy may post the next update").
	LastModifiedByRouting int    `gorm:"not null;column:last_modified_by_routing"`
	LastModifiedByID      string `gorm:"not null;size:64;column:last_modified_by_id"`

	// State.
	Status          PeerNegotiationStatus `gorm:"not null;size:16;column:status"`
	IsAuthoritative bool                  `gorm:"not null;column:is_authoritative"`

	// Version is a reserved counter column. NOTE: plain GORM does not treat a
	// field named Version as an optimistic-lock column (that needs the
	// gorm.io/plugin/optimisticlock type), so this field is currently inert and
	// never incremented. Concurrent counter-offers are actually serialized via
	// FindByIDForUpdate (SELECT ... FOR UPDATE) inside UpdateCounter, which is
	// sufficient. Kept for a future migration to real optimistic locking.
	Version uint `gorm:"not null;default:0;column:version"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (PeerNegotiation) TableName() string { return "interbank_peer_negotiations" }
