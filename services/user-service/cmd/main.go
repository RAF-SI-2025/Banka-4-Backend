package main

import (
	"context"

	"go.uber.org/fx"
	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/audit"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/jwt"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/logging"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/client"
	clientgrpc "github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/client/grpc"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/config"
	servicegrpc "github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/grpc"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/handler"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/permission"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/repository"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/seed"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/server"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/service"
)

// @title User Service API
// @version 1.0
// @description API for managing employees, clients, authentication, and permissions.
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter "Bearer" followed by a space and your token. Example: "Bearer eyJhbGci..."
func main() {
	fx.New(
		fx.Provide(
			config.Load,
			func(cfg *config.Configuration) (*gorm.DB, error) {
				return db.New(cfg.DB.DSN())
			},
			func(cfg *config.Configuration) auth.TokenVerifier {
				return jwt.NewJWTVerifier(cfg.JWTSecret)
			},
			func(database *gorm.DB) auth.PermissionProvider {
				return permission.NewDBPermissionProvider(database)
			},

			client.NewTradingServiceConnection,
			fx.Annotate(
				clientgrpc.NewTradingServiceClient,
				fx.As(new(client.TradingClient)),
			),
			client.NewEmailServiceConnection,
			fx.Annotate(
				clientgrpc.NewEmailClient,
				fx.As(new(service.Mailer)),
			),

			audit.NewRepository,
			audit.NewService,
			repository.NewIdentityRepository,
			repository.NewEmployeeRepository,
			repository.NewActuaryRepository,
			repository.NewClientRepository,
			repository.NewActivationTokenRepository,
			repository.NewResetTokenRepository,
			repository.NewRefreshTokenRepository,
			repository.NewGormTransactionManager,
			repository.NewPositionRepository,
			service.NewAuthService,
			service.NewEmployeeService,
			service.NewActuaryService,
			service.NewClientService,
			service.NewActuaryLimitScheduler,
			service.NewAuditLogService,
			handler.NewAuthHandler,
			handler.NewEmployeeHandler,
			handler.NewActuaryHandler,
			handler.NewClientHandler,
			handler.NewHealthHandler,
			handler.NewAuditLogHandler,
			servicegrpc.NewPermissionService,
			servicegrpc.NewUserService,
		),
		fx.Invoke(func(cfg *config.Configuration) error {
			return logging.Init(cfg.Env)
		}),
		fx.Invoke(func(db *gorm.DB) error {
			if err := db.AutoMigrate(
				&model.Identity{},
				&model.Employee{},
				&model.ActuaryInfo{},
				&model.Client{},
				&model.Position{},
				&model.ActivationToken{},
				&model.ResetToken{},
				&model.RefreshToken{},
				&model.EmployeePermission{},
				&model.ClientPermission{},
				&audit.AuditLog{},
			); err != nil {
				return err
			}
			return seed.Run(db)
		}),
		fx.Invoke(func(lc fx.Lifecycle, scheduler *service.ActuaryLimitScheduler) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					scheduler.Start()
					return nil
				},
				OnStop: func(ctx context.Context) error {
					scheduler.Stop()
					return nil
				},
			})
		}),
		fx.Invoke(server.NewServer, server.NewGRPCServer),
	).Run()
}
