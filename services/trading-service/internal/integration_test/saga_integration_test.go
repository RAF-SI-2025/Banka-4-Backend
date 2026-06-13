//go:build integration

package integration_test

// SAGA test specification (SG-01 .. SG-08) for the OTC exercise settlement
// saga. The spec's five phases map onto the implementation as follows:
//
//	F1 reserve buyer funds      -> reserveFunds (banking ReserveOtcFunds)
//	F2 reserve/verify shares    -> confirmShares (shares were already reserved
//	                               at contract activation; F2 re-validates)
//	F3 transfer funds           -> commitFunds (banking CommitOtcFunds)
//	F4 transfer share ownership -> transferOwnership (local transaction)
//	F5 finalize contract state  -> completeExecution
//
//	C1 release funds reservation -> banking ReleaseOtcFunds
//	C3 refund committed funds    -> banking RefundOtcFunds
//
// Spec status mapping: Running -> IN_PROGRESS, Compensating -> COMPENSATING,
// Compensated -> FAILED, Completed -> COMPLETED.
//
// Known deviations from the spec, dictated by the implementation's design:
//
//   - F2 and F4 apply their effects inside local DB transactions, so they have
//     no separate compensators (no C2/C4 log records); a failed F2/F4 rolls
//     back atomically and only banking-side work needs compensating.
//   - F4 marks the contract exercised together with the share transfer, making
//     it the point of no return: an F5 failure is retried forward until the
//     saga completes (no C5..C1 rollback of an already settled trade).
//   - Pre-saga rejection of an expired contract (SG-02d) expires the contract
//     and releases its share reservation as a side effect; share and money
//     conservation (I1/I2) still hold.
//
// Invariants asserted throughout (spec I1-I6):
//
//	I1 money conserved per currency: SUM(available+reserved) constant
//	I2 shares conserved per symbol: SUM(ownership amounts) constant
//	I3 no dangling reservations once the saga is terminal
//	I4 the log holds one ordered record per attempted step
//	I5 the saga ends in a terminal status (COMPLETED or FAILED)
//	I6 the contract is consumed if and only if the saga COMPLETED
import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

const (
	sagaBuyerID   = 10
	sagaSellerID  = 20
	sagaBuyerAcc  = "buyer-acc"
	sagaSellerAcc = "seller-acc"
	sagaQty       = 10
	sagaStrike    = 300.0
	sagaTradeSum  = sagaQty * sagaStrike // 3000
)

// --- stateful fake bank ---

type sagaBankReservation struct {
	status pb.OtcFundsReservationStatus
	amount float64
	buyer  string
	seller string
}

// sagaBank is a stateful in-memory banking-service double: it tracks
// available/reserved balances per account, the OTC funds reservation
// lifecycle, and the number of booked transfers (bank.transactions rows in
// spec terms). Non-OTC calls fall through to the stateless fake.
type sagaBank struct {
	*fakeBankingClient
	mu           sync.Mutex
	available    map[string]float64
	reserved     map[string]float64
	reservations map[string]*sagaBankReservation
	transactions int
}

func newSagaBank(buyerBalance float64) *sagaBank {
	return &sagaBank{
		fakeBankingClient: &fakeBankingClient{
			accountByNumber: map[string]uint64{
				sagaSellerAcc: sagaSellerID,
				sagaBuyerAcc:  sagaBuyerID,
			},
		},
		available:    map[string]float64{sagaBuyerAcc: buyerBalance, sagaSellerAcc: 0},
		reserved:     map[string]float64{sagaBuyerAcc: 0, sagaSellerAcc: 0},
		reservations: map[string]*sagaBankReservation{},
	}
}

func (b *sagaBank) response(key string, r *sagaBankReservation) *pb.OtcFundsReservationResponse {
	return &pb.OtcFundsReservationResponse{
		ExecutionId:         key,
		Status:              r.status,
		TradeAmount:         r.amount,
		TradeCurrencyCode:   "RSD",
		BuyerAccountNumber:  r.buyer,
		SellerAccountNumber: r.seller,
	}
}

