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

type preparedItem struct {
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
		statusCode, vote, err := p.PrepareLocalTransaction(ctx, tx)
		return statusCode, vote, err
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

func (p *MessageProcessor) PrepareLocalTransaction(ctx context.Context, tx *dto.Transaction) (int, dto.TransactionVote, error) {
	statusCode := http.StatusOK
	var vote dto.TransactionVote
	err := p.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		if tx == nil {
			vote = dto.TransactionVote{}
			return fmt.Errorf("invalid transaction")
		}
		if reason := p.balanceFailure(tx); reason != nil {
			vote = dto.TransactionVote{Vote: dto.VoteNo, Reasons: []dto.NoVoteReason{*reason}}
			return nil
		}

		existing, err := p.prepared.FindByID(ctx, tx.TransactionID.RoutingNumber, tx.TransactionID.ID)
		if err != nil {
			statusCode = http.StatusInternalServerError
			vote = dto.TransactionVote{}
			return err
		}

		if existing != nil {
			if existing.Status == model.PreparedTransactionRolledBack {
				vote = dto.TransactionVote{Vote: dto.VoteNo, Reasons: []dto.NoVoteReason{{Reason: dto.ReasonUnbalancedTx}}}
				return nil
			}
			vote = dto.TransactionVote{Vote: dto.VoteYes}
			return nil
		}

		body, err := json.Marshal(tx)
		if err != nil {
			statusCode = http.StatusInternalServerError
			vote = dto.TransactionVote{}
			return err
		}

		var preparedItem []preparedItem
		for i := range tx.Postings {
			if !p.isLocalPosting(tx.Postings[i]) {
				continue
			}
			item, reason, err := p.preparePosting(ctx, tx, i)
			if err != nil {
				p.rollbackEffects(ctx, preparedItem)
				statusCode = http.StatusOK
				vote = noVote(reason, &tx.Postings[i])
				return nil
			}
			if item != nil {
				preparedItem = append(preparedItem, *item)
			}
		}

		prepared := &model.PreparedTransaction{
			RoutingNumber: tx.TransactionID.RoutingNumber,
			ID:            tx.TransactionID.ID,
			Status:        model.PreparedTransactionPrepared,
			RequestBody:   body,
		}
		if err := p.prepared.Create(ctx, prepared); err != nil {
			p.rollbackEffects(ctx, preparedItem)
			statusCode = http.StatusInternalServerError
			vote = dto.TransactionVote{}
			return err
		}

		statusCode = http.StatusOK
		vote = dto.TransactionVote{Vote: dto.VoteYes}
		return nil
	})
	return statusCode, vote, err
}

func (p *MessageProcessor) CommitLocalTransaction(ctx context.Context, txID dto.ForeignBankId) (int, error) {
	var statusCode int
	err := p.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		stored, tx, err := p.loadStoredTransaction(ctx, txID)
		if err != nil {
			statusCode = http.StatusInternalServerError
			return err
		}
		if stored == nil {
			statusCode = http.StatusAccepted
			return nil
		}
		if stored.Status == model.PreparedTransactionCommitted {
			statusCode = http.StatusNoContent
			return nil
		}
		if stored.Status == model.PreparedTransactionRolledBack {
			statusCode = http.StatusInternalServerError
			return fmt.Errorf("transaction already rolled back")
		}

		for i := range tx.Postings {
			if !p.isLocalPosting(tx.Postings[i]) {
				continue
			}
			if err := p.commitPosting(ctx, tx, i); err != nil {
				statusCode = http.StatusInternalServerError
				return err
			}
		}
		stored.Status = model.PreparedTransactionCommitted
		if err := p.prepared.Update(ctx, stored); err != nil {
			statusCode = http.StatusInternalServerError
			return err
		}
		statusCode = http.StatusNoContent
		return nil
	})
	return statusCode, err
}

