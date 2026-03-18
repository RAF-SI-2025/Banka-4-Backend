package server

import (
	"banking-service/internal/config"
	"banking-service/internal/handler"
	"banking-service/internal/validator"
	"common/pkg/auth"
	"common/pkg/errors"
	"common/pkg/logging"
	"context"
	stderrors "errors"
	"log"
	"net/http"
	"time"

	_ "banking-service/docs"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/fx"
)

func NewServer(
	lc fx.Lifecycle,
	cfg *config.Configuration,
	healthHandler *handler.HealthHandler,
	accountHandler *handler.AccountHandler,
	companyHandler *handler.CompanyHandler,
	exchangeHandler *handler.ExchangeHandler,
	paymentHandler *handler.PaymentHandler,
	verifier auth.TokenVerifier,
	permissions auth.PermissionProvider,
) {
	r := gin.New()

	InitRouter(r, cfg)
	SetupRoutes(r, healthHandler, accountHandler, companyHandler, exchangeHandler, paymentHandler, verifier, permissions)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	RegisterServerLifecycle(lc, server)
}

func InitRouter(r *gin.Engine, cfg *config.Configuration) {
	r.Use(gin.Recovery())

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.URLs.FrontendBaseURL},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.Use(logging.Logger())
	r.Use(errors.ErrorHandler())

	validator.RegisterValidators()
}

func SetupRoutes(
	r *gin.Engine,
	healthHandler *handler.HealthHandler,
	accountHandler *handler.AccountHandler,
	companyHandler *handler.CompanyHandler,
	exchangeHandler *handler.ExchangeHandler,
	paymentHandler *handler.PaymentHandler,
	verifier auth.TokenVerifier,
	permissions auth.PermissionProvider,
) {
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := r.Group("/api")
	{
		api.GET("/health", healthHandler.Health)

		accounts := api.Group("/accounts")
		accounts.Use(auth.Middleware(verifier, permissions))
		{
			accounts.POST("", accountHandler.Create)
		}

		companies := api.Group("/companies")
		companies.Use(auth.Middleware(verifier, permissions))
		{
			companies.POST("", companyHandler.Create)
		}

		exchange := api.Group("/exchange")
		{
			exchange.GET("/rates", exchangeHandler.GetRates)
			exchange.GET("/calculate", exchangeHandler.Calculate)
    }
    
		payments := api.Group("/payments")
		payments.Use(auth.Middleware(verifier, permissions))
		{
			payments.POST("", paymentHandler.CreatePayment)
			payments.POST("/:id/verify", paymentHandler.VerifyPayment)
		}
	}
}

func RegisterServerLifecycle(lc fx.Lifecycle, server *http.Server) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := server.ListenAndServe(); err != nil && !stderrors.Is(err, http.ErrServerClosed) {
					log.Fatal(err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return server.Shutdown(ctx)
		},
	})
}