func (b *sagaBank) ReserveOtcFunds(_ context.Context, req *pb.ReserveOtcFundsRequest) (*pb.OtcFundsReservationResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if r, ok := b.reservations[req.ExecutionId]; ok {
		return b.response(req.ExecutionId, r), nil
	}

	if b.available[req.BuyerAccountNumber] < req.Amount {
		return nil, status.Error(codes.FailedPrecondition, "insufficient available funds")
	}

	b.available[req.BuyerAccountNumber] -= req.Amount
	b.reserved[req.BuyerAccountNumber] += req.Amount
	r := &sagaBankReservation{
		status: pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_RESERVED,
		amount: req.Amount,
		buyer:  req.BuyerAccountNumber,
		seller: req.SellerAccountNumber,
	}
	b.reservations[req.ExecutionId] = r
	return b.response(req.ExecutionId, r), nil
}

func (b *sagaBank) CommitOtcFunds(_ context.Context, executionID string) (*pb.OtcFundsReservationResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	r, ok := b.reservations[executionID]
	if !ok {
		return nil, status.Error(codes.NotFound, "reservation not found")
	}

	switch r.status {
	case pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_COMMITTED:
		return b.response(executionID, r), nil
	case pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_RESERVED:
		b.reserved[r.buyer] -= r.amount
		b.available[r.seller] += r.amount
		r.status = pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_COMMITTED
		b.transactions++
		return b.response(executionID, r), nil
	default:
		return nil, status.Error(codes.FailedPrecondition, "reservation is not committable")
	}
}

func (b *sagaBank) ReleaseOtcFunds(_ context.Context, executionID string) (*pb.OtcFundsReservationResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	r, ok := b.reservations[executionID]
	if !ok {
		// Releasing an unknown reservation is a no-op so the compensator
		// stays idempotent.
		return &pb.OtcFundsReservationResponse{
			ExecutionId: executionID,
			Status:      pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_RELEASED,
		}, nil
	}

	switch r.status {
	case pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_RELEASED:
		return b.response(executionID, r), nil
	case pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_RESERVED:
		b.reserved[r.buyer] -= r.amount
		b.available[r.buyer] += r.amount
		r.status = pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_RELEASED
		return b.response(executionID, r), nil
	default:
		return nil, status.Error(codes.FailedPrecondition, "reservation is not releasable")
	}
}

func (b *sagaBank) RefundOtcFunds(_ context.Context, executionID string) (*pb.OtcFundsReservationResponse, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	r, ok := b.reservations[executionID]
	if !ok {
		return nil, status.Error(codes.NotFound, "reservation not found")
	}

	switch r.status {
	case pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_REFUNDED:
		return b.response(executionID, r), nil
	case pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_COMMITTED:
		b.available[r.seller] -= r.amount
		b.available[r.buyer] += r.amount
		r.status = pb.OtcFundsReservationStatus_OTC_FUNDS_RESERVATION_STATUS_REFUNDED
		b.transactions++
		return b.response(executionID, r), nil
	default:
		return nil, status.Error(codes.FailedPrecondition, "reservation is not refundable")
	}
}

func (b *sagaBank) snapshotTotal() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	total := 0.0
	for acc := range b.available {
		total += b.available[acc] + b.reserved[acc]
	}
	return total
}

func (b *sagaBank) availableOf(acc string) float64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.available[acc]
}

func (b *sagaBank) reservedTotal() float64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	total := 0.0
	for _, v := range b.reserved {
		total += v
	}
	return total
}

func (b *sagaBank) transactionCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.transactions
}

// --- setup, action, wait helpers (spec: Setup / Action / Wait / Assert) ---

func setupSagaTest(t *testing.T, buyerBalance float64) (*gin.Engine, *gorm.DB, *sagaBank) {
	t.Helper()

	db := setupTestDB(t)
	bank := newSagaBank(buyerBalance)
	router, _ := setupTestRouterWithBanking(t, db, nil, bank)
	return router, db, bank
}

