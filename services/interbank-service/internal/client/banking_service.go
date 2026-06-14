package client

import (
	"context"
	"time"

	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/config"
)

type BankingServiceConn struct{ *grpc.ClientConn }

func NewBankingServiceConnection(lc fx.Lifecycle, cfg *config.Configuration) (*BankingServiceConn, error) {
	// Keepalive keeps this re-entrant callback connection (interbank -> banking,
	// invoked mid-payment from PrepareInterbankCashPosting) healthy so an idle
	// drop is re-dialed proactively rather than failing the prepare. Time (30s)
	// must stay >= banking's gRPC EnforcementPolicy MinTime.
	conn, err := grpc.NewClient(
		cfg.BankingServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
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
