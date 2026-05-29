package client

import (
	"context"

	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/config"
)

// UserServiceConn is a typed wrapper around the gRPC connection to
// user-service. The typed wrapper exists so fx can distinguish this
// connection from the one to trading-service in the dependency graph.
type UserServiceConn struct{ *grpc.ClientConn }

func NewUserServiceConnection(lc fx.Lifecycle, cfg *config.Configuration) (*UserServiceConn, error) {
	conn, err := grpc.NewClient(
		cfg.UserServiceAddr,
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

	return &UserServiceConn{ClientConn: conn}, nil
}
