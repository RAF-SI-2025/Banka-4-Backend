package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/db"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
)

type outboundMessageRepository struct {
	db *gorm.DB
}

func NewOutboundMessageRepository(database *gorm.DB) OutboundMessageRepository {
	return &outboundMessageRepository{db: database}
}

func (r *outboundMessageRepository) Enqueue(ctx context.Context, m *model.OutboundMessage) error {
	if m.NextRetryAt.IsZero() {
		m.NextRetryAt = time.Now()
	}
	if m.Status == "" {
		m.Status = model.OutboundPending
	}

	return db.DBFromContext(ctx, r.db).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "idempotence_key_local"}},
			DoNothing: true,
		}).
		Create(m).Error
}

// outboxLease is how far into the future a claimed row's next_retry_at is
// pushed while a worker processes it. It must comfortably exceed the outbound
// HTTP timeout so a slow send doesn't let another worker re-claim the row.
const outboxLease = 2 * time.Minute

// NextBatch atomically claims up to limit due PENDING rows using
// FOR UPDATE SKIP LOCKED and leases them (bumps next_retry_at into the future)
// so concurrent workers never process the same row twice. A terminal
// MarkSent/MarkFailed or an explicit Reschedule overrides the lease; if a
// worker crashes mid-send, the lease lapses and the row is retried later.
func (r *outboundMessageRepository) NextBatch(ctx context.Context, limit int) ([]model.OutboundMessage, error) {
	var rows []model.OutboundMessage

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND next_retry_at <= ?", model.OutboundPending, time.Now()).
			Order("next_retry_at ASC").
			Limit(limit).
			Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}

		ids := make([]uint, len(rows))
		for i := range rows {
			ids[i] = rows[i].ID
		}
		return tx.Model(&model.OutboundMessage{}).
			Where("id IN ?", ids).
			Update("next_retry_at", time.Now().Add(outboxLease)).Error
	})

	return rows, err
}

func (r *outboundMessageRepository) MarkSent(ctx context.Context, id uint, status int, body []byte) error {
	return db.DBFromContext(ctx, r.db).
		Model(&model.OutboundMessage{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":               model.OutboundSent,
			"last_response_status": status,
			"last_response_body":   body,
			"last_error":           "",
		}).Error
}

func (r *outboundMessageRepository) MarkFailed(ctx context.Context, id uint, lastErr string) error {
	return db.DBFromContext(ctx, r.db).
		Model(&model.OutboundMessage{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":     model.OutboundFailed,
			"last_error": lastErr,
		}).Error
}

func (r *outboundMessageRepository) Reschedule(ctx context.Context, id uint, attempts int, lastErr string, lastStatus int, lastBody []byte, nextRetryAt time.Time) error {
	return db.DBFromContext(ctx, r.db).
		Model(&model.OutboundMessage{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"attempts":             attempts,
			"last_error":           lastErr,
			"last_response_status": lastStatus,
			"last_response_body":   lastBody,
			"next_retry_at":        nextRetryAt,
		}).Error
}

func (r *outboundMessageRepository) Cancel(ctx context.Context, id uint) error {
	return db.DBFromContext(ctx, r.db).
		Model(&model.OutboundMessage{}).
		Where("id = ? AND status = ?", id, model.OutboundPending).
		Update("status", model.OutboundCanceled).Error
}
