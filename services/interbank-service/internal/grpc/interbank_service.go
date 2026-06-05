package grpc

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/repository"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/service"
)

// InterbankGRPCService handles InitiatePayment calls from the banking-service.
type InterbankGRPCService struct {
	pb.UnimplementedInterbankServiceServer
	outboundPaymentRepo repository.OutboundPaymentRepository
	outboundMessageRepo repository.OutboundMessageRepository
	txManager           repository.TransactionManager
	resolver            *service.PeerResolver
}

func NewInterbankGRPCService(
	outboundPaymentRepo repository.OutboundPaymentRepository,
	outboundMessageRepo repository.OutboundMessageRepository,
	txManager repository.TransactionManager,
	resolver *service.PeerResolver,
) *InterbankGRPCService {
	return &InterbankGRPCService{
		outboundPaymentRepo: outboundPaymentRepo,
		outboundMessageRepo: outboundMessageRepo,
		txManager:           txManager,
		resolver:            resolver,
	}
}

// InitiatePayment creates an OutboundPayment and enqueues a NEW_TX outbound
// message. Returns immediately — the outbox worker drives the 2PC from here.
func (s *InterbankGRPCService) InitiatePayment(ctx context.Context, req *pb.InitiateInterbankPaymentRequest) (*pb.InitiateInterbankPaymentResponse, error) {
	if len(req.PayeeAccountNumber) < 3 {
		return nil, status.Error(codes.InvalidArgument, "payee_account_number too short")
	}

	peerRoutingNumber, err := strconv.Atoi(req.PayeeAccountNumber[:3])
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "cannot parse peer routing number from payee account")
	}

	if _, ok := s.resolver.ByRoutingNumber(peerRoutingNumber); !ok {
		return nil, status.Errorf(codes.NotFound, "no peer configured for routing number %d", peerRoutingNumber)
	}

	ourRouting := s.resolver.OurRoutingNumber()
	transactionIDKey := uuid.New().String()
	idempotenceKey := uuid.New().String()

	// Build the wire Transaction with two postings that sum to zero.
	payerNum := req.PayerAccountNumber
	payeeNum := req.PayeeAccountNumber
	asset := dto.Asset{
		Type: dto.AssetMonas,
		Body: map[string]any{"currency": req.Currency},
	}
	txMsg := dto.Transaction{
		TransactionID: dto.ForeignBankId{
			RoutingNumber: ourRouting,
			ID:            transactionIDKey,
		},
		Postings: []dto.Posting{
			{
				Account: dto.TxAccount{Type: dto.TxAccountAccount, Num: &payerNum},
				Amount:  req.Amount,
				Asset:   asset,
			},
			{
				Account: dto.TxAccount{Type: dto.TxAccountAccount, Num: &payeeNum},
				Amount:  -req.Amount,
				Asset:   asset,
			},
		},
		Message:        req.Message,
		PaymentCode:    req.PaymentCode,
		PaymentPurpose: req.PaymentPurpose,
	}

	wireMsg := dto.NewTxMessage{
		IdempotenceKey: dto.IdempotenceKey{
			RoutingNumber:       ourRouting,
			LocallyGeneratedKey: idempotenceKey,
		},
		MessageType: dto.MessageTypeNewTx,
		Message:     txMsg,
	}

	payload, err := json.Marshal(wireMsg)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to marshal NEW_TX payload: %v", err)
	}

	payment := &model.OutboundPayment{
		TransactionIDKey:  transactionIDKey,
		BankingTxID:       req.BankingTxId,
		PeerRoutingNumber: peerRoutingNumber,
	}
	outMsg := &model.OutboundMessage{
		PeerRoutingNumber:   peerRoutingNumber,
		MessageType:         string(dto.MessageTypeNewTx),
		IdempotenceKeyLocal: idempotenceKey,
		Payload:             payload,
		Status:              model.OutboundPending,
	}

	if err := s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.outboundPaymentRepo.Create(ctx, payment); err != nil {
			return err
		}
		return s.outboundMessageRepo.Enqueue(ctx, outMsg)
	}); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to enqueue inter-bank payment: %v", err)
	}

	return &pb.InitiateInterbankPaymentResponse{}, nil
}
