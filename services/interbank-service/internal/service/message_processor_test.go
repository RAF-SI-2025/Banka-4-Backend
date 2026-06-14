package service

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
)

const ourRouting = 444

// localAcct/remoteAcct produce 18-digit account numbers whose first three
// digits encode the owning bank — 444 (us) or 111 (a peer).
func localAcct() string  { return "444000000000000011" }
func remoteAcct() string { return "111000000000000011" }

func monas(currency string) dto.Asset {
	return dto.Asset{Type: dto.AssetMonas, Body: map[string]any{"currency": currency}}
}

func acctPosting(num string, amount float64, asset dto.Asset) dto.Posting {
	n := num
	return dto.Posting{Account: dto.TxAccount{Type: dto.TxAccountAccount, Num: &n}, Amount: amount, Asset: asset}
}

// cashTx is a balanced two-posting RSD transfer: the local account is credited
// (reserved) and a remote account is debited. The transaction is initiated by
// the peer, so its id carries the peer's routing number.
func cashTx(id string, amount float64) *dto.Transaction {
	return &dto.Transaction{
		TransactionID: dto.ForeignBankId{RoutingNumber: 111, ID: id},
		Postings: []dto.Posting{
			acctPosting(localAcct(), -amount, monas("RSD")), // local CREDIT → reserved
			acctPosting(remoteAcct(), amount, monas("RSD")), // remote DEBIT → skipped here
		},
		Message: "test transfer",
	}
}

func newProcessor(banking *fakeBanking, trading *fakeTrading) (*MessageProcessor, *fakeInbound, *fakePrepared) {
	if banking == nil {
		banking = &fakeBanking{}
	}
	if trading == nil {
		trading = &fakeTrading{}
	}
	inbound := newFakeInbound()
	prepared := newFakePrepared()
	peers := testResolver(ourRouting)
	p := NewMessageProcessor(
		inbound, prepared, newFakeOutbound(), fakeTxManager{}, peers,
		banking, trading, newFakeContracts(), newFakeNegotiations(), &fakeUserClient{},
	)
	return p, inbound, prepared
}

func firstReason(t *testing.T, vote dto.TransactionVote) dto.NoVoteReasonKind {
	t.Helper()
	require.Equal(t, dto.VoteNo, vote.Vote)
	require.NotEmpty(t, vote.Reasons)
	return vote.Reasons[0].Reason
}

// ---------------------------------------------------------------------------
// Prepare (NEW_TX) — verification, voting, reservation.
// ---------------------------------------------------------------------------

func TestPrepareLocalTransaction_Verification(t *testing.T) {
	cases := []struct {
		name       string
		tx         *dto.Transaction
		banking    *fakeBanking
		wantVote   dto.VoteKind
		wantReason dto.NoVoteReasonKind
		wantStatus model.PreparedTransactionStatus // "" = no record expected
		wantPrep   int                             // banking prepare calls
	}{
		{
			name: "balanced cash posting → YES and PREPARED",
			tx:   cashTx("tx-yes", 100),
			// banking nil → success
			wantVote:   dto.VoteYes,
			wantStatus: model.PreparedTransactionPrepared,
			wantPrep:   1,
		},
		{
			name: "unbalanced tx → NO UNBALANCED_TX, no record, no reservation",
			tx: &dto.Transaction{
				TransactionID: dto.ForeignBankId{RoutingNumber: 111, ID: "tx-unbal"},
				Postings: []dto.Posting{
					acctPosting(localAcct(), -100, monas("RSD")),
					acctPosting(remoteAcct(), 90, monas("RSD")),
				},
			},
			wantVote:   dto.VoteNo,
			wantReason: dto.ReasonUnbalancedTx,
			wantStatus: "",
			wantPrep:   0,
		},
		{
			name: "empty postings → NO UNBALANCED_TX",
			tx: &dto.Transaction{
				TransactionID: dto.ForeignBankId{RoutingNumber: 111, ID: "tx-empty"},
				Postings:      nil,
			},
			wantVote:   dto.VoteNo,
			wantReason: dto.ReasonUnbalancedTx,
			wantPrep:   0,
		},
		{
			name: "missing currency → NO UNACCEPTABLE_ASSET",
			tx: &dto.Transaction{
				TransactionID: dto.ForeignBankId{RoutingNumber: 111, ID: "tx-nocur"},
				Postings: []dto.Posting{
					acctPosting(localAcct(), -100, monas("")),
					acctPosting(remoteAcct(), 100, monas("")),
				},
			},
			wantVote:   dto.VoteNo,
			wantReason: dto.ReasonUnacceptableAsset,
			wantPrep:   0,
		},
		{
			name:       "insufficient funds → NO INSUFFICIENT_ASSET, record ROLLED_BACK",
			tx:         cashTx("tx-insuf", 100),
			banking:    &fakeBanking{prepareErr: status.Error(codes.InvalidArgument, "insufficient funds")},
			wantVote:   dto.VoteNo,
			wantReason: dto.ReasonInsufficientAsset,
			wantStatus: model.PreparedTransactionRolledBack,
			wantPrep:   1,
		},
		{
			name:       "unknown account → NO NO_SUCH_ACCOUNT, record ROLLED_BACK",
			tx:         cashTx("tx-noacct", 100),
			banking:    &fakeBanking{prepareErr: status.Error(codes.NotFound, "no such account")},
			wantVote:   dto.VoteNo,
			wantReason: dto.ReasonNoSuchAccount,
			wantStatus: model.PreparedTransactionRolledBack,
			wantPrep:   1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, _, prepared := newProcessor(tc.banking, nil)

			statusCode, vote, err := p.PrepareLocalTransaction(context.Background(), tc.tx, 0)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, statusCode)
			require.Equal(t, tc.wantVote, vote.Vote)
			if tc.wantVote == dto.VoteNo {
				require.Equal(t, tc.wantReason, firstReason(t, vote))
			}

			gotStatus, ok := prepared.status(tc.tx.TransactionID.RoutingNumber, tc.tx.TransactionID.ID)
			if tc.wantStatus == "" {
				require.False(t, ok, "expected no prepared record")
			} else {
				require.True(t, ok, "expected a prepared record")
				require.Equal(t, tc.wantStatus, gotStatus)
			}

			if tc.banking != nil {
				require.Equal(t, tc.wantPrep, tc.banking.prepareCount())
				if tc.wantStatus == model.PreparedTransactionRolledBack {
					// A failed reservation rolls back already-issued effects; the
					// failing posting itself was never reserved, so no extra
					// rollback call is expected for this single-local-posting tx.
					require.Equal(t, 0, tc.banking.rollbackCount())
				}
			}
		})
	}
}

