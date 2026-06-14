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
	outboundRepo repository.OutboundMessageRepository
	txManager    repository.TransactionManager
	peers        *PeerResolver
	banking      client.BankingClient
	trading      client.TradingClient
	contracts    repository.PeerContractRepository
	negotiations repository.PeerNegotiationRepository
	userClient   client.UserClient
}

func NewMessageProcessor(
	inbound repository.InboundMessageRepository,
	prepared repository.PreparedTransactionRepository,
	outboundRepo repository.OutboundMessageRepository,
	txManager repository.TransactionManager,
	peers *PeerResolver,
	banking client.BankingClient,
	trading client.TradingClient,
	contracts repository.PeerContractRepository,
	negotiations repository.PeerNegotiationRepository,
	userClient client.UserClient,
) *MessageProcessor {
	return &MessageProcessor{
		inbound:      inbound,
		prepared:     prepared,
		outboundRepo: outboundRepo,
		txManager:    txManager,
		peers:        peers,
		banking:      banking,
		trading:      trading,
		contracts:    contracts,
		negotiations: negotiations,
		userClient:   userClient,
	}
}

// resolveLocalUser maps a local Identity.ID (as carried by PERSON-account ids on
// the wire) to the role-scoped user id (Client.ClientID / Employee.EmployeeID)
// and its user type ("CLIENT" | "EMPLOYEE") that banking/trading expect.
func (p *MessageProcessor) resolveLocalUser(ctx context.Context, identityID uint) (uint64, string, error) {
	resp, err := p.userClient.GetUserByIdentityID(ctx, uint64(identityID))
	if err != nil {
		return 0, "", err
	}
	return resp.GetUserId(), resp.GetUserType(), nil
}

func (p *MessageProcessor) ProcessNewTx(ctx context.Context, peerRouting int, key dto.IdempotenceKey, tx *dto.Transaction) (int, any, error) {
	// Check inbound dedup first — same logic as processInbound but without
	// wrapping PrepareLocalTransaction in a transaction (the phased prepare
	// manages its own commits and must NOT run inside an outer tx).
	existing, err := p.inbound.FindByKey(ctx, peerRouting, key.LocallyGeneratedKey)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	if existing != nil {
		if existing.ResponseStatus != http.StatusAccepted {
			var cached any
			if len(existing.ResponseBody) > 0 {
				var vote dto.TransactionVote
				if err := json.Unmarshal(existing.ResponseBody, &vote); err == nil {
					cached = vote
				}
			}
			return existing.ResponseStatus, cached, nil
		}
		// 202 means the previous attempt was unfinished; fall through to retry.
	}

	// Inbound (we are the participant): no banking tx id — record creation is
	// driven purely by the local posting's sign/metadata at commit time.
	statusCode, vote, err := p.PrepareLocalTransaction(ctx, tx, 0)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}

	// Persist the inbound record non-transactionally. The PreparedTransaction
	// row written by the phased prepare is the real recovery anchor — losing
	// this inbound row just means a retransmit recomputes the identical vote.
	requestBody, _ := json.Marshal(tx)
	responseBody, _ := json.Marshal(vote)
	_ = p.inbound.Save(ctx, &model.InboundMessage{
		PeerRoutingNumber:   peerRouting,
		LocallyGeneratedKey: key.LocallyGeneratedKey,
		MessageType:         string(dto.MessageTypeNewTx),
		RequestBody:         requestBody,
		ResponseStatus:      statusCode,
		ResponseBody:        responseBody,
		ProcessedAt:         time.Now(),
	})

	return statusCode, vote, nil
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

