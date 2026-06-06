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

func (t *tradingClient) ReservePeerOtcShares(ctx context.Context, req *pb.ReservePeerOtcSharesRequest) (*pb.PeerOtcSharesResponse, error) {
	return t.c.ReservePeerOtcShares(ctx, req)
}

func (t *tradingClient) ReleasePeerOtcShares(ctx context.Context, contractID string) (*pb.PeerOtcSharesResponse, error) {
	return t.c.ReleasePeerOtcShares(ctx, &pb.ReleasePeerOtcSharesRequest{ContractId: contractID})
}

func (t *tradingClient) ConsumePeerOtcShares(ctx context.Context, contractID string) (*pb.PeerOtcSharesResponse, error) {
	return t.c.ConsumePeerOtcShares(ctx, &pb.ConsumePeerOtcSharesRequest{ContractId: contractID})
}

func (t *tradingClient) CreditPeerOtcShares(ctx context.Context, req *pb.CreditPeerOtcSharesRequest) (*pb.PeerOtcSharesResponse, error) {
	return t.c.CreditPeerOtcShares(ctx, req)
}
