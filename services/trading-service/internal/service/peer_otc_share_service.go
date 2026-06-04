package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	appErrors "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
)

type PeerOtcShareService struct {
	shareRepo          repository.PeerOtcShareRepository
	assetOwnershipRepo repository.AssetOwnershipRepository
	stockRepo          repository.StockRepository
	txManager          repository.TransactionManager
}

func NewPeerOtcShareService(
	shareRepo repository.PeerOtcShareRepository,
	assetOwnershipRepo repository.AssetOwnershipRepository,
	stockRepo repository.StockRepository,
	txManager repository.TransactionManager,
) *PeerOtcShareService {
	return &PeerOtcShareService{
		shareRepo:          shareRepo,
		assetOwnershipRepo: assetOwnershipRepo,
		stockRepo:          stockRepo,
		txManager:          txManager,
	}
}

func (s *PeerOtcShareService) Reserve(ctx context.Context, contractID string, sellerID uint, ticker string, amount float64) (string, error) {
	contractID = strings.TrimSpace(contractID)
	ticker = strings.TrimSpace(ticker)
	if contractID == "" || ticker == "" || sellerID == 0 || amount <= 0 {
		return "", appErrors.BadRequestErr("contract id, seller id, ticker and positive amount are required")
	}

	stock, err := s.findStockByTicker(ctx, ticker)
	if err != nil {
		return "", err
	}

	statusValue := string(model.PeerOtcShareReservationActive)
	err = s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		existing, err := s.shareRepo.FindReservationByContractIDForUpdate(ctx, contractID)
		if err != nil {
			return appErrors.InternalErr(err)
		}
		if existing != nil {
			if existing.SellerID != sellerID || existing.StockAssetID != stock.AssetID || existing.ReservedAmount != amount {
				return appErrors.ConflictErr("contract id already has a different share reservation")
			}
			statusValue = string(existing.Status)
			return nil
		}

		ownership, err := s.assetOwnershipRepo.FindByUserAndAssetForUpdate(ctx, sellerID, model.OwnerTypeClient, stock.AssetID)
		if err != nil {
			return appErrors.InternalErr(err)
		}
		if ownership == nil {
			return appErrors.BadRequestErr("seller does not own the requested stock")
		}

		requiredPublic := ownership.ReservedAmount + amount
		if ownership.PublicAmount < requiredPublic || ownership.Amount-ownership.ReservedAmount < amount {
			return appErrors.BadRequestErr("seller does not have enough available public shares")
		}

		now := time.Now()
		reservation := &model.PeerOtcShareReservation{
			ContractID:     contractID,
			SellerID:       sellerID,
			StockAssetID:   stock.AssetID,
			ReservedAmount: amount,
			Status:         model.PeerOtcShareReservationActive,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		if err := s.shareRepo.CreateReservation(ctx, reservation); err != nil {
			return appErrors.InternalErr(err)
		}

		ownership.ReservedAmount += amount
		ownership.UpdatedAt = now
		if err := s.assetOwnershipRepo.Upsert(ctx, ownership); err != nil {
			return appErrors.InternalErr(err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return statusValue, nil
}

func (s *PeerOtcShareService) Release(ctx context.Context, contractID string) (string, error) {
	return s.transitionReservation(ctx, contractID, func(ctx context.Context, reservation *model.PeerOtcShareReservation) error {
		if reservation.Status == model.PeerOtcShareReservationReleased {
			return nil
		}
		if reservation.Status == model.PeerOtcShareReservationConsumed {
			return appErrors.BadRequestErr("cannot release consumed shares")
		}

		ownership, err := s.assetOwnershipRepo.FindByUserAndAssetForUpdate(ctx, reservation.SellerID, model.OwnerTypeClient, reservation.StockAssetID)
		if err != nil {
			return appErrors.InternalErr(err)
		}
		if ownership != nil {
			ownership.ReservedAmount = maxFloat(0, ownership.ReservedAmount-reservation.ReservedAmount)
			ownership.UpdatedAt = time.Now()
			if err := s.assetOwnershipRepo.Upsert(ctx, ownership); err != nil {
				return appErrors.InternalErr(err)
			}
		}
		reservation.Status = model.PeerOtcShareReservationReleased
		reservation.UpdatedAt = time.Now()
		return nil
	})
}

func (s *PeerOtcShareService) Consume(ctx context.Context, contractID string) (string, error) {
	return s.transitionReservation(ctx, contractID, func(ctx context.Context, reservation *model.PeerOtcShareReservation) error {
		if reservation.Status == model.PeerOtcShareReservationConsumed {
			return nil
		}
		if reservation.Status == model.PeerOtcShareReservationReleased {
			return appErrors.BadRequestErr("cannot consume released shares")
		}

		ownership, err := s.assetOwnershipRepo.FindByUserAndAssetForUpdate(ctx, reservation.SellerID, model.OwnerTypeClient, reservation.StockAssetID)
		if err != nil {
			return appErrors.InternalErr(err)
		}
		if ownership == nil {
			return appErrors.BadRequestErr("seller ownership is missing")
		}
		if ownership.Amount < reservation.ReservedAmount || ownership.ReservedAmount < reservation.ReservedAmount {
			return appErrors.BadRequestErr("seller no longer has enough reserved shares")
		}

		ownership.Amount -= reservation.ReservedAmount
		ownership.PublicAmount = maxFloat(0, ownership.PublicAmount-reservation.ReservedAmount)
		ownership.ReservedAmount = maxFloat(0, ownership.ReservedAmount-reservation.ReservedAmount)
		ownership.UpdatedAt = time.Now()
		if err := s.assetOwnershipRepo.Upsert(ctx, ownership); err != nil {
			return appErrors.InternalErr(err)
		}
		reservation.Status = model.PeerOtcShareReservationConsumed
		reservation.UpdatedAt = time.Now()
		return nil
	})
}

func (s *PeerOtcShareService) Credit(ctx context.Context, contractID string, buyerID uint, ticker string, amount, pricePerUnitRSD float64) (string, error) {
	contractID = strings.TrimSpace(contractID)
	ticker = strings.TrimSpace(ticker)
	if contractID == "" || buyerID == 0 || ticker == "" || amount <= 0 {
		return "", appErrors.BadRequestErr("contract id, buyer id, ticker and positive amount are required")
	}

	stock, err := s.findStockByTicker(ctx, ticker)
	if err != nil {
		return "", err
	}

	statusValue := "CREDITED"
	err = s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		existing, err := s.shareRepo.FindCreditByContractID(ctx, contractID)
		if err != nil {
			return appErrors.InternalErr(err)
		}
		if existing != nil {
			if existing.BuyerID != buyerID || existing.StockAssetID != stock.AssetID || existing.Amount != amount {
				return appErrors.ConflictErr("contract id already has a different share credit")
			}
			return nil
		}

		ownership, err := s.assetOwnershipRepo.FindByUserAndAssetForUpdate(ctx, buyerID, model.OwnerTypeClient, stock.AssetID)
		if err != nil {
			return appErrors.InternalErr(err)
		}
		if ownership == nil {
			ownership = &model.AssetOwnership{
				UserId:       buyerID,
				OwnerType:    model.OwnerTypeClient,
				AssetID:      stock.AssetID,
				Amount:       0,
				PublicAmount: 0,
			}
		}

		newAmount := ownership.Amount + amount
		if newAmount > 0 {
			ownership.AvgBuyPriceRSD = (ownership.AvgBuyPriceRSD*ownership.Amount + pricePerUnitRSD*amount) / newAmount
		}
		ownership.Amount = newAmount
		ownership.UpdatedAt = time.Now()
		if err := s.assetOwnershipRepo.Upsert(ctx, ownership); err != nil {
			return appErrors.InternalErr(err)
		}

		if err := s.shareRepo.CreateCredit(ctx, &model.PeerOtcShareCredit{
			ContractID:      contractID,
			BuyerID:         buyerID,
			StockAssetID:    stock.AssetID,
			Amount:          amount,
			PricePerUnitRSD: pricePerUnitRSD,
			CreatedAt:       time.Now(),
		}); err != nil {
			return appErrors.InternalErr(err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return statusValue, nil
}

func (s *PeerOtcShareService) transitionReservation(
	ctx context.Context,
	contractID string,
	fn func(context.Context, *model.PeerOtcShareReservation) error,
) (string, error) {
	contractID = strings.TrimSpace(contractID)
	if contractID == "" {
		return "", appErrors.BadRequestErr("contract id is required")
	}

	var statusValue string
	err := s.txManager.WithinTransaction(ctx, func(ctx context.Context) error {
		reservation, err := s.shareRepo.FindReservationByContractIDForUpdate(ctx, contractID)
		if err != nil {
			return appErrors.InternalErr(err)
		}
		if reservation == nil {
			return appErrors.NotFoundErr("peer OTC share reservation not found")
		}

		if err := fn(ctx, reservation); err != nil {
			return err
		}
		if err := s.shareRepo.SaveReservation(ctx, reservation); err != nil {
			return appErrors.InternalErr(err)
		}
		statusValue = string(reservation.Status)
		return nil
	})
	if err != nil {
		return "", err
	}
	return statusValue, nil
}

func (s *PeerOtcShareService) findStockByTicker(ctx context.Context, ticker string) (*model.Stock, error) {
	stocks, err := s.stockRepo.FindAll(ctx)
	if err != nil {
		return nil, appErrors.InternalErr(err)
	}
	for i := range stocks {
		if strings.EqualFold(stocks[i].Asset.Ticker, ticker) {
			return &stocks[i], nil
		}
	}
	return nil, appErrors.BadRequestErr(fmt.Sprintf("stock %s not found", ticker))
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
