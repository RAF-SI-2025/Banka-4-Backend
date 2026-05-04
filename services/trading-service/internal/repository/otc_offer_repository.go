package repository

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type OtcOfferRepository interface {
	Create(ctx context.Context, offer *model.OtcOffer) error
	Save(ctx context.Context, offer *model.OtcOffer) error
	FindByID(ctx context.Context, id uint) (*model.OtcOffer, error)

	// FindActiveForUser vraća sve aktivne ponude u kojima učestvuje korisnik
	// (bilo kao kupac, bilo kao prodavac). Koristi se za stranicu "Aktivne ponude".
	FindActiveForUser(ctx context.Context, userID uint) ([]model.OtcOffer, error)

	// FindActiveBySellerAndStock — interna validacija kapaciteta prodavca:
	// koliko akcija je već "rezervisano" u drugim aktivnim pregovorima za isti
	// stock. excludeOfferID se postavlja kada updateujemo postojeću ponudu.
	FindActiveBySellerAndStock(ctx context.Context, sellerID, stockAssetID uint, excludeOfferID *uint) ([]model.OtcOffer, error)
}
