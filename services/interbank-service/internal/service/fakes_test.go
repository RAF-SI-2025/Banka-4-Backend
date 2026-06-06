package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/config"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
)

// This file holds in-memory fakes shared by the service-layer unit tests.
// Per the project conventions we never use a mock framework: every dependency
// is a hand-written struct with map-backed storage and optional injectable
// behaviour (func fields) so a test can force errors or specific votes.

// ---------------------------------------------------------------------------
// Transaction manager — runs the closure directly (no real DB transaction).
// ---------------------------------------------------------------------------

type fakeTxManager struct{}

func (fakeTxManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// ---------------------------------------------------------------------------
// Inbound message repository.
// ---------------------------------------------------------------------------

type fakeInbound struct {
	mu   sync.Mutex
	rows map[string]model.InboundMessage
}

func newFakeInbound() *fakeInbound {
	return &fakeInbound{rows: map[string]model.InboundMessage{}}
}

func inboundKey(routing int, key string) string { return fmt.Sprintf("%d|%s", routing, key) }

func (r *fakeInbound) FindByKey(_ context.Context, routing int, key string) (*model.InboundMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.rows[inboundKey(routing, key)]
	if !ok {
		return nil, nil
	}
	cp := row
	return &cp, nil
}

func (r *fakeInbound) Save(_ context.Context, m *model.InboundMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[inboundKey(m.PeerRoutingNumber, m.LocallyGeneratedKey)] = *m
	return nil
}

// ---------------------------------------------------------------------------
// Prepared transaction repository.
// ---------------------------------------------------------------------------

type fakePrepared struct {
	mu   sync.Mutex
	rows map[string]model.PreparedTransaction
}

func newFakePrepared() *fakePrepared {
	return &fakePrepared{rows: map[string]model.PreparedTransaction{}}
}

func preparedKey(routing int, id string) string { return fmt.Sprintf("%d|%s", routing, id) }

func (r *fakePrepared) Create(_ context.Context, tx *model.PreparedTransaction) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := preparedKey(tx.RoutingNumber, tx.ID)
	if _, exists := r.rows[k]; exists {
		return fmt.Errorf("duplicate prepared transaction %s", k)
	}
	r.rows[k] = *tx
	return nil
}

func (r *fakePrepared) FindByID(_ context.Context, routing int, id string) (*model.PreparedTransaction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.rows[preparedKey(routing, id)]
	if !ok {
		return nil, nil
	}
	cp := row
	return &cp, nil
}

func (r *fakePrepared) Update(_ context.Context, tx *model.PreparedTransaction) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[preparedKey(tx.RoutingNumber, tx.ID)] = *tx
	return nil
}

func (r *fakePrepared) seed(tx model.PreparedTransaction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[preparedKey(tx.RoutingNumber, tx.ID)] = tx
}

func (r *fakePrepared) status(routing int, id string) (model.PreparedTransactionStatus, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.rows[preparedKey(routing, id)]
	if !ok {
		return "", false
	}
	return row.Status, true
}

// ---------------------------------------------------------------------------
// Outbound message repository.
// ---------------------------------------------------------------------------

type fakeOutbound struct {
	mu      sync.Mutex
	nextID  uint
	rows    map[uint]*model.OutboundMessage
	byKey   map[string]uint
	enqueue []string // idempotence keys in enqueue order
}

func newFakeOutbound() *fakeOutbound {
	return &fakeOutbound{rows: map[uint]*model.OutboundMessage{}, byKey: map[string]uint{}}
}

func (r *fakeOutbound) Enqueue(_ context.Context, m *model.OutboundMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Mirror the real ON CONFLICT DO NOTHING on idempotence_key_local.
	if _, dup := r.byKey[m.IdempotenceKeyLocal]; dup {
		return nil
	}
	r.nextID++
	m.ID = r.nextID
	if m.Status == "" {
		m.Status = model.OutboundPending
	}
	cp := *m
	r.rows[m.ID] = &cp
	r.byKey[m.IdempotenceKeyLocal] = m.ID
	r.enqueue = append(r.enqueue, m.IdempotenceKeyLocal)
	return nil
}

