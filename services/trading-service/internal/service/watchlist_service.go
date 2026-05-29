package service

import (
	"context"
	"strings"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	commonErrors "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
)

// WatchlistService implements the watchlist use cases: creating watchlists,
// adding/removing tracked listings and reading a user's watchlists. Every
// operation is scoped to the authenticated owner (a client or an actuary)
// resolved from the request context.
type WatchlistService struct {
	watchlistRepo repository.WatchlistRepository
	listingRepo   repository.ListingRepository
}

func NewWatchlistService(
	watchlistRepo repository.WatchlistRepository,
	listingRepo repository.ListingRepository,
) *WatchlistService {
	return &WatchlistService{
		watchlistRepo: watchlistRepo,
		listingRepo:   listingRepo,
	}
}

// ownerIdentity resolves the (userID, ownerType) pair for the authenticated
// caller. Clients are identified by their ClientID; any employee (actuary,
// supervisor, ...) by their EmployeeID. The OwnerType discriminator only keeps
// the two ID namespaces from colliding — it is not a role check (authorization
// to reach these endpoints is enforced by the Trading permission upstream).
func ownerIdentity(ctx context.Context) (uint, model.OwnerType, error) {
	authCtx := auth.GetAuthFromContext(ctx)
	if authCtx == nil {
		return 0, "", commonErrors.UnauthorizedErr("not authenticated")
	}

	switch authCtx.IdentityType {
	case auth.IdentityClient:
		if authCtx.ClientID == nil {
			return 0, "", commonErrors.UnauthorizedErr("client identity missing")
		}
		return *authCtx.ClientID, model.OwnerTypeClient, nil
	case auth.IdentityEmployee:
		if authCtx.EmployeeID == nil {
			return 0, "", commonErrors.UnauthorizedErr("employee identity missing")
		}
		return *authCtx.EmployeeID, model.OwnerTypeActuary, nil
	default:
		return 0, "", commonErrors.ForbiddenErr("access denied for this identity type")
	}
}

// loadOwnedWatchlist loads a watchlist and verifies it belongs to the caller.
// A watchlist that does not exist or is owned by someone else is reported as
// not found so ownership cannot be probed.
func (s *WatchlistService) loadOwnedWatchlist(ctx context.Context, watchlistID, userID uint, ownerType model.OwnerType) (*model.Watchlist, error) {
	watchlist, err := s.watchlistRepo.FindByID(ctx, watchlistID)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}
	if watchlist == nil || watchlist.UserID != userID || watchlist.OwnerType != ownerType {
		return nil, commonErrors.NotFoundErr("watchlist not found")
	}
	return watchlist, nil
}

// CreateWatchlist creates a new, empty watchlist for the caller. Names are
// unique per owner.
func (s *WatchlistService) CreateWatchlist(ctx context.Context, req dto.CreateWatchlistRequest) (*dto.WatchlistResponse, error) {
	userID, ownerType, err := ownerIdentity(ctx)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, commonErrors.BadRequestErr("watchlist name must not be empty")
	}

	existing, err := s.watchlistRepo.FindByOwnerAndName(ctx, userID, ownerType, name)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}
	if existing != nil {
		return nil, commonErrors.ConflictErr("a watchlist with this name already exists")
	}

	watchlist := &model.Watchlist{
		UserID:    userID,
		OwnerType: ownerType,
		Name:      name,
	}
	if err := s.watchlistRepo.Create(ctx, watchlist); err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	return &dto.WatchlistResponse{
		WatchlistID: watchlist.WatchlistID,
		Name:        watchlist.Name,
		ItemCount:   0,
		CreatedAt:   watchlist.CreatedAt,
	}, nil
}

// GetWatchlists returns every watchlist owned by the caller, with item counts.
func (s *WatchlistService) GetWatchlists(ctx context.Context) ([]dto.WatchlistResponse, error) {
	userID, ownerType, err := ownerIdentity(ctx)
	if err != nil {
		return nil, err
	}

	watchlists, err := s.watchlistRepo.FindByOwner(ctx, userID, ownerType)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	responses := make([]dto.WatchlistResponse, len(watchlists))
	for i, w := range watchlists {
		responses[i] = dto.WatchlistResponse{
			WatchlistID: w.WatchlistID,
			Name:        w.Name,
			ItemCount:   len(w.Items),
			CreatedAt:   w.CreatedAt,
		}
	}
	return responses, nil
}

