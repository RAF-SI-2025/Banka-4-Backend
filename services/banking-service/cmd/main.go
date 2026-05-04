package main

import (
	"context"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/jwt"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/logging"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/client"
	clientgrpc "github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/client/grpc"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/config"
	servicegrpc "github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/grpc"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/handler"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/permission"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/repository"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/seed"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/server"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/service"
)

// @title Banking Service API
// @version 1.0
// @description API for managing accounts, balances, and banking operations.
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT Authorization header using the Bearer scheme.
func main() {
	fx.New(
		fx.Provide(
			config.Load,
			func(cfg *config.Configuration) *redis.Client {
				return redis.NewClient(&redis.Options{
					Addr:     cfg.Redis.Addr,
					Password: cfg.Redis.Password,
					DB:       cfg.Redis.DB,
				})
			},
			func(cfg *config.Configuration, redisClient *redis.Client) service.WorkCodeCache {
				return service.NewWorkCodeRedisCache(redisClient, cfg.Redis.WorkCodesCacheTTL)
			},
			func(cfg *config.Configuration) (*gorm.DB, error) {
				return db.New(cfg.DB.DSN())
			},
			func(cfg *config.Configuration) auth.TokenVerifier {
				return jwt.NewJWTVerifier(cfg.JWTSecret)
			},
			func(cfg *config.Configuration) client.ExchangeRateClient {
				return client.NewExchangeRateClient(cfg.ExchangeRateAPIKey)
			},
			fx.Annotate(
				client.NewMobileSecretClient,
				fx.As(new(client.MobileSecretClient)),
			),
			client.NewUserServiceConnection,
			fx.Annotate(
				clientgrpc.NewUserServiceClient,
				fx.As(new(client.UserClient)),
			),
			func(conn *grpc.ClientConn) pb.PermissionServiceClient {
				return pb.NewPermissionServiceClient(conn)
			},
			func(c pb.PermissionServiceClient) auth.PermissionProvider {
				return permission.NewGrpcPermissionProvider(c)
			},
			handler.NewHealthHandler,
			repository.NewAccountRepository,
			repository.NewCompanyRepository,
			repository.NewPayeeRepository,
			repository.NewCardRepository,
			repository.NewAuthorizedPersonRepository,
			repository.NewCardRequestRepository,
			repository.NewExchangeRateRepository,
			repository.NewCurrencyRepository,
			service.NewExchangeService,
			func(svc *service.ExchangeService) service.CurrencyConverter {
				return svc
			},
			repository.NewPaymentRepository,
			repository.NewTransactionRepository,
			repository.NewVerificationTokenRepository,
			repository.NewGormTransactionManager,
			repository.NewLoanRepository,
			repository.NewLoanRequestRepository,
			repository.NewLoanTypeRepository,
			service.NewAccountService,
			service.NewCompanyService,
			service.NewPayeeService,
			service.NewPaymentService,
			service.NewTransactionProcessor,
			service.NewCardService,
			service.NewEmailService,
			service.NewLoanService,
			service.NewLoanScheduler,
			handler.NewAccountHandler,
			handler.NewCompanyHandler,
			handler.NewPayeeHandler,
			handler.NewExchangeHandler,
			handler.NewPaymentHandler,
			repository.NewTransferRepository,
			service.NewTransferService,
			handler.NewTransferHandler,
			handler.NewCardHandler,
			handler.NewLoanHandler,
			servicegrpc.NewBankingService,
		),
		fx.Invoke(func(cfg *config.Configuration) error {
			return logging.Init(cfg.Env)
		}),
		fx.Invoke(func(db *gorm.DB) error {
			if err := normalizeVerificationTokensSchema(db); err != nil {
				return err
			}

			if err := db.AutoMigrate(
				&model.Currency{},
				&model.WorkCode{},
				&model.Company{},
				&model.Account{},
				&model.Payee{},
				&model.Card{},
				&model.AuthorizedPerson{},
				&model.CardRequest{},
				&model.ExchangeRate{},
				&model.Transaction{},
				&model.Payment{},
				&model.Transfer{},
				&model.VerificationToken{},
				&model.LoanType{},
				&model.LoanRequest{},
				&model.Loan{},
				&model.LoanInstallment{},
			); err != nil {
				return err
			}
			return seed.Run(db)
		}),
		fx.Invoke(func(lc fx.Lifecycle, svc *service.ExchangeService) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					svc.Initialize(ctx)
					svc.Start()
					return nil
				},
				OnStop: func(ctx context.Context) error {
					svc.Stop()
					return nil
				},
			})
		}),
		fx.Invoke(func(lc fx.Lifecycle, scheduler *service.LoanScheduler) {
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

func normalizeVerificationTokensSchema(db *gorm.DB) error {
	if db.Migrator().HasColumn("verification_tokens", "code") {
		if err := db.Migrator().DropColumn("verification_tokens", "code"); err != nil {
			return err
		}
	}

	if db.Migrator().HasColumn("verification_tokens", "expires_at") {
		if err := db.Migrator().DropColumn("verification_tokens", "expires_at"); err != nil {
			return err
		}
	}

	return nil
}
