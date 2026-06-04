package dto

// OtcOffer is the §3.2 wire body for creating or counter-offering a
// cross-bank OTC negotiation. Both parties are tagged with their owning
// bank's routing number.
type OtcOffer struct {
	BuyerID         ForeignBankId `json:"buyerId"         binding:"required"`
	SellerID        ForeignBankId `json:"sellerId"        binding:"required"`
	Ticker          string        `json:"ticker"          binding:"required,max=16"`
	Amount          int           `json:"amount"          binding:"required,min=1"`
	PricePerStock   float64       `json:"pricePerStock"   binding:"required"`
	PriceCurrency   string        `json:"priceCurrency"   binding:"required,max=8"`
	Premium         float64       `json:"premium"         binding:"required"`
	PremiumCurrency string        `json:"premiumCurrency" binding:"required,max=8"`
	SettlementDate  string        `json:"settlementDate"  binding:"required"`
	LastModifiedBy  ForeignBankId `json:"lastModifiedBy"  binding:"required"`
}

// OtcNegotiation is the §3.4 wire response carrying the full negotiation
// state. The Offer is embedded with all its fields; the outer object adds
// the negotiation id, status and last-update timestamp.
type OtcNegotiation struct {
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
}

type PeerContractExercise struct {
	ID           uint          `json:"id"`
	ContractID   ForeignBankId `json:"contractId"`
	ExecutionKey string        `json:"executionKey"`
	CurrentStep  string        `json:"currentStep"`
	Status       string        `json:"status"`
	RetryCount   int           `json:"retryCount"`
	LastError    string        `json:"lastError,omitempty"`
	CompletedAt  *string       `json:"completedAt,omitempty"`
	CreatedAt    string        `json:"createdAt"`
	UpdatedAt    string        `json:"updatedAt"`
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