// PrepareLocalTransaction runs in three separate phases so that the recovery
// anchor (the PreparedTransaction row with its body) is always committed to
// disk before any external reservation is issued.  This eliminates the
// dual-write leak where a reservation succeeds but our transaction rolls back,
// leaving the reservation stranded with no record to release it.
//
// Phase 1 (own DB tx): balance check + write PREPARING record (or short-circuit
//
//	on an existing terminal/in-progress record).
//
// Phase 2 (no tx):     run each local posting reservation (idempotent by
//
//	posting/contract id).  On failure: rollback already-issued effects, mark
//	the record ROLLED_BACK, return NO vote.
//
// Phase 3 (own DB tx): flip PREPARING → PREPARED, return YES vote.
//
// CRITICAL: this function manages its own transactions.  It must NOT be called
// inside an outer WithinTransaction — that would collapse all three phases into
// one tx and reintroduce the leak.
func (p *MessageProcessor) PrepareLocalTransaction(ctx context.Context, tx *dto.Transaction, bankingTxID uint64) (int, dto.TransactionVote, error) {
	if tx == nil {
		return http.StatusInternalServerError, dto.TransactionVote{}, fmt.Errorf("invalid transaction")
	}
	if reason := p.balanceFailure(tx); reason != nil {
		return http.StatusOK, dto.TransactionVote{Vote: dto.VoteNo, Reasons: []dto.NoVoteReason{*reason}}, nil
	}

	body, err := json.Marshal(tx)
	if err != nil {
		return http.StatusInternalServerError, dto.TransactionVote{}, err
	}

	// --- Phase 1: persist intent -------------------------------------------
	var rec *model.PreparedTransaction
	err = p.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		existing, err := p.prepared.FindByID(ctx, tx.TransactionID.RoutingNumber, tx.TransactionID.ID)
		if err != nil {
			return err
		}
		if existing != nil {
			rec = existing
			return nil
		}
		rec = &model.PreparedTransaction{
			RoutingNumber: tx.TransactionID.RoutingNumber,
			ID:            tx.TransactionID.ID,
			Status:        model.PreparedTransactionPreparing,
			RequestBody:   body,
		}
		return p.prepared.Create(ctx, rec)
	})
	if err != nil {
		return http.StatusInternalServerError, dto.TransactionVote{}, err
	}

	// Short-circuit on existing terminal or in-progress state.
	switch rec.Status {
	case model.PreparedTransactionCommitted, model.PreparedTransactionPrepared:
		return http.StatusOK, dto.TransactionVote{Vote: dto.VoteYes}, nil
	case model.PreparedTransactionRolledBack:
		return http.StatusOK, dto.TransactionVote{Vote: dto.VoteNo, Reasons: []dto.NoVoteReason{{Reason: dto.ReasonUnbalancedTx}}}, nil
	case model.PreparedTransactionPreparing:
		// Either we just created it, or a previous attempt left it here
		// (crash/retry).  Re-run the reserves — they are all idempotent.
	}

	// --- Phase 2: external reservations (no wrapping tx) -------------------
	var effects []preparedItem
	var noVoteResult *dto.TransactionVote
	for i := range tx.Postings {
		if !p.isLocalPosting(tx.Postings[i]) {
			continue
		}
		item, reason, reserveErr := p.preparePosting(ctx, tx, i, bankingTxID)
		if reserveErr != nil {
			p.rollbackEffects(ctx, effects)
			v := noVote(reason, &tx.Postings[i])
			noVoteResult = &v
			break
		}
		if item != nil {
			effects = append(effects, *item)
		}
	}

	if noVoteResult != nil {
		// Mark the record so a future retransmit or ROLLBACK_TX can find it.
		_ = p.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
			rec.Status = model.PreparedTransactionRolledBack
			return p.prepared.Update(ctx, rec)
		})
		return http.StatusOK, *noVoteResult, nil
	}

	// --- Phase 3: confirm ---------------------------------------------------
	err = p.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		rec.Status = model.PreparedTransactionPrepared
		return p.prepared.Update(ctx, rec)
	})
	if err != nil {
		// Reserves are out but we couldn't confirm. Leave record as PREPARING
		// so a retransmit of NEW_TX will re-run the (idempotent) reserves and
		// retry phase 3.
		return http.StatusInternalServerError, dto.TransactionVote{}, err
	}

	return http.StatusOK, dto.TransactionVote{Vote: dto.VoteYes}, nil
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
		if stored.Status == model.PreparedTransactionPreparing {
			// Reservations are not confirmed yet — tell the coordinator to
			// retransmit later. Committing now would hit NotFound errors on
			// the un-prepared postings and produce a torn partial commit.
			statusCode = http.StatusAccepted
			return nil
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
			// Nothing to roll back: either the NEW_TX never reached us (e.g. the
			// coordinator voted NO on its own side and never prepared here) or it
			// failed before a PreparedTransaction row was written (e.g. an
			// unbalanced tx is rejected before Phase 1). Treat ROLLBACK of an
			// unknown transaction as a successful no-op so the coordinator's
			// rollback is robustly idempotent and never wedges in a pointless
			// retry loop against a 500.
			statusCode = http.StatusNoContent
			return nil
		}
		if stored.Status == model.PreparedTransactionRolledBack {
			statusCode = http.StatusNoContent
			return nil
		}
		if stored.Status == model.PreparedTransactionCommitted {
			statusCode = http.StatusInternalServerError
			return fmt.Errorf("transaction already committed")
		}
		// PREPARING is treated like PREPARED: reserves may be partially out,
		// so we enumerate the stored body and release each one. rollbackPosting
		// swallows per-posting errors, so releasing a reservation that was
		// never issued is safe (banking NotFound / trading idempotent release).

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
			// 202 means the previous attempt was unfinished; retry to advance.
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

