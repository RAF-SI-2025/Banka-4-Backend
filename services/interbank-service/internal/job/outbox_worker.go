package job

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/config"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/repository"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/service"
)

const outboxBatchSize = 20

type OutboxWorker struct {
	outboundMessageRepo repository.OutboundMessageRepository
	txManager           repository.TransactionManager
	messageProcessor    *service.MessageProcessor
	peerClient          *service.PeerOtcClient
	resolver            *service.PeerResolver
	httpClient          *http.Client
	pollEvery           time.Duration
	maxAttempts         int
	ourRoutingNumber    int
	stop                chan struct{}
}

func NewOutboxWorker(
	outboundMessageRepo repository.OutboundMessageRepository,
	txManager repository.TransactionManager,
	messageProcessor *service.MessageProcessor,
	peerClient *service.PeerOtcClient,
	resolver *service.PeerResolver,
	cfg *config.Configuration,
) *OutboxWorker {
	return &OutboxWorker{
		outboundMessageRepo: outboundMessageRepo,
		txManager:           txManager,
		messageProcessor:    messageProcessor,
		peerClient:          peerClient,
		resolver:            resolver,
		httpClient:          &http.Client{Timeout: cfg.OutboundHTTPTO},
		pollEvery:           cfg.OutboxPollEvery,
		maxAttempts:         cfg.OutboxMaxAttempts,
		ourRoutingNumber:    resolver.OurRoutingNumber(),
		stop:                make(chan struct{}),
	}
}

func (w *OutboxWorker) Start() {
	go w.loop()
}

func (w *OutboxWorker) Stop() {
	close(w.stop)
}

func (w *OutboxWorker) loop() {
	ticker := time.NewTicker(w.pollEvery)
	defer ticker.Stop()
	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			w.processBatch(context.Background())
		}
	}
}

func (w *OutboxWorker) processBatch(ctx context.Context) {
	msgs, err := w.outboundMessageRepo.NextBatch(ctx, outboxBatchSize)
	if err != nil {
		zap.L().Error("outbox: NextBatch failed", zap.Error(err))
		return
	}
	for i := range msgs {
		w.processOne(ctx, &msgs[i])
	}
}

func (w *OutboxWorker) processOne(ctx context.Context, msg *model.OutboundMessage) {
	peer, ok := w.resolver.ByRoutingNumber(msg.PeerRoutingNumber)
	if !ok {
		_ = w.outboundMessageRepo.MarkFailed(ctx, msg.ID, fmt.Sprintf("no peer for routing number %d", msg.PeerRoutingNumber))
		return
	}

	newAttempts := msg.Attempts + 1

	// Same-bank COMMIT/ROLLBACK: drive the processor directly instead of HTTP.
	// This is the recovery path for same-bank 2PC — no remote message needed,
	// CommitLocalTransaction/RollbackLocalTransaction are idempotent.
	if msg.PeerRoutingNumber == w.ourRoutingNumber &&
		(msg.MessageType == string(dto.MessageTypeCommitTx) || msg.MessageType == string(dto.MessageTypeRollbackTx)) {
		w.handleSameBankFollowUp(ctx, msg, newAttempts)
		return
	}

	respStatus, respBody, err := w.sendHTTP(ctx, peer, msg.Payload)
	if err != nil {
		w.rescheduleOrFail(ctx, msg, newAttempts, err.Error(), 0, nil)
		return
	}

	switch msg.MessageType {
	case string(dto.MessageTypeNewTx):
		w.handleNewTxResponse(ctx, msg, newAttempts, respStatus, respBody)
	case string(dto.MessageTypeCommitTx), string(dto.MessageTypeRollbackTx):
		if respStatus == http.StatusNoContent {
			if err := w.outboundMessageRepo.MarkSent(ctx, msg.ID, respStatus, respBody); err != nil {
				zap.L().Error("outbox: MarkSent failed", zap.Uint("id", msg.ID), zap.Error(err))
			}
		} else {
			w.rescheduleOrFail(ctx, msg, newAttempts, fmt.Sprintf("unexpected status %d", respStatus), respStatus, respBody)
		}
	}
}

