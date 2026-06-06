package grpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
)

type InterbankServiceClient struct {
	client pb.InterbankServiceClient
}

func NewInterbankServiceClient(addr string) (*InterbankServiceClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &InterbankServiceClient{client: pb.NewInterbankServiceClient(conn)}, nil
}

func (c *InterbankServiceClient) InitiatePayment(ctx context.Context, req *pb.InitiateInterbankPaymentRequest) error {
	_, err := c.client.InitiatePayment(ctx, req)
	return err
}
