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

	"user-service/internal/config"
	"user-service/internal/handler"
	"user-service/internal/validator"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

func NewServer(lc fx.Lifecycle, cfg *config.Configuration, healthHandler *handler.HealthHandler, empHandler *handler.EmployeeHandler, verifier auth.TokenVerifier, permissions auth.PermissionProvider) {
	r := gin.New()

	InitRouter(r)
	SetupRoutes(r, healthHandler, empHandler, verifier, permissions)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	RegisterServerLifecycle(lc, server)
}

func InitRouter(r *gin.Engine) {
	r.Use(gin.Recovery())
	r.Use(logging.Logger())
	r.Use(errors.ErrorHandler())

	// Registrujemo custom validator za password
	validator.RegisterValidators()
}

func SetupRoutes(r *gin.Engine, healthHandler *handler.HealthHandler, empHandler *handler.EmployeeHandler, verifier auth.TokenVerifier, permissions auth.PermissionProvider) {
	api := r.Group("/api")
	{
		api.GET("/health", healthHandler.Health)

		api.POST("/login", empHandler.Login)
		api.POST("/activate", empHandler.Activate)

		api.POST("/forgot-password", empHandler.ForgotPassword)
		api.POST("/reset-password", empHandler.ResetPassword)

		protected := api.Group("/")
		protected.Use(auth.Middleware(verifier, permissions)) 
		{
			protected.POST("/register", auth.RequirePermission(permission.EmployeeCreate) ,empHandler.Register)
			protected.PATCH("/employees/:id", auth.RequirePermission(permission.EmployeeUpdate), empHandler.UpdateEmployee)
			protected.GET("/employees", auth.RequirePermission(permission.EmployeeView), empHandler.ListEmployees)	
			protected.POST("/change-password", empHandler.ChangePassword)
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