// createSagaContract builds an active OTC option contract (qty=10,
// strike=300 RSD, settlement tomorrow-ish) through the public offer/accept
// flow, with the seller holding exactly the reserved 10 shares.
func createSagaContract(t *testing.T, router *gin.Engine, db *gorm.DB) (contractID uint, assetID uint) {
	t.Helper()

	asset, _ := seedAssetAndStock(t, db, uniqueValue(t, "SAGA"))
	ownership := seedAssetOwnership(t, db, sagaSellerID, model.OwnerTypeClient, asset.AssetID, sagaQty)
	setPublicAmount(t, db, ownership.AssetOwnershipID, sagaQty, 0)

	offerBody := map[string]any{
		"asset_ownership_id":   ownership.AssetOwnershipID,
		"amount":               sagaQty,
		"price_per_stock_rsd":  sagaStrike,
		"premium_rsd":          5.0,
		"settlement_date":      time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"buyer_account_number": sagaBuyerAcc,
	}
	rec := performRequest(t, router, http.MethodPost, "/api/otc/offers", offerBody, clientAuthHeader(t, sagaBuyerID, sagaBuyerID))
	requireStatus(t, rec, http.StatusCreated)
	offer := decodeResponse[map[string]any](t, rec)
	offerID := uint(offer["otc_offer_id"].(float64))

	acceptBody := map[string]any{"account_number": sagaSellerAcc}
	rec = performRequest(t, router, http.MethodPatch, fmt.Sprintf("/api/otc/offers/%d/accept", offerID), acceptBody, clientAuthHeader(t, sagaSellerID, sagaSellerID))
	requireStatus(t, rec, http.StatusCreated)

	contract := decodeResponse[map[string]any](t, rec)
	return uint(contract["otc_option_contract_id"].(float64)), asset.AssetID
}

