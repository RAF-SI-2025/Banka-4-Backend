package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/config"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
)

// mockBank is a stand-in for a remote bank's §2 /interbank ingress. It records
// the message types it receives and answers NEW_TX with a configurable vote so
// a test can drive the coordinator down the YES, NO and failure branches.
type mockBank struct {
	server *httptest.Server

	mu          sync.Mutex
	received    []string
	apiKeysSeen []string
	otcReqs     []string // "<METHOD> <path>" for §3 negotiation endpoints

	newTxStatus int                 // status for NEW_TX (default 200)
	newTxVote   dto.TransactionVote // body for NEW_TX
}

func newMockBank() *mockBank {
	m := &mockBank{newTxStatus: http.StatusOK, newTxVote: dto.TransactionVote{Vote: dto.VoteYes}}
	mux := http.NewServeMux()
	mux.HandleFunc("/interbank", m.handle)
	mux.HandleFunc("/interbank/negotiations", m.handleOtc)
	mux.HandleFunc("/interbank/negotiations/", m.handleOtc)
	m.server = httptest.NewServer(mux)
	return m
}

// handleOtc records §3 negotiation requests (POST/PUT/DELETE) and returns a
// minimal valid response so the client treats the call as successful.
func (m *mockBank) handleOtc(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	m.otcReqs = append(m.otcReqs, r.Method+" "+r.URL.Path)
	m.apiKeysSeen = append(m.apiKeysSeen, r.Header.Get("X-Api-Key"))
	m.mu.Unlock()

	switch r.Method {
	case http.MethodPost: // §3.2 create → returns a ForeignBankId
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(dto.ForeignBankId{RoutingNumber: 111, ID: "remote-neg"})
	default: // §3.3 PUT, §3.5 DELETE
		w.WriteHeader(http.StatusNoContent)
	}
}

func (m *mockBank) otcGot() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.otcReqs))
	copy(out, m.otcReqs)
	return out
}

func (m *mockBank) handle(w http.ResponseWriter, r *http.Request) {
	var env struct {
		MessageType dto.MessageType `json:"messageType"`
	}
	_ = json.NewDecoder(r.Body).Decode(&env)

	m.mu.Lock()
	m.received = append(m.received, string(env.MessageType))
	m.apiKeysSeen = append(m.apiKeysSeen, r.Header.Get("X-Api-Key"))
	status := m.newTxStatus
	vote := m.newTxVote
	m.mu.Unlock()

	switch env.MessageType {
	case dto.MessageTypeNewTx:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status == http.StatusOK {
			_ = json.NewEncoder(w).Encode(vote)
		}
	case dto.MessageTypeCommitTx, dto.MessageTypeRollbackTx:
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

func (m *mockBank) close() { m.server.Close() }

func (m *mockBank) got() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.received))
	copy(out, m.received)
	return out
}

// coordSetup builds a PeerOtcService and the MessageProcessor it drives, both
// sharing the same outbox + peer registry so outbox row ids line up exactly as
// they would in production.
type coordSetup struct {
	svc      *PeerOtcService
	banking  *fakeBanking
	prepared *fakePrepared
	outbound *fakeOutbound
}

func newCoordSetup(peers *PeerResolver) *coordSetup {
	banking := &fakeBanking{}
	prepared := newFakePrepared()
	outbound := newFakeOutbound()
	processor := NewMessageProcessor(
		newFakeInbound(), prepared, outbound, fakeTxManager{}, peers,
		banking, &fakeTrading{}, newFakeContracts(), newFakeNegotiations(),
	)
	peerClient := NewPeerOtcClient(peers)
	svc := NewPeerOtcService(
		newFakeNegotiations(), newFakeContracts(), peers, peerClient,
		nil, nil, nil, processor, outbound, fakeTxManager{},
	)
	return &coordSetup{svc: svc, banking: banking, prepared: prepared, outbound: outbound}
}

func crossBankTx(id string, amount float64) dto.Transaction {
	return dto.Transaction{
		TransactionID: dto.ForeignBankId{RoutingNumber: ourRouting, ID: id},
		Postings: []dto.Posting{
			acctPosting(localAcct(), -amount, monas("RSD")), // local CREDIT (reserved here)
			acctPosting(remoteAcct(), amount, monas("RSD")), // remote DEBIT (peer's job)
		},
		Message: "coordinated transfer",
	}
}

func peersTo(bank *mockBank) *PeerResolver {
	return testResolver(ourRouting, config.Peer{
		RoutingNumber: 111, BaseURL: bank.server.URL, OurAPIKey: "to-peer", TheirAPIKey: "from-peer",
	})
}

// ---------------------------------------------------------------------------
// Cross-bank coordination against a mock remote bank.
// ---------------------------------------------------------------------------