// ---------------------------------------------------------------------------
// preparePosting dispatches by asset type.
//
// Sign convention (§2.8): a NEGATIVE amount is a CREDIT (the asset leaves the
// account) and a POSITIVE amount is a DEBIT (the asset enters the account).
// Per §2.8.4 only credited accounts (negative amounts) have assets reserved
// during prepare; debited accounts (positive amounts) only need validation.
// ---------------------------------------------------------------------------

func (p *MessageProcessor) preparePosting(ctx context.Context, tx *dto.Transaction, index int, bankingTxID uint64) (*preparedItem, dto.NoVoteReasonKind, error) {
	posting := tx.Postings[index]

	switch posting.Asset.Type {
	case dto.AssetMonas:
		return p.prepareMonasPosting(ctx, tx, index, bankingTxID)
	case dto.AssetOption:
		return p.prepareOptionPosting(ctx, tx, index)
	case dto.AssetStock:
		return p.prepareStockPosting(ctx, tx, index)
	default:
		return nil, dto.ReasonUnacceptableAsset, fmt.Errorf("unsupported asset %s", posting.Asset.Type)
	}
}

// prepareMonasPosting handles MONAS for ACCOUNT, PERSON, and OPTION account types.
func (p *MessageProcessor) prepareMonasPosting(ctx context.Context, tx *dto.Transaction, index int, bankingTxID uint64) (*preparedItem, dto.NoVoteReasonKind, error) {
	posting := tx.Postings[index]
	currency, ok := monetaryCurrency(posting.Asset)
	if !ok {
		return nil, dto.ReasonUnacceptableAsset, fmt.Errorf("invalid MONAS asset")
	}
	pid := postingID(tx, index)
	counterparty := counterpartyAccountNumber(tx, index)

	switch posting.Account.Type {
	case dto.TxAccountAccount:
		isValid, accountNumber := p.localCashAccount(posting.Account)
		if !isValid {
			return nil, dto.ReasonNoSuchAccount, fmt.Errorf("invalid local cash account")
		}
		_, err := p.banking.PrepareInterbankCashPosting(ctx, &pb.PrepareInterbankCashPostingRequest{
			PostingId:                 pid,
			AccountNumber:             accountNumber,
			CurrencyCode:              currency,
			Amount:                    posting.Amount,
			BankingTxId:               bankingTxID,
			CounterpartyAccountNumber: counterparty,
			PaymentCode:               tx.PaymentCode,
			PaymentPurpose:            tx.PaymentPurpose,
		})
		if err != nil {
			return nil, cashNoVoteReason(err), err
		}
		return &preparedItem{kind: "cash", id: pid}, "", nil

	case dto.TxAccountPerson:
		isValid, identityID := p.localPersonAccount(posting.Account)
		if !isValid {
			return nil, dto.ReasonNoSuchAccount, fmt.Errorf("invalid person account for cash")
		}
		userID, userType, err := p.resolveLocalUser(ctx, identityID)
		if err != nil {
			return nil, dto.ReasonNoSuchAccount, err
		}
		_, err = p.banking.PrepareInterbankCashPosting(ctx, &pb.PrepareInterbankCashPostingRequest{
			PostingId:                 pid,
			ClientId:                  userID,
			UserType:                  userType,
			CurrencyCode:              currency,
			Amount:                    posting.Amount,
			BankingTxId:               bankingTxID,
			CounterpartyAccountNumber: counterparty,
			PaymentCode:               tx.PaymentCode,
			PaymentPurpose:            tx.PaymentPurpose,
		})
		if err != nil {
			return nil, cashNoVoteReason(err), err
		}
		return &preparedItem{kind: "cash", id: pid}, "", nil

	case dto.TxAccountOption:
		isValid, negotiationID := p.localOptionAccount(posting.Account)
		if !isValid {
			return nil, dto.ReasonNoSuchAccount, fmt.Errorf("invalid option account for cash")
		}
		contract, err := p.contracts.FindByID(ctx, negotiationID.RoutingNumber, negotiationID.ID)
		if err != nil {
			return nil, dto.ReasonOptionNegotiationNotFound, err
		}
		if contract == nil {
			return nil, dto.ReasonOptionNegotiationNotFound, fmt.Errorf("option contract not found")
		}
		sellerIdentityID, parseErr := strconv.ParseUint(contract.SellerID, 10, 64)
		if parseErr != nil || sellerIdentityID == 0 {
			return nil, dto.ReasonNoSuchAccount, fmt.Errorf("invalid seller id on contract")
		}
		sellerID, sellerType, err := p.resolveLocalUser(ctx, uint(sellerIdentityID))
		if err != nil {
			return nil, dto.ReasonNoSuchAccount, err
		}
		_, err = p.banking.PrepareInterbankCashPosting(ctx, &pb.PrepareInterbankCashPostingRequest{
			PostingId:                 pid,
			ClientId:                  sellerID,
			UserType:                  sellerType,
			CurrencyCode:              currency,
			Amount:                    posting.Amount,
			BankingTxId:               bankingTxID,
			CounterpartyAccountNumber: counterparty,
			PaymentCode:               tx.PaymentCode,
			PaymentPurpose:            tx.PaymentPurpose,
		})
		if err != nil {
			return nil, cashNoVoteReason(err), err
		}
		return &preparedItem{kind: "cash", id: pid}, "", nil
	}

	return nil, dto.ReasonNoSuchAccount, fmt.Errorf("unsupported account type for MONAS")
}

