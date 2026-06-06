package grpc

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/client"
)

type bankingClient struct {
	c pb.BankingServiceClient
}

func NewBankingClient(conn *client.BankingServiceConn) client.BankingClient {
	return &bankingClient{c: pb.NewBankingServiceClient(conn.ClientConn)}
}

func (b *bankingClient) ReserveOtcFunds(ctx context.Context, req *pb.ReserveOtcFundsRequest) (*pb.OtcFundsReservationResponse, error) {
	return b.c.ReserveOtcFunds(ctx, req)
}

func (b *bankingClient) ReleaseOtcFunds(ctx context.Context, executionID string) (*pb.OtcFundsReservationResponse, error) {
	return b.c.ReleaseOtcFunds(ctx, &pb.OtcFundsRequest{ExecutionId: executionID})
}

func (b *bankingClient) CommitOtcFunds(ctx context.Context, executionID string) (*pb.OtcFundsReservationResponse, error) {
	return b.c.CommitOtcFunds(ctx, &pb.OtcFundsRequest{ExecutionId: executionID})
}

func (b *bankingClient) RefundOtcFunds(ctx context.Context, executionID string) (*pb.OtcFundsReservationResponse, error) {
	return b.c.RefundOtcFunds(ctx, &pb.OtcFundsRequest{ExecutionId: executionID})
}

func (b *bankingClient) PrepareInterbankCashPosting(ctx context.Context, req *pb.PrepareInterbankCashPostingRequest) (*pb.InterbankCashPostingResponse, error) {
	return b.c.PrepareInterbankCashPosting(ctx, req)
}

func (b *bankingClient) CommitInterbankCashPosting(ctx context.Context, postingID string) (*pb.InterbankCashPostingResponse, error) {
	return b.c.CommitInterbankCashPosting(ctx, &pb.InterbankCashPostingRequest{PostingId: postingID})
}

func (b *bankingClient) RollbackInterbankCashPosting(ctx context.Context, postingID string) (*pb.InterbankCashPostingResponse, error) {
	return b.c.RollbackInterbankCashPosting(ctx, &pb.InterbankCashPostingRequest{PostingId: postingID})
}

func (b *bankingClient) FinalizeInterbankPayment(ctx context.Context, bankingTxID uint64, success bool) error {
	_, err := b.c.FinalizeInterbankPayment(ctx, &pb.FinalizeInterbankPaymentRequest{BankingTxId: bankingTxID, Success: success})
	return err
}
