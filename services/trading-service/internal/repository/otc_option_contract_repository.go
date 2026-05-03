package repository

import (
	"context"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type OtcOptionContractRepository interface {
	Create(ctx context.Context, contract *model.OtcOptionContract) error
	Save(ctx context.Context, contract *model.OtcOptionContract) error
	FindByID(ctx context.Context, id uint) (*model.OtcOptionContract, error)

	// FindForUser — sve sklopljene ugovore u kojima je korisnik kupac ili prodavac.
	// Koristi se za stranicu "Sklopljeni ugovori".
	FindForUser(ctx context.Context, userID uint) ([]model.OtcOptionContract, error)

	// FindActiveBySellerAndStock — neiskorišćeni ugovori sa SettlementDate > now,
	// koji još drže prodavčeve akcije rezervisane (validacija kapaciteta po speci).
	FindActiveBySellerAndStock(ctx context.Context, sellerID, stockAssetID uint, now time.Time) ([]model.OtcOptionContract, error)
}
