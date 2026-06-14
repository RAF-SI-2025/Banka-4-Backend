package server

import (
	"context"
	"net"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/config"
	grpchandler "github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/grpc"
)

func NewGRPCServer(lc fx.Lifecycle, cfg *config.Configuration, svc *grpchandler.InterbankGRPCService) {
	// EnforcementPolicy.MinTime must be <= clients' keepalive Time (30s) and
	// PermitWithoutStream must match, otherwise the server answers client pings
	// with GOAWAY "too_many_pings" — which itself surfaces as codes.Unavailable.
	srv := grpc.NewServer(
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             10 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    30 * time.Second,
			Timeout: 10 * time.Second,
		}),
	)
	pb.RegisterInterbankServiceServer(srv, svc)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			lis, err := net.Listen("tcp", ":"+cfg.GrpcPort)
			if err != nil {
				return err
			}
			zap.L().Info("interbank gRPC server listening", zap.String("port", cfg.GrpcPort))
			go func() { _ = srv.Serve(lis) }()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			srv.GracefulStop()
			return nil
		},
	})
}
