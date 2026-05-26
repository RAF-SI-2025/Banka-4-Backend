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
// the seller's bank (the authoritative party). Buyer-side mirrors carry
// the same ID in their RemoteNegotiationID to enable cross-referencing.
//
// Each party — buyer, seller, last-modifier — is encoded as the protocol's
// ForeignBankId pair (routing number + opaque id). A row may represent a
// negotiation where both parties live in our bank (router == ours for all),
// or any cross-bank combination.
type PeerNegotiation struct {
	// ID is the negotiation identifier owned by the authoritative bank
	// (the seller's bank, per §3.2). For mirrors held by the buyer's
	// bank, this is the same value that the seller's bank generated.
	ID string `gorm:"primaryKey;size:64;column:id"`

	// Parties — flat (routing, id) encoding of ForeignBankId.
	BuyerRoutingNumber  int    `gorm:"not null;index;column:buyer_routing_number"`
	BuyerID             string `gorm:"not null;size:64;column:buyer_id"`
	SellerRoutingNumber int    `gorm:"not null;index;column:seller_routing_number"`
	SellerID            string `gorm:"not null;size:64;column:seller_id"`

	// Offer contents (§3.2 OtcOffer body).
	Ticker          string  `gorm:"not null;size:16;column:ticker"`
	Amount          int     `gorm:"not null;column:amount"`
	PricePerStock   float64 `gorm:"not null;column:price_per_stock"`
	PriceCurrency   string  `gorm:"not null;size:8;column:price_currency"`
	Premium         float64 `gorm:"not null;column:premium"`
	PremiumCurrency string  `gorm:"not null;size:8;column:premium_currency"`
	SettlementDate  string  `gorm:"not null;column:settlement_date"`

	// Tracking the last modifier — required for §3.3 turn enforcement
	// ("the party opposite to lastModifiedBy may post the next update").
	LastModifiedByRouting int    `gorm:"not null;column:last_modified_by_routing"`
	LastModifiedByID      string `gorm:"not null;size:64;column:last_modified_by_id"`

	// State.
	Status          PeerNegotiationStatus `gorm:"not null;size:16;column:status"`
	IsAuthoritative bool                  `gorm:"not null;column:is_authoritative"`

	// RemoteNegotiationID is populated only when we are the mirror
	// (i.e., the buyer's bank) — holds the partner's ID for the same
	// negotiation so we can address them on outbound PUT/GET/DELETE.
	RemoteNegotiationID *string `gorm:"size:64;column:remote_negotiation_id"`

	// Version is GORM's optimistic-locking column. Concurrent counter-offers
	// from the same peer are rejected at the DB layer rather than via
	// application-level locks.
	Version uint `gorm:"not null;default:0;column:version"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (PeerNegotiation) TableName() string { return "interbank_peer_negotiations" }