func TestCoordinateTwoBankTransaction_RemoteYes(t *testing.T) {
	bank := newMockBank()
	defer bank.close()
	bank.newTxVote = dto.TransactionVote{Vote: dto.VoteYes}

	s := newCoordSetup(peersTo(bank))
	tx := crossBankTx("otc-yes", 50)

	err := s.svc.coordinateTwoBankTransaction(t.Context(), 111, tx, "k-yes")
	require.NoError(t, err)

	// Local side: reserved then committed.
	require.Equal(t, 1, s.banking.prepareCount())
	require.Equal(t, 1, s.banking.commitCount())
	require.Equal(t, 0, s.banking.rollbackCount())

	// Remote side: NEW_TX then COMMIT_TX, in that order.
	require.Equal(t, []string{string(dto.MessageTypeNewTx), string(dto.MessageTypeCommitTx)}, bank.got())

	// The peer was addressed with the key it issued to us.
	require.Equal(t, "to-peer", bank.apiKeysSeen[0])

	st, _ := s.prepared.status(ourRouting, "otc-yes")
	require.Equal(t, model.PreparedTransactionCommitted, st)

	newSt, _ := s.outbound.statusByKey("k-yes-new")
	require.Equal(t, model.OutboundSent, newSt)
	commitSt, _ := s.outbound.statusByKey("k-yes-commit")
	require.Equal(t, model.OutboundSent, commitSt)
}

func TestCoordinateTwoBankTransaction_RemoteNo(t *testing.T) {
	bank := newMockBank()
	defer bank.close()
	bank.newTxVote = dto.TransactionVote{
		Vote:    dto.VoteNo,
		Reasons: []dto.NoVoteReason{{Reason: dto.ReasonInsufficientAsset}},
	}

	s := newCoordSetup(peersTo(bank))
	tx := crossBankTx("otc-no", 50)

	err := s.svc.coordinateTwoBankTransaction(t.Context(), 111, tx, "k-no")
	require.Error(t, err)

	// Local side rolled back; never committed.
	require.Equal(t, 1, s.banking.prepareCount())
	require.Equal(t, 1, s.banking.rollbackCount())
	require.Equal(t, 0, s.banking.commitCount())

	// Peer that voted NO never prepared, so we must NOT send it a ROLLBACK_TX.
	require.Equal(t, []string{string(dto.MessageTypeNewTx)}, bank.got())

	st, _ := s.prepared.status(ourRouting, "otc-no")
	require.Equal(t, model.PreparedTransactionRolledBack, st)

	// The NEW_TX row was already marked SENT (it was delivered and answered with
	// a NO), so the defensive Cancel is a no-op and the row stays SENT.
	newSt, _ := s.outbound.statusByKey("k-no-new")
	require.Equal(t, model.OutboundSent, newSt)
}

func TestCoordinateTwoBankTransaction_RemoteUnreachable(t *testing.T) {
	bank := newMockBank()
	bank.close() // peer is down before we even start

	s := newCoordSetup(peersTo(bank))
	tx := crossBankTx("otc-down", 50)

	err := s.svc.coordinateTwoBankTransaction(t.Context(), 111, tx, "k-down")
	require.Error(t, err)

	// We prepared locally but must NOT commit or roll back: the outbox worker
	// will retransmit NEW_TX. The reservation stays held.
	require.Equal(t, 1, s.banking.prepareCount())
	require.Equal(t, 0, s.banking.commitCount())
	require.Equal(t, 0, s.banking.rollbackCount())

	st, _ := s.prepared.status(ourRouting, "otc-down")
	require.Equal(t, model.PreparedTransactionPrepared, st)

	// The NEW_TX outbox row remains PENDING for the worker to retry.
	newSt, _ := s.outbound.statusByKey("k-down-new")
	require.Equal(t, model.OutboundPending, newSt)
}

func TestCoordinateTwoBankTransaction_SameBank(t *testing.T) {
	bank := newMockBank()
	defer bank.close()

	s := newCoordSetup(peersTo(bank))

	// Both postings are local — a wholly same-bank transfer.
	tx := dto.Transaction{
		TransactionID: dto.ForeignBankId{RoutingNumber: ourRouting, ID: "same-1"},
		Postings: []dto.Posting{
			acctPosting("444000000000000011", -50, monas("RSD")),
			acctPosting("444000000000000022", 50, monas("RSD")),
		},
	}

	err := s.svc.coordinateTwoBankTransaction(t.Context(), ourRouting, tx, "k-same")
	require.NoError(t, err)

	// Two local postings → reserved twice, committed twice. No HTTP to the peer.
	require.Equal(t, 2, s.banking.prepareCount())
	require.Equal(t, 2, s.banking.commitCount())
	require.Empty(t, bank.got(), "same-bank transaction must not contact any peer")

	st, _ := s.prepared.status(ourRouting, "same-1")
	require.Equal(t, model.PreparedTransactionCommitted, st)
}
