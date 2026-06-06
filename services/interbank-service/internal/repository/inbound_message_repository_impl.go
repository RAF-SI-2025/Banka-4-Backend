package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
)

type inboundMessageRepository struct {
	db *gorm.DB
}

func NewInboundMessageRepository(database *gorm.DB) InboundMessageRepository {
	return &inboundMessageRepository{db: database}
}

func (r *inboundMessageRepository) FindByKey(ctx context.Context, peerRoutingNumber int, locallyGeneratedKey string) (*model.InboundMessage, error) {
	var m model.InboundMessage

	err := db.DBFromContext(ctx, r.db).
		Where("peer_routing_number = ? AND locally_generated_key = ?", peerRoutingNumber, locallyGeneratedKey).
		First(&m).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &m, nil
}

func (r *inboundMessageRepository) Save(ctx context.Context, m *model.InboundMessage) error {
	return db.DBFromContext(ctx, r.db).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "peer_routing_number"},
				{Name: "locally_generated_key"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"message_type",
				"request_body",
				"response_status",
				"response_body",
				"processed_at",
			}),
		}).
		Create(m).Error
}