// handleSameBankFollowUp drives a COMMIT_TX or ROLLBACK_TX for a same-bank
// transaction by calling the MessageProcessor directly. This is the recovery
// path: the PreparedTransaction record already exists (written atomically with
// the outbox row), so CommitLocalTransaction/RollbackLocalTransaction replay
// the effects idempotently.
func (w *OutboxWorker) handleSameBankFollowUp(ctx context.Context, msg *model.OutboundMessage, attempts int) {
	// Extract the transaction ID from the payload.
	var txID dto.ForeignBankId
	switch msg.MessageType {
	case string(dto.MessageTypeCommitTx):
		var m dto.CommitTxMessage
		if err := json.Unmarshal(msg.Payload, &m); err != nil {
			_ = w.outboundMessageRepo.MarkFailed(ctx, msg.ID, "failed to decode commit payload: "+err.Error())
			return
		}
		txID = m.Message.TransactionID
	case string(dto.MessageTypeRollbackTx):
		var m dto.RollbackTxMessage
		if err := json.Unmarshal(msg.Payload, &m); err != nil {
			_ = w.outboundMessageRepo.MarkFailed(ctx, msg.ID, "failed to decode rollback payload: "+err.Error())
			return
		}
		txID = m.Message.TransactionID
	}

	var statusCode int
	var err error
	if msg.MessageType == string(dto.MessageTypeCommitTx) {
		statusCode, err = w.messageProcessor.CommitLocalTransaction(ctx, txID)
	} else {
		statusCode, err = w.messageProcessor.RollbackLocalTransaction(ctx, txID)
	}

	if err != nil {
		w.rescheduleOrFail(ctx, msg, attempts, err.Error(), 0, nil)
		return
	}
	switch statusCode {
	case http.StatusNoContent:
		if err := w.outboundMessageRepo.MarkSent(ctx, msg.ID, statusCode, nil); err != nil {
			zap.L().Error("outbox: same-bank MarkSent failed", zap.Uint("id", msg.ID), zap.Error(err))
		}
	case http.StatusAccepted:
		// PREPARING gate — reserves not yet confirmed; retry later.
		w.rescheduleOrFail(ctx, msg, attempts, "transaction still preparing", statusCode, nil)
	default:
		w.rescheduleOrFail(ctx, msg, attempts, fmt.Sprintf("unexpected status %d", statusCode), statusCode, nil)
	}
}

func (w *OutboxWorker) handleNewTxResponse(ctx context.Context, msg *model.OutboundMessage, attempts, respStatus int, respBody []byte) {
	if respStatus == http.StatusAccepted {
		w.rescheduleOrFail(ctx, msg, attempts, "peer returned 202 (still preparing)", respStatus, respBody)
		return
	}

	if respStatus != http.StatusOK {
		w.rescheduleOrFail(ctx, msg, attempts, fmt.Sprintf("unexpected status %d", respStatus), respStatus, respBody)
		return
	}

	var vote dto.TransactionVote
	if err := json.Unmarshal(respBody, &vote); err != nil {
		w.rescheduleOrFail(ctx, msg, attempts, "failed to parse vote: "+err.Error(), respStatus, respBody)
		return
	}

	// Commit/rollback locally + enqueue the follow-up BEFORE marking NEW_TX as
	// SENT. If we crash after the follow-up enqueue but before MarkSent, NEW_TX
	// is retried and CommitAndEnqueue is a no-op (idempotent key + ON CONFLICT
	// DO NOTHING). If we crash before the follow-up enqueue, NEW_TX is retried
	// and the whole path is retried cleanly. Both OTC and PAYMENT flows share
	// this path — the initiating bank's postings settle through the same
	// MessageProcessor 2PC.
	w.handleNewTxVote(ctx, msg, vote)
	if err := w.outboundMessageRepo.MarkSent(ctx, msg.ID, respStatus, respBody); err != nil {
		zap.L().Error("outbox: MarkSent failed", zap.Uint("id", msg.ID), zap.Error(err))
	}
}