// GetWatchlistDetail returns a single watchlist with all of its tracked
// listings' market data. When assetType is non-empty the listings are filtered
// by that asset type (stock, option, future, forexPair).
func (s *WatchlistService) GetWatchlistDetail(ctx context.Context, watchlistID uint, assetType string) (*dto.WatchlistDetailResponse, error) {
	userID, ownerType, err := ownerIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := s.loadOwnedWatchlist(ctx, watchlistID, userID, ownerType); err != nil {
		return nil, err
	}

	typeFilter, err := parseAssetTypeFilter(assetType)
	if err != nil {
		return nil, err
	}

	watchlist, err := s.watchlistRepo.FindDetail(ctx, watchlistID, typeFilter)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}
	if watchlist == nil {
		return nil, commonErrors.NotFoundErr("watchlist not found")
	}

	listings := make([]dto.WatchlistListingResponse, 0, len(watchlist.Items))
	for _, item := range watchlist.Items {
		listings = append(listings, toWatchlistListingResponse(item))
	}

	return &dto.WatchlistDetailResponse{
		WatchlistID: watchlist.WatchlistID,
		Name:        watchlist.Name,
		CreatedAt:   watchlist.CreatedAt,
		Listings:    listings,
	}, nil
}

// AddListing adds a listing to one of the caller's watchlists. The listing must
// exist and must not already be tracked in that watchlist.
func (s *WatchlistService) AddListing(ctx context.Context, watchlistID uint, req dto.AddWatchlistItemRequest) error {
	userID, ownerType, err := ownerIdentity(ctx)
	if err != nil {
		return err
	}

	if _, err := s.loadOwnedWatchlist(ctx, watchlistID, userID, ownerType); err != nil {
		return err
	}

	listing, err := s.listingRepo.FindByID(ctx, req.ListingID, -1)
	if err != nil {
		return commonErrors.InternalErr(err)
	}
	if listing == nil {
		return commonErrors.NotFoundErr("listing not found")
	}

	existing, err := s.watchlistRepo.FindItem(ctx, watchlistID, req.ListingID)
	if err != nil {
		return commonErrors.InternalErr(err)
	}
	if existing != nil {
		return commonErrors.ConflictErr("listing is already in this watchlist")
	}

	item := &model.WatchlistItem{
		WatchlistID: watchlistID,
		ListingID:   req.ListingID,
	}
	if err := s.watchlistRepo.AddItem(ctx, item); err != nil {
		return commonErrors.InternalErr(err)
	}
	return nil
}

// RemoveListing removes a listing from one of the caller's watchlists.
func (s *WatchlistService) RemoveListing(ctx context.Context, watchlistID, listingID uint) error {
	userID, ownerType, err := ownerIdentity(ctx)
	if err != nil {
		return err
	}

	if _, err := s.loadOwnedWatchlist(ctx, watchlistID, userID, ownerType); err != nil {
		return err
	}

	removed, err := s.watchlistRepo.RemoveItem(ctx, watchlistID, listingID)
	if err != nil {
		return commonErrors.InternalErr(err)
	}
	if removed == 0 {
		return commonErrors.NotFoundErr("listing not found in this watchlist")
	}
	return nil
}

// DeleteWatchlist deletes one of the caller's watchlists and all of its items.
func (s *WatchlistService) DeleteWatchlist(ctx context.Context, watchlistID uint) error {
	userID, ownerType, err := ownerIdentity(ctx)
	if err != nil {
		return err
	}

	if _, err := s.loadOwnedWatchlist(ctx, watchlistID, userID, ownerType); err != nil {
		return err
	}

	if err := s.watchlistRepo.Delete(ctx, watchlistID); err != nil {
		return commonErrors.InternalErr(err)
	}
	return nil
}

// parseAssetTypeFilter validates an optional asset_type query value.
func parseAssetTypeFilter(raw string) (*model.AssetType, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	switch model.AssetType(raw) {
	case model.AssetTypeStock, model.AssetTypeOption, model.AssetTypeFuture, model.AssetTypeForexPair:
		t := model.AssetType(raw)
		return &t, nil
	default:
		return nil, commonErrors.BadRequestErr("invalid asset_type")
	}
}

// toWatchlistListingResponse maps a watchlist item (with its listing hydrated)
// to the API response, reusing the listing service's baseResponse/latestDaily
// helpers so the market-data shape matches the listing endpoints.
func toWatchlistListingResponse(item model.WatchlistItem) dto.WatchlistListingResponse {
	resp := dto.WatchlistListingResponse{AddedAt: item.CreatedAt}
	if item.Listing == nil {
		return resp
	}

	resp.BaseListingResponse = baseResponse(*item.Listing, latestDaily(item.Listing.DailyPriceInfos))
	if item.Listing.Asset != nil {
		resp.AssetType = string(item.Listing.Asset.AssetType)
	}
	return resp
}
