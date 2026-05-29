package client

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
)

// TradingClient wraps the trading-service gRPC surface used by
// interbank-service to serve §3 OTC lookups.
type TradingClient interface {
	ListPublicStocks(ctx context.Context) (*pb.ListPublicStocksResponse, error)
}
