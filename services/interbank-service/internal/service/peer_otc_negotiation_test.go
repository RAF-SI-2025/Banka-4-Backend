package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
)

// newOtcSvc builds a PeerOtcService wired only with the dependencies the
// negotiation-lifecycle methods use (negotiations repo, peers, peer client,
// tx manager). The MessageProcessor and the various gRPC clients are not
// exercised by counter/close/create, so they are left nil.
func newOtcSvc(peers *PeerResolver) (*PeerOtcService, *fakeNegotiations) {
	negs := newFakeNegotiations()
	svc := NewPeerOtcService(
		negs, newFakeContracts(), peers, NewPeerOtcClient(peers),
		nil, nil, nil, nil, newFakeOutbound(), fakeTxManager{},
	)
	return svc, negs
}

// ---------------------------------------------------------------------------
// PAYMENT flow: InitiatePayment's core (PrepareAndEnqueueNewTx with PAYMENT).
// ---------------------------------------------------------------------------

func paymentTx(id string, amount float64) *dto.Transaction {
	return &dto.Transaction{
		TransactionID: dto.ForeignBankId{RoutingNumber: ourRouting, ID: id},
		Postings: []dto.Posting{
			acctPosting(localAcct(), -amount, monas("RSD")), // payer CREDIT (reserved locally)
			acctPosting(remoteAcct(), amount, monas("RSD")), // payee DEBIT (peer credits)
		},
		Message: "interbank payment",
	}
}

func TestPrepareAndEnqueueNewTx_PaymentFlow(t *testing.T) {
	t.Run("YES reserves payer and enqueues a PAYMENT NEW_TX row", func(t *testing.T) {
		banking := &fakeBanking{}
		outbound := newFakeOutbound()
		prepared := newFakePrepared()
		p := NewMessageProcessor(
			newFakeInbound(), prepared, outbound, fakeTxManager{}, testResolver(ourRouting),
			banking, &fakeTrading{}, newFakeContracts(), newFakeNegotiations(),
		)

		_, vote, msg, err := p.PrepareAndEnqueueNewTx(context.Background(), paymentTx("pay-yes", 100), 111, "pay-yes-new", model.FlowTypePayment, 42)
		require.NoError(t, err)
		require.Equal(t, dto.VoteYes, vote.Vote)
		require.NotNil(t, msg)
		require.Equal(t, 1, banking.prepareCount(), "payer funds must be reserved")

		row, ok := outbound.byKeyRow("pay-yes-new")
		require.True(t, ok)
		require.Equal(t, model.FlowTypePayment, row.FlowType)
		require.Equal(t, uint64(42), row.BankingTxID, "NEW_TX row carries the banking tx id for the settlement callback")
		require.Equal(t, string(dto.MessageTypeNewTx), row.MessageType)
		require.Equal(t, model.OutboundPending, row.Status)

		st, _ := prepared.status(ourRouting, "pay-yes")
		require.Equal(t, model.PreparedTransactionPrepared, st)
	})

	t.Run("insufficient funds → NO vote, no outbox row, rolled back", func(t *testing.T) {
		banking := &fakeBanking{prepareErr: status.Error(codes.InvalidArgument, "insufficient funds")}
		outbound := newFakeOutbound()
		prepared := newFakePrepared()
		p := NewMessageProcessor(
			newFakeInbound(), prepared, outbound, fakeTxManager{}, testResolver(ourRouting),
			banking, &fakeTrading{}, newFakeContracts(), newFakeNegotiations(),
		)

		_, vote, msg, err := p.PrepareAndEnqueueNewTx(context.Background(), paymentTx("pay-no", 100), 111, "pay-no-new", model.FlowTypePayment, 42)
		require.NoError(t, err)
		require.Equal(t, dto.VoteNo, vote.Vote)
		require.Nil(t, msg, "no outbox row should be enqueued on a NO vote")

		_, ok := outbound.byKeyRow("pay-no-new")
		require.False(t, ok)

		st, _ := prepared.status(ourRouting, "pay-no")
		require.Equal(t, model.PreparedTransactionRolledBack, st)
	})
}

// ---------------------------------------------------------------------------
// Seller-side bidirectional negotiation (Workstream 2).
// ---------------------------------------------------------------------------

// seedAuthoritativeNegotiation stores a negotiation where the seller is us
// (authoritative) and the buyer is a remote peer who made the last offer.
func seedAuthoritativeNegotiation(negs *fakeNegotiations, id string) {
	negs.seed(model.PeerNegotiation{
		ID:                    id,
		BuyerRoutingNumber:    111,
		BuyerID:               "buyer-1",
		SellerRoutingNumber:   ourRouting,
		SellerID:              "7",
		Ticker:                "AAPL",
		Amount:                10,
		PricePerStock:         100,
		PriceCurrency:         "RSD",
		Premium:               5,
		PremiumCurrency:       "RSD",
		SettlementDate:        "2030-01-01",
		BuyerAccountNumber:    "111000000000000011",
		LastModifiedByRouting: 111, // buyer moved last → seller's turn
		LastModifiedByID:      "buyer-1",
		Status:                model.PeerNegotiationOngoing,
		IsAuthoritative:       true,
	})
}

