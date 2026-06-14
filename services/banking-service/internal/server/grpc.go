package server

import (
	"context"
	"errors"
	"log"
	"net"
	"time"

	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/config"
	service "github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/grpc"
)

func NewGRPCServer(
	lc fx.Lifecycle,
	cfg *config.Configuration,
	bankingService *service.BankingService,
) error {
	listener, err := net.Listen("tcp", ":"+cfg.GrpcPort)
	if err != nil {
		return err
	}

	// EnforcementPolicy.MinTime must be <= the interbank client's keepalive Time
	// (30s) and PermitWithoutStream must match, otherwise the server answers
	// client pings with GOAWAY "too_many_pings" — which itself surfaces as
	// codes.Unavailable on the caller.
	grpcServer := grpc.NewServer(
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             10 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    30 * time.Second,
			Timeout: 10 * time.Second,
		}),
	)
	pb.RegisterBankingServiceServer(grpcServer, bankingService)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if serveErr := grpcServer.Serve(listener); serveErr != nil && !errors.Is(serveErr, grpc.ErrServerStopped) {
					log.Printf("gRPC server stopped: %v", serveErr)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			done := make(chan struct{})
			go func() {
				grpcServer.GracefulStop()
				close(done)
			}()

			select {
			case <-done:
				if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
					return err
				}
				return nil
			case <-ctx.Done():
				grpcServer.Stop()
				_ = listener.Close()
				return ctx.Err()
			}
		},
	})

	return nil
}
