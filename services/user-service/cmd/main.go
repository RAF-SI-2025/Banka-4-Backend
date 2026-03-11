package main

import (
	"common/pkg/auth"
	"common/pkg/db"
	"common/pkg/jwt"
	"common/pkg/logging"
	"user-service/internal/config"
	"user-service/internal/handler"
	"user-service/internal/model"
	"user-service/internal/permission"
	"user-service/internal/repository"
	"user-service/internal/seed"
	"user-service/internal/server"
	"user-service/internal/service"

	"go.uber.org/fx"
	"gorm.io/gorm"
)

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

			repository.NewEmployeeRepository,
			repository.NewActivationTokenRepository,
			repository.NewResetTokenRepository,
			repository.NewPositionRepository,
			service.NewEmployeeService,
			service.NewEmailService,
			handler.NewEmployeeHandler,
			handler.NewHealthHandler,
		),
		fx.Invoke(func(cfg *config.Configuration) error {
			return logging.Init(cfg.Env)
		}),
		fx.Invoke(func(db *gorm.DB) error {
			if err := db.AutoMigrate(&model.Employee{}, &model.Position{}, &model.ActivationToken{}, &model.ResetToken{}, &model.EmployeePermission{}); err != nil {
				return err
			}
			return seed.Run(db)
		}),
		fx.Invoke(server.NewServer),
	).Run()
}
