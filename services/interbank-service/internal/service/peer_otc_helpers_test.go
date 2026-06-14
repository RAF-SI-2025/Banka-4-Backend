package service

import (
	"strings"
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/config"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
)

func testPeerOtcService() *PeerOtcService {
	return &PeerOtcService{
		peers: NewPeerResolver(
			config.NewPeerRegistry(nil),
			&config.Configuration{OurRoutingNumber: 444, OurBankDisplayName: "Banka 4"},
		),
	}
}

func validWireOffer() dto.OtcOffer {
	return dto.OtcOffer{
		Stock:              dto.StockDescription{Ticker: "AAPL"},
		SettlementDate:     time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		PricePerUnit:       dto.MonetaryValue{Currency: dto.CurrencyCode("RSD"), Amount: 100},
		Premium:            dto.MonetaryValue{Currency: dto.CurrencyCode("RSD"), Amount: 10},
		BuyerID:            dto.ForeignBankId{RoutingNumber: 555, ID: "buyer"},
		SellerID:           dto.ForeignBankId{RoutingNumber: 444, ID: "seller"},
		Amount:             5,
		LastModifiedBy:     dto.ForeignBankId{RoutingNumber: 555, ID: "buyer"},
		BuyerAccountNumber: "555000100000000001",
	}
}

func TestValidateOfferRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	svc := testPeerOtcService()
	tests := []struct {
		name   string
		mutate func(*dto.OtcOffer)
		want   string
	}{
		{name: "missing ticker", mutate: func(o *dto.OtcOffer) { o.Stock.Ticker = " " }, want: "ticker is required"},
		{name: "non-positive amount", mutate: func(o *dto.OtcOffer) { o.Amount = 0 }, want: "amount must be positive"},
		{name: "non-positive price", mutate: func(o *dto.OtcOffer) { o.PricePerUnit.Amount = 0 }, want: "pricePerUnit.amount must be positive"},
		{name: "negative premium", mutate: func(o *dto.OtcOffer) { o.Premium.Amount = -1 }, want: "premium.amount must be non-negative"},
		{name: "missing buyer account", mutate: func(o *dto.OtcOffer) { o.BuyerAccountNumber = " " }, want: "buyerAccountNumber is required"},
		{name: "bad settlement", mutate: func(o *dto.OtcOffer) { o.SettlementDate = "not-a-date" }, want: "settlementDate must be ISO 8601"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			offer := validWireOffer()
			tt.mutate(&offer)
			err := svc.validateOffer(offer)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestPeerNegotiationMappingHelpers(t *testing.T) {
	t.Parallel()

	offer := validWireOffer()
	n := &model.PeerNegotiation{
		ID:                  "neg-1",
		BuyerRoutingNumber:  offer.BuyerID.RoutingNumber,
		BuyerID:             offer.BuyerID.ID,
		SellerRoutingNumber: offer.SellerID.RoutingNumber,
		SellerID:            offer.SellerID.ID,
		Ticker:              offer.Stock.Ticker,
		BuyerAccountNumber:  offer.BuyerAccountNumber,
		Status:              model.PeerNegotiationOngoing,
		UpdatedAt:           time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}

	applyNegotiableTerms(n, offer)
	if n.Amount != offer.Amount || n.PriceCurrency != "RSD" || n.LastModifiedByID != "buyer" {
		t.Fatalf("negotiable terms were not applied: %#v", n)
	}

	wire := offerFromModel(n)
	if wire.Stock.Ticker != "AAPL" || wire.PricePerUnit.Amount != 100 || wire.BuyerID.ID != "buyer" {
		t.Fatalf("unexpected wire offer %#v", wire)
	}

	negotiation := toPeerNegotiationDTO(n)
	if !negotiation.IsOngoing || negotiation.Amount != offer.Amount {
		t.Fatalf("unexpected negotiation dto %#v", negotiation)
	}

	view := toNegotiationView(n)
	if view.ID.ID != "neg-1" || view.Status != "ongoing" || view.Offer.Stock.Ticker != "AAPL" {
		t.Fatalf("unexpected negotiation view %#v", view)
	}
}

func TestPeerContractAndTransactionHelpers(t *testing.T) {
	t.Parallel()

	svc := testPeerOtcService()
	exercisedAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	contract := &model.PeerContract{
		AuthorityRoutingNumber: 444,
		ID:                     "contract-1",
		NegotiationID:          "neg-1",
		BuyerRoutingNumber:     555,
		BuyerID:                "buyer",
		SellerRoutingNumber:    444,
		SellerID:               "seller",
		Ticker:                 "AAPL",
		Amount:                 5,
		StrikePrice:            100,
		StrikeCurrency:         "RSD",
		Premium:                10,
		PremiumCurrency:        "RSD",
		SettlementDate:         "2026-12-31",
		Status:                 model.PeerContractExercised,
		ExercisedAt:            &exercisedAt,
		CreatedAt:              exercisedAt,
		UpdatedAt:              exercisedAt,
	}

	contractDTO := toPeerContractDTO(contract)
	if contractDTO.ID.ID != "contract-1" || contractDTO.Status != "exercised" || contractDTO.ExercisedAt == nil {
		t.Fatalf("unexpected contract dto %#v", contractDTO)
	}

	negotiation := &model.PeerNegotiation{
		ID:                  "neg-1",
		BuyerRoutingNumber:  555,
		BuyerID:             "buyer",
		SellerRoutingNumber: 444,
		SellerID:            "seller",
		Ticker:              "AAPL",
		Amount:              5,
		PricePerStock:       100,
		PriceCurrency:       "RSD",
		Premium:             10,
		PremiumCurrency:     "RSD",
		SettlementDate:      "2026-12-31",
		BuyerAccountNumber:  "buyer-account",
	}
	acceptTx := svc.acceptTransaction(negotiation)
	if acceptTx.TransactionID.RoutingNumber != 444 || len(acceptTx.Postings) != 4 {
		t.Fatalf("unexpected accept transaction %#v", acceptTx)
	}
	if acceptTx.Postings[0].Amount != -10 || acceptTx.Postings[3].Amount != 1 {
		t.Fatalf("unexpected accept postings %#v", acceptTx.Postings)
	}

	exerciseTx := svc.exerciseTransaction(contract, "buyer-account", "execution-key")
	if exerciseTx.TransactionID.ID != "execution-key" || len(exerciseTx.Postings) != 4 {
		t.Fatalf("unexpected exercise transaction %#v", exerciseTx)
	}
	if exerciseTx.Postings[0].Amount != -500 || exerciseTx.Postings[3].Amount != 5 {
		t.Fatalf("unexpected exercise postings %#v", exerciseTx.Postings)
	}
}

func TestPeerOtcSmallHelpers(t *testing.T) {
	t.Parallel()

	if got := monetary("EUR", 12.5); got.Currency != "EUR" || got.Amount != 12.5 {
		t.Fatalf("unexpected monetary value %#v", got)
	}

	account := personAccount(555, "user-1")
	if account.Type != dto.TxAccountPerson || account.ID == nil || account.ID.ID != "user-1" {
		t.Fatalf("unexpected person account %#v", account)
	}

	if got := voteReasons(dto.TransactionVote{}); got != "no reason provided" {
		t.Fatalf("unexpected empty vote reasons %q", got)
	}
	if got := voteReasonsValue(nil); got != "no vote returned" {
		t.Fatalf("unexpected nil vote reason %q", got)
	}
}
