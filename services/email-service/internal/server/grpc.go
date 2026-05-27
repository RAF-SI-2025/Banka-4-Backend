package server

import (
	"context"
	"errors"
	"log"
	"net"

	"go.uber.org/fx"
	"google.golang.org/grpc"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/email-service/internal/config"
	service "github.com/RAF-SI-2025/Banka-4-Backend/services/email-service/internal/grpc"
)

func NewGRPCServer(
	lc fx.Lifecycle,
	cfg *config.Configuration,
	emailService *service.EmailService,
) error {
	listener, err := net.Listen("tcp", ":"+cfg.GrpcPort)
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()
	pb.RegisterEmailServiceServer(grpcServer, emailService)

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