// counterpartyAccountNumber returns the account number of the other side of a
// two-account MONAS posting (best-effort, for payment-history display). It picks
// the first ACCOUNT-type posting other than index whose number fits the banking
// account-number column (≤18). Returns "" when none is suitable (e.g. OTC postings
// that reference PERSON/OPTION pseudo-accounts).
func counterpartyAccountNumber(tx *dto.Transaction, index int) string {
	for j := range tx.Postings {
		if j == index {
			continue
		}
		acc := tx.Postings[j].Account
		if acc.Type == dto.TxAccountAccount && acc.Num != nil {
			num := strings.TrimSpace(string(*acc.Num))
			if num != "" && len(num) <= 18 {
				return num
			}
		}
	}
	return ""
}

// prepareOptionPosting handles OPTION asset (accept TX).
func (p *MessageProcessor) prepareOptionPosting(ctx context.Context, tx *dto.Transaction, index int) (*preparedItem, dto.NoVoteReasonKind, error) {
	posting := tx.Postings[index]
	if math.Abs(math.Abs(posting.Amount)-1) > txBalanceEpsilon {
		return nil, dto.ReasonOptionAmountIncorrect, fmt.Errorf("option posting amount must be +/-1")
	}
	isValid, identityID := p.localPersonAccount(posting.Account)
	if !isValid {
		return nil, dto.ReasonNoSuchAccount, fmt.Errorf("invalid option account")
	}
	if posting.Amount > 0 {
		// Buyer side (DEBIT, amount > 0): receiving the option — nothing to reserve.
		return nil, "", nil
	}

	// Seller side (CREDIT, amount < 0): giving the option — reserve the shares
	// that back the contract so it can be exercised before settlement.
	option, ok := optionDescription(posting.Asset)
	if !ok {
		return nil, dto.ReasonUnacceptableAsset, fmt.Errorf("invalid OPTION asset")
	}
	if option.NegotiationID.RoutingNumber != p.peers.OurRoutingNumber() {
		return nil, dto.ReasonUnacceptableAsset, fmt.Errorf("routing number mismatch")
	}
	negotiation, err := p.negotiations.FindByID(ctx, option.NegotiationID.RoutingNumber, option.NegotiationID.ID)
	if err != nil {
		return nil, dto.ReasonOptionNegotiationNotFound, err
	}
	if negotiation == nil {
		return nil, dto.ReasonOptionNegotiationNotFound, fmt.Errorf("negotiation not found")
	}
	sellerID, sellerType, err := p.resolveLocalUser(ctx, identityID)
	if err != nil {
		return nil, dto.ReasonNoSuchAccount, err
	}
	_, err = p.trading.ReservePeerOtcShares(ctx, &pb.ReservePeerOtcSharesRequest{
		ContractId: fmt.Sprintf("%d:%s", option.NegotiationID.RoutingNumber, option.NegotiationID.ID),
		SellerId:   sellerID,
		UserType:   sellerType,
		Ticker:     option.Stock.Ticker,
		Amount:     option.Amount,
	})
	if err != nil {
		return nil, dto.ReasonInsufficientAsset, err
	}
	return &preparedItem{kind: "option", id: fmt.Sprintf("%d:%s", option.NegotiationID.RoutingNumber, option.NegotiationID.ID)}, "", nil
}

