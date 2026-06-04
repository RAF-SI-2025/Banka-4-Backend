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

	// FindByIDForUpdate — same as FindByID but takes a SELECT FOR UPDATE row lock
	// so concurrent SAGA steps cannot race on the same contract.
	FindByIDForUpdate(ctx context.Context, id uint) (*model.OtcOptionContract, error)

	// FindByOfferID — reverse lookup from a negotiation to the contract it produced.
	// Returns nil if the offer was rejected/expired before acceptance.
	FindByOfferID(ctx context.Context, offerID uint) (*model.OtcOptionContract, error)

	// FindForUser — sve sklopljene ugovore u kojima je korisnik kupac ili prodavac.
	// Koristi se za stranicu "Sklopljeni ugovori".
	FindForUser(ctx context.Context, userID uint) ([]model.OtcOptionContract, error)

	// FindActiveBySellerAndStock — neiskorišćeni ugovori sa SettlementDate > now,
	// koji još drže prodavčeve akcije rezervisane (validacija kapaciteta po speci).
	FindActiveBySellerAndStock(ctx context.Context, sellerID, stockAssetID uint, now time.Time) ([]model.OtcOptionContract, error)

	// FindExpiredActive — contracts still in ACTIVE state whose SettlementDate has
	// passed. The maintenance worker uses this to expire stale contracts and
	// release their share reservations.
	FindExpiredActive(ctx context.Context, before time.Time, limit int) ([]model.OtcOptionContract, error)

	// FindExpiringContracts — returns active OTC option contracts whose settlement
	// date falls before the specified time threshold.
	// Used for expiration reminder notifications (e.g. 3-day warning emails).
	// Only contracts with status ACTIVE are included.
	FindExpiringContracts(ctx context.Context, before time.Time) ([]model.OtcOptionContract, error)
}
