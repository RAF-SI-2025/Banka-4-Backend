package client

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
)

// TradingClient wraps the trading-service gRPC surface used by
// interbank-service to serve §3 OTC lookups.
type TradingClient interface {
	ListPublicStocks(ctx context.Context) (*pb.ListPublicStocksResponse, error)
	ReservePeerOtcShares(ctx context.Context, req *pb.ReservePeerOtcSharesRequest) (*pb.PeerOtcSharesResponse, error)
	ReleasePeerOtcShares(ctx context.Context, contractID string) (*pb.PeerOtcSharesResponse, error)
	ConsumePeerOtcShares(ctx context.Context, contractID string) (*pb.PeerOtcSharesResponse, error)
	CreditPeerOtcShares(ctx context.Context, req *pb.CreditPeerOtcSharesRequest) (*pb.PeerOtcSharesResponse, error)
}
