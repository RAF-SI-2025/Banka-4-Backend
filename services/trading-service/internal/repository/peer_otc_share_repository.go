package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type PeerOtcShareRepository interface {
	CreateReservation(ctx context.Context, reservation *model.PeerOtcShareReservation) error
	FindReservationByContractID(ctx context.Context, contractID string) (*model.PeerOtcShareReservation, error)
	FindReservationByContractIDForUpdate(ctx context.Context, contractID string) (*model.PeerOtcShareReservation, error)
	SaveReservation(ctx context.Context, reservation *model.PeerOtcShareReservation) error
	CreateCredit(ctx context.Context, credit *model.PeerOtcShareCredit) error
	FindCreditByContractID(ctx context.Context, contractID string) (*model.PeerOtcShareCredit, error)
}
