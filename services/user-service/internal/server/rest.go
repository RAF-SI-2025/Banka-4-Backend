package server

import (
	"common/pkg/auth"
	"common/pkg/errors"
	"common/pkg/logging"
	"common/pkg/permission"
	"context"
	stderrors "errors"
	"log"
	"net/http"
	"time"

	"user-service/internal/config"
	"user-service/internal/handler"
	"user-service/internal/validator"

	"github.com/gin-gonic/gin"
	"github.com/gin-contrib/cors"
	"go.uber.org/fx"
)

func NewServer(lc fx.Lifecycle, cfg *config.Configuration, healthHandler *handler.HealthHandler, empHandler *handler.EmployeeHandler, verifier auth.TokenVerifier, permissions auth.PermissionProvider) {
	r := gin.New()

	InitRouter(r, cfg)
	SetupRoutes(r, healthHandler, empHandler, verifier, permissions)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	RegisterServerLifecycle(lc, server)
}

func InitRouter(r *gin.Engine, cfg *config.Configuration) {
	r.Use(gin.Recovery())
	r.Use(logging.Logger())
	r.Use(errors.ErrorHandler())

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.URLs.FrontendBaseURL},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge: 12 * time.Hour,
	}))

	// Registrujemo custom validator za password
	validator.RegisterValidators()
}

func SetupRoutes(r *gin.Engine, healthHandler *handler.HealthHandler, empHandler *handler.EmployeeHandler, verifier auth.TokenVerifier, permissions auth.PermissionProvider) {
	api := r.Group("/api")
	{
		api.GET("/health", healthHandler.Health)

		emp := api.Group("/employees")
		{
			emp.POST("/login", empHandler.Login)
			emp.POST("/activate", empHandler.Activate)

			emp.POST("/forgot-password", empHandler.ForgotPassword)
			emp.POST("/reset-password", empHandler.ResetPassword)

			protected := emp.Group("/")
			protected.Use(auth.Middleware(verifier, permissions))
			{
				protected.POST("/register", auth.RequirePermission(permission.EmployeeCreate) ,empHandler.Register)
				protected.PATCH("/:id", auth.RequirePermission(permission.EmployeeUpdate), empHandler.UpdateEmployee)
				protected.GET("/", auth.RequirePermission(permission.EmployeeView), empHandler.ListEmployees)
				protected.POST("/change-password", empHandler.ChangePassword)
			}
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
