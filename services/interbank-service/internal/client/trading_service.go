package client

import (
	"context"

	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/config"
)

// TradingServiceConn is a typed wrapper around the gRPC connection to
// trading-service. The typed wrapper exists so fx can distinguish this
// connection from the one to user-service in the dependency graph.
type TradingServiceConn struct{ *grpc.ClientConn }

func NewTradingServiceConnection(lc fx.Lifecycle, cfg *config.Configuration) (*TradingServiceConn, error) {
	conn, err := grpc.NewClient(
		cfg.TradingServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return conn.Close()
		},
	})

	return &TradingServiceConn{ClientConn: conn}, nil
}