func counterReq() LocalCounterRequest {
	return LocalCounterRequest{
		Amount: 10, PricePerStock: 90, PriceCurrency: "RSD",
		Premium: 4, PremiumCurrency: "RSD", SettlementDate: "2030-01-01",
	}
}

func TestSendCounterOfferAsLocal_SellerSide(t *testing.T) {
	bank := newMockBank()
	defer bank.close()
	peers := peersTo(bank)
	svc, negs := newOtcSvc(peers)
	seedAuthoritativeNegotiation(negs, "neg-seller")

	// Our seller (local id 7) counters the remote buyer's offer.
	err := svc.SendCounterOfferAsLocal(context.Background(), 7, dto.ForeignBankId{RoutingNumber: ourRouting, ID: "neg-seller"}, counterReq())
	require.NoError(t, err)

	// The counter must be PUT to the BUYER's bank (the opposing party), with the
	// authoritative routing in the path.
	require.Equal(t, []string{"PUT /negotiations/444/neg-seller"}, bank.otcGot())

	// Local authoritative row reflects the counter and that we moved last.
	updated, _ := negs.FindByID(context.Background(), "neg-seller")
	require.Equal(t, 90.0, updated.PricePerStock)
	require.Equal(t, ourRouting, updated.LastModifiedByRouting)
	require.Equal(t, "7", updated.LastModifiedByID)
}

func TestSendCounterOfferAsLocal_SellerCannotCounterTwice(t *testing.T) {
	bank := newMockBank()
	defer bank.close()
	svc, negs := newOtcSvc(peersTo(bank))
	seedAuthoritativeNegotiation(negs, "neg-turn")

	// Make the seller the last modifier → it is now the buyer's turn.
	row, _ := negs.FindByID(context.Background(), "neg-turn")
	row.LastModifiedByRouting = ourRouting
	row.LastModifiedByID = "7"
	_ = negs.Update(context.Background(), row)

	err := svc.SendCounterOfferAsLocal(context.Background(), 7, dto.ForeignBankId{RoutingNumber: ourRouting, ID: "neg-turn"}, counterReq())
	require.Error(t, err, "seller must not counter its own latest offer")
	require.Empty(t, bank.otcGot(), "no notification should be sent on a turn violation")
}

// ---------------------------------------------------------------------------
// Inbound UpdateCounter / Close must accept updates to a MIRROR row (i.e. when
// the seller's bank notifies us, the buyer's bank, of the seller's action).
// ---------------------------------------------------------------------------

// seedMirrorNegotiation stores a negotiation where the buyer is us (mirror) and
// the seller is the authoritative remote peer who made the last offer.
func seedMirrorNegotiation(negs *fakeNegotiations, id string) {
	negs.seed(model.PeerNegotiation{
		ID:                    id,
		BuyerRoutingNumber:    ourRouting,
		BuyerID:               "9",
		SellerRoutingNumber:   111,
		SellerID:              "seller-1",
		Ticker:                "MSFT",
		Amount:                3,
		PricePerStock:         200,
		PriceCurrency:         "RSD",
		Premium:               2,
		PremiumCurrency:       "RSD",
		SettlementDate:        "2030-06-01",
		BuyerAccountNumber:    "444000000000000011",
		LastModifiedByRouting: ourRouting, // buyer (us) moved last → seller's turn
		LastModifiedByID:      "9",
		Status:                model.PeerNegotiationOngoing,
		IsAuthoritative:       false,
	})
}

func specOffer(n *model.PeerNegotiation, lastModifiedBy dto.ForeignBankId, price float64) dto.OtcOffer {
	return dto.OtcOffer{
		Stock:              dto.StockDescription{Ticker: n.Ticker},
		SettlementDate:     n.SettlementDate,
		PricePerUnit:       monetary(n.PriceCurrency, price),
		Premium:            monetary(n.PremiumCurrency, n.Premium),
		BuyerID:            dto.ForeignBankId{RoutingNumber: n.BuyerRoutingNumber, ID: n.BuyerID},
		SellerID:           dto.ForeignBankId{RoutingNumber: n.SellerRoutingNumber, ID: n.SellerID},
		Amount:             n.Amount,
		LastModifiedBy:     lastModifiedBy,
		BuyerAccountNumber: n.BuyerAccountNumber,
	}
}

