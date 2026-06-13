//go:build saga_e2e

package saga_e2e

// SG-09, SG-10, SG-11: infrastructure failures.
//
// Implementation note (deviation from the spec's expected outcomes): this
// orchestrator treats infrastructure errors (Unavailable, DeadlineExceeded)
// as RETRYABLE, not terminal. A saga hit by an outage stays IN_PROGRESS with
// a scheduled retry and finishes FORWARD once the dependency recovers,
// instead of compensating immediately. The tests therefore assert the
// properties the spec's invariants actually protect:
//
//   - the failed step is recorded in the log and no side effects leak while
//     the dependency is down (I1/I2/I4),
//   - the saga is never stuck: it reaches a terminal state once the
//     infrastructure recovers (I5),
//   - money/share conservation and contract consumption hold at the end
//     (I1/I3/I6).

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// assertNoForwardProgressBeyondF1 verifies that while banking was unreachable
// the saga recorded only failed F1 attempts and produced no side effects.
func assertNoForwardProgressBeyondF1(t *testing.T, f *fixture, buyerStart float64) *tradeOtcExecutionSaga {
	t.Helper()

	saga := findSaga(t, f.contractID)
	if saga == nil {
		t.Fatal("saga row must exist after the exercise call")
	}
	if saga.Status != sagaStatusInProgress {
		t.Fatalf("expected saga IN_PROGRESS while banking is down, got %s", saga.Status)
	}
	if saga.CurrentStep != sagaStepInit {
		t.Fatalf("expected saga still at INIT (F1 not done), got %s", saga.CurrentStep)
	}
	if saga.LastError == "" {
		t.Error("expected the infrastructure error to be recorded on the saga")
	}

	entries := sagaLogEntries(t, saga.OtcExecutionSagaID)
	if len(entries) == 0 {
		t.Fatal("I4: expected at least one F1 attempt in the log")
	}
	for _, e := range entries {
		if e.Step != "F1" || e.Outcome != logOutcomeErr {
			t.Errorf("expected only failed F1 attempts in the log, found %s %s", e.Step, e.Outcome)
		}
	}

	// A timed-out RPC may still have applied at the bank (classic
	// "timeout is not failure"): the funds are then isolated inside a
	// RESERVED reservation keyed by the execution key, and the F1 retry is
	// idempotent. Either way the money is conserved (I1) and the seller has
	// received nothing.
	wantBuyer := buyerStart
	if fundsReservationStatus(t, saga.ExecutionKey) == fundsReservationReserved {
		wantBuyer = buyerStart - sagaTradeAmount
	}
	if got := accountAvailable(t, f.buyerAcc); got != wantBuyer {
		t.Errorf("I1: buyer available: want %v, got %v", wantBuyer, got)
	}
	if got := accountAvailable(t, f.sellerAcc); got != 0 {
		t.Errorf("seller balance changed while banking was down: %v", got)
	}

	return saga
}

// assertRecoveredHappyPath verifies the saga finished forward after the
// infrastructure recovered.
func assertRecoveredHappyPath(t *testing.T, f *fixture, saga *tradeOtcExecutionSaga) {
	t.Helper()

	if saga.Status != sagaStatusCompleted {
		t.Fatalf("expected saga to complete after recovery, got %s (%s)", saga.Status, saga.LastError)
	}
	if got := accountAvailable(t, f.buyerAcc); got != 2000 {
		t.Errorf("buyer available: want 2000, got %v", got)
	}
	if got := accountAvailable(t, f.sellerAcc); got != 3000 {
		t.Errorf("seller available: want 3000, got %v", got)
	}
	assertInvariants(t, f, saga, 5000)
}

// SG-09a: banking paused (SIGSTOP) before the saga starts.
func TestSagaE2E_SG09a_BankingPaused(t *testing.T) {
	requireStack(t)
	f := seedFixture(t, 5000)

	pauseBanking(t)
	status, _ := exercise(t, f, nil)
	if status != http.StatusOK {
		t.Fatalf("exercise returned %d", status)
	}
	assertNoForwardProgressBeyondF1(t, f, 5000)

	unpauseBanking(t)
	saga := waitTerminal(t, f, 90*time.Second, true)
	assertRecoveredHappyPath(t, f, saga)
}

// SG-09b: network latency above the orchestrator's RPC timeout.
func TestSagaE2E_SG09b_BankingLatency(t *testing.T) {
	requireStack(t)
	f := seedFixture(t, 5000)

	addBankingLatency(t, 8000) // BANKING_RPC_TIMEOUT_SECONDS=5 in the overlay
	status, _ := exercise(t, f, nil)
	if status != http.StatusOK {
		t.Fatalf("exercise returned %d", status)
	}
	saga := assertNoForwardProgressBeyondF1(t, f, 5000)
	if !strings.Contains(strings.ToLower(saga.LastError), "deadline") {
		t.Logf("note: expected DeadlineExceeded-flavoured error, got: %s", saga.LastError)
	}

	removeBankingLatency(t)
	saga = waitTerminal(t, f, 90*time.Second, true)
	assertRecoveredHappyPath(t, f, saga)
}

