package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
)

type InterbankServiceClient struct {
	client pb.InterbankServiceClient
}

func NewInterbankServiceClient(addr string) (*InterbankServiceClient, error) {
	// Keepalive pings keep the long-lived connection healthy so an idle TCP
	// connection silently dropped between payments is detected and re-dialed
	// proactively, instead of surfacing as a codes.Unavailable on the next RPC.
	// Time (30s) must stay >= the server's EnforcementPolicy MinTime or the
	// server replies with GOAWAY "too_many_pings".
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return nil, err
	}
	return &InterbankServiceClient{client: pb.NewInterbankServiceClient(conn)}, nil
}

func (c *InterbankServiceClient) InitiatePayment(ctx context.Context, req *pb.InitiateInterbankPaymentRequest) error {
	_, err := c.client.InitiatePayment(ctx, req)
	return err
}