func TestPrepareLocalTransaction_Idempotent(t *testing.T) {
	banking := &fakeBanking{}
	p, _, prepared := newProcessor(banking, nil)
	tx := cashTx("tx-idem", 100)

	// First prepare reserves and records PREPARED.
	_, vote1, err := p.PrepareLocalTransaction(context.Background(), tx, 0)
	require.NoError(t, err)
	require.Equal(t, dto.VoteYes, vote1.Vote)
	require.Equal(t, 1, banking.prepareCount())

	// A retransmit of the identical NEW_TX must short-circuit on the PREPARED
	// record and re-cast the same YES vote without issuing a second reservation.
	_, vote2, err := p.PrepareLocalTransaction(context.Background(), tx, 0)
	require.NoError(t, err)
	require.Equal(t, dto.VoteYes, vote2.Vote)
	require.Equal(t, 1, banking.prepareCount(), "reservation must not be issued twice")

	st, _ := prepared.status(111, "tx-idem")
	require.Equal(t, model.PreparedTransactionPrepared, st)
}

func TestPrepareLocalTransaction_OptionAmountIncorrect(t *testing.T) {
	// Balanced on the OPTION asset key (+2 and -2) but each option posting must
	// move exactly one contract, so the +/-2 amount is rejected.
	optAsset := dto.Asset{
		Type: dto.AssetOption,
		Body: map[string]any{
			"negotiationId":  map[string]any{"routingNumber": ourRouting, "id": "neg-1"},
			"stock":          map[string]any{"ticker": "AAPL"},
			"pricePerUnit":   map[string]any{"currency": "RSD", "amount": 10.0},
			"settlementDate": "2030-01-01",
			"amount":         5.0,
		},
	}
	tx := &dto.Transaction{
		TransactionID: dto.ForeignBankId{RoutingNumber: 111, ID: "tx-opt-bad"},
		Postings: []dto.Posting{
			{Account: personAccount(ourRouting, "7"), Amount: -2, Asset: optAsset},
			{Account: personAccount(ourRouting, "8"), Amount: 2, Asset: optAsset},
		},
	}

	p, _, _ := newProcessor(nil, nil)
	_, vote, err := p.PrepareLocalTransaction(context.Background(), tx, 0)
	require.NoError(t, err)
	require.Equal(t, dto.ReasonOptionAmountIncorrect, firstReason(t, vote))
}

// ---------------------------------------------------------------------------
// Commit (COMMIT_TX).
// ---------------------------------------------------------------------------

func seedPrepared(prepared *fakePrepared, tx *dto.Transaction, st model.PreparedTransactionStatus) {
	body, _ := json.Marshal(tx)
	prepared.seed(model.PreparedTransaction{
		RoutingNumber: tx.TransactionID.RoutingNumber,
		ID:            tx.TransactionID.ID,
		Status:        st,
		RequestBody:   body,
	})
}

