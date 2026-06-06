package grpc

import (
	"context"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/service"
)

// InterbankGRPCService handles InitiatePayment calls from the banking-service.
type InterbankGRPCService struct {
	pb.UnimplementedInterbankServiceServer
	processor *service.MessageProcessor
	resolver  *service.PeerResolver
}

func NewInterbankGRPCService(
	processor *service.MessageProcessor,
	resolver *service.PeerResolver,
) *InterbankGRPCService {
	return &InterbankGRPCService{
		processor: processor,
		resolver:  resolver,
	}
}

// InitiatePayment starts a cross-bank payment. It builds a balanced two-posting
// transaction (payer credited, payee debited) and drives it through the same
// MessageProcessor 2PC as OTC: the payer's funds are reserved locally via
// PrepareInterbankCashPosting and a NEW_TX outbox row is enqueued atomically.
// The outbox worker then sends NEW_TX, collects the vote, and commits or rolls
// back — so this call returns as soon as the local reservation succeeds.
//
// Because interbank reserves the payer funds itself, the banking-service caller
// must NOT pre-reserve; it only needs the payer account to hold the funds.
func (s *InterbankGRPCService) InitiatePayment(ctx context.Context, req *pb.InitiateInterbankPaymentRequest) (*pb.InitiateInterbankPaymentResponse, error) {
	if len(req.PayeeAccountNumber) < 3 {
		return nil, status.Error(codes.InvalidArgument, "payee_account_number too short")
	}

	peerRoutingNumber, err := strconv.Atoi(req.PayeeAccountNumber[:3])
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "cannot parse peer routing number from payee account")
	}

	if peerRoutingNumber == s.resolver.OurRoutingNumber() {
		return nil, status.Error(codes.InvalidArgument, "payee is on this bank — settle internally, not via interbank")
	}
	if _, ok := s.resolver.ByRoutingNumber(peerRoutingNumber); !ok {
		return nil, status.Errorf(codes.NotFound, "no peer configured for routing number %d", peerRoutingNumber)
	}

	ourRouting := s.resolver.OurRoutingNumber()
	transactionIDKey := uuid.New().String()

	// Balanced postings (§2.8): the payer (local) is CREDITED (negative →
	// reserved here, debited on commit); the payee (remote) is DEBITED.
	payerNum := req.PayerAccountNumber
	payeeNum := req.PayeeAccountNumber
	asset := dto.Asset{
		Type: dto.AssetMonas,
		Body: map[string]any{"currency": req.Currency},
	}
	tx := dto.Transaction{
		TransactionID: dto.ForeignBankId{RoutingNumber: ourRouting, ID: transactionIDKey},
		Postings: []dto.Posting{
			{
				Account: dto.TxAccount{Type: dto.TxAccountAccount, Num: &payerNum},
				Amount:  -req.Amount,
				Asset:   asset,
			},
			{
				Account: dto.TxAccount{Type: dto.TxAccountAccount, Num: &payeeNum},
				Amount:  req.Amount,
				Asset:   asset,
			},
		},
		Message:        req.Message,
		PaymentCode:    req.PaymentCode,
		PaymentPurpose: req.PaymentPurpose,
	}

	_, vote, _, err := s.processor.PrepareAndEnqueueNewTx(ctx, &tx, peerRoutingNumber, transactionIDKey+"-new", model.FlowTypePayment, req.BankingTxId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to prepare inter-bank payment: %v", err)
	}
	if vote.Vote != dto.VoteYes {
		return nil, status.Errorf(codes.FailedPrecondition, "payer side rejected the payment: %s", noVoteReasons(vote))
	}

	return &pb.InitiateInterbankPaymentResponse{}, nil
}

// noVoteReasons renders a TransactionVote's NO reasons for a gRPC error string.
func noVoteReasons(vote dto.TransactionVote) string {
	if len(vote.Reasons) == 0 {
		return "no reason provided"
	}
	parts := make([]string, 0, len(vote.Reasons))
	for _, r := range vote.Reasons {
		parts = append(parts, string(r.Reason))
	}
	return strings.Join(parts, ",")
}