func TestUpdateCounter_AcceptsMirrorRowFromSeller(t *testing.T) {
	svc, negs := newOtcSvc(testResolver(ourRouting))
	seedMirrorNegotiation(negs, "neg-mirror")
	n, _ := negs.FindByID(context.Background(), "neg-mirror")

	// The seller's bank (routing 111) notifies us of the seller's counter. The
	// path rn is the authoritative (seller's) routing number 111.
	sellerFBID := dto.ForeignBankId{RoutingNumber: 111, ID: "seller-1"}
	offer := specOffer(n, sellerFBID, 180)

	err := svc.UpdateCounter(context.Background(), 111, 111, "neg-mirror", offer)
	require.NoError(t, err)

	updated, _ := negs.FindByID(context.Background(), "neg-mirror")
	require.Equal(t, 180.0, updated.PricePerStock)
	require.Equal(t, 111, updated.LastModifiedByRouting)
	require.Equal(t, "seller-1", updated.LastModifiedByID)
}

func TestClose_AcceptsMirrorRow(t *testing.T) {
	svc, negs := newOtcSvc(testResolver(ourRouting))
	seedMirrorNegotiation(negs, "neg-close")

	// Seller's bank (111) closes; we hold the mirror. Path rn = authoritative 111.
	err := svc.Close(context.Background(), 111, 111, "neg-close")
	require.NoError(t, err)

	updated, _ := negs.FindByID(context.Background(), "neg-close")
	require.Equal(t, model.PeerNegotiationCancelled, updated.Status)
}

// ---------------------------------------------------------------------------
// Strict interop: spec-shaped OtcOffer / OtcNegotiation wire format.
// ---------------------------------------------------------------------------

func TestOtcOffer_SpecShapeJSON(t *testing.T) {
	offer := dto.OtcOffer{
		Stock:              dto.StockDescription{Ticker: "AAPL"},
		SettlementDate:     "2030-01-01",
		PricePerUnit:       monetary("RSD", 100),
		Premium:            monetary("RSD", 5),
		BuyerID:            dto.ForeignBankId{RoutingNumber: 111, ID: "b"},
		SellerID:           dto.ForeignBankId{RoutingNumber: 444, ID: "s"},
		Amount:             10,
		LastModifiedBy:     dto.ForeignBankId{RoutingNumber: 111, ID: "b"},
		BuyerAccountNumber: "111000000000000011",
	}

	raw, err := json.Marshal(offer)
	require.NoError(t, err)

	var generic map[string]any
	require.NoError(t, json.Unmarshal(raw, &generic))

	// Spec shape: nested stock + MonetaryValue price/premium.
	stock, ok := generic["stock"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "AAPL", stock["ticker"])
	price, ok := generic["pricePerUnit"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "RSD", price["currency"])
	require.Equal(t, 100.0, price["amount"])
	_, ok = generic["premium"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "111000000000000011", generic["buyerAccountNumber"])

	// §3.4 negotiation = OtcOffer + isOngoing (flat).
	neg := dto.OtcNegotiation{OtcOffer: offer, IsOngoing: true}
	raw, err = json.Marshal(neg)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &generic))
	require.Equal(t, true, generic["isOngoing"])
	require.Contains(t, generic, "stock", "OtcOffer fields are flattened into the negotiation")
}

func TestCreateFromPeer_SpecShapedOffer(t *testing.T) {
	svc, negs := newOtcSvc(testResolver(ourRouting))

	buyer := dto.ForeignBankId{RoutingNumber: 111, ID: "buyer-1"}
	offer := dto.OtcOffer{
		Stock:              dto.StockDescription{Ticker: "AAPL"},
		SettlementDate:     "2030-01-01",
		PricePerUnit:       monetary("RSD", 100),
		Premium:            monetary("RSD", 5),
		BuyerID:            buyer,
		SellerID:           dto.ForeignBankId{RoutingNumber: ourRouting, ID: "7"},
		Amount:             10,
		LastModifiedBy:     buyer,
		BuyerAccountNumber: "111000000000000011",
	}

	id, err := svc.CreateFromPeer(context.Background(), 111, offer)
	require.NoError(t, err)
	require.Equal(t, ourRouting, id.RoutingNumber)

	stored, _ := negs.FindByID(context.Background(), id.ID)
	require.NotNil(t, stored)
	require.Equal(t, "AAPL", stored.Ticker)
	require.Equal(t, 100.0, stored.PricePerStock)
	require.Equal(t, "RSD", stored.PriceCurrency)
	require.Equal(t, 5.0, stored.Premium)
	require.True(t, stored.IsAuthoritative)
}

// ---------------------------------------------------------------------------
// Expiry date correctness (Workstream 4a) — the helper the job relies on.
// ---------------------------------------------------------------------------

func TestSettlementPassed(t *testing.T) {
	cases := []struct {
		name string
		date string
		want bool
	}{
		{"past date-only", "2000-01-01", true},
		{"future date-only", "2999-01-01", false},
		{"past RFC3339", "2000-01-02T15:04:05Z", true},
		{"future RFC3339", "2999-01-02T15:04:05+02:00", false},
		{"unparseable → not passed", "not-a-date", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, SettlementPassed(tc.date))
		})
	}
}
