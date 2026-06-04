package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/repository"
)

const txBalanceEpsilon = 0.000001

type preparedEffect struct {
	kind string
	id   string
}

// MessageProcessor handles inbound and local bank-to-bank transactions.
type MessageProcessor struct {
	inbound      repository.InboundMessageRepository
	prepared     repository.PreparedTransactionRepository
	txManager    repository.TransactionManager
	peers        *PeerResolver
	banking      client.BankingClient
	trading      client.TradingClient
	contracts    repository.PeerContractRepository
	negotiations repository.PeerNegotiationRepository
}

func NewMessageProcessor(
	inbound repository.InboundMessageRepository,
	prepared repository.PreparedTransactionRepository,
	txManager repository.TransactionManager,
	peers *PeerResolver,
	banking client.BankingClient,
	trading client.TradingClient,
	contracts repository.PeerContractRepository,
	negotiations repository.PeerNegotiationRepository,
) *MessageProcessor {
	return &MessageProcessor{
		inbound:      inbound,
		prepared:     prepared,
		txManager:    txManager,
		peers:        peers,
		banking:      banking,
		trading:      trading,
		contracts:    contracts,
		negotiations: negotiations,
	}
}

func (p *MessageProcessor) ProcessNewTx(ctx context.Context, peerRouting int, key dto.IdempotenceKey, tx *dto.Transaction) (int, any, error) {
	return p.processInbound(ctx, peerRouting, key, dto.MessageTypeNewTx, tx, func(ctx context.Context) (int, any, error) {
		vote, err := p.PrepareLocalTransaction(ctx, tx)
		if err != nil {
			return http.StatusOK, dto.TransactionVote{Vote: dto.VoteNo, Reasons: []dto.NoVoteReason{{Reason: dto.ReasonUnacceptableAsset}}}, nil
		}
		return http.StatusOK, vote, nil
	})
}

func (p *MessageProcessor) ProcessCommitTx(ctx context.Context, peerRouting int, key dto.IdempotenceKey, msg *dto.CommitTransaction) (int, any, error) {
	return p.processInbound(ctx, peerRouting, key, dto.MessageTypeCommitTx, msg, func(ctx context.Context) (int, any, error) {
		statusCode, err := p.CommitLocalTransaction(ctx, msg.TransactionID)
		return statusCode, nil, err
	})
}

func (p *MessageProcessor) ProcessRollbackTx(ctx context.Context, peerRouting int, key dto.IdempotenceKey, msg *dto.RollbackTransaction) (int, any, error) {
	return p.processInbound(ctx, peerRouting, key, dto.MessageTypeRollbackTx, msg, func(ctx context.Context) (int, any, error) {
		statusCode, err := p.RollbackLocalTransaction(ctx, msg.TransactionID)
		return statusCode, nil, err
	})
}

func (p *MessageProcessor) PrepareLocalTransaction(ctx context.Context, tx *dto.Transaction) (dto.TransactionVote, error) {
	var vote dto.TransactionVote
	err := p.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		var err error
		vote, err = p.prepareLocalTransaction(ctx, tx)
		return err
	})
	return vote, err
}

