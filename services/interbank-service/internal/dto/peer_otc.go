package dto

// OtcOffer is the §3.2/§3.3 wire body for creating or counter-offering a
// cross-bank OTC negotiation. The shape follows the protocol spec: the stock
// is nested and monetary terms are MonetaryValue objects. BuyerAccountNumber
// is our agreed extension to the spec (carries the buyer's settlement account).
type OtcOffer struct {
	Stock          StockDescription `json:"stock"          binding:"required"`
	SettlementDate string           `json:"settlementDate" binding:"required"`
	PricePerUnit   MonetaryValue    `json:"pricePerUnit"   binding:"required"`
	Premium        MonetaryValue    `json:"premium"`
	BuyerID        ForeignBankId    `json:"buyerId"        binding:"required"`
	SellerID       ForeignBankId    `json:"sellerId"       binding:"required"`
	Amount         int              `json:"amount"         binding:"required,min=1"`
	LastModifiedBy ForeignBankId    `json:"lastModifiedBy" binding:"required"`

	// BuyerAccountNumber is a Banka-4 extension to the protocol's OtcOffer.
	BuyerAccountNumber string `json:"buyerAccountNumber"`
}

// OtcNegotiation is the §3.4 wire response: the current OtcOffer plus an
// isOngoing flag (the negotiation is closed when !isOngoing). It is the
// authoritative state served by the seller's bank to a peer.
type OtcNegotiation struct {
	OtcOffer
	IsOngoing bool `json:"isOngoing"`
}

// OtcNegotiationView is the user-facing (frontend) representation of a
// negotiation. Unlike the §3.4 wire shape it keeps the negotiation id, a
// human status string and the last-update timestamp for our own UI.
type OtcNegotiationView struct {
	ID        ForeignBankId `json:"id"`
	Status    string        `json:"status"` // ongoing | accepted | cancelled | expired
	UpdatedAt string        `json:"updatedAt"`
	Offer     OtcOffer      `json:"offer"`
}

// PublicStockSeller is one entry in PublicStock.Sellers — a user at this
// bank who has flagged some quantity of the stock as public.
type PeerContract struct {
	ID             ForeignBankId `json:"id"`
	NegotiationID  ForeignBankId `json:"negotiationId"`
	BuyerID        ForeignBankId `json:"buyerId"`
	SellerID       ForeignBankId `json:"sellerId"`
	Ticker         string        `json:"ticker"`
	Amount         int           `json:"amount"`
	StrikePrice    MonetaryValue `json:"strikePrice"`
	Premium        MonetaryValue `json:"premium"`
	SettlementDate string        `json:"settlementDate"`
	Status         string        `json:"status"`
	ExercisedAt    *string       `json:"exercisedAt,omitempty"`
	CreatedAt      string        `json:"createdAt"`
	UpdatedAt      string        `json:"updatedAt"`

	// MyContract is true when the requesting local user is the buyer on this
	// contract. It is a frontend convenience field, populated per-request.
	MyContract bool `json:"myContract"`
}

type PublicStockSeller struct {
	Seller ForeignBankId `json:"seller"`
	Amount int           `json:"amount"`
}

// PublicStock is one row in the §3.1 public-stock response. Each unique
// ticker is reported once with the list of sellers offering it.
type PublicStock struct {
	Stock   StockDescription    `json:"stock"`
	Sellers []PublicStockSeller `json:"sellers"`
}

// UserInformation is the §3.7 response shape for resolving display names
// from foreign bank user identifiers.
type UserInformation struct {
	BankDisplayName string `json:"bankDisplayName"`
	DisplayName     string `json:"displayName"`
}