// exerciseContract fires the spec Action: one POST on the exercise endpoint,
// optionally with adversarial X-Saga-* headers.
func exerciseContract(t *testing.T, router *gin.Engine, contractID uint, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/otc/contracts/%d/exercise", contractID), nil)
	req.Header.Set("Authorization", clientAuthHeader(t, sagaBuyerID, sagaBuyerID))
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func findSaga(t *testing.T, db *gorm.DB, contractID uint) *model.OtcExecutionSaga {
	t.Helper()

	var saga model.OtcExecutionSaga
	err := db.Where("contract_id = ?", contractID).First(&saga).Error
	if err == gorm.ErrRecordNotFound {
		return nil
	}
	require.NoError(t, err)
	return &saga
}

func isTerminal(s model.OtcExecutionStatus) bool {
	return s == model.OtcExecutionStatusCompleted || s == model.OtcExecutionStatusFailed
}

// waitSagaTerminal implements the spec Wait phase. In production the recovery
// worker pumps a non-terminal saga every poll tick; the test drives the same
// code path by re-POSTing the exercise endpoint (which resumes an in-flight
// saga) until the persisted status is terminal.
func waitSagaTerminal(t *testing.T, router *gin.Engine, db *gorm.DB, contractID uint) *model.OtcExecutionSaga {
	t.Helper()

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		saga := findSaga(t, db, contractID)
		require.NotNil(t, saga, "saga row must exist while waiting for terminal status")
		if isTerminal(saga.Status) {
			return saga
		}

		exerciseContract(t, router, contractID, nil)
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("saga for contract %d did not reach a terminal status within 30s (I5 violated)", contractID)
	return nil
}

func sagaLog(t *testing.T, db *gorm.DB, sagaID uint) []model.OtcExecutionSagaLogEntry {
	t.Helper()

	var entries []model.OtcExecutionSagaLogEntry
	require.NoError(t, db.Where("otc_execution_saga_id = ?", sagaID).
		Order("otc_execution_saga_log_entry_id ASC").Find(&entries).Error)
	return entries
}

type logStep struct {
	step    string
	outcome model.OtcExecutionLogOutcome
}

// assertSagaLog asserts spec invariant I4: one record per attempted step, in
// order.
func assertSagaLog(t *testing.T, db *gorm.DB, sagaID uint, expected []logStep) {
	t.Helper()

	entries := sagaLog(t, db, sagaID)
	require.Len(t, entries, len(expected), "saga log length mismatch: %+v", entries)
	for i, want := range expected {
		assert.Equal(t, want.step, entries[i].Step, "log entry %d step", i)
		assert.Equal(t, want.outcome, entries[i].Outcome, "log entry %d outcome", i)
	}
}

func ownershipOf(t *testing.T, db *gorm.DB, userID, assetID uint) *model.AssetOwnership {
	t.Helper()

	var o model.AssetOwnership
	err := db.Where("user_id = ? AND owner_type = ? AND asset_id = ?", userID, model.OwnerTypeClient, assetID).First(&o).Error
	if err == gorm.ErrRecordNotFound {
		return nil
	}
	require.NoError(t, err)
	return &o
}

// assertSharesConserved asserts spec invariant I2 for the contract's symbol.
func assertSharesConserved(t *testing.T, db *gorm.DB, assetID uint, wantTotal float64) {
	t.Helper()

	var total float64
	require.NoError(t, db.Model(&model.AssetOwnership{}).
		Where("asset_id = ?", assetID).
		Select("COALESCE(SUM(amount), 0)").Scan(&total).Error)
	assert.Equal(t, wantTotal, total, "I2: share total for asset changed")
}

// assertCoreInvariants checks I1 (money conserved), I3 (no dangling banking
// reservations), I5 (terminal status) and I6 (contract consumed iff
// completed) after a saga finished.
func assertCoreInvariants(t *testing.T, db *gorm.DB, bank *sagaBank, saga *model.OtcExecutionSaga, moneyTotal float64) {
	t.Helper()

	assert.Equal(t, moneyTotal, bank.snapshotTotal(), "I1: money was created or destroyed")
	assert.Equal(t, 0.0, bank.reservedTotal(), "I3: banking funds remain reserved after terminal saga")
	require.True(t, isTerminal(saga.Status), "I5: saga stuck in %s", saga.Status)

	var contract model.OtcOptionContract
	require.NoError(t, db.First(&contract, saga.ContractID).Error)
	if saga.Status == model.OtcExecutionStatusCompleted {
		assert.Equal(t, model.OtcOptionContractStatusExercised, contract.Status, "I6: completed saga must consume the contract")
	} else {
		assert.NotEqual(t, model.OtcOptionContractStatusExercised, contract.Status, "I6: failed saga must not consume the contract")
	}
}

// --- SG-01: happy path ---

func TestSaga_SG01_HappyPath(t *testing.T) {
	t.Parallel()
	router, db, bank := setupSagaTest(t, 5000)
	contractID, assetID := createSagaContract(t, router, db)

	rec := exerciseContract(t, router, contractID, nil)
	requireStatus(t, rec, http.StatusOK)
	resp := decodeResponse[map[string]any](t, rec)
	assert.Equal(t, "COMPLETED", resp["status"])
	assert.NotZero(t, resp["otc_execution_saga_id"], "response must carry the saga id")
	assert.Len(t, resp["log"], 5, "response must include the step log")

	saga := waitSagaTerminal(t, router, db, contractID)
	assert.Equal(t, model.OtcExecutionStatusCompleted, saga.Status)
	assert.Equal(t, model.OtcExecutionStepCompleted, saga.CurrentStep)

	// Buyer paid qty x strike, seller received it, exactly one booked transfer.
	assert.Equal(t, 2000.0, bank.availableOf(sagaBuyerAcc))
	assert.Equal(t, 3000.0, bank.availableOf(sagaSellerAcc))
	assert.Equal(t, 1, bank.transactionCount())

	// All shares moved to the buyer; nothing is left reserved (I3 local side).
	seller := ownershipOf(t, db, sagaSellerID, assetID)
	require.NotNil(t, seller)
	assert.Equal(t, 0.0, seller.Amount)
	assert.Equal(t, 0.0, seller.ReservedAmount)
	buyer := ownershipOf(t, db, sagaBuyerID, assetID)
	require.NotNil(t, buyer)
	assert.Equal(t, float64(sagaQty), buyer.Amount)

	var reservation model.OtcShareReservation
	require.NoError(t, db.Where("contract_id = ?", contractID).First(&reservation).Error)
	assert.Equal(t, model.OtcShareReservationStatusConsumed, reservation.Status)

	assertSagaLog(t, db, saga.OtcExecutionSagaID, []logStep{
		{"F1", model.OtcExecutionLogOutcomeOK},
		{"F2", model.OtcExecutionLogOutcomeOK},
		{"F3", model.OtcExecutionLogOutcomeOK},
		{"F4", model.OtcExecutionLogOutcomeOK},
		{"F5", model.OtcExecutionLogOutcomeOK},
	})
	assertSharesConserved(t, db, assetID, sagaQty)
	assertCoreInvariants(t, db, bank, saga, 5000)
}

// --- SG-02: pre-saga validation (parameterized) ---
// Rejected requests return an HTTP error, create no saga row and leave
// accounts and holdings untouched.

func TestSaga_SG02a_CallerIsNotBuyer(t *testing.T) {
	t.Parallel()
	router, db, bank := setupSagaTest(t, 5000)
	contractID, assetID := createSagaContract(t, router, db)

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/otc/contracts/%d/exercise", contractID), nil)
	req.Header.Set("Authorization", clientAuthHeader(t, 30, 30))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	requireStatus(t, rec, http.StatusForbidden)

	assert.Nil(t, findSaga(t, db, contractID), "pre-saga rejection must not create a saga row")
	assert.Equal(t, 5000.0, bank.availableOf(sagaBuyerAcc))
	assert.Equal(t, 0, bank.transactionCount())
	assertSharesConserved(t, db, assetID, sagaQty)
}

func TestSaga_SG02b_ContractDoesNotExist(t *testing.T) {
	t.Parallel()
	router, db, bank := setupSagaTest(t, 5000)

	rec := exerciseContract(t, router, 999999, nil)
	requireStatus(t, rec, http.StatusNotFound)

	assert.Nil(t, findSaga(t, db, 999999))
	assert.Equal(t, 5000.0, bank.availableOf(sagaBuyerAcc))
}

func TestSaga_SG02c_ContractNotActive(t *testing.T) {
	t.Parallel()
	router, db, bank := setupSagaTest(t, 5000)
	contractID, assetID := createSagaContract(t, router, db)

	// "iskorišćen" — already exercised.
	require.NoError(t, db.Model(&model.OtcOptionContract{}).
		Where("otc_option_contract_id = ?", contractID).
		Update("status", model.OtcOptionContractStatusExercised).Error)
	rec := exerciseContract(t, router, contractID, nil)
	requireStatus(t, rec, http.StatusConflict)

	// "istekao" — expired.
	require.NoError(t, db.Model(&model.OtcOptionContract{}).
		Where("otc_option_contract_id = ?", contractID).
		Update("status", model.OtcOptionContractStatusExpired).Error)
	rec = exerciseContract(t, router, contractID, nil)
	requireStatus(t, rec, http.StatusBadRequest)

	assert.Nil(t, findSaga(t, db, contractID))
	assert.Equal(t, 5000.0, bank.availableOf(sagaBuyerAcc))
	assert.Equal(t, 0, bank.transactionCount())
	assertSharesConserved(t, db, assetID, sagaQty)
}

func TestSaga_SG02d_SettlementDatePassed(t *testing.T) {
	t.Parallel()
	router, db, bank := setupSagaTest(t, 5000)
	contractID, assetID := createSagaContract(t, router, db)

	require.NoError(t, db.Model(&model.OtcOptionContract{}).
		Where("otc_option_contract_id = ?", contractID).
		Update("settlement_date", time.Now().Add(-1*time.Hour)).Error)

	rec := exerciseContract(t, router, contractID, nil)
	requireStatus(t, rec, http.StatusBadRequest)

	assert.Nil(t, findSaga(t, db, contractID), "no saga row for a pre-saga rejection")
	assert.Equal(t, 5000.0, bank.availableOf(sagaBuyerAcc))
	assert.Equal(t, 0, bank.transactionCount())

	// Implementation detail: hitting an expired contract expires it on the
	// spot and releases the seller's share reservation. The shares stay with
	// the seller (I2 holds); only the reservation bookkeeping changes.
	var contract model.OtcOptionContract
	require.NoError(t, db.First(&contract, contractID).Error)
	assert.Equal(t, model.OtcOptionContractStatusExpired, contract.Status)
	assertSharesConserved(t, db, assetID, sagaQty)
}

// --- SG-03: insufficient funds (F1 failure) ---

func TestSaga_SG03_InsufficientFunds(t *testing.T) {
	t.Parallel()
	router, db, bank := setupSagaTest(t, 500) // contract needs 3000
	contractID, assetID := createSagaContract(t, router, db)

	rec := exerciseContract(t, router, contractID, nil)
	requireStatus(t, rec, http.StatusOK)

	saga := waitSagaTerminal(t, router, db, contractID)
	assert.Equal(t, model.OtcExecutionStatusFailed, saga.Status)
	assert.Equal(t, model.OtcExecutionStepInit, saga.CurrentStep, "failure happened attempting F1")

	assertSagaLog(t, db, saga.OtcExecutionSagaID, []logStep{
		{"F1", model.OtcExecutionLogOutcomeErr},
	})

	// No side effects at all.
	assert.Equal(t, 500.0, bank.availableOf(sagaBuyerAcc))
	assert.Equal(t, 0.0, bank.availableOf(sagaSellerAcc))
	assert.Equal(t, 0, bank.transactionCount())
	seller := ownershipOf(t, db, sagaSellerID, assetID)
	require.NotNil(t, seller)
	assert.Equal(t, float64(sagaQty), seller.Amount)
	assertSharesConserved(t, db, assetID, sagaQty)
	assertCoreInvariants(t, db, bank, saga, 500)
}

// --- SG-04: insufficient shares (F2 failure) ---

func TestSaga_SG04_InsufficientShares(t *testing.T) {
	t.Parallel()
	router, db, bank := setupSagaTest(t, 5000)
	contractID, assetID := createSagaContract(t, router, db)

	// The seller somehow lost most of the holding after activation: 3 < 10.
	require.NoError(t, db.Model(&model.AssetOwnership{}).
		Where("user_id = ? AND owner_type = ? AND asset_id = ?", sagaSellerID, model.OwnerTypeClient, assetID).
		Update("amount", 3).Error)

	rec := exerciseContract(t, router, contractID, nil)
	requireStatus(t, rec, http.StatusOK)

	saga := waitSagaTerminal(t, router, db, contractID)
	assert.Equal(t, model.OtcExecutionStatusFailed, saga.Status)
	assert.Equal(t, model.OtcExecutionStepFundsReserved, saga.CurrentStep, "failure happened attempting F2")

	assertSagaLog(t, db, saga.OtcExecutionSagaID, []logStep{
		{"F1", model.OtcExecutionLogOutcomeOK},
		{"F2", model.OtcExecutionLogOutcomeErr},
		{"C1", model.OtcExecutionLogOutcomeOK},
	})

	// The F1 reservation was released by C1; account state is unchanged.
	assert.Equal(t, 5000.0, bank.availableOf(sagaBuyerAcc))
	assert.Equal(t, 0.0, bank.availableOf(sagaSellerAcc))
	assert.Equal(t, 0, bank.transactionCount())
	assertSharesConserved(t, db, assetID, 3)
	assertCoreInvariants(t, db, bank, saga, 5000)
}

// --- SG-05: F3 failure, compensation C1 ---
// (Spec lists C2+C1; F2 has no side effects in this implementation, so the
// only compensator is C1.)

func TestSaga_SG05_F3FailureCompensates(t *testing.T) {
	t.Parallel()
	router, db, bank := setupSagaTest(t, 5000)
	contractID, assetID := createSagaContract(t, router, db)

	rec := exerciseContract(t, router, contractID, map[string]string{
		"X-Saga-Force-Fail": "F3",
	})
	requireStatus(t, rec, http.StatusOK)

	saga := waitSagaTerminal(t, router, db, contractID)
	assert.Equal(t, model.OtcExecutionStatusFailed, saga.Status)
	assert.Equal(t, model.OtcExecutionStepFundsReserved, saga.CurrentStep, "failure happened attempting F3")

	assertSagaLog(t, db, saga.OtcExecutionSagaID, []logStep{
		{"F1", model.OtcExecutionLogOutcomeOK},
		{"F2", model.OtcExecutionLogOutcomeOK},
		{"F3", model.OtcExecutionLogOutcomeErr},
		{"C1", model.OtcExecutionLogOutcomeOK},
	})

	// State identical to before the attempt.
	assert.Equal(t, 5000.0, bank.availableOf(sagaBuyerAcc))
	assert.Equal(t, 0.0, bank.availableOf(sagaSellerAcc))
	assert.Equal(t, 0, bank.transactionCount())
	seller := ownershipOf(t, db, sagaSellerID, assetID)
	require.NotNil(t, seller)
	assert.Equal(t, float64(sagaQty), seller.Amount)
	assert.Equal(t, float64(sagaQty), seller.ReservedAmount, "contract reservation stays active for a future retry")
	assertSharesConserved(t, db, assetID, sagaQty)
	assertCoreInvariants(t, db, bank, saga, 5000)
}

// --- SG-06: F4 failure, compensation C3 ---
// (Spec lists C3+C2+C1; after F3 committed the funds, the banking-side undo is
// a single refund, and F2/F1 reservations no longer exist to release.)

func TestSaga_SG06_F4FailureCompensates(t *testing.T) {
	t.Parallel()
	router, db, bank := setupSagaTest(t, 5000)
	contractID, assetID := createSagaContract(t, router, db)

	rec := exerciseContract(t, router, contractID, map[string]string{
		"X-Saga-Force-Fail": "F4",
	})
	requireStatus(t, rec, http.StatusOK)

	saga := waitSagaTerminal(t, router, db, contractID)
	assert.Equal(t, model.OtcExecutionStatusFailed, saga.Status)
	assert.Equal(t, model.OtcExecutionStepFundsCommitted, saga.CurrentStep, "failure happened attempting F4")

	assertSagaLog(t, db, saga.OtcExecutionSagaID, []logStep{
		{"F1", model.OtcExecutionLogOutcomeOK},
		{"F2", model.OtcExecutionLogOutcomeOK},
		{"F3", model.OtcExecutionLogOutcomeOK},
		{"F4", model.OtcExecutionLogOutcomeErr},
		{"C3", model.OtcExecutionLogOutcomeOK},
	})

	// The committed transfer was refunded: two booked transactions, money back
	// with the buyer, shares untouched.
	assert.Equal(t, 5000.0, bank.availableOf(sagaBuyerAcc))
	assert.Equal(t, 0.0, bank.availableOf(sagaSellerAcc))
	assert.Equal(t, 2, bank.transactionCount(), "commit + compensating refund must both be booked")
	seller := ownershipOf(t, db, sagaSellerID, assetID)
	require.NotNil(t, seller)
	assert.Equal(t, float64(sagaQty), seller.Amount)
	buyer := ownershipOf(t, db, sagaBuyerID, assetID)
	if buyer != nil {
		assert.Equal(t, 0.0, buyer.Amount)
	}
	assertSharesConserved(t, db, assetID, sagaQty)
	assertCoreInvariants(t, db, bank, saga, 5000)
}

// --- SG-07: F5 failure ---
// Deviation from the spec: F4 settles funds and shares atomically and marks
// the contract exercised, so F5 (finalizing the saga record) has nothing left
// to compensate. An injected F5 failure is recovered FORWARD: the saga retries
// F5 and completes. Rolling back here would un-exercise a settled trade.

func TestSaga_SG07_F5FailureRecoversForward(t *testing.T) {
	t.Parallel()
	router, db, bank := setupSagaTest(t, 5000)
	contractID, assetID := createSagaContract(t, router, db)

	rec := exerciseContract(t, router, contractID, map[string]string{
		"X-Saga-Force-Fail": "F5",
	})
	// The injected failure surfaces as an error on the triggering request.
	require.Equal(t, http.StatusInternalServerError, rec.Code, "body=%s", rec.Body.String())

	saga := waitSagaTerminal(t, router, db, contractID)
	assert.Equal(t, model.OtcExecutionStatusCompleted, saga.Status)
	assert.Equal(t, model.OtcExecutionStepCompleted, saga.CurrentStep)

	assertSagaLog(t, db, saga.OtcExecutionSagaID, []logStep{
		{"F1", model.OtcExecutionLogOutcomeOK},
		{"F2", model.OtcExecutionLogOutcomeOK},
		{"F3", model.OtcExecutionLogOutcomeOK},
		{"F4", model.OtcExecutionLogOutcomeOK},
		{"F5", model.OtcExecutionLogOutcomeErr},
		{"F5", model.OtcExecutionLogOutcomeOK},
	})

	// Final state is the happy-path state.
	assert.Equal(t, 2000.0, bank.availableOf(sagaBuyerAcc))
	assert.Equal(t, 3000.0, bank.availableOf(sagaSellerAcc))
	assert.Equal(t, 1, bank.transactionCount())
	buyer := ownershipOf(t, db, sagaBuyerID, assetID)
	require.NotNil(t, buyer)
	assert.Equal(t, float64(sagaQty), buyer.Amount)
	assertSharesConserved(t, db, assetID, sagaQty)
	assertCoreInvariants(t, db, bank, saga, 5000)
}

// --- SG-08: compensator fails once, then succeeds ---
// Mirrors the spec example: F3 is forced to fail and its compensator (C1 in
// this implementation) fails once before succeeding. The saga must keep
// retrying the compensator until it goes through.

func TestSaga_SG08_CompensatorFailsOnceThenSucceeds(t *testing.T) {
	t.Parallel()
	router, db, bank := setupSagaTest(t, 5000)
	contractID, assetID := createSagaContract(t, router, db)

	rec := exerciseContract(t, router, contractID, map[string]string{
		"X-Saga-Force-Fail":            "F3",
		"X-Saga-Compensate-Fail":       "C1",
		"X-Saga-Compensate-Fail-Times": "1",
	})
	requireStatus(t, rec, http.StatusOK)

	saga := waitSagaTerminal(t, router, db, contractID)
	assert.Equal(t, model.OtcExecutionStatusFailed, saga.Status)
	assert.Equal(t, model.OtcExecutionStepFundsReserved, saga.CurrentStep)

	assertSagaLog(t, db, saga.OtcExecutionSagaID, []logStep{
		{"F1", model.OtcExecutionLogOutcomeOK},
		{"F2", model.OtcExecutionLogOutcomeOK},
		{"F3", model.OtcExecutionLogOutcomeErr},
		{"C1", model.OtcExecutionLogOutcomeErr},
		{"C1", model.OtcExecutionLogOutcomeOK},
	})

	assert.Equal(t, 5000.0, bank.availableOf(sagaBuyerAcc))
	assert.Equal(t, 0.0, bank.availableOf(sagaSellerAcc))
	assert.Equal(t, 0, bank.transactionCount())
	assertSharesConserved(t, db, assetID, sagaQty)
	assertCoreInvariants(t, db, bank, saga, 5000)
}
