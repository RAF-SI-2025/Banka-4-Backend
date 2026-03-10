package server

import (
	"common/pkg/errors"
	"common/pkg/logging"
	"context"
	stderrors "errors"
	"log"
	"net/http"
	"user-service/internal/validator"

	"user-service/internal/config"
	"user-service/internal/handler"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

func NewServer(lc fx.Lifecycle, config *config.Configuration, healthHandler *handler.HealthHandler, empHandler *handler.EmployeeHandler) {
	r := gin.New()

	InitRouter(r)
	SetupRoutes(r, healthHandler, empHandler)

	server := &http.Server{
		Addr:    ":" + config.Port,
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

func SetupRoutes(r *gin.Engine, healthHandler *handler.HealthHandler, empHandler *handler.EmployeeHandler) {
	r.GET("/health", healthHandler.Health)
	r.POST("/register", empHandler.Register)
	r.POST("/activate", empHandler.Activate)
	r.GET("/employees", empHandler.ListEmployees)
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
