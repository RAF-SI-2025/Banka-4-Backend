package server

import (
	"context"
	stderrors "errors"
	"log"
	"net/http"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/handler"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/config"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/faultinject"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/middleware"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/validator"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/fx"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/logging"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/permission"
	_ "github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/docs"
)

func NewServer(
	lc fx.Lifecycle,
	cfg *config.Configuration,
	healthHandler *handler.HealthHandler,
	taxHandler *handler.TaxHandler,
	exchangeHandler *handler.ExchangeHandler,
	orderHandler *handler.OrderHandler,
	portfolioHandler *handler.PortfolioHandler,
	listingHandler *handler.ListingHandler,
	otcHandler *handler.OTCHandler,
	otcOfferHandler *handler.OtcOfferHandler,
	fundHandler *handler.InvestmentFundHandler,
	watchlistHandler *handler.WatchlistHandler,
	recurringOrderHandler *handler.RecurringOrderHandler,
	dividendHandler *handler.DividendHandler,
	priceAlertHandler *handler.PriceAlertHandler,
	verifier auth.TokenVerifier,
	permProvider auth.PermissionProvider,
	userClient client.UserServiceClient,
	otcNegotiationHistoryHandler *handler.OtcNegotiationHistoryHandler,
) {

	r := gin.New()

	InitRouter(r, cfg)

	SetupRoutes(r, healthHandler, taxHandler, exchangeHandler, orderHandler, portfolioHandler, listingHandler, otcHandler, otcOfferHandler, fundHandler, watchlistHandler, otcNegotiationHistoryHandler, recurringOrderHandler, dividendHandler, priceAlertHandler, verifier, permProvider, userClient)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	RegisterServerLifecycle(lc, server)
}