func TestCommitLocalTransaction(t *testing.T) {
	cases := []struct {
		name        string
		seedStatus  model.PreparedTransactionStatus // "" = do not seed
		wantStatus  int
		wantErr     bool
		wantCommits int
		wantFinal   model.PreparedTransactionStatus
	}{
		{name: "no prepared record → 202 (retransmit later)", seedStatus: "", wantStatus: http.StatusAccepted},
		{name: "still PREPARING → 202", seedStatus: model.PreparedTransactionPreparing, wantStatus: http.StatusAccepted, wantFinal: model.PreparedTransactionPreparing},
		{name: "PREPARED → 204, commits, COMMITTED", seedStatus: model.PreparedTransactionPrepared, wantStatus: http.StatusNoContent, wantCommits: 1, wantFinal: model.PreparedTransactionCommitted},
		{name: "already COMMITTED → 204, no double commit", seedStatus: model.PreparedTransactionCommitted, wantStatus: http.StatusNoContent, wantCommits: 0, wantFinal: model.PreparedTransactionCommitted},
		{name: "ROLLED_BACK → 500 error", seedStatus: model.PreparedTransactionRolledBack, wantStatus: http.StatusInternalServerError, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			banking := &fakeBanking{}
			p, _, prepared := newProcessor(banking, nil)
			tx := cashTx("tx-commit", 100)
			if tc.seedStatus != "" {
				seedPrepared(prepared, tx, tc.seedStatus)
			}

			statusCode, err := p.CommitLocalTransaction(context.Background(), tx.TransactionID)
			require.Equal(t, tc.wantStatus, statusCode)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantCommits, banking.commitCount())
			if tc.wantFinal != "" {
				st, _ := prepared.status(111, "tx-commit")
				require.Equal(t, tc.wantFinal, st)
			}
		})
	}
}

func TestCommitLocalTransaction_Idempotent(t *testing.T) {
	banking := &fakeBanking{}
	p, _, prepared := newProcessor(banking, nil)
	tx := cashTx("tx-commit-idem", 100)
	seedPrepared(prepared, tx, model.PreparedTransactionPrepared)

	st1, err := p.CommitLocalTransaction(context.Background(), tx.TransactionID)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, st1)

	// A redelivered COMMIT_TX must be a no-op: the posting is committed once.
	st2, err := p.CommitLocalTransaction(context.Background(), tx.TransactionID)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, st2)
	require.Equal(t, 1, banking.commitCount(), "commit must be applied exactly once")
}

// ---------------------------------------------------------------------------
// Rollback (ROLLBACK_TX).
// ---------------------------------------------------------------------------

func TestRollbackLocalTransaction(t *testing.T) {
	cases := []struct {
		name          string
		seedStatus    model.PreparedTransactionStatus // "" = do not seed
		wantStatus    int
		wantErr       bool
		wantRollbacks int
		wantFinal     model.PreparedTransactionStatus
	}{
		{name: "PREPARED → 204, releases reservation, ROLLED_BACK", seedStatus: model.PreparedTransactionPrepared, wantStatus: http.StatusNoContent, wantRollbacks: 1, wantFinal: model.PreparedTransactionRolledBack},
		{name: "PREPARING → 204, releases reservation", seedStatus: model.PreparedTransactionPreparing, wantStatus: http.StatusNoContent, wantRollbacks: 1, wantFinal: model.PreparedTransactionRolledBack},
		{name: "unknown tx → 204 no-op (idempotent rollback)", seedStatus: "", wantStatus: http.StatusNoContent, wantRollbacks: 0},
		{name: "already ROLLED_BACK → 204", seedStatus: model.PreparedTransactionRolledBack, wantStatus: http.StatusNoContent, wantRollbacks: 0, wantFinal: model.PreparedTransactionRolledBack},
		{name: "COMMITTED → 500 error (commit is final)", seedStatus: model.PreparedTransactionCommitted, wantStatus: http.StatusInternalServerError, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			banking := &fakeBanking{}
			p, _, prepared := newProcessor(banking, nil)
			tx := cashTx("tx-rb", 100)
			if tc.seedStatus != "" {
				seedPrepared(prepared, tx, tc.seedStatus)
			}

			statusCode, err := p.RollbackLocalTransaction(context.Background(), tx.TransactionID)
			require.Equal(t, tc.wantStatus, statusCode)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantRollbacks, banking.rollbackCount())
			if tc.wantFinal != "" {
				st, _ := prepared.status(111, "tx-rb")
				require.Equal(t, tc.wantFinal, st)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ProcessNewTx — inbound dedup / cached response replay.
// ---------------------------------------------------------------------------

func TestProcessNewTx_DedupReplaysCachedVote(t *testing.T) {
	banking := &fakeBanking{}
	p, _, _ := newProcessor(banking, nil)
	tx := cashTx("tx-dedup", 100)
	key := dto.IdempotenceKey{RoutingNumber: 111, LocallyGeneratedKey: "key-1"}

	st1, body1, err := p.ProcessNewTx(context.Background(), 111, key, tx)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, st1)
	vote1, ok := body1.(dto.TransactionVote)
	require.True(t, ok)
	require.Equal(t, dto.VoteYes, vote1.Vote)
	require.Equal(t, 1, banking.prepareCount())

	// Same key again → cached vote, no re-preparation.
	st2, body2, err := p.ProcessNewTx(context.Background(), 111, key, tx)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, st2)
	vote2, ok := body2.(dto.TransactionVote)
	require.True(t, ok)
	require.Equal(t, dto.VoteYes, vote2.Vote)
	require.Equal(t, 1, banking.prepareCount(), "duplicate inbound key must not re-run prepare")
}