func (p *MessageProcessor) prepareLocalTransaction(ctx context.Context, tx *dto.Transaction) (dto.TransactionVote, error) {
	if tx == nil {
		return noVote(dto.ReasonUnacceptableAsset, nil), nil
	}
	if reason := p.balanceFailure(tx); reason != nil {
		return dto.TransactionVote{Vote: dto.VoteNo, Reasons: []dto.NoVoteReason{*reason}}, nil
	}

	existing, err := p.prepared.FindByID(ctx, tx.TransactionID.RoutingNumber, tx.TransactionID.ID)
	if err != nil {
		return noVote(dto.ReasonUnacceptableAsset, nil), err
	}
	if existing != nil {
		if existing.Status == model.PreparedTransactionRolledBack {
			return noVote(dto.ReasonUnacceptableAsset, nil), nil
		}
		return dto.TransactionVote{Vote: dto.VoteYes}, nil
	}

	body, err := json.Marshal(tx)
	if err != nil {
		return noVote(dto.ReasonUnacceptableAsset, nil), err
	}

	var effects []preparedEffect
	for i := range tx.Postings {
		effect, reason, err := p.preparePosting(ctx, tx, i)
		if err != nil {
			p.rollbackEffects(ctx, effects)
			return noVote(reason, &tx.Postings[i]), nil
		}
		if effect != nil {
			effects = append(effects, *effect)
		}
	}

	prepared := &model.PreparedTransaction{
		RoutingNumber: tx.TransactionID.RoutingNumber,
		ID:            tx.TransactionID.ID,
		Status:        model.PreparedTransactionPrepared,
		RequestBody:   body,
	}
	if err := p.prepared.Create(ctx, prepared); err != nil {
		p.rollbackEffects(ctx, effects)
		return noVote(dto.ReasonUnacceptableAsset, nil), err
	}

	return dto.TransactionVote{Vote: dto.VoteYes}, nil
}

func (p *MessageProcessor) CommitLocalTransaction(ctx context.Context, txID dto.ForeignBankId) (int, error) {
	var statusCode int
	err := p.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		var err error
		statusCode, err = p.commitLocalTransaction(ctx, txID)
		return err
	})
	return statusCode, err
}

func (p *MessageProcessor) commitLocalTransaction(ctx context.Context, txID dto.ForeignBankId) (int, error) {
	stored, tx, err := p.loadStoredTransaction(ctx, txID)
	if err != nil {
		return http.StatusNoContent, err
	}
	if stored == nil {
		return http.StatusAccepted, nil
	}
	if stored.Status == model.PreparedTransactionCommitted {
		return http.StatusNoContent, nil
	}
	if stored.Status == model.PreparedTransactionRolledBack {
		return http.StatusNoContent, nil
	}

	for i := range tx.Postings {
		if err := p.commitPosting(ctx, tx, i); err != nil {
			return http.StatusAccepted, err
		}
	}
	stored.Status = model.PreparedTransactionCommitted
	if err := p.prepared.Update(ctx, stored); err != nil {
		return http.StatusAccepted, err
	}
	return http.StatusNoContent, nil
}

func (p *MessageProcessor) RollbackLocalTransaction(ctx context.Context, txID dto.ForeignBankId) (int, error) {
	var statusCode int
	err := p.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		var err error
		statusCode, err = p.rollbackLocalTransaction(ctx, txID)
		return err
	})
	return statusCode, err
}

func (p *MessageProcessor) rollbackLocalTransaction(ctx context.Context, txID dto.ForeignBankId) (int, error) {
	stored, tx, err := p.loadStoredTransaction(ctx, txID)
	if err != nil {
		return http.StatusNoContent, err
	}
	if stored == nil {
		return http.StatusNoContent, nil
	}
	if stored.Status == model.PreparedTransactionRolledBack {
		return http.StatusNoContent, nil
	}
	if stored.Status == model.PreparedTransactionCommitted {
		return http.StatusNoContent, nil
	}

	for i := range tx.Postings {
		_ = p.rollbackPosting(ctx, tx, i)
	}
	stored.Status = model.PreparedTransactionRolledBack
	if err := p.prepared.Update(ctx, stored); err != nil {
		return http.StatusAccepted, err
	}
	return http.StatusNoContent, nil
}

