package service

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"time"

	app_errors "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
)

type OtcNegotiationHistoryService interface {
	CreateNegotiationHistory(
		ctx context.Context,
		offerID uint,
		oldOffer, newOffer *model.OtcOffer,
		modifiedBy uint,
	) error
	GetNegotiationHistory(
		ctx context.Context,
		offerID uint,
		statusFilter string,
		dateFrom, dateTo *time.Time,
		counterpartyFilter uint,
	) ([]*model.OtcNegotiationHistory, error)
}

type otcNegotiationHistoryService struct {
	otcOfferRepo              repository.OtcOfferRepository
	otcNegotiationHistoryRepo repository.OtcNegotiationHistoryRepository
}

func NewOtcNegotiationHistoryService(
	otcOfferRepo repository.OtcOfferRepository,
	otcNegotiationHistoryRepo repository.OtcNegotiationHistoryRepository,
) OtcNegotiationHistoryService {
	return &otcNegotiationHistoryService{
		otcOfferRepo:              otcOfferRepo,
		otcNegotiationHistoryRepo: otcNegotiationHistoryRepo,
	}
}

func (s *otcNegotiationHistoryService) CreateNegotiationHistory(
	ctx context.Context,
	offerID uint,
	oldOffer, newOffer *model.OtcOffer,
	modifiedBy uint,
) error {
	history := &model.OtcNegotiationHistory{
		OtcOfferID:          offerID,
		OldAmount:           oldOffer.Amount,
		NewAmount:           newOffer.Amount,
		OldPricePerStockRSD: oldOffer.PricePerStockRSD,
		NewPricePerStockRSD: newOffer.PricePerStockRSD,
		OldPremiumRSD:       oldOffer.PremiumRSD,
		NewPremiumRSD:       newOffer.PremiumRSD,
		OldSettlementDate:   oldOffer.SettlementDate,
		NewSettlementDate:   newOffer.SettlementDate,
		Timestamp:           time.Now(),
		ModifiedBy:          modifiedBy,
	}
	return s.otcNegotiationHistoryRepo.Create(ctx, history)
}

func (s *otcNegotiationHistoryService) GetNegotiationHistory(
	ctx context.Context,
	offerID uint,
	statusFilter string,
	dateFrom, dateTo *time.Time,
	counterpartyFilter uint,
) ([]*model.OtcNegotiationHistory, error) {
	offer, err := s.otcOfferRepo.FindByID(ctx, offerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, app_errors.NotFoundErr("offer not found")
		}
		return nil, app_errors.InternalErr(err)
	}
	if offer == nil {
		return nil, app_errors.NotFoundErr("offer not found")
	}

	if offer.Status == model.OtcOfferStatusActive {
		return nil, app_errors.BadRequestErr("cannot view history of an active negotiation")
	}

	// Apply filters
	if statusFilter != "" && string(offer.Status) != statusFilter {
		return nil, nil
	}
	if dateFrom != nil && offer.LastModified.Before(*dateFrom) {
		return nil, nil
	}
	if dateTo != nil && offer.LastModified.After(*dateTo) {
		return nil, nil
	}
	if counterpartyFilter != 0 && (offer.BuyerID != counterpartyFilter && offer.SellerID != counterpartyFilter) {
		return nil, nil
	}

	return s.otcNegotiationHistoryRepo.GetByOfferID(ctx, offerID)
}
