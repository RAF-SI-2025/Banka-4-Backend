package client

import (
	"context"

	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/config"
)

// EmailServiceConn wraps *grpc.ClientConn so it can coexist with the trading
// service connection without causing an fx duplicate-type error.
type EmailServiceConn struct{ *grpc.ClientConn }

func NewEmailServiceConnection(lc fx.Lifecycle, cfg *config.Configuration) (*EmailServiceConn, error) {
	conn, err := grpc.NewClient(
		cfg.EmailServiceAddr,
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

	return &EmailServiceConn{conn}, nil
}
