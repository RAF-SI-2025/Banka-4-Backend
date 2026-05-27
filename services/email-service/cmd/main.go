package main

import (
	"go.uber.org/fx"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/logging"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/email-service/internal/config"
	servicegrpc "github.com/RAF-SI-2025/Banka-4-Backend/services/email-service/internal/grpc"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/email-service/internal/handler"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/email-service/internal/server"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/email-service/internal/service"
)

// @title Email Service API
// @version 1.0
// @description Internal gRPC service for sending emails on behalf of other services.
func main() {
	fx.New(
		fx.Provide(
			config.Load,
			service.NewEmailService,
			handler.NewHealthHandler,
			servicegrpc.NewEmailService,
		),
		fx.Invoke(func(cfg *config.Configuration) error {
			return logging.Init(cfg.Env)
		}),
		fx.Invoke(server.NewServer, server.NewGRPCServer),
	).Run()
}