// prepareStockPosting handles STOCK asset (exercise TX).
func (p *MessageProcessor) prepareStockPosting(ctx context.Context, tx *dto.Transaction, index int) (*preparedItem, dto.NoVoteReasonKind, error) {
	posting := tx.Postings[index]

	if posting.Amount < 0 {
		// Seller side (CREDIT, amount < 0) via the OPTION pseudo-account:
		// validate the contract is still active. The shares were already
		// reserved by the accept TX, so no new reservation is taken here.
		isValid, negotiationID := p.localOptionAccount(posting.Account)
		if !isValid {
			return nil, dto.ReasonNoSuchAccount, fmt.Errorf("invalid option account for stock debit")
		}
		contract, err := p.contracts.FindByID(ctx, negotiationID.RoutingNumber, negotiationID.ID)
		if err != nil {
			return nil, dto.ReasonOptionNegotiationNotFound, err
		}
		if contract == nil {
			return nil, dto.ReasonOptionNegotiationNotFound, fmt.Errorf("option contract not found")
		}
		if contract.Status != model.PeerContractActive {
			return nil, dto.ReasonOptionUsedOrExpired, fmt.Errorf("option contract is not active")
		}
		if SettlementPassed(contract.SettlementDate) {
			return nil, dto.ReasonOptionUsedOrExpired, fmt.Errorf("option contract has expired")
		}
		// Shares are already reserved by the accept TX — no new reservation.
		return nil, "", nil
	}

	// Buyer side (DEBIT, amount > 0) via PERSON account — receiving the shares,
	// no preparation needed.
	return nil, "", nil
}

// ---------------------------------------------------------------------------
// commitPosting dispatches by asset type.
// ---------------------------------------------------------------------------

func (p *MessageProcessor) commitPosting(ctx context.Context, tx *dto.Transaction, index int) error {
	posting := tx.Postings[index]

	switch posting.Asset.Type {
	case dto.AssetMonas:
		if p.isLocalMonasAccount(posting.Account) {
			_, err := p.banking.CommitInterbankCashPosting(ctx, postingID(tx, index))
			return err
		}
		return nil

	case dto.AssetOption:
		return p.commitOptionPosting(ctx, tx, index)

	case dto.AssetStock:
		return p.commitStockPosting(ctx, tx, index)
	}
	return nil
}

