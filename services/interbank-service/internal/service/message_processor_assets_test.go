package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
)

func optionAsset(routing int, id string, ticker string, amount float64) dto.Asset {
	return dto.Asset{
		Type: dto.AssetOption,
		Body: map[string]any{
			"negotiationId":  map[string]any{"routingNumber": routing, "id": id},
			"stock":          map[string]any{"ticker": ticker},
			"pricePerUnit":   map[string]any{"currency": "RSD", "amount": 100.0},
			"settlementDate": "2030-01-01",
			"amount":         amount,
		},
	}
}

func stockAsset(ticker string) dto.Asset {
	return dto.Asset{Type: dto.AssetStock, Body: map[string]any{"ticker": ticker}}
}

func newProcessorWithStores(banking *fakeBanking, trading *fakeTrading) (*MessageProcessor, *fakePrepared, *fakeNegotiations, *fakeContracts) {
	if banking == nil {
		banking = &fakeBanking{}
	}
	if trading == nil {
		trading = &fakeTrading{}
	}
	prepared := newFakePrepared()
	negs := newFakeNegotiations()
	contracts := newFakeContracts()
	p := NewMessageProcessor(
		newFakeInbound(), prepared, newFakeOutbound(), fakeTxManager{}, testResolver(ourRouting),
		banking, trading, contracts, negs, &fakeUserClient{},
	)
	return p, prepared, negs, contracts
}

func TestPrepareLocalTransaction_MonasPersonPosting(t *testing.T) {
	banking := &fakeBanking{}
	p, _, _, _ := newProcessorWithStores(banking, nil)
	tx := &dto.Transaction{
		TransactionID: dto.ForeignBankId{RoutingNumber: 111, ID: "person-cash"},
		Postings: []dto.Posting{
			{Account: personAccount(ourRouting, "7"), Amount: -125, Asset: monas("RSD")},
			acctPosting(remoteAcct(), 125, monas("RSD")),
		},
	}

	statusCode, vote, err := p.PrepareLocalTransaction(context.Background(), tx, 0)
	require.NoError(t, err)
	require.Equal(t, 200, statusCode)
	require.Equal(t, dto.VoteYes, vote.Vote)
	require.Len(t, banking.prepareCalls, 1)
	require.Equal(t, uint64(7), banking.prepareCalls[0].ClientId)
	require.Equal(t, "CLIENT", banking.prepareCalls[0].UserType)
	require.Equal(t, "RSD", banking.prepareCalls[0].CurrencyCode)
	require.Equal(t, -125.0, banking.prepareCalls[0].Amount)
}

func TestPrepareLocalTransaction_OptionReserveAndRollbackOnLaterFailure(t *testing.T) {
	banking := &fakeBanking{prepareErr: status.Error(codes.InvalidArgument, "insufficient funds")}
	trading := &fakeTrading{}
	p, prepared, negs, _ := newProcessorWithStores(banking, trading)
	seedAuthoritativeNegotiation(negs, "neg-option")

	tx := &dto.Transaction{
		TransactionID: dto.ForeignBankId{RoutingNumber: 111, ID: "option-prepare"},
		Postings: []dto.Posting{
			{Account: personAccount(ourRouting, "7"), Amount: -1, Asset: optionAsset(ourRouting, "neg-option", "AAPL", 10)},
			{Account: personAccount(111, "buyer-1"), Amount: 1, Asset: optionAsset(ourRouting, "neg-option", "AAPL", 10)},
			acctPosting(localAcct(), -50, monas("RSD")),
			acctPosting(remoteAcct(), 50, monas("RSD")),
		},
	}

	statusCode, vote, err := p.PrepareLocalTransaction(context.Background(), tx, 0)
	require.NoError(t, err)
	require.Equal(t, 200, statusCode)
	require.Equal(t, dto.ReasonInsufficientAsset, firstReason(t, vote))
	require.Len(t, trading.reserveCalls, 1)
	require.Equal(t, "444:neg-option", trading.reserveCalls[0].ContractId)
	require.Equal(t, []string{"444:neg-option"}, trading.releaseCalls)
	st, ok := prepared.status(111, "option-prepare")
	require.True(t, ok)
	require.Equal(t, model.PreparedTransactionRolledBack, st)
}

func TestCommitLocalTransaction_OptionCreatesAuthoritativeContract(t *testing.T) {
	p, prepared, negs, contracts := newProcessorWithStores(nil, nil)
	seedAuthoritativeNegotiation(negs, "neg-authority")
	tx := &dto.Transaction{
		TransactionID: dto.ForeignBankId{RoutingNumber: 111, ID: "option-commit-authority"},
		Postings: []dto.Posting{
			{Account: personAccount(ourRouting, "7"), Amount: -1, Asset: optionAsset(ourRouting, "neg-authority", "AAPL", 10)},
			{Account: personAccount(111, "buyer-1"), Amount: 1, Asset: optionAsset(ourRouting, "neg-authority", "AAPL", 10)},
		},
	}
	seedPrepared(prepared, tx, model.PreparedTransactionPrepared)

	statusCode, err := p.CommitLocalTransaction(context.Background(), tx.TransactionID)
	require.NoError(t, err)
	require.Equal(t, 204, statusCode)

	contract, err := contracts.FindByID(context.Background(), ourRouting, "neg-authority")
	require.NoError(t, err)
	require.NotNil(t, contract)
	require.True(t, contract.IsAuthoritative)
	require.Equal(t, 111, contract.BuyerRoutingNumber)
	require.Equal(t, "buyer-1", contract.BuyerID)

	negotiation, err := negs.FindByID(context.Background(), ourRouting, "neg-authority")
	require.NoError(t, err)
	require.Equal(t, model.PeerNegotiationAccepted, negotiation.Status)
}

