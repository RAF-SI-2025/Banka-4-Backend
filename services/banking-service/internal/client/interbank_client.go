package client

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
)

// InterbankClient is the outbound gRPC surface banking-service uses to hand a
// payment to interbank-service when the recipient lives at another bank.
type InterbankClient interface {
	InitiatePayment(ctx context.Context, req *pb.InitiateInterbankPaymentRequest) error
}
