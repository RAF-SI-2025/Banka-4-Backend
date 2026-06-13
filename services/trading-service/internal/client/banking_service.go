package client

import (
	"context"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/config"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// defaultRPCTimeout bounds every banking call that arrives without its own
// deadline. Without it, an unreachable or paused banking service would hang
// the OTC settlement saga (and its HTTP callers) indefinitely instead of
// surfacing DeadlineExceeded and letting the saga retry or compensate.
func defaultRPCTimeout() time.Duration {
	seconds := config.GetAsIntOrDefault("BANKING_RPC_TIMEOUT_SECONDS", 10)
	return time.Duration(seconds) * time.Second
}

func withDefaultTimeout(timeout time.Duration) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func NewBankingServiceConnection(lc fx.Lifecycle, cfg *config.Configuration) (*BankingConn, error) {
	conn, err := grpc.NewClient(
		cfg.BankingServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(withDefaultTimeout(defaultRPCTimeout())),
	)

	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return conn.Close()
		},
	})

	return &BankingConn{ClientConn: conn}, nil
}

type BankingConn struct {
	*grpc.ClientConn
}
