package server

import (
	"context"
	stderrors "errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/fx"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/logging"
	_ "github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/docs"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/config"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/handler"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/middleware"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/service"
)

func NewServer(
	lc fx.Lifecycle,
	cfg *config.Configuration,
	healthHandler *handler.HealthHandler,
	interbankHandler *handler.InterbankHandler,
	peerOtcHandler *handler.PeerOtcHandler,
	peerOtcFrontendHandler *handler.PeerOtcFrontendHandler,
	peers *service.PeerResolver,
	verifier auth.TokenVerifier,
	permissions auth.PermissionProvider,
) {
	r := gin.New()
	initRouter(r, cfg)
	setupRoutes(r, healthHandler, interbankHandler, peerOtcHandler, peerOtcFrontendHandler, peers, verifier, permissions)

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: r}
	registerLifecycle(lc, srv)
}

func initRouter(r *gin.Engine, cfg *config.Configuration) {
	r.Use(gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.URLs.FrontendBaseURL},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "X-Api-Key"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.Use(logging.Logger())
	r.Use(errors.ErrorHandler())
}

func setupRoutes(
	r *gin.Engine,
	healthHandler *handler.HealthHandler,
	interbankHandler *handler.InterbankHandler,
	peerOtcHandler *handler.PeerOtcHandler,
	peerOtcFrontendHandler *handler.PeerOtcFrontendHandler,
	peers *service.PeerResolver,
	verifier auth.TokenVerifier,
	permissions auth.PermissionProvider,
) {
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := r.Group("/api")
	{
		api.GET("/health", healthHandler.Health)

		// /api/peer-otc/* — user-facing cross-bank OTC routes. JWT-authenticated,
		// authorised clients only.
		peerOtc := api.Group("/peer-otc")
		peerOtc.Use(auth.Middleware(verifier, permissions))
		peerOtc.Use(auth.RequireIdentityType(auth.IdentityClient))
		{
			peerOtc.GET("/public-stocks", peerOtcFrontendHandler.ListPublicStocks)

			peerOtcNegotiations := peerOtc.Group("/negotiations")
			{
				peerOtcNegotiations.GET("", peerOtcFrontendHandler.ListMyNegotiations)
				peerOtcNegotiations.POST("", peerOtcFrontendHandler.CreateNegotiation)
				peerOtcNegotiations.PUT("/:rn/:id/counter", peerOtcFrontendHandler.SendCounterOffer)
				peerOtcNegotiations.POST("/:rn/:id/accept", peerOtcFrontendHandler.AcceptNegotiation)
				peerOtcNegotiations.DELETE("/:rn/:id", peerOtcFrontendHandler.Withdraw)
			}

			peerOtc.GET("/contracts", peerOtcFrontendHandler.ListMyContracts)
			peerOtc.POST("/contracts/:rn/:id/exercise", peerOtcFrontendHandler.ExerciseContract)
		}
	}

	// The /interbank endpoint is mounted at the root, not under /api, so its
	// URL matches the spec exactly. Authentication is per-peer via X-Api-Key.
	interbank := r.Group("/interbank")
	interbank.Use(middleware.APIKeyAuth(peers))
	{
		// §2 transaction protocol.
		interbank.POST("", interbankHandler.Receive)

		// §3.1 + §3.7 OTC lookups.
		interbank.GET("/public-stock", peerOtcHandler.PublicStock)
		interbank.GET("/user/:rn/:id", peerOtcHandler.UserLookup)

		// §3.2–§3.6 OTC negotiation lifecycle.
		negotiations := interbank.Group("/negotiations")
		{
			negotiations.POST("", peerOtcHandler.CreateNegotiation)
			negotiations.GET("/:rn/:id", peerOtcHandler.GetNegotiation)
			negotiations.PUT("/:rn/:id", peerOtcHandler.UpdateNegotiation)
			negotiations.DELETE("/:rn/:id", peerOtcHandler.DeleteNegotiation)
			negotiations.GET("/:rn/:id/accept", peerOtcHandler.AcceptNegotiation)
		}

		contracts := interbank.Group("/contracts")
		{
			contracts.POST("/:rn/:id/reserve-shares", peerOtcHandler.ReserveContractShares)
			contracts.POST("/:rn/:id/consume-shares", peerOtcHandler.ConsumeContractShares)
			contracts.POST("/:rn/:id/release-shares", peerOtcHandler.ReleaseContractShares)
		}
	}
}

func registerLifecycle(lc fx.Lifecycle, server *http.Server) {
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