func (p *MessageProcessor) RollbackLocalTransaction(ctx context.Context, txID dto.ForeignBankId) (int, error) {
	var statusCode int
	err := p.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		stored, tx, err := p.loadStoredTransaction(ctx, txID)
		if err != nil {
			statusCode = http.StatusInternalServerError
			return err
		}
		if stored == nil {
			statusCode = http.StatusInternalServerError
			return fmt.Errorf("transaction not found")
		}
		if stored.Status == model.PreparedTransactionRolledBack {
			statusCode = http.StatusNoContent
			return nil
		}
		if stored.Status == model.PreparedTransactionCommitted {
			statusCode = http.StatusInternalServerError
			return fmt.Errorf("transaction already committed")
		}

		for i := range tx.Postings {
			if !p.isLocalPosting(tx.Postings[i]) {
				continue
			}
			_ = p.rollbackPosting(ctx, tx, i)
		}
		stored.Status = model.PreparedTransactionRolledBack
		if err := p.prepared.Update(ctx, stored); err != nil {
			statusCode = http.StatusInternalServerError
			return err
		}
		statusCode = http.StatusNoContent
		return nil
	})
	return statusCode, err
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
				} else {
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

func (p *MessageProcessor) preparePosting(ctx context.Context, tx *dto.Transaction, index int) (*preparedItem, dto.NoVoteReasonKind, error) {
	posting := tx.Postings[index]

	switch posting.Asset.Type {
	case dto.AssetMonas:
		isValid, accountNumber := p.localCashAccount(posting.Account)
		if !isValid {
			return nil, dto.ReasonNoSuchAccount, fmt.Errorf("invalid local cash account")
		}
		currency, ok := monetaryCurrency(posting.Asset)
		if !ok {
			return nil, dto.ReasonUnacceptableAsset, fmt.Errorf("invalid MONAS asset")
		}
		postingID := postingID(tx, index)
		_, err := p.banking.PrepareInterbankCashPosting(ctx, &pb.PrepareInterbankCashPostingRequest{
			PostingId:     postingID,
			AccountNumber: accountNumber,
			ClientId:      uint64(0), // this is for when we dont have account number, not needed here
			CurrencyCode:  currency,
			Amount:        posting.Amount,
		})
		if err != nil {
			return nil, cashNoVoteReason(err), err
		}
		return &preparedItem{kind: "cash", id: postingID}, "", nil

	case dto.AssetOption:
		return p.prepareOptionPosting(ctx, tx, index)

	default:
		return nil, dto.ReasonUnacceptableAsset, fmt.Errorf("unsupported asset %s", posting.Asset.Type)
	}
}

func (p *MessageProcessor) prepareOptionPosting(ctx context.Context, tx *dto.Transaction, index int) (*preparedItem, dto.NoVoteReasonKind, error) {
	posting := tx.Postings[index]
	if math.Abs(math.Abs(posting.Amount)-1) > txBalanceEpsilon {
		return nil, dto.ReasonOptionAmountIncorrect, fmt.Errorf("option posting amount must be +/-1")
	}
	isValid, clientID := p.localPersonAccount(posting.Account)
	if !isValid {
		return nil, dto.ReasonNoSuchAccount, fmt.Errorf("invalid option account")
	}
	if posting.Amount > 0 { //no preparation needed
		return nil, "", nil
	}

	option, ok := optionDescription(posting.Asset)
	if !ok {
		return nil, dto.ReasonUnacceptableAsset, fmt.Errorf("invalid OPTION asset")
	}
	if option.NegotiationID.RoutingNumber != p.peers.OurRoutingNumber() { // we are sellers, id is ours
		return nil, dto.ReasonUnacceptableAsset, fmt.Errorf("routing number mismatch")
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
	return &preparedItem{kind: "option", id: fmt.Sprintf("%d:%s", option.NegotiationID.RoutingNumber, option.NegotiationID.ID)}, "", nil
}

func (p *MessageProcessor) commitPosting(ctx context.Context, tx *dto.Transaction, index int) error {
	posting := tx.Postings[index]
	switch posting.Asset.Type {
	case dto.AssetMonas:
		isValid, _ := p.localCashAccount(posting.Account)
		if !isValid {
			return nil
		}
		_, err := p.banking.CommitInterbankCashPosting(ctx, postingID(tx, index))
		return err
	case dto.AssetOption:
		option, ok := optionDescription(posting.Asset)
		if !ok || posting.Amount > 0 {
			return nil
		}
		_, err := p.trading.ConsumePeerOtcShares(ctx,
			fmt.Sprintf("%d:%s", option.NegotiationID.RoutingNumber, option.NegotiationID.ID))
		return err
	default:
		return nil
	}
}

func (p *MessageProcessor) rollbackPosting(ctx context.Context, tx *dto.Transaction, index int) error {
	posting := tx.Postings[index]
	switch posting.Asset.Type {
	case dto.AssetMonas:
		isValid, account := p.localCashAccount(posting.Account)
		if !isValid || account == "" {
			return fmt.Errorf("invalid account")
		}
		_, err := p.banking.RollbackInterbankCashPosting(ctx, postingID(tx, index))
		return err
	case dto.AssetOption:
		option, ok := optionDescription(posting.Asset)
		if !ok || option.NegotiationID.RoutingNumber != p.peers.OurRoutingNumber() || posting.Amount > 0 {
			return fmt.Errorf("invalid option")
		}
		_, err := p.trading.ReleasePeerOtcShares(ctx, fmt.Sprintf("%d:%s", option.NegotiationID.RoutingNumber, option.NegotiationID.ID))
		return err
	default:
		return fmt.Errorf("posting not recognized")
	}
}

func (p *MessageProcessor) rollbackEffects(ctx context.Context, effects []preparedItem) {
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
		return stored, nil, err
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

func (p *MessageProcessor) isLocalPosting(posting dto.Posting) bool {
	switch posting.Account.Type {
	case dto.TxAccountAccount:
		if posting.Account.Num == nil {
			return false
		}
		prefix := fmt.Sprintf("%03d", p.peers.OurRoutingNumber())
		return strings.HasPrefix(strings.TrimSpace(*posting.Account.Num), prefix)
	case dto.TxAccountPerson, dto.TxAccountOption:
		if posting.Account.ID == nil {
			return false
		}
		return posting.Account.ID.RoutingNumber == p.peers.OurRoutingNumber()
	default:
		return false
	}
}

func (p *MessageProcessor) localCashAccount(account dto.TxAccount) (bool, string) {
	switch account.Type {
	case dto.TxAccountAccount:
		if account.Num == nil || strings.TrimSpace(*account.Num) == "" {
			return false, ""
		}
		num := strings.TrimSpace(*account.Num)
		prefix := fmt.Sprintf("%03d", p.peers.OurRoutingNumber())
		return strings.HasPrefix(num, prefix), num
	default:
		return false, ""
	}
}

func (p *MessageProcessor) localPersonAccount(account dto.TxAccount) (bool, uint) {
	if account.Type != dto.TxAccountPerson || account.ID == nil {
		return false, 0
	}
	if account.ID.RoutingNumber != p.peers.OurRoutingNumber() {
		return false, 0
	}
	id, err := strconv.ParseUint(account.ID.ID, 10, 64)
	if err != nil || id == 0 {
		return false, 0
	}
	return true, uint(id)
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
