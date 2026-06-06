package client

import (
	"context"

	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/config"
)

type BankingServiceConn struct{ *grpc.ClientConn }

func NewBankingServiceConnection(lc fx.Lifecycle, cfg *config.Configuration) (*BankingServiceConn, error) {
	conn, err := grpc.NewClient(
		cfg.BankingServiceAddr,
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

	return &BankingServiceConn{ClientConn: conn}, nil
}