// commitOptionPosting creates the PeerContract when the accept TX is committed.
func (p *MessageProcessor) commitOptionPosting(ctx context.Context, tx *dto.Transaction, index int) error {
	posting := tx.Postings[index]
	option, ok := optionDescription(posting.Asset)
	if !ok {
		return nil
	}

	if posting.Amount < 0 {
		// Seller DEBIT (PERSON account): create authoritative contract on seller's bank.
		isValid, _ := p.localPersonAccount(posting.Account)
		if !isValid {
			return nil
		}

		negotiation, err := p.negotiations.FindByID(ctx, option.NegotiationID.RoutingNumber, option.NegotiationID.ID)
		if err != nil || negotiation == nil {
			return err
		}

		// Find buyer from the OPTION CREDIT posting.
		var buyerRouting int
		var buyerID string
		for j := range tx.Postings {
			p2 := tx.Postings[j]
			if p2.Asset.Type == dto.AssetOption && p2.Amount > 0 && p2.Account.Type == dto.TxAccountPerson && p2.Account.ID != nil {
				buyerRouting = p2.Account.ID.RoutingNumber
				buyerID = p2.Account.ID.ID
				break
			}
		}

		contract := &model.PeerContract{
			AuthorityRoutingNumber: p.peers.OurRoutingNumber(),
			ID:                     option.NegotiationID.ID,
			NegotiationID:          option.NegotiationID.ID,
			BuyerRoutingNumber:     buyerRouting,
			BuyerID:                buyerID,
			SellerRoutingNumber:    negotiation.SellerRoutingNumber,
			SellerID:               negotiation.SellerID,
			Ticker:                 negotiation.Ticker,
			Amount:                 negotiation.Amount,
			StrikePrice:            negotiation.PricePerStock,
			StrikeCurrency:         negotiation.PriceCurrency,
			Premium:                negotiation.Premium,
			PremiumCurrency:        negotiation.PremiumCurrency,
			SettlementDate:         negotiation.SettlementDate,
			Status:                 model.PeerContractActive,
			IsAuthoritative:        true,
		}
		if err := p.contracts.Create(ctx, contract); err != nil {
			return err
		}

		if negotiation.Status == model.PeerNegotiationOngoing {
			negotiation.Status = model.PeerNegotiationAccepted
			_ = p.negotiations.Update(ctx, negotiation)
		}
		return nil
	}

	// Buyer CREDIT (PERSON account): create mirror contract on buyer's bank.
	isValid, buyerLocalID := p.localPersonAccount(posting.Account)
	if !isValid {
		return nil
	}

	// Find seller from the OPTION DEBIT posting.
	var sellerRouting int
	var sellerID string
	for j := range tx.Postings {
		p2 := tx.Postings[j]
		if p2.Asset.Type == dto.AssetOption && p2.Amount < 0 && p2.Account.Type == dto.TxAccountPerson && p2.Account.ID != nil {
			sellerRouting = p2.Account.ID.RoutingNumber
			sellerID = p2.Account.ID.ID
			break
		}
	}

	// Extract premium amount and currency from the MONAS ACCOUNT posting.
	var premium float64
	var premiumCurrency string
	for j := range tx.Postings {
		p2 := tx.Postings[j]
		if p2.Asset.Type == dto.AssetMonas && p2.Account.Type == dto.TxAccountAccount {
			premium = math.Abs(p2.Amount)
			premiumCurrency, _ = monetaryCurrency(p2.Asset)
			break
		}
	}

	contract := &model.PeerContract{
		AuthorityRoutingNumber: option.NegotiationID.RoutingNumber,
		ID:                     option.NegotiationID.ID,
		NegotiationID:          option.NegotiationID.ID,
		BuyerRoutingNumber:     p.peers.OurRoutingNumber(),
		BuyerID:                strconv.FormatUint(uint64(buyerLocalID), 10),
		SellerRoutingNumber:    sellerRouting,
		SellerID:               sellerID,
		Ticker:                 option.Stock.Ticker,
		Amount:                 int(option.Amount),
		StrikePrice:            option.PricePerUnit.Amount,
		StrikeCurrency:         string(option.PricePerUnit.Currency),
		Premium:                premium,
		PremiumCurrency:        premiumCurrency,
		SettlementDate:         string(option.SettlementDate),
		Status:                 model.PeerContractActive,
		IsAuthoritative:        false,
	}
	if err := p.contracts.Create(ctx, contract); err != nil {
		return err
	}
	return nil
}

// commitStockPosting handles STOCK asset commit for both seller and buyer.
func (p *MessageProcessor) commitStockPosting(ctx context.Context, tx *dto.Transaction, index int) error {
	posting := tx.Postings[index]
	stock, ok := stockDescription(posting.Asset)
	if !ok {
		return nil
	}

	if posting.Amount < 0 {
		// Seller DEBIT via OPTION account: consume reservation and mark EXERCISED.
		isValid, negotiationID := p.localOptionAccount(posting.Account)
		if !isValid {
			return nil
		}
		contract, err := p.contracts.FindByID(ctx, negotiationID.RoutingNumber, negotiationID.ID)
		if err != nil || contract == nil {
			return err
		}
		contractKey := fmt.Sprintf("%d:%s", contract.AuthorityRoutingNumber, contract.ID)
		if _, err := p.trading.ConsumePeerOtcShares(ctx, contractKey); err != nil {
			return err
		}
		now := time.Now()
		contract.Status = model.PeerContractExercised
		contract.ExercisedAt = &now
		return p.contracts.Update(ctx, contract)
	}

	// Buyer CREDIT via PERSON account: add ownership and mark EXERCISED.
	isValid, buyerIdentityID := p.localPersonAccount(posting.Account)
	if !isValid {
		return nil
	}
	buyerLocalID, buyerType, err := p.resolveLocalUser(ctx, buyerIdentityID)
	if err != nil {
		return err
	}

	// Find negotiation ID from the paired STOCK DEBIT (OPTION account) posting.
	var negotiationID dto.ForeignBankId
	for j := range tx.Postings {
		p2 := tx.Postings[j]
		if p2.Asset.Type == dto.AssetStock && p2.Amount < 0 && p2.Account.Type == dto.TxAccountOption && p2.Account.ID != nil {
			negotiationID = *p2.Account.ID
			break
		}
	}
	contract, err := p.contracts.FindByID(ctx, negotiationID.RoutingNumber, negotiationID.ID)
	if err != nil || contract == nil {
		return err
	}
	contractKey := fmt.Sprintf("%d:%s", contract.AuthorityRoutingNumber, contract.ID)
	if _, err := p.trading.CreditPeerOtcShares(ctx, &pb.CreditPeerOtcSharesRequest{
		ContractId:      contractKey,
		BuyerId:         buyerLocalID,
		UserType:        buyerType,
		Ticker:          stock.Ticker,
		Amount:          posting.Amount,
		PricePerUnitRsd: contract.StrikePrice,
	}); err != nil {
		return err
	}
	now := time.Now()
	contract.Status = model.PeerContractExercised
	contract.ExercisedAt = &now
	return p.contracts.Update(ctx, contract)
}