func InitRouter(r *gin.Engine, cfg *config.Configuration) {
	r.Use(gin.Recovery())

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.URLs.FrontendBaseURL, "https://banka-4-frontend.vercel.app"},
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
	taxHandler *handler.TaxHandler,
	exchangeHandler *handler.ExchangeHandler,
	orderHandler *handler.OrderHandler,
	portfolioHandler *handler.PortfolioHandler,
	listingHandler *handler.ListingHandler,
	otcHandler *handler.OTCHandler,
	otcOfferHandler *handler.OtcOfferHandler,
	fundHandler *handler.InvestmentFundHandler,
	watchlistHandler *handler.WatchlistHandler,
	otcNegotiationHistoryHandler *handler.OtcNegotiationHistoryHandler,
	recurringOrderHandler *handler.RecurringOrderHandler,
	dividendHandler *handler.DividendHandler,
	priceAlertHandler *handler.PriceAlertHandler,
	verifier auth.TokenVerifier,
	permProvider auth.PermissionProvider,
	userClient client.UserServiceClient,
) {
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := r.Group("/api")
	{
		authMw := auth.Middleware(verifier, permProvider)

		api.GET("/health", healthHandler.Health)

		exchanges := api.Group("/exchanges")
		{
			exchanges.GET("", exchangeHandler.GetAll)
			exchanges.PATCH("/:micCode/toggle", exchangeHandler.ToggleTradingEnabled)
		}

		listings := api.Group("/listings")
		listings.Use(authMw, auth.RequirePermission(permission.Trading))
		{
			// Stocks
			stocks := listings.Group("/stocks")
			stocks.Use(auth.AnyOf(middleware.RequireSupervisor(userClient), middleware.RequireAgent(userClient), auth.RequireIdentityType(auth.IdentityClient)))
			{
				stocks.GET("", listingHandler.GetStocks)
				stocks.GET("/:listingId", listingHandler.GetStockDetails)
			}

			// Futures
			futures := listings.Group("/futures")
			futures.Use(auth.AnyOf(middleware.RequireSupervisor(userClient), middleware.RequireAgent(userClient), auth.RequireIdentityType(auth.IdentityClient)))
			{
				futures.GET("", listingHandler.GetFutures)
				futures.GET("/:listingId", listingHandler.GetFutureDetails)
			}

			// Forex
			forex := listings.Group("/forex")
			forex.Use(auth.AnyOf(middleware.RequireSupervisor(userClient), middleware.RequireAgent(userClient)))
			{
				forex.GET("", listingHandler.GetForex)
				forex.GET("/:listingId", listingHandler.GetForexDetails)
			}

			// Options
			options := listings.Group("/options")
			options.Use(auth.AnyOf(middleware.RequireSupervisor(userClient), middleware.RequireAgent(userClient)))
			{
				options.GET("", listingHandler.GetOptions)
				options.GET("/:listingId", listingHandler.GetOptionDetails)
			}
		}

		// Watchlists — personalne liste praćenih hartija. Dostupne svim korisnicima
		// sa Trading permisijom (klijent, aktuar, supervizor). Svaki korisnik vidi i
		// menja samo svoje liste — vlasništvo se određuje preko (UserID, OwnerType)
		// iz tokena.
		watchlists := api.Group("/watchlists")
		watchlists.Use(
			authMw,
			auth.RequirePermission(permission.Trading),
		)
		{
			watchlists.GET("", watchlistHandler.GetWatchlists)
			watchlists.POST("", watchlistHandler.CreateWatchlist)
			watchlists.GET("/:watchlistId", watchlistHandler.GetWatchlistDetail)
			watchlists.DELETE("/:watchlistId", watchlistHandler.DeleteWatchlist)
			watchlists.POST("/:watchlistId/items", watchlistHandler.AddListing)
			watchlists.DELETE("/:watchlistId/items/:listingId", watchlistHandler.RemoveListing)
		}

		// Price alerts — personalne threshold notifikacije za listings. Dostupne
		// svim korisnicima sa Trading permisijom (klijent, aktuar, supervizor).
		// Vlasništvo se određuje preko (UserID, OwnerType) iz tokena, isto kao
		// kod watchlists.
		priceAlerts := api.Group("/price-alerts")
		priceAlerts.Use(
			authMw,
			auth.RequirePermission(permission.Trading),
		)
		{
			priceAlerts.GET("", priceAlertHandler.GetMyPriceAlerts)
			priceAlerts.POST("", priceAlertHandler.CreatePriceAlert)
			priceAlerts.DELETE("/:priceAlertId", priceAlertHandler.DeletePriceAlert)
		}

		funds := api.Group("/investment-funds")
		funds.Use(authMw, auth.RequirePermission(permission.Trading))
		{
			// Supervisori, agenti i klijenti mogu da vide sve fondove
			funds.GET("",
				auth.AnyOf(
					middleware.RequireSupervisor(userClient),
					middleware.RequireAgent(userClient),
					auth.RequireIdentityType(auth.IdentityClient),
				),
				fundHandler.GetAllFunds,
			)
			// Samo supervisor može da kreira fond
			funds.POST("",
				auth.RequireIdentityType(auth.IdentityEmployee),
				middleware.RequireSupervisor(userClient),
				fundHandler.CreateFund,
			)
			// Klijenti i supervizori mogu da investiraju
			funds.POST("/:fundId/invest",
				auth.AnyOf(
					auth.RequireIdentityType(auth.IdentityClient),
					middleware.RequireSupervisor(userClient),
				),
				fundHandler.InvestInFund,
			)
			funds.POST("/:fundId/withdraw",
				auth.AnyOf(
					auth.RequireIdentityType(auth.IdentityClient),
					middleware.RequireSupervisor(userClient),
				),
				fundHandler.WithdrawFromFund,
			)
			funds.GET("/:fundId", fundHandler.GetFundDetail)
			// Samo supervisor može da obriše fond
			funds.DELETE("/:fundId",
				auth.RequireIdentityType(auth.IdentityEmployee),
				middleware.RequireSupervisor(userClient),
				fundHandler.DeleteFund,
			)
		}

		client := api.Group("/client")
		client.Use(authMw, auth.RequirePermission(permission.Trading), auth.RequireClientSelf("clientId", true))
		{
			client.GET("/:clientId/assets", portfolioHandler.GetClientPortfolio)
			client.GET("/:clientId/assets/profit", portfolioHandler.GetClientPortfolioProfit)
			client.GET("/:clientId/accumulated-tax", taxHandler.GetClientAccumulatedTax)
			client.PATCH("/:clientId/assets/:ownershipId/publish", otcHandler.PublishAssetClient)
			client.GET("/:clientId/funds", fundHandler.GetClientFundPositions)
			client.GET("/:clientId/assets/:assetOwnershipId/dividends", dividendHandler.GetClientDividendPayoutsForAssetOwnership)
		}

		actuary := api.Group("/actuary")
		actuary.Use(authMw, auth.RequirePermission(permission.Trading), auth.RequireIdentityType(auth.IdentityEmployee))
		{
			actuary.GET("/:actId/assets", portfolioHandler.GetActuaryPortfolio)
			actuary.GET("/:actId/assets/profit", portfolioHandler.GetActuaryPortfolioProfit)
			actuary.GET("/:actId/assets/funds", fundHandler.GetActuaryFunds)
			actuary.GET("/:actId/accumulated-tax", taxHandler.GetActuaryAccumulatedTax)
			actuary.POST("/:actId/options/:assetId/exercise", portfolioHandler.ExerciseOption)
			actuary.PATCH("/:actId/assets/:ownershipId/publish", otcHandler.PublishAssetActuary)
			actuary.GET("/:actId/assets/:assetOwnershipId/dividends", dividendHandler.GetActuaryDividendPayoutsForAssetOwnership)
		}

		otc := api.Group("/otc")
		otc.Use(auth.Middleware(verifier, permProvider))
		{
			otc.GET("/public", otcHandler.GetPublicOTCAssets)

			// Stranica: Aktivne ponude — pregovori u kojima učestvuje ulogovani korisnik.
			otc.GET("/offers/active", otcOfferHandler.GetMyActiveOffers)

			// Stranica: Sklopljeni ugovori — opcioni ugovori (CALL) sklopljeni iz prihvaćenih ponuda.
			otc.GET("/contracts", otcOfferHandler.GetMyOptionContracts)

			// Exercise postojećeg OTC contract-a — buyer pokreće settlement SAGA.
			// X-Saga-* adversarni headeri se parsiraju samo u test buildovima
			// (SAGA_FAULT_INJECTION); u ostalim slučajevima zahtev sa tim
			// headerima biva odbijen.
			otc.POST("/contracts/:id/exercise", faultinject.Middleware(), otcOfferHandler.ExerciseContract)

			// Stanje settlement SAGA-e sa step-by-step logom (buyer ili seller).
			otc.GET("/executions/:id", otcOfferHandler.GetExecution)

			// Kreiranje nove ponude — radi je kupac (klijent sa permisijom za trgovinu).
			otc.POST("/offers", otcOfferHandler.CreateOffer)

			// Kontraponuda — bilo koja strana ažurira parametre. PUT jer je update, ne insert.
			otc.PUT("/offers/:id/counter", otcOfferHandler.SendCounterOffer)

			// Prihvatanje — strana suprotna od ModifiedBy (kreira opcioni ugovor + premium transfer).
			otc.PATCH("/offers/:id/accept", otcOfferHandler.AcceptOffer)

			// Odustajanje — bilo koja strana može odustati od pregovora.
			otc.PATCH("/offers/:id/reject", otcOfferHandler.RejectOffer)

			otc.GET("/offers/:id/history", otcNegotiationHistoryHandler.GetNegotiationHistory)
		}

		orders := api.Group("/orders")
		orders.Use(authMw, auth.RequirePermission(permission.Trading))
		{
			orders.GET("", middleware.RequireSupervisor(userClient), orderHandler.GetOrders)
			orders.POST("", orderHandler.CreateOrder)
			orders.POST("/invest", middleware.RequireSupervisor(userClient), orderHandler.CreateFundOrder)
			orders.PATCH("/:id/approve", middleware.RequireSupervisor(userClient), orderHandler.ApproveOrder)
			orders.PATCH("/:id/decline", middleware.RequireSupervisor(userClient), orderHandler.DeclineOrder)
			orders.PATCH("/:id/cancel", orderHandler.CancelOrder)
			orders.GET("/my", orderHandler.GetMyOrders)
		}

		tax := api.Group("/tax")
		tax.Use(authMw, auth.RequirePermission(permission.Trading))
		{
			tax.GET("", middleware.RequireSupervisor(userClient), taxHandler.ListTaxUsers)
			tax.POST("/collect", middleware.RequireSupervisor(userClient), taxHandler.CollectTaxes)
		}

		dividends := api.Group("/dividends")
		dividends.Use(authMw, auth.RequirePermission(permission.Trading))
		{
			dividends.GET("", middleware.RequireSupervisor(userClient), dividendHandler.GetAllDividendPayouts)
			// DEV ONLY - manual trigger
			dividends.POST("/process", middleware.RequireSupervisor(userClient), dividendHandler.TriggerDividends)
		}

		recurringOrders := api.Group("/recurring-orders")
		recurringOrders.Use(authMw, auth.RequirePermission(permission.Trading))
		{
			recurringOrders.GET("", recurringOrderHandler.GetMyRecurringOrders)
			recurringOrders.POST("", recurringOrderHandler.CreateRecurringOrder)
			recurringOrders.DELETE("/:id", recurringOrderHandler.DeleteRecurringOrder)
			recurringOrders.PATCH("/:id/pause", recurringOrderHandler.PauseRecurringOrder)
		}

		profit := api.Group("/profit")
		profit.Use(authMw, auth.RequirePermission(permission.Trading))
		{
			profit.GET("/actuaries", middleware.RequireSupervisor(userClient), portfolioHandler.GetAllActuaryProfits)
			profit.GET("/funds", middleware.RequireSupervisor(userClient), fundHandler.GetBankFundPositions)
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
