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

	"github.com/google/uuid"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/config"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/repository"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/service"
)

const outboxBatchSize = 20

type OutboxWorker struct {
	outboundMessageRepo repository.OutboundMessageRepository
	outboundPaymentRepo repository.OutboundPaymentRepository
	txManager           repository.TransactionManager
	bankingClient       service.BankingServiceClient
	resolver            *service.PeerResolver
	httpClient          *http.Client
	pollEvery           time.Duration
	maxAttempts         int
	ourRoutingNumber    int
	stop                chan struct{}
}

func NewOutboxWorker(
	outboundMessageRepo repository.OutboundMessageRepository,
	outboundPaymentRepo repository.OutboundPaymentRepository,
	txManager repository.TransactionManager,
	bankingClient service.BankingServiceClient,
	resolver *service.PeerResolver,
	cfg *config.Configuration,
) *OutboxWorker {
	return &OutboxWorker{
		outboundMessageRepo: outboundMessageRepo,
		outboundPaymentRepo: outboundPaymentRepo,
		txManager:           txManager,
		bankingClient:       bankingClient,
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

	// Mark NEW_TX SENT with the vote before calling banking gRPC.
	if err := w.outboundMessageRepo.MarkSent(ctx, msg.ID, respStatus, respBody); err != nil {
		zap.L().Error("outbox: MarkSent failed", zap.Uint("id", msg.ID), zap.Error(err))
		return
	}

	// Decode transactionId from payload to look up OutboundPayment.
	var wireMsg dto.NewTxMessage
	if err := json.Unmarshal(msg.Payload, &wireMsg); err != nil {
		zap.L().Error("outbox: failed to decode NEW_TX payload", zap.Uint("id", msg.ID), zap.Error(err))
		return
	}
	txIDKey := wireMsg.Message.TransactionID.ID

	payment, err := w.outboundPaymentRepo.FindByTransactionIDKey(ctx, txIDKey)
	if err != nil || payment == nil {
		zap.L().Error("outbox: OutboundPayment not found", zap.String("transactionIDKey", txIDKey), zap.Error(err))
		return
	}

	if vote.Vote == dto.VoteYes {
		w.commitAndEnqueue(ctx, payment, txIDKey, msg.PeerRoutingNumber, &wireMsg.Message)
	} else {
		w.rollbackAndEnqueue(ctx, payment, txIDKey, msg.PeerRoutingNumber, &wireMsg.Message)
	}
}

func (w *OutboxWorker) commitAndEnqueue(ctx context.Context, payment *model.OutboundPayment, txIDKey string, peerRouting int, tx *dto.Transaction) {
	if err := w.bankingClient.CommitInterbankPayment(ctx, uint(payment.BankingTxID)); err != nil {
		zap.L().Error("outbox: CommitInterbankPayment gRPC failed", zap.String("txIDKey", txIDKey), zap.Error(err))
		return
	}
	w.enqueueFollowUp(ctx, peerRouting, txIDKey, dto.MessageTypeCommitTx, tx)
}

func (w *OutboxWorker) rollbackAndEnqueue(ctx context.Context, payment *model.OutboundPayment, txIDKey string, peerRouting int, tx *dto.Transaction) {
	if err := w.bankingClient.RollbackInterbankPayment(ctx, uint(payment.BankingTxID)); err != nil {
		zap.L().Error("outbox: RollbackInterbankPayment gRPC failed", zap.String("txIDKey", txIDKey), zap.Error(err))
		return
	}
	w.enqueueFollowUp(ctx, peerRouting, txIDKey, dto.MessageTypeRollbackTx, tx)
}

func (w *OutboxWorker) enqueueFollowUp(ctx context.Context, peerRouting int, txIDKey string, msgType dto.MessageType, tx *dto.Transaction) {
	idempotenceKey := uuid.New().String()
	var body any
	if msgType == dto.MessageTypeCommitTx {
		body = dto.CommitTxMessage{
			IdempotenceKey: dto.IdempotenceKey{RoutingNumber: w.ourRoutingNumber, LocallyGeneratedKey: idempotenceKey},
			MessageType:    msgType,
			Message:        dto.CommitTransaction{TransactionID: tx.TransactionID},
		}
	} else {
		body = dto.RollbackTxMessage{
			IdempotenceKey: dto.IdempotenceKey{RoutingNumber: w.ourRoutingNumber, LocallyGeneratedKey: idempotenceKey},
			MessageType:    msgType,
			Message:        dto.RollbackTransaction{TransactionID: tx.TransactionID},
		}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		zap.L().Error("outbox: failed to marshal follow-up message", zap.Error(err))
		return
	}

	outMsg := &model.OutboundMessage{
		PeerRoutingNumber:   peerRouting,
		MessageType:         string(msgType),
		IdempotenceKeyLocal: idempotenceKey,
		Payload:             payload,
		Status:              model.OutboundPending,
	}
	if err := w.outboundMessageRepo.Enqueue(ctx, outMsg); err != nil {
		zap.L().Error("outbox: failed to enqueue follow-up message", zap.String("type", string(msgType)), zap.Error(err))
	}
}

func (w *OutboxWorker) rescheduleOrFail(ctx context.Context, msg *model.OutboundMessage, attempts int, errMsg string, lastStatus int, lastBody []byte) {
	if attempts >= w.maxAttempts {
		zap.L().Warn("outbox: max attempts reached, failing message", zap.Uint("id", msg.ID), zap.String("error", errMsg))
		// For NEW_TX that exceeded max attempts, treat as NO vote and rollback.
		if msg.MessageType == string(dto.MessageTypeNewTx) {
			var wireMsg dto.NewTxMessage
			if err := json.Unmarshal(msg.Payload, &wireMsg); err == nil {
				txIDKey := wireMsg.Message.TransactionID.ID
				if payment, err := w.outboundPaymentRepo.FindByTransactionIDKey(ctx, txIDKey); err == nil && payment != nil {
					w.rollbackAndEnqueue(ctx, payment, txIDKey, msg.PeerRoutingNumber, &wireMsg.Message)
				}
			}
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
	defer resp.Body.Close()

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
