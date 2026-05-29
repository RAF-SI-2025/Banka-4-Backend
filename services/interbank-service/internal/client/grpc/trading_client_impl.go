package grpc

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/client"
)

type tradingClient struct {
	c pb.TradingServiceClient
}

func NewTradingClient(conn *client.TradingServiceConn) client.TradingClient {
	return &tradingClient{c: pb.NewTradingServiceClient(conn.ClientConn)}
}

func (t *tradingClient) ListPublicStocks(ctx context.Context) (*pb.ListPublicStocksResponse, error) {
	return t.c.ListPublicStocks(ctx, &pb.ListPublicStocksRequest{})
}