func (r *fakeOutbound) NextBatch(_ context.Context, limit int) ([]model.OutboundMessage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []model.OutboundMessage
	for _, m := range r.rows {
		if m.Status == model.OutboundPending {
			out = append(out, *m)
		}
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (r *fakeOutbound) MarkSent(_ context.Context, id uint, status int, body []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.rows[id]; ok {
		m.Status = model.OutboundSent
		m.LastResponseStatus = status
		m.LastResponseBody = body
	}
	return nil
}

func (r *fakeOutbound) MarkFailed(_ context.Context, id uint, lastErr string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.rows[id]; ok {
		m.Status = model.OutboundFailed
		m.LastError = lastErr
	}
	return nil
}

func (r *fakeOutbound) Reschedule(_ context.Context, id uint, attempts int, lastErr string, lastStatus int, lastBody []byte, nextRetryAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.rows[id]; ok {
		m.Attempts = attempts
		m.LastError = lastErr
		m.LastResponseStatus = lastStatus
		m.LastResponseBody = lastBody
		m.NextRetryAt = nextRetryAt
	}
	return nil
}

func (r *fakeOutbound) Cancel(_ context.Context, id uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.rows[id]; ok && m.Status == model.OutboundPending {
		m.Status = model.OutboundCanceled
	}
	return nil
}

func (r *fakeOutbound) statusByKey(key string) (model.OutboundMessageStatus, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byKey[key]
	if !ok {
		return "", false
	}
	return r.rows[id].Status, true
}

func (r *fakeOutbound) byKeyRow(key string) (model.OutboundMessage, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byKey[key]
	if !ok {
		return model.OutboundMessage{}, false
	}
	return *r.rows[id], true
}

// ---------------------------------------------------------------------------
// Peer contract repository.
// ---------------------------------------------------------------------------

type fakeContracts struct {
	mu   sync.Mutex
	rows map[string]model.PeerContract
}

func newFakeContracts() *fakeContracts {
	return &fakeContracts{rows: map[string]model.PeerContract{}}
}

func contractKey(authority int, id string) string { return fmt.Sprintf("%d|%s", authority, id) }

func (r *fakeContracts) Create(_ context.Context, c *model.PeerContract) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := contractKey(c.AuthorityRoutingNumber, c.ID)
	if _, exists := r.rows[k]; exists {
		return fmt.Errorf("duplicate contract %s", k)
	}
	r.rows[k] = *c
	return nil
}

func (r *fakeContracts) FindByID(_ context.Context, authority int, id string) (*model.PeerContract, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.rows[contractKey(authority, id)]
	if !ok {
		return nil, nil
	}
	cp := row
	return &cp, nil
}

func (r *fakeContracts) FindByNegotiationID(_ context.Context, authority int, negotiationID string) (*model.PeerContract, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, row := range r.rows {
		if row.AuthorityRoutingNumber == authority && row.NegotiationID == negotiationID {
			cp := row
			return &cp, nil
		}
	}
	return nil, nil
}

func (r *fakeContracts) ListByParty(_ context.Context, routing int, partyID string) ([]model.PeerContract, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []model.PeerContract
	for _, row := range r.rows {
		if (row.BuyerRoutingNumber == routing && row.BuyerID == partyID) ||
			(row.SellerRoutingNumber == routing && row.SellerID == partyID) {
			out = append(out, row)
		}
	}
	return out, nil
}

func (r *fakeContracts) Update(_ context.Context, c *model.PeerContract) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[contractKey(c.AuthorityRoutingNumber, c.ID)] = *c
	return nil
}