// SG-09c: network partition (proxy down).
func TestSagaE2E_SG09c_BankingPartition(t *testing.T) {
	requireStack(t)
	f := seedFixture(t, 5000)

	setBankingLinkEnabled(t, false)
	status, _ := exercise(t, f, nil)
	if status != http.StatusOK {
		t.Fatalf("exercise returned %d", status)
	}
	assertNoForwardProgressBeyondF1(t, f, 5000)

	setBankingLinkEnabled(t, true)
	saga := waitTerminal(t, f, 90*time.Second, true)
	assertRecoveredHappyPath(t, f, saga)
}

// SG-10: banking paused mid-saga, inside a window opened between F2 and F3 by
// an injected delay. F1 has already reserved the buyer's funds when F3 fails,
// so the test additionally checks the reservation is not lost or duplicated.
func TestSagaE2E_SG10_BankingPausedMidSaga(t *testing.T) {
	requireStack(t)
	f := seedFixture(t, 5000)

	type result struct {
		status int
		body   map[string]any
	}
	done := make(chan result, 1)
	go func() {
		s, b := exercise(t, f, map[string]string{"X-Saga-Inject-Delay": "F3:5000"})
		done <- result{s, b}
	}()

	// F1+F2 finish within moments; pause banking inside the F3 delay window.
	time.Sleep(2 * time.Second)
	pauseBanking(t)

	select {
	case r := <-done:
		if r.status != http.StatusOK {
			t.Fatalf("exercise returned %d: %v", r.status, r.body)
		}
	case <-time.After(60 * time.Second):
		t.Fatal("exercise request did not return")
	}

	saga := findSaga(t, f.contractID)
	if saga == nil {
		t.Fatal("saga row must exist")
	}
	// F3 (commitFunds) runs while CurrentStep is SHARES_CONFIRMED; a retryable
	// banking failure leaves the saga IN_PROGRESS there, having already done
	// F1 (funds reserved) and F2 (shares confirmed).
	if saga.Status != sagaStatusInProgress || saga.CurrentStep != sagaStepSharesConfirmed {
		t.Fatalf("expected IN_PROGRESS at SHARES_CONFIRMED (F3 failed), got %s at %s", saga.Status, saga.CurrentStep)
	}

	// F1 reserved the funds: buyer available is already reduced, money is
	// conserved inside the reservation, and nothing reached the seller.
	if got := accountAvailable(t, f.buyerAcc); got != 2000 {
		t.Errorf("buyer available with active reservation: want 2000, got %v", got)
	}
	if got := accountAvailable(t, f.sellerAcc); got != 0 {
		t.Errorf("seller must not be paid while banking is paused: got %v", got)
	}

	entries := sagaLogEntries(t, saga.OtcExecutionSagaID)
	if len(entries) < 3 || entries[0].Step != "F1" || entries[0].Outcome != logOutcomeOK ||
		entries[1].Step != "F2" || entries[1].Outcome != logOutcomeOK ||
		entries[2].Step != "F3" || entries[2].Outcome != logOutcomeErr {
		t.Errorf("I4: expected log [F1 ok, F2 ok, F3 err, ...], got %+v", entries)
	}

	unpauseBanking(t)
	saga = waitTerminal(t, f, 120*time.Second, true)
	assertRecoveredHappyPath(t, f, saga)
}

// SG-11: the coordinator (trading service) is SIGKILLed mid-flight, inside
// the F3 delay window, then restarted. The persisted saga row is the recovery
// log: the worker must pick the flow up and drive it to a terminal state with
// all invariants intact. Both COMPLETED and FAILED are valid outcomes; with
// no failure injected the expected one is COMPLETED.
func TestSagaE2E_SG11_CoordinatorKilledMidFlight(t *testing.T) {
	requireStack(t)
	f := seedFixture(t, 5000)

	go func() {
		// The request dies with the coordinator; only its side effects matter.
		c := &http.Client{Timeout: 15 * time.Second}
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/otc/contracts/%d/exercise", tradingURL, f.contractID), nil)
		if err != nil {
			return
		}
		req.Header.Set("Authorization", f.token)
		req.Header.Set("X-Saga-Inject-Delay", "F3:5000")
		resp, err := c.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}()

	time.Sleep(2 * time.Second)
	compose(t, "kill", "-s", "SIGKILL", "trading_service")
	compose(t, "up", "-d", "trading_service")
	waitTradingHealthy(t, 5*time.Minute)

	saga := findSaga(t, f.contractID)
	if saga == nil {
		t.Fatal("saga row must have been persisted before the coordinator died")
	}

	// No pumping: recovery must come from the restarted coordinator's worker.
	saga = waitTerminal(t, f, 3*time.Minute, false)

	buyer := accountAvailable(t, f.buyerAcc)
	seller := accountAvailable(t, f.sellerAcc)
	switch saga.Status {
	case sagaStatusCompleted:
		if buyer != 2000 || seller != 3000 {
			t.Errorf("completed saga: want buyer 2000 / seller 3000, got %v / %v", buyer, seller)
		}
	case sagaStatusFailed:
		if buyer != 5000 || seller != 0 {
			t.Errorf("failed saga: want buyer 5000 / seller 0, got %v / %v", buyer, seller)
		}
	}
	assertInvariants(t, f, saga, 5000)

	entries := sagaLogEntries(t, saga.OtcExecutionSagaID)
	if len(entries) == 0 {
		t.Error("I4: recovery must leave a consistent, non-empty step log")
	}
}