// handleNewTxVote handles the post-vote action for a 2PC transaction (any
// flow). Keys are derived deterministically from the NEW_TX key so retries are
// idempotent. MarkSent for NEW_TX is done by the caller after this returns.
func (w *OutboxWorker) handleNewTxVote(ctx context.Context, msg *model.OutboundMessage, vote dto.TransactionVote) {
	var wireMsg dto.NewTxMessage
	if err := json.Unmarshal(msg.Payload, &wireMsg); err != nil {
		zap.L().Error("outbox: failed to decode NEW_TX payload", zap.Uint("id", msg.ID), zap.Error(err))
		return
	}
	txID := wireMsg.Message.TransactionID

	if vote.Vote == dto.VoteYes {
		commitKey := msg.IdempotenceKeyLocal + "-commit"
		_, commitMsg, err := w.messageProcessor.CommitAndEnqueueFollowUp(ctx, txID, msg.PeerRoutingNumber, commitKey, msg.FlowType)
		if err != nil {
			zap.L().Error("outbox: CommitAndEnqueueFollowUp failed", zap.String("txID", txID.ID), zap.Error(err))
			return
		}
		// Our side has committed (payer debited). Report success to banking.
		w.finalizePayment(ctx, msg, true)
		if commitMsg != nil {
			if err := w.peerClient.SendCommitTx(ctx, msg.PeerRoutingNumber, commitKey, txID); err == nil {
				_ = w.outboundMessageRepo.MarkSent(ctx, commitMsg.ID, http.StatusNoContent, nil)
			}
		}
	} else {
		// Peer voted NO — it never prepared, so it holds no reservations and has
		// nothing to roll back. Mirror the synchronous coordinator path: release
		// our own side locally and do NOT send a pointless ROLLBACK_TX (which the
		// peer would answer with a no-op anyway).
		if _, err := w.messageProcessor.RollbackLocalTransaction(ctx, txID); err != nil {
			zap.L().Error("outbox: local rollback after peer NO failed", zap.String("txID", txID.ID), zap.Error(err))
		}
		// Reservation released. Report failure to banking.
		w.finalizePayment(ctx, msg, false)
	}
}

// finalizePayment reports a PAYMENT's final outcome back to banking-service so
// it can move the originating transaction out of Processing. It is a no-op for
// OTC / follow-up rows (BankingTxID == 0) and best-effort: a failed callback
// leaves the banking transaction Processing (truthful and reconcilable).
func (w *OutboxWorker) finalizePayment(ctx context.Context, msg *model.OutboundMessage, success bool) {
	if msg.BankingTxID == 0 {
		return
	}
	if err := w.messageProcessor.FinalizeInterbankPayment(ctx, msg.BankingTxID, success); err != nil {
		zap.L().Error("outbox: FinalizeInterbankPayment failed",
			zap.Uint64("bankingTxID", msg.BankingTxID), zap.Bool("success", success), zap.Error(err))
	}
}

func (w *OutboxWorker) rescheduleOrFail(ctx context.Context, msg *model.OutboundMessage, attempts int, errMsg string, lastStatus int, lastBody []byte) {
	if attempts >= w.maxAttempts {
		zap.L().Warn("outbox: max attempts reached, failing message", zap.Uint("id", msg.ID), zap.String("error", errMsg))
		if msg.MessageType == string(dto.MessageTypeNewTx) {
			// A NEW_TX we could never deliver: release our local reservation and
			// enqueue a best-effort ROLLBACK_TX. If the peer never prepared it
			// answers the rollback with a harmless 204 no-op.
			var wireMsg dto.NewTxMessage
			if err := json.Unmarshal(msg.Payload, &wireMsg); err == nil {
				rollbackKey := msg.IdempotenceKeyLocal + "-rollback"
				_, _, _ = w.messageProcessor.RollbackAndEnqueueFollowUp(ctx, wireMsg.Message.TransactionID, msg.PeerRoutingNumber, rollbackKey, msg.FlowType)
			}
			// Undeliverable payment: report failure so banking stops waiting.
			w.finalizePayment(ctx, msg, false)
		}
		_ = w.outboundMessageRepo.MarkFailed(ctx, msg.ID, errMsg)
		return
	}

	backoff := backoffDuration(attempts)
	nextRetry := time.Now().Add(backoff)
	if err := w.outboundMessageRepo.Reschedule(ctx, msg.ID, attempts, errMsg, lastStatus, lastBody, nextRetry); err != nil {
		zap.L().Error("outbox: Reschedule failed", zap.Uint("id", msg.ID), zap.Error(err))
	}
}

func (w *OutboxWorker) sendHTTP(ctx context.Context, peer config.Peer, payload []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, peer.BaseURL+"/interbank", bytes.NewReader(payload))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", peer.OurAPIKey)

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	return resp.StatusCode, body, err
}

func backoffDuration(attempts int) time.Duration {
	d := time.Duration(attempts) * 5 * time.Second
	if d > 5*time.Minute {
		d = 5 * time.Minute
	}
	return d
}