func (p *MessageProcessor) processInbound(
	ctx context.Context,
	peerRouting int,
	key dto.IdempotenceKey,
	messageType dto.MessageType,
	request any,
	fn func(context.Context) (int, any, error),
) (int, any, error) {
	existing, err := p.inbound.FindByKey(ctx, peerRouting, key.LocallyGeneratedKey)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	if existing != nil {
		if existing.ResponseStatus == http.StatusAccepted {
			// 202 means the previous attempt was logged but unfinished; retrying
			// must re-enter processing so the response can advance to 200/204.
		} else {
			var cached any
			if len(existing.ResponseBody) > 0 {
				if messageType == dto.MessageTypeNewTx {
					var vote dto.TransactionVote
					if err := json.Unmarshal(existing.ResponseBody, &vote); err == nil {
						cached = vote
					}
				}
				if cached == nil {
					var body map[string]any
					if err := json.Unmarshal(existing.ResponseBody, &body); err == nil {
						cached = body
					}
				}
			}
			return existing.ResponseStatus, cached, nil
		}
	}

	var statusCode int
	var body any
	err = p.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		var err error
		statusCode, body, err = fn(ctx)
		if err != nil {
			return err
		}

		requestBody, err := json.Marshal(request)
		if err != nil {
			return err
		}
		var responseBody []byte
		if body != nil {
			responseBody, err = json.Marshal(body)
			if err != nil {
				return err
			}
		}

		return p.inbound.Save(ctx, &model.InboundMessage{
			PeerRoutingNumber:   peerRouting,
			LocallyGeneratedKey: key.LocallyGeneratedKey,
			MessageType:         string(messageType),
			RequestBody:         requestBody,
			ResponseStatus:      statusCode,
			ResponseBody:        responseBody,
			ProcessedAt:         time.Now(),
		})
	})
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return statusCode, body, nil
}

func (p *MessageProcessor) preparePosting(ctx context.Context, tx *dto.Transaction, index int) (*preparedEffect, dto.NoVoteReasonKind, error) {
	posting := tx.Postings[index]

	switch posting.Asset.Type {
	case dto.AssetMonas:
		local, accountNumber, clientID, reason := p.localCashAccount(posting.Account)
		if reason != "" {
			return nil, reason, fmt.Errorf("invalid local cash account")
		}
		if !local {
			return nil, "", nil
		}
		currency, ok := monetaryCurrency(posting.Asset)
		if !ok {
			return nil, dto.ReasonUnacceptableAsset, fmt.Errorf("invalid MONAS asset")
		}
		postingID := postingID(tx, index)
		_, err := p.banking.PrepareInterbankCashPosting(ctx, &pb.PrepareInterbankCashPostingRequest{
			PostingId:     postingID,
			AccountNumber: accountNumber,
			ClientId:      uint64(clientID),
			CurrencyCode:  currency,
			Amount:        posting.Amount,
		})
		if err != nil {
			return nil, cashNoVoteReason(err), err
		}
		return &preparedEffect{kind: "cash", id: postingID}, "", nil

	case dto.AssetOption:
		return p.prepareOptionPosting(ctx, tx, index)

	default:
		return nil, dto.ReasonUnacceptableAsset, fmt.Errorf("unsupported asset %s", posting.Asset.Type)
	}
}

func (p *MessageProcessor) prepareOptionPosting(ctx context.Context, tx *dto.Transaction, index int) (*preparedEffect, dto.NoVoteReasonKind, error) {
	posting := tx.Postings[index]
	if math.Abs(math.Abs(posting.Amount)-1) > txBalanceEpsilon {
		return nil, dto.ReasonOptionAmountIncorrect, fmt.Errorf("option posting amount must be +/-1")
	}
	local, _, clientID, reason := p.localPersonAccount(posting.Account)
	if reason != "" {
		return nil, reason, fmt.Errorf("invalid option account")
	}
	if !local || posting.Amount > 0 {
		return nil, "", nil
	}

	option, ok := optionDescription(posting.Asset)
	if !ok {
		return nil, dto.ReasonUnacceptableAsset, fmt.Errorf("invalid OPTION asset")
	}
	if option.NegotiationID.RoutingNumber != p.peers.OurRoutingNumber() {
		return nil, "", nil
	}
	negotiation, err := p.negotiations.FindByID(ctx, option.NegotiationID.ID)
	if err != nil {
		return nil, dto.ReasonOptionNegotiationNotFound, err
	}
	if negotiation == nil {
		return nil, dto.ReasonOptionNegotiationNotFound, fmt.Errorf("negotiation not found")
	}
	_, err = p.trading.ReservePeerOtcShares(ctx, &pb.ReservePeerOtcSharesRequest{
		ContractId: fmt.Sprintf("%d:%s", option.NegotiationID.RoutingNumber, option.NegotiationID.ID),
		SellerId:   uint64(clientID),
		Ticker:     option.Stock.Ticker,
		Amount:     option.Amount,
	})
	if err != nil {
		return nil, dto.ReasonInsufficientAsset, err
	}
	return &preparedEffect{kind: "option", id: fmt.Sprintf("%d:%s", option.NegotiationID.RoutingNumber, option.NegotiationID.ID)}, "", nil
}

