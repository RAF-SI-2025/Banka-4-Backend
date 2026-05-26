package main

// @title Interbank Service API
// @version 1.0
// @description Bank-to-bank coordination service implementing the inter-bank
// @description transaction and OTC negotiation protocol. Authentication is
// @description per-peer via the X-Api-Key header (see §2.10).

import (
	"go.uber.org/fx"
	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/logging"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/config"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/handler"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/repository"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/server"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/service"
)

func main() {
	fx.New(
		fx.Provide(
			config.Load,
			func(cfg *config.Configuration) (*config.PeerRegistry, error) {
				return config.LoadPeers(cfg.PeersConfigPath)
			},
			func(cfg *config.Configuration) (*gorm.DB, error) {
				return db.New(cfg.DB.DSN())
			},

			service.NewPeerResolver,

			repository.NewGormTransactionManager,
			repository.NewInboundMessageRepository,
			repository.NewOutboundMessageRepository,
			repository.NewPeerNegotiationRepository,

			service.NewMessageProcessor,
			service.NewPeerOtcService,
			service.NewPeerOtcClient,

			handler.NewHealthHandler,
			handler.NewInterbankHandler,
			handler.NewPeerOtcHandler,
		),

		fx.Invoke(func(cfg *config.Configuration) error {
			return logging.Init(cfg.Env)
		}),
		fx.Invoke(func(db *gorm.DB) error {
			return db.AutoMigrate(
				&model.InboundMessage{},
				&model.OutboundMessage{},
				&model.PeerNegotiation{},
			)
		}),
		fx.Invoke(server.NewServer),
	).Run()
}