func (r *fakeContracts) FindActive(_ context.Context) ([]model.PeerContract, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []model.PeerContract
	for _, row := range r.rows {
		if row.Status == model.PeerContractActive {
			out = append(out, row)
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Peer negotiation repository.
// ---------------------------------------------------------------------------

type fakeNegotiations struct {
	mu   sync.Mutex
	rows map[string]model.PeerNegotiation
}

func newFakeNegotiations() *fakeNegotiations {
	return &fakeNegotiations{rows: map[string]model.PeerNegotiation{}}
}

func (r *fakeNegotiations) Create(_ context.Context, n *model.PeerNegotiation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.rows[n.ID]; exists {
		return fmt.Errorf("duplicate negotiation %s", n.ID)
	}
	r.rows[n.ID] = *n
	return nil
}

func (r *fakeNegotiations) FindByID(_ context.Context, id string) (*model.PeerNegotiation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	row, ok := r.rows[id]
	if !ok {
		return nil, nil
	}
	cp := row
	return &cp, nil
}

func (r *fakeNegotiations) FindByIDForUpdate(ctx context.Context, id string) (*model.PeerNegotiation, error) {
	return r.FindByID(ctx, id)
}

func (r *fakeNegotiations) Update(_ context.Context, n *model.PeerNegotiation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[n.ID] = *n
	return nil
}

func (r *fakeNegotiations) ListByParty(_ context.Context, routing int, partyID string) ([]model.PeerNegotiation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []model.PeerNegotiation
	for _, row := range r.rows {
		if (row.BuyerRoutingNumber == routing && row.BuyerID == partyID) ||
			(row.SellerRoutingNumber == routing && row.SellerID == partyID) {
			out = append(out, row)
		}
	}
	return out, nil
}

func (r *fakeNegotiations) FindOngoing(_ context.Context) ([]model.PeerNegotiation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []model.PeerNegotiation
	for _, row := range r.rows {
		if row.Status == model.PeerNegotiationOngoing {
			out = append(out, row)
		}
	}
	return out, nil
}

func (r *fakeNegotiations) seed(n model.PeerNegotiation) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[n.ID] = n
}

// ---------------------------------------------------------------------------
// Banking client (gRPC surface used by MessageProcessor).
// ---------------------------------------------------------------------------

type fakeBanking struct {
	mu sync.Mutex

	prepareErr  error
	commitErr   error
	rollbackErr error

	prepareCalls  []*pb.PrepareInterbankCashPostingRequest
	commitCalls   []string
	rollbackCalls []string
	finalizeCalls []finalizeCall
}

type finalizeCall struct {
	bankingTxID uint64
	success     bool
}

func (b *fakeBanking) ReserveOtcFunds(context.Context, *pb.ReserveOtcFundsRequest) (*pb.OtcFundsReservationResponse, error) {
	return &pb.OtcFundsReservationResponse{}, nil
}
func (b *fakeBanking) ReleaseOtcFunds(context.Context, string) (*pb.OtcFundsReservationResponse, error) {
	return &pb.OtcFundsReservationResponse{}, nil
}
func (b *fakeBanking) CommitOtcFunds(context.Context, string) (*pb.OtcFundsReservationResponse, error) {
	return &pb.OtcFundsReservationResponse{}, nil
}
func (b *fakeBanking) RefundOtcFunds(context.Context, string) (*pb.OtcFundsReservationResponse, error) {
	return &pb.OtcFundsReservationResponse{}, nil
}

func (b *fakeBanking) PrepareInterbankCashPosting(_ context.Context, req *pb.PrepareInterbankCashPostingRequest) (*pb.InterbankCashPostingResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.prepareCalls = append(b.prepareCalls, req)
	if b.prepareErr != nil {
		return nil, b.prepareErr
	}
	return &pb.InterbankCashPostingResponse{}, nil
}

func (b *fakeBanking) CommitInterbankCashPosting(_ context.Context, postingID string) (*pb.InterbankCashPostingResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.commitCalls = append(b.commitCalls, postingID)
	if b.commitErr != nil {
		return nil, b.commitErr
	}
	return &pb.InterbankCashPostingResponse{}, nil
}

func (b *fakeBanking) RollbackInterbankCashPosting(_ context.Context, postingID string) (*pb.InterbankCashPostingResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rollbackCalls = append(b.rollbackCalls, postingID)
	if b.rollbackErr != nil {
		return nil, b.rollbackErr
	}
	return &pb.InterbankCashPostingResponse{}, nil
}

func (b *fakeBanking) FinalizeInterbankPayment(_ context.Context, bankingTxID uint64, success bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.finalizeCalls = append(b.finalizeCalls, finalizeCall{bankingTxID: bankingTxID, success: success})
	return nil
}

func (b *fakeBanking) prepareCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.prepareCalls)
}

func (b *fakeBanking) commitCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.commitCalls)
}

func (b *fakeBanking) rollbackCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.rollbackCalls)
}

// ---------------------------------------------------------------------------
// Trading client (gRPC surface used by MessageProcessor).
// ---------------------------------------------------------------------------

type fakeTrading struct {
	mu sync.Mutex

	reserveErr error
	releaseErr error
	consumeErr error
	creditErr  error

	reserveCalls []*pb.ReservePeerOtcSharesRequest
	releaseCalls []string
	consumeCalls []string
	creditCalls  []*pb.CreditPeerOtcSharesRequest
}

func (t *fakeTrading) ListPublicStocks(context.Context) (*pb.ListPublicStocksResponse, error) {
	return &pb.ListPublicStocksResponse{}, nil
}

func (t *fakeTrading) ReservePeerOtcShares(_ context.Context, req *pb.ReservePeerOtcSharesRequest) (*pb.PeerOtcSharesResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.reserveCalls = append(t.reserveCalls, req)
	if t.reserveErr != nil {
		return nil, t.reserveErr
	}
	return &pb.PeerOtcSharesResponse{}, nil
}

func (t *fakeTrading) ReleasePeerOtcShares(_ context.Context, contractID string) (*pb.PeerOtcSharesResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.releaseCalls = append(t.releaseCalls, contractID)
	if t.releaseErr != nil {
		return nil, t.releaseErr
	}
	return &pb.PeerOtcSharesResponse{}, nil
}

func (t *fakeTrading) ConsumePeerOtcShares(_ context.Context, contractID string) (*pb.PeerOtcSharesResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.consumeCalls = append(t.consumeCalls, contractID)
	if t.consumeErr != nil {
		return nil, t.consumeErr
	}
	return &pb.PeerOtcSharesResponse{}, nil
}

func (t *fakeTrading) CreditPeerOtcShares(_ context.Context, req *pb.CreditPeerOtcSharesRequest) (*pb.PeerOtcSharesResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.creditCalls = append(t.creditCalls, req)
	if t.creditErr != nil {
		return nil, t.creditErr
	}
	return &pb.PeerOtcSharesResponse{}, nil
}

// ---------------------------------------------------------------------------
// Peer resolver helpers.
// ---------------------------------------------------------------------------

func testResolver(ourRouting int, peers ...config.Peer) *PeerResolver {
	reg := config.NewPeerRegistry(peers)
	return NewPeerResolver(reg, &config.Configuration{
		OurRoutingNumber:   ourRouting,
		OurBankDisplayName: "Banka 4",
	})
}
