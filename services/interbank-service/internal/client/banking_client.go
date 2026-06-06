package client

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
)

type BankingClient interface {
	ReserveOtcFunds(ctx context.Context, req *pb.ReserveOtcFundsRequest) (*pb.OtcFundsReservationResponse, error)
	ReleaseOtcFunds(ctx context.Context, executionID string) (*pb.OtcFundsReservationResponse, error)
	CommitOtcFunds(ctx context.Context, executionID string) (*pb.OtcFundsReservationResponse, error)
	RefundOtcFunds(ctx context.Context, executionID string) (*pb.OtcFundsReservationResponse, error)
	PrepareInterbankCashPosting(ctx context.Context, req *pb.PrepareInterbankCashPostingRequest) (*pb.InterbankCashPostingResponse, error)
	CommitInterbankCashPosting(ctx context.Context, postingID string) (*pb.InterbankCashPostingResponse, error)
	RollbackInterbankCashPosting(ctx context.Context, postingID string) (*pb.InterbankCashPostingResponse, error)
	// FinalizeInterbankPayment reports the final 2PC outcome of a banking-initiated
	// inter-bank payment back to banking-service so it can flip the transaction
	// out of its Processing state.
	FinalizeInterbankPayment(ctx context.Context, bankingTxID uint64, success bool) error
}
