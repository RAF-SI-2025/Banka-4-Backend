package server

import (
	"context"
	"net"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/config"
	grpchandler "github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/grpc"
)

func NewGRPCServer(lc fx.Lifecycle, cfg *config.Configuration, svc *grpchandler.InterbankGRPCService) {
	srv := grpc.NewServer()
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