func TestCommitLocalTransaction_OptionCreatesMirrorContract(t *testing.T) {
	p, prepared, _, contracts := newProcessorWithStores(nil, nil)
	tx := &dto.Transaction{
		TransactionID: dto.ForeignBankId{RoutingNumber: 111, ID: "option-commit-mirror"},
		Postings: []dto.Posting{
			acctPosting(localAcct(), -5, monas("RSD")),
			{Account: personAccount(111, "seller-1"), Amount: 5, Asset: monas("RSD")},
			{Account: personAccount(111, "seller-1"), Amount: -1, Asset: optionAsset(111, "neg-mirror", "MSFT", 3)},
			{Account: personAccount(ourRouting, "9"), Amount: 1, Asset: optionAsset(111, "neg-mirror", "MSFT", 3)},
		},
	}
	seedPrepared(prepared, tx, model.PreparedTransactionPrepared)

	statusCode, err := p.CommitLocalTransaction(context.Background(), tx.TransactionID)
	require.NoError(t, err)
	require.Equal(t, 204, statusCode)

	contract, err := contracts.FindByID(context.Background(), 111, "neg-mirror")
	require.NoError(t, err)
	require.NotNil(t, contract)
	require.False(t, contract.IsAuthoritative)
	require.Equal(t, ourRouting, contract.BuyerRoutingNumber)
	require.Equal(t, "9", contract.BuyerID)
	require.Equal(t, 5.0, contract.Premium)
	require.Equal(t, "RSD", contract.PremiumCurrency)
}

func TestPrepareAndCommitStockExercisePaths(t *testing.T) {
	t.Run("seller option account validates and consumes reserved shares", func(t *testing.T) {
		trading := &fakeTrading{}
		p, prepared, _, contracts := newProcessorWithStores(nil, trading)
		require.NoError(t, contracts.Create(context.Background(), activePeerContract(ourRouting, "stock-seller", 111, "buyer", ourRouting, "7")))
		tx := &dto.Transaction{
			TransactionID: dto.ForeignBankId{RoutingNumber: 111, ID: "stock-seller"},
			Postings: []dto.Posting{
				{Account: dto.TxAccount{Type: dto.TxAccountOption, ID: &dto.ForeignBankId{RoutingNumber: ourRouting, ID: "stock-seller"}}, Amount: -10, Asset: stockAsset("AAPL")},
				{Account: personAccount(111, "buyer"), Amount: 10, Asset: stockAsset("AAPL")},
			},
		}

		statusCode, vote, err := p.PrepareLocalTransaction(context.Background(), tx, 0)
		require.NoError(t, err)
		require.Equal(t, 200, statusCode)
		require.Equal(t, dto.VoteYes, vote.Vote)
		seedPrepared(prepared, tx, model.PreparedTransactionPrepared)

		statusCode, err = p.CommitLocalTransaction(context.Background(), tx.TransactionID)
		require.NoError(t, err)
		require.Equal(t, 204, statusCode)
		require.Equal(t, []string{"444:stock-seller"}, trading.consumeCalls)
		contract, err := contracts.FindByID(context.Background(), ourRouting, "stock-seller")
		require.NoError(t, err)
		require.Equal(t, model.PeerContractExercised, contract.Status)
		require.NotNil(t, contract.ExercisedAt)
	})

	t.Run("local buyer receives shares", func(t *testing.T) {
		trading := &fakeTrading{}
		p, prepared, _, contracts := newProcessorWithStores(nil, trading)
		require.NoError(t, contracts.Create(context.Background(), activePeerContract(111, "stock-buyer", ourRouting, "9", 111, "seller")))
		tx := &dto.Transaction{
			TransactionID: dto.ForeignBankId{RoutingNumber: 111, ID: "stock-buyer"},
			Postings: []dto.Posting{
				{Account: dto.TxAccount{Type: dto.TxAccountOption, ID: &dto.ForeignBankId{RoutingNumber: 111, ID: "stock-buyer"}}, Amount: -3, Asset: stockAsset("MSFT")},
				{Account: personAccount(ourRouting, "9"), Amount: 3, Asset: stockAsset("MSFT")},
			},
		}
		seedPrepared(prepared, tx, model.PreparedTransactionPrepared)

		statusCode, err := p.CommitLocalTransaction(context.Background(), tx.TransactionID)
		require.NoError(t, err)
		require.Equal(t, 204, statusCode)
		require.Len(t, trading.creditCalls, 1)
		require.Equal(t, "111:stock-buyer", trading.creditCalls[0].ContractId)
		require.Equal(t, uint64(9), trading.creditCalls[0].BuyerId)
		require.Equal(t, "MSFT", trading.creditCalls[0].Ticker)
		contract, err := contracts.FindByID(context.Background(), 111, "stock-buyer")
		require.NoError(t, err)
		require.Equal(t, model.PeerContractExercised, contract.Status)
	})
}
