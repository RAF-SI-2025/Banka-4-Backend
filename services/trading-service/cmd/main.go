package main

import (
	"context"
	"log"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/handler"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/client"
	clientgrpc "github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/client/grpc"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/config"
	tradinggrpc "github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/grpc"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/job"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/permission"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/seed"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/server"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/service"
	"go.uber.org/zap"

	"go.uber.org/fx"
	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/audit"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/jwt"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/logging"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/robfig/cron/v3"
)

// @title Trading Service API
// @version 1.0
// @description API for managing portfolios, executing trades, and handling financial market operations.
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT Authorization header using the Bearer scheme.
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
			client.NewUserServiceConnection,
			func(conn *client.UserConn) pb.PermissionServiceClient {
				return pb.NewPermissionServiceClient(conn.ClientConn)
			},
			func(conn *client.UserConn) client.UserServiceClient {
				return clientgrpc.NewUserServiceClient(conn)
			},
			client.NewBankingServiceConnection,
			func(conn *client.BankingConn) pb.BankingServiceClient {
				return pb.NewBankingServiceClient(conn.ClientConn)
			},
			func(conn *client.BankingConn) client.BankingClient {
				return clientgrpc.NewBankingServiceClient(conn)
			},
			func(c pb.PermissionServiceClient) auth.PermissionProvider {
				return permission.NewGrpcPermissionProvider(c)
			},
			audit.NewRepository,
			audit.NewService,
			handler.NewHealthHandler,
			repository.NewAssetRepository,
			repository.NewAssetOwnershipRepository,
			repository.NewForexRepository,
			func(cfg *config.Configuration) client.ExchangeRateClient {
				return client.NewExchangeRateClient(cfg.ExchangeRateAPIKey)
			},
			service.NewForexService,
			func(cfg *config.Configuration) *client.StockClient {
				return client.NewStockClient(cfg.FinnhubAPIKey)
			},
			repository.NewListingRepository,
			repository.NewStockRepository,
			repository.NewOptionRepository,
			job.NewDailyPriceJob,
			job.NewFundHistoryJob,
			job.NewFundRedemptionJob,
			service.NewStockService,
			repository.NewExchangeRepository,
			service.NewExchangeService,
			handler.NewExchangeHandler,
			service.NewListingService,
			handler.NewListingHandler,
			repository.NewFuturesContractRepository,
			service.NewPortfolioService,
			handler.NewPortfolioHandler,
			repository.NewOrderRepository,
			repository.NewOrderTransactionRepository,
			service.NewOrderService,
			func(svc *service.TaxService) service.TaxRecorder {
				return svc
			},
			handler.NewOrderHandler,
			repository.NewTaxRepository,
			service.NewTaxService,
			handler.NewTaxHandler,
			service.NewTaxScheduler,
			repository.NewOtcOfferRepository,
			repository.NewOtcOptionContractRepository,
			repository.NewOtcShareReservationRepository,
			repository.NewOtcExecutionSagaRepository,
			repository.NewGormTransactionManager,
			service.NewOtcDealProcessingService,
			service.NewOtcOfferService,
			handler.NewOtcOfferHandler,
			service.NewOTCService,
			handler.NewOTCHandler,
			repository.NewInvestmentFundRepository,
			repository.NewClientFundPositionRepository,
			repository.NewClientFundInvestmentRepository,
			repository.NewClientFundRedemptionRepository,
			service.NewInvestmentFundService,
			handler.NewInvestmentFundHandler,
			repository.NewWatchlistRepository,
			service.NewWatchlistService,
			handler.NewWatchlistHandler,
			repository.NewDividendPayoutRepository,
			service.NewDividendPayoutService,
			handler.NewDividendHandler,
			job.NewDividendPayoutJob,
			client.NewEmailServiceConnection,
			func(conn *clientgrpc.EmailClient) service.Mailer {
				return conn
			},
			clientgrpc.NewEmailClient,
			repository.NewRecurringOrderRepository,
			service.NewRecurringOrderService,
			handler.NewRecurringOrderHandler,
			service.NewRecurringOrderScheduler,
			tradinggrpc.NewTradingServiceServer,
			job.NewOtcOptionExpirationJob,
		),
		fx.Invoke(func(cfg *config.Configuration) error {
			return logging.Init(cfg.Env)
		}),
		fx.Invoke(func(db *gorm.DB) error {
			return db.AutoMigrate(&model.Exchange{})
		}),
		fx.Invoke(func(db *gorm.DB) error {
			return seed.RunExchangeSeed(db)
		}),
		fx.Invoke(func(db *gorm.DB) error {
			return seed.NormalizeListingExchangeMICs(db)
		}),
		fx.Invoke(func(db *gorm.DB) error {
			return db.AutoMigrate(
				&audit.AuditLog{},
				&model.Asset{},
				&model.Listing{},
				&model.Stock{},
				&model.Option{},
				&model.ListingDailyPriceInfo{},
				&model.Order{},
				&model.AssetOwnership{},
				&model.OrderTransaction{},
				&model.ForexPair{},
				&model.FuturesContract{},
				&model.AccumulatedTax{},
				&model.TaxCollection{},
				&model.OtcOffer{},
				&model.OtcOptionContract{},
				&model.OtcShareReservation{},
				&model.OtcExecutionSaga{},
				&model.InvestmentFund{},
				&model.ClientFundPosition{},
				&model.ClientFundInvestment{},
				&model.ClientFundRedemption{},
				&model.FundPerformance{},
				&model.Watchlist{},
				&model.WatchlistItem{},
				&model.DividendPayout{},
				&model.RecurringOrder{},
			)
		}),
		fx.Invoke(func(lc fx.Lifecycle, svc *service.StockService) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					go svc.Initialize(context.Background())
					svc.Start()
					return nil
				},
				OnStop: func(ctx context.Context) error {
					svc.Stop()
					return nil
				},
			})
		}),
		fx.Invoke(func(db *gorm.DB) error {
			return seed.SeedFuturesContracts(db)
		}),
		fx.Invoke(func(db *gorm.DB) error { return seed.InvestmentFunds(db) }),
		fx.Invoke(func(db *gorm.DB) error { return seed.SeedAssetOwnerships(db) }),
		fx.Invoke(func(db *gorm.DB) error {
			return seed.AccumulatedTax(db)
		}),
		fx.Invoke(server.NewServer, server.NewGRPCServer),
		fx.Invoke(func(lc fx.Lifecycle, scheduler *service.TaxScheduler) {
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
		fx.Invoke(func(lifecycle fx.Lifecycle, forexService *service.ForexService) {
			lifecycle.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					forexService.Initialize(ctx)
					forexService.Start()
					return nil
				},
				OnStop: func(ctx context.Context) error {
					forexService.Stop()
					return nil
				},
			})
		}),
		fx.Invoke(func(db *gorm.DB) {
			go func() {
				time.Sleep(1 * time.Minute)
				if err := seed.SeedDailyPriceHistory(db, 365); err != nil {
					log.Printf("Failed to seed daily price history after delay: %v", err)
				} else {
					log.Println("Daily price history seeded successfully")
				}
			}()
		}),
		fx.Invoke(func(lc fx.Lifecycle, dailyJob *job.DailyPriceJob, fundHistoryJob *job.FundHistoryJob) {
			c := cron.New(cron.WithLocation(time.UTC))
			_, err := c.AddFunc("0 0 * * *", func() {
				ctx := context.Background()
				if err := dailyJob.Run(ctx); err != nil {
					logging.Error("Daily price job failed", zap.Error(err))
				}
				if err := fundHistoryJob.Run(ctx); err != nil {
					logging.Error("Fund history job failed", zap.Error(err))
				}
			})
			if err != nil {
				log.Fatal("Failed to schedule daily jobs", zap.Error(err))
			}

			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					c.Start()
					return nil
				},
				OnStop: func(ctx context.Context) error {
					c.Stop()
					return nil
				},
			})
		}),
		fx.Invoke(func(lifecycle fx.Lifecycle, orderService *service.OrderService) {
			lifecycle.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					orderService.Start()
					return nil
				},
				OnStop: func(ctx context.Context) error {
					orderService.Stop()
					return nil
				},
			})
		}),
		fx.Invoke(func(lifecycle fx.Lifecycle, otcProcessingService *service.OtcDealProcessingService) {
			lifecycle.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					otcProcessingService.Start()
					return nil
				},
				OnStop: func(ctx context.Context) error {
					otcProcessingService.Stop()
					return nil
				},
			})
		}),
		fx.Invoke(func(lifecycle fx.Lifecycle, scheduler *service.RecurringOrderScheduler) {
			lifecycle.Append(fx.Hook{
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
		fx.Invoke(func(lc fx.Lifecycle, fundRedemptionJob *job.FundRedemptionJob) {
			c := cron.New(cron.WithLocation(time.UTC))
			_, err := c.AddFunc("@every 5m", func() {
				ctx := context.Background()
				if err := fundRedemptionJob.Run(ctx); err != nil {
					logging.Error("Fund redemption job failed", zap.Error(err))
				}
			})
			if err != nil {
				log.Fatal("Failed to schedule fund redemption job", zap.Error(err))
			}

			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					c.Start()
					return nil
				},
				OnStop: func(ctx context.Context) error {
					c.Stop()
					return nil
				},
			})
		}),
		// Dividend payout job — pokreće se svakog radnog dana u 17:00 UTC.
		// Sam job interno proverava da li je danas poslednji radni dan kvartala.
		fx.Invoke(func(lc fx.Lifecycle, dividendJob *job.DividendPayoutJob) {
			c := cron.New(cron.WithLocation(time.UTC))
			_, err := c.AddFunc("0 17 * * 1-5", func() {
				ctx := context.Background()
				if err := dividendJob.Run(ctx); err != nil {
					logging.Error("Dividend payout job failed", zap.Error(err))
				}
			})
			if err != nil {
				log.Fatal("Failed to schedule dividend payout job", zap.Error(err))
			}

			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					c.Start()
					return nil
				},
				OnStop: func(ctx context.Context) error {
					c.Stop()
					return nil
				},
			})
		}),
		fx.Invoke(func(lc fx.Lifecycle, otcJob *job.OtcOptionExpirationJob) {
			c := cron.New(cron.WithLocation(time.UTC))

			// svakog dana u 00:00 (ili kako želiš)
			_, err := c.AddFunc("0 0 * * *", func() {
				ctx := context.Background()

				if err := otcJob.Run(ctx); err != nil {
					logging.Error("OTC expiration job failed", zap.Error(err))
				}
			})

			if err != nil {
				log.Fatal("Failed to schedule OTC expiration job", zap.Error(err))
			}

			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					c.Start()
					return nil
				},
				OnStop: func(ctx context.Context) error {
					c.Stop()
					return nil
				},
			})
		}),
	).Run()
}
