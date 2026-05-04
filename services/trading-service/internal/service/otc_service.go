package service

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
)

type OTCService struct {
	ownershipRepo repository.AssetOwnershipRepository
	listingRepo   repository.ListingRepository
	userClient    client.UserServiceClient
}

func NewOTCService(
	ownershipRepo repository.AssetOwnershipRepository,
	listingRepo repository.ListingRepository,
	userClient client.UserServiceClient,
) *OTCService {
	return &OTCService{
		ownershipRepo: ownershipRepo,
		listingRepo:   listingRepo,
		userClient:    userClient,
	}
}

func (s *OTCService) PublishAsset(ctx context.Context, ownershipID, userId uint, ownerType model.OwnerType, amount float64) error {
	ownership, err := s.ownershipRepo.FindByID(ctx, ownershipID)
	if err != nil {
		return errors.InternalErr(err)
	}
	if ownership == nil {
		return errors.NotFoundErr("asset ownership not found")
	}
	if ownership.Asset.AssetType != model.AssetTypeStock {
		return errors.BadRequestErr("only stocks can be published for OTC trading")
	}

	if ownership.UserId != userId || ownership.OwnerType != ownerType {
		return errors.ForbiddenErr("you do not own this asset")
	}

	if amount < 0 {
		return errors.BadRequestErr("amount must be non-negative")
	}
	if ownership.PublicAmount+amount > ownership.Amount-ownership.ReservedAmount {
		return errors.BadRequestErr("amount exceeds available (owned minus reserved) stocks")
	}

	if err := s.ownershipRepo.UpdateOTCFields(ctx, ownershipID, ownership.PublicAmount+amount, ownership.ReservedAmount); err != nil {
		return errors.InternalErr(err)
	}
	return nil
}

func (s *OTCService) GetPublicOTCAssets(ctx context.Context, page, pageSize int) ([]dto.OTCAssetResponse, int64, error) {
	ownerships, total, err := s.ownershipRepo.FindAllPublic(ctx, page, pageSize)
	if err != nil {
		return nil, 0, errors.InternalErr(err)
	}

	if len(ownerships) == 0 {
		return []dto.OTCAssetResponse{}, 0, nil
	}

	assetIDs := make([]uint, 0, len(ownerships))
	for _, o := range ownerships {
		assetIDs = append(assetIDs, o.AssetID)
	}

	listings, err := s.listingRepo.FindByAssetIDs(ctx, assetIDs)
	if err != nil {
		return nil, 0, errors.InternalErr(err)
	}

	listingByAssetID := make(map[uint]*model.Listing, len(listings))
	for i := range listings {
		listingByAssetID[listings[i].AssetID] = &listings[i]
	}

	responses := make([]dto.OTCAssetResponse, 0, len(ownerships))
	for _, o := range ownerships {
		resp := dto.OTCAssetResponse{
			AssetOwnershipID: o.AssetOwnershipID,
			Name:             o.Asset.Name,
			Ticker:           o.Asset.Ticker,
			SecurityType:     o.Asset.AssetType,
			AvailableAmount:  o.PublicAmount - o.ReservedAmount,
			UpdatedAt:        o.UpdatedAt,
		}

		if listing, ok := listingByAssetID[o.AssetID]; ok {
			resp.Price = listing.Price
			if listing.Exchange != nil {
				resp.Currency = listing.Exchange.Currency
			}
		}

		var ownerName string

		if o.OwnerType == model.OwnerTypeClient {

			clientResp, clientErr := s.userClient.GetClientById(ctx, uint64(o.UserId))
			if clientResp != nil && clientErr == nil {
				ownerName = clientResp.FullName
			}
		} else {
			empResp, empErr := s.userClient.GetEmployeeById(ctx, uint64(o.UserId))
			if empResp != nil && empErr == nil {
				ownerName = empResp.FullName
			}
		}
		resp.OwnerName = ownerName

		responses = append(responses, resp)
	}

	return responses, total, nil
}