// ---------------------------------------------------------------------------
// rollbackPosting dispatches by asset type.
// ---------------------------------------------------------------------------

func (p *MessageProcessor) rollbackPosting(ctx context.Context, tx *dto.Transaction, index int) error {
	posting := tx.Postings[index]

	switch posting.Asset.Type {
	case dto.AssetMonas:
		if p.isLocalMonasAccount(posting.Account) {
			_, err := p.banking.RollbackInterbankCashPosting(ctx, postingID(tx, index))
			return err
		}
		return nil

	case dto.AssetOption:
		option, ok := optionDescription(posting.Asset)
		if !ok || option.NegotiationID.RoutingNumber != p.peers.OurRoutingNumber() || posting.Amount > 0 {
			return nil
		}
		_, err := p.trading.ReleasePeerOtcShares(ctx, fmt.Sprintf("%d:%s", option.NegotiationID.RoutingNumber, option.NegotiationID.ID))
		return err

	case dto.AssetStock:
		// No reservation was made in prepare for STOCK postings — nothing to undo.
		return nil
	}
	return nil
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

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

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

// isLocalMonasAccount returns true if the account is any locally-owned type.
func (p *MessageProcessor) isLocalMonasAccount(account dto.TxAccount) bool {
	switch account.Type {
	case dto.TxAccountAccount:
		isValid, _ := p.localCashAccount(account)
		return isValid
	case dto.TxAccountPerson:
		isValid, _ := p.localPersonAccount(account)
		return isValid
	case dto.TxAccountOption:
		isValid, _ := p.localOptionAccount(account)
		return isValid
	}
	return false
}

func (p *MessageProcessor) localCashAccount(account dto.TxAccount) (bool, string) {
	if account.Type != dto.TxAccountAccount || account.Num == nil || strings.TrimSpace(*account.Num) == "" {
		return false, ""
	}
	num := strings.TrimSpace(*account.Num)
	prefix := fmt.Sprintf("%03d", p.peers.OurRoutingNumber())
	return strings.HasPrefix(num, prefix), num
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

func (p *MessageProcessor) localOptionAccount(account dto.TxAccount) (bool, dto.ForeignBankId) {
	if account.Type != dto.TxAccountOption || account.ID == nil {
		return false, dto.ForeignBankId{}
	}
	if account.ID.RoutingNumber != p.peers.OurRoutingNumber() {
		return false, dto.ForeignBankId{}
	}
	return true, *account.ID
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

func stockDescription(asset dto.Asset) (dto.StockDescription, bool) {
	var desc dto.StockDescription
	if asset.Type != dto.AssetStock {
		return desc, false
	}
	raw, err := json.Marshal(asset.Body)
	if err != nil {
		return desc, false
	}
	if err := json.Unmarshal(raw, &desc); err != nil {
		return desc, false
	}
	return desc, desc.Ticker != ""
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
	case dto.AssetStock:
		desc, ok := stockDescription(asset)
		return "STOCK:" + desc.Ticker, ok
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

// ---------------------------------------------------------------------------
// Atomic outbox methods for OTC 2PC
// ---------------------------------------------------------------------------

// PrepareAndEnqueueNewTx prepares the local transaction and, if the local
// vote is YES and the peer is remote, enqueues a NEW_TX outbox row within
// the same DB transaction. The returned OutboundMessage (may be nil for
// same-bank or NO vote) lets the caller mark it SENT after a successful
// optimistic sync send.
func (p *MessageProcessor) PrepareAndEnqueueNewTx(
	ctx context.Context,
	tx *dto.Transaction,
	peerRouting int,
	idempotenceKey string,
	flowType string,
	bankingTxID uint64,
) (int, dto.TransactionVote, *model.OutboundMessage, error) {
	// Phase 1-3: phased prepare manages its own transactions — must NOT be
	// called inside a wrapping WithinTransaction.
	statusCode, vote, err := p.PrepareLocalTransaction(ctx, tx, bankingTxID)
	if err != nil {
		return statusCode, vote, nil, err
	}
	if vote.Vote != dto.VoteYes || peerRouting == p.peers.OurRoutingNumber() {
		return statusCode, vote, nil, nil
	}

	// Enqueue the outbox row in its own transaction, separate from the prepare
	// phases above. If this fails the PREPARED record is already committed, so
	// the outbox row can be re-inserted on retry without leaking reservations.
	payload, err := json.Marshal(dto.NewTxMessage{
		IdempotenceKey: dto.IdempotenceKey{RoutingNumber: p.peers.OurRoutingNumber(), LocallyGeneratedKey: idempotenceKey},
		MessageType:    dto.MessageTypeNewTx,
		Message:        *tx,
	})
	if err != nil {
		return http.StatusInternalServerError, vote, nil, err
	}
	outMsg := buildOutboundMessage(peerRouting, dto.MessageTypeNewTx, idempotenceKey, payload, flowType, bankingTxID)
	if err := p.outboundRepo.Enqueue(ctx, outMsg); err != nil {
		return http.StatusInternalServerError, vote, nil, err
	}
	return statusCode, vote, outMsg, nil
}

// CommitAndEnqueueFollowUp commits the local transaction and, if the peer is
// remote, enqueues a COMMIT_TX outbox row within the same DB transaction.
func (p *MessageProcessor) CommitAndEnqueueFollowUp(
	ctx context.Context,
	txID dto.ForeignBankId,
	peerRouting int,
	idempotenceKey string,
	flowType string,
) (int, *model.OutboundMessage, error) {
	var statusCode int
	var outMsg *model.OutboundMessage

	err := p.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		var err error
		statusCode, err = p.CommitLocalTransaction(ctx, txID)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(dto.CommitTxMessage{
			IdempotenceKey: dto.IdempotenceKey{RoutingNumber: p.peers.OurRoutingNumber(), LocallyGeneratedKey: idempotenceKey},
			MessageType:    dto.MessageTypeCommitTx,
			Message:        dto.CommitTransaction{TransactionID: txID},
		})
		if err != nil {
			return err
		}
		outMsg = buildOutboundMessage(peerRouting, dto.MessageTypeCommitTx, idempotenceKey, payload, flowType, 0)
		return p.outboundRepo.Enqueue(ctx, outMsg)
	})
	return statusCode, outMsg, err
}

// RollbackAndEnqueueFollowUp rolls back the local transaction and, if the
// peer is remote, enqueues a ROLLBACK_TX outbox row within the same DB
// transaction.
func (p *MessageProcessor) RollbackAndEnqueueFollowUp(
	ctx context.Context,
	txID dto.ForeignBankId,
	peerRouting int,
	idempotenceKey string,
	flowType string,
) (int, *model.OutboundMessage, error) {
	var statusCode int
	var outMsg *model.OutboundMessage

	err := p.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		var err error
		statusCode, err = p.RollbackLocalTransaction(ctx, txID)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(dto.RollbackTxMessage{
			IdempotenceKey: dto.IdempotenceKey{RoutingNumber: p.peers.OurRoutingNumber(), LocallyGeneratedKey: idempotenceKey},
			MessageType:    dto.MessageTypeRollbackTx,
			Message:        dto.RollbackTransaction{TransactionID: txID},
		})
		if err != nil {
			return err
		}
		outMsg = buildOutboundMessage(peerRouting, dto.MessageTypeRollbackTx, idempotenceKey, payload, flowType, 0)
		return p.outboundRepo.Enqueue(ctx, outMsg)
	})
	return statusCode, outMsg, err
}

// FinalizeInterbankPayment reports a PAYMENT's final 2PC outcome to
// banking-service so it can move the originating transaction out of Processing.
func (p *MessageProcessor) FinalizeInterbankPayment(ctx context.Context, bankingTxID uint64, success bool) error {
	return p.banking.FinalizeInterbankPayment(ctx, bankingTxID, success)
}

func buildOutboundMessage(peerRouting int, msgType dto.MessageType, idempotenceKey string, payload []byte, flowType string, bankingTxID uint64) *model.OutboundMessage {
	return &model.OutboundMessage{
		PeerRoutingNumber:   peerRouting,
		MessageType:         string(msgType),
		IdempotenceKeyLocal: idempotenceKey,
		Payload:             payload,
		FlowType:            flowType,
		BankingTxID:         bankingTxID,
		Status:              model.OutboundPending,
	}
}