func (p *MessageProcessor) commitPosting(ctx context.Context, tx *dto.Transaction, index int) error {
	posting := tx.Postings[index]
	if posting.Asset.Type != dto.AssetMonas {
		return nil
	}
	local, _, _, reason := p.localCashAccount(posting.Account)
	if reason != "" || !local {
		return nil
	}
	_, err := p.banking.CommitInterbankCashPosting(ctx, postingID(tx, index))
	return err
}

func (p *MessageProcessor) rollbackPosting(ctx context.Context, tx *dto.Transaction, index int) error {
	posting := tx.Postings[index]
	switch posting.Asset.Type {
	case dto.AssetMonas:
		local, _, _, reason := p.localCashAccount(posting.Account)
		if reason != "" || !local {
			return nil
		}
		_, err := p.banking.RollbackInterbankCashPosting(ctx, postingID(tx, index))
		return err
	case dto.AssetOption:
		option, ok := optionDescription(posting.Asset)
		if !ok || option.NegotiationID.RoutingNumber != p.peers.OurRoutingNumber() || posting.Amount > 0 {
			return nil
		}
		_, err := p.trading.ReleasePeerOtcShares(ctx, fmt.Sprintf("%d:%s", option.NegotiationID.RoutingNumber, option.NegotiationID.ID))
		return err
	default:
		return nil
	}
}

func (p *MessageProcessor) rollbackEffects(ctx context.Context, effects []preparedEffect) {
	for i := len(effects) - 1; i >= 0; i-- {
		switch effects[i].kind {
		case "cash":
			_, _ = p.banking.RollbackInterbankCashPosting(ctx, effects[i].id)
		case "option":
			_, _ = p.trading.ReleasePeerOtcShares(ctx, effects[i].id)
		}
	}
}

func (p *MessageProcessor) loadStoredTransaction(ctx context.Context, txID dto.ForeignBankId) (*model.PreparedTransaction, *dto.Transaction, error) {
	stored, err := p.prepared.FindByID(ctx, txID.RoutingNumber, txID.ID)
	if err != nil || stored == nil {
		return stored, nil, err
	}
	var tx dto.Transaction
	if err := json.Unmarshal(stored.RequestBody, &tx); err != nil {
		return nil, nil, err
	}
	return stored, &tx, nil
}

func (p *MessageProcessor) balanceFailure(tx *dto.Transaction) *dto.NoVoteReason {
	if len(tx.Postings) == 0 {
		return &dto.NoVoteReason{Reason: dto.ReasonUnbalancedTx}
	}
	totals := make(map[string]float64)
	for i := range tx.Postings {
		key, ok := balanceKey(tx.Postings[i].Asset)
		if !ok {
			return &dto.NoVoteReason{Reason: dto.ReasonUnacceptableAsset, Posting: &tx.Postings[i]}
		}
		totals[key] += tx.Postings[i].Amount
	}
	for _, total := range totals {
		if math.Abs(total) > txBalanceEpsilon {
			return &dto.NoVoteReason{Reason: dto.ReasonUnbalancedTx}
		}
	}
	return nil
}

