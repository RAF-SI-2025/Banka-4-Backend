package client

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/config"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

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
