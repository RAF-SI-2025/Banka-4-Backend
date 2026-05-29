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

	commonauth "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/logging"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/permission"
	_ "github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/docs"
	userauth "github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/config"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/handler"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/repository"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/user-service/internal/validator"
)

func NewServer(
	lc fx.Lifecycle,
	cfg *config.Configuration,
	healthHandler *handler.HealthHandler,
	authHandler *handler.AuthHandler,
	empHandler *handler.EmployeeHandler,
	actuaryHandler *handler.ActuaryHandler,
	clientHandler *handler.ClientHandler,
	auditLogHandler *handler.AuditLogHandler,
	employeeRepo repository.EmployeeRepository,
	verifier commonauth.TokenVerifier,
	permissions commonauth.PermissionProvider,
) {
	r := gin.New()

	InitRouter(r, cfg)
	SetupRoutes(r, healthHandler, authHandler, empHandler, actuaryHandler, clientHandler, auditLogHandler, employeeRepo, verifier, permissions)

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
	authHandler *handler.AuthHandler,
	empHandler *handler.EmployeeHandler,
	actuaryHandler *handler.ActuaryHandler,
	clientHandler *handler.ClientHandler,
	auditLogHandler *handler.AuditLogHandler,
	employeeRepo repository.EmployeeRepository,
	verifier commonauth.TokenVerifier,
	permissions commonauth.PermissionProvider,
) {
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := r.Group("/api")
	{
		api.GET("/health", healthHandler.Health)

		authGroup := api.Group("/auth")
		{
			authGroup.POST("/login", authHandler.Login)
			authGroup.POST("/activate", authHandler.Activate)
			authGroup.POST("/resend-activation", authHandler.ResendActivation)
			authGroup.POST("/forgot-password", authHandler.ForgotPassword)
			authGroup.POST("/reset-password", authHandler.ResetPassword)
			authGroup.POST("/refresh", authHandler.RefreshToken)
		}

		authProtected := api.Group("/auth")
		authProtected.Use(commonauth.Middleware(verifier, permissions))
		{
			authProtected.POST("/change-password", authHandler.ChangePassword)
		}

		emp := api.Group("/employees")
		emp.Use(commonauth.Middleware(verifier, permissions))
		{
			emp.POST("/register", commonauth.RequirePermission(permission.EmployeeCreate), empHandler.Register)
			emp.GET("/:id", commonauth.RequirePermission(permission.EmployeeView), empHandler.GetEmployee)
			emp.PATCH("/:id", commonauth.RequirePermission(permission.EmployeeUpdate), empHandler.UpdateEmployee)
			emp.GET("", commonauth.RequirePermission(permission.EmployeeView), empHandler.ListEmployees)
			emp.POST("/:id/deactivate", commonauth.RequirePermission(permission.EmployeeUpdate), empHandler.DeactivateEmployee)
		}

		act := api.Group("/actuaries")
		act.Use(commonauth.Middleware(verifier, permissions))
		{
			act.GET("", commonauth.RequirePermission(permission.EmployeeView), actuaryHandler.ListActuaries)
			act.PATCH("/:id", userauth.RequireSupervisor(employeeRepo), actuaryHandler.UpdateActuarySettings)
			act.POST("/:id/reset-used-limit", userauth.RequireSupervisor(employeeRepo), actuaryHandler.ResetUsedLimit)
		}

		cli := api.Group("/clients")
		cli.Use(commonauth.Middleware(verifier, permissions))
		{
			cli.GET("", commonauth.RequirePermission(permission.ClientView), clientHandler.ListClients)
			cli.POST("/register", clientHandler.Register)
			cli.PATCH("/:id", commonauth.RequirePermission(permission.ClientUpdate), clientHandler.UpdateClient)
		}

		mobileSecret := api.Group("")
		mobileSecret.Use(commonauth.Middleware(verifier, permissions), commonauth.RequireIdentityType(commonauth.IdentityClient))
		{
			mobileSecret.GET("/secret-mobile", clientHandler.GetMobileSecret)
		}

		auditLog := api.Group("/audit-log")
		auditLog.Use(commonauth.Middleware(verifier, permissions))
		{
			auditLog.GET("", userauth.RequireSupervisor(employeeRepo), auditLogHandler.ListAuditLogs)
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