func (p *MessageProcessor) localCashAccount(account dto.TxAccount) (bool, string, uint, dto.NoVoteReasonKind) {
	switch account.Type {
	case dto.TxAccountPerson:
		return p.localPersonAccount(account)
	case dto.TxAccountAccount:
		if account.Num == nil || strings.TrimSpace(*account.Num) == "" {
			return false, "", 0, dto.ReasonNoSuchAccount
		}
		num := strings.TrimSpace(*account.Num)
		prefix := fmt.Sprintf("%03d", p.peers.OurRoutingNumber())
		return strings.HasPrefix(num, prefix), num, 0, ""
	default:
		return false, "", 0, dto.ReasonUnacceptableAsset
	}
}

func (p *MessageProcessor) localPersonAccount(account dto.TxAccount) (bool, string, uint, dto.NoVoteReasonKind) {
	if account.Type != dto.TxAccountPerson || account.ID == nil {
		return false, "", 0, dto.ReasonNoSuchAccount
	}
	if account.ID.RoutingNumber != p.peers.OurRoutingNumber() {
		return false, "", 0, ""
	}
	id, err := strconv.ParseUint(account.ID.ID, 10, 64)
	if err != nil || id == 0 {
		return false, "", 0, dto.ReasonNoSuchAccount
	}
	return true, "", uint(id), ""
}

func monetaryCurrency(asset dto.Asset) (string, bool) {
	if asset.Type != dto.AssetMonas {
		return "", false
	}
	currency, ok := asset.Body["currency"].(string)
	return strings.TrimSpace(currency), ok && strings.TrimSpace(currency) != ""
}

func optionDescription(asset dto.Asset) (dto.OptionDescription, bool) {
	var option dto.OptionDescription
	if asset.Type != dto.AssetOption {
		return option, false
	}
	raw, err := json.Marshal(asset.Body)
	if err != nil {
		return option, false
	}
	if err := json.Unmarshal(raw, &option); err != nil {
		return option, false
	}
	return option, option.NegotiationID.ID != "" && option.Stock.Ticker != "" && option.Amount > 0
}

func balanceKey(asset dto.Asset) (string, bool) {
	switch asset.Type {
	case dto.AssetMonas:
		currency, ok := monetaryCurrency(asset)
		return "MONAS:" + currency, ok
	case dto.AssetOption:
		option, ok := optionDescription(asset)
		if !ok {
			return "", false
		}
		return fmt.Sprintf("OPTION:%d:%s", option.NegotiationID.RoutingNumber, option.NegotiationID.ID), true
	default:
		return "", false
	}
}

func postingID(tx *dto.Transaction, index int) string {
	return fmt.Sprintf("%d:%s:%d", tx.TransactionID.RoutingNumber, tx.TransactionID.ID, index)
}

func cashNoVoteReason(err error) dto.NoVoteReasonKind {
	if grpcStatus, ok := status.FromError(err); ok {
		switch grpcStatus.Code() {
		case codes.NotFound:
			return dto.ReasonNoSuchAccount
		case codes.InvalidArgument:
			if strings.Contains(strings.ToLower(grpcStatus.Message()), "insufficient") {
				return dto.ReasonInsufficientAsset
			}
			return dto.ReasonUnacceptableAsset
		}
	}
	if strings.Contains(strings.ToLower(err.Error()), "insufficient") {
		return dto.ReasonInsufficientAsset
	}
	return dto.ReasonUnacceptableAsset
}

func noVote(reason dto.NoVoteReasonKind, posting *dto.Posting) dto.TransactionVote {
	return dto.TransactionVote{Vote: dto.VoteNo, Reasons: []dto.NoVoteReason{{Reason: reason, Posting: posting}}}
}
