package service

import (
	"context"

	commonErrors "github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/errors"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
)

type ListingService struct {
	listingRepo repository.ListingRepository
	futuresRepo repository.FuturesContractRepository
	forexRepo   repository.ForexRepository
	optionRepo  repository.OptionRepository
}

func NewListingService(
	listingRepo repository.ListingRepository,
	futuresRepo repository.FuturesContractRepository,
	forexRepo repository.ForexRepository,
	optionRepo repository.OptionRepository,
) *ListingService {
	return &ListingService{
		listingRepo: listingRepo,
		futuresRepo: futuresRepo,
		forexRepo:   forexRepo,
		optionRepo:  optionRepo,
	}
}

// --- Helpers ---

func latestDaily(infos []model.ListingDailyPriceInfo) *model.ListingDailyPriceInfo {
	if len(infos) == 0 {
		return nil
	}

	latest := infos[0]
	for _, info := range infos[1:] {
		if info.Date.After(latest.Date) {
			latest = info
		}
	}
	return &latest
}

func baseResponse(l model.Listing, daily *model.ListingDailyPriceInfo) dto.BaseListingResponse {
	var ticker, name string
	if l.Asset != nil {
		ticker = l.Asset.Ticker
		name = l.Asset.Name
	}

	r := dto.BaseListingResponse{
		ListingID:         l.ListingID,
		Ticker:            ticker,
		Name:              name,
		Exchange:          l.ExchangeMIC,
		Price:             l.Price,
		Ask:               l.Ask,
		MaintenanceMargin: l.MaintenanceMargin,
		InitialMarginCost: l.MaintenanceMargin * 1.1,
	}

	if l.Exchange != nil {
		r.Currency = l.Exchange.Currency
	}

	if daily != nil {
		r.Bid = daily.Bid
		r.Change = daily.Change
		r.Volume = daily.Volume
	}
	return r
}

func toFilter(q dto.ListingQuery) (repository.ListingFilter, error) {
	f := repository.ListingFilter{
		Search:    q.Search,
		Exchange:  q.Exchange,
		PriceMin:  q.PriceMin,
		PriceMax:  q.PriceMax,
		AskMin:    q.AskMin,
		AskMax:    q.AskMax,
		BidMin:    q.BidMin,
		BidMax:    q.BidMax,
		VolumeMin: q.VolumeMin,
		VolumeMax: q.VolumeMax,
		SortBy:    q.SortBy,
		SortDir:   q.SortDir,
		Page:      q.Page,
		PageSize:  q.PageSize,
	}
	sd, err := q.ParseSettlementDate()
	if err != nil {
		return f, err
	}
	f.SettlementDate = sd
	return f, nil
}

func mapHistory(infos []model.ListingDailyPriceInfo) []dto.DailyPriceResponse {
	history := make([]dto.DailyPriceResponse, len(infos))
	for i, h := range infos {
		history[i] = dto.DailyPriceResponse{
			Date: h.Date, Price: h.Price, Ask: h.Ask, Bid: h.Bid, Change: h.Change, Volume: h.Volume,
		}
	}
	return history
}

// --- Stocks ---

func (s *ListingService) GetStocks(ctx context.Context, q dto.ListingQuery) (*dto.PaginatedStockResponse, error) {
	filter, err := toFilter(q)
	if err != nil {
		return nil, commonErrors.BadRequestErr("invalid settlement_date format")
	}

	listings, total, err := s.listingRepo.FindStocks(ctx, filter)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	data := make([]dto.StockResponse, len(listings))
	for i, l := range listings {
		daily := latestDaily(l.DailyPriceInfos)
		var outstandingShares, dividendYield float64
		if l.Stock != nil {
			outstandingShares = l.Stock.OutstandingShares
			dividendYield = l.Stock.DividendYield
		}
		data[i] = dto.StockResponse{
			BaseListingResponse: baseResponse(l, daily),
			OutstandingShares:   outstandingShares,
			DividendYield:       dividendYield,
		}
	}

	return &dto.PaginatedStockResponse{
		Data:     data,
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
	}, nil
}

func (s *ListingService) GetStockDetails(ctx context.Context, listingID uint, daysBack int) (*dto.StockDetailedResponse, error) {
	l, err := s.listingRepo.FindByID(ctx, listingID, daysBack)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}
	if l == nil || l.Asset == nil || l.Asset.AssetType != model.AssetTypeStock || l.Stock == nil {
		return nil, commonErrors.NotFoundErr("stock not found")
	}

	daily := latestDaily(l.DailyPriceInfos)
	stockResp := dto.StockResponse{
		BaseListingResponse: baseResponse(*l, daily),
		OutstandingShares:   l.Stock.OutstandingShares,
		DividendYield:       l.Stock.DividendYield,
	}

	history := mapHistory(l.DailyPriceInfos)

	options, err := s.optionRepo.FindByStockID(ctx, l.Stock.StockID)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	optionResponses := make([]dto.OptionResponse, len(options))
	for i, o := range options {
		var optListing model.Listing
		if o.Listing != nil {
			optListing = *o.Listing
		}
		optionResponses[i] = dto.OptionResponse{
			BaseListingResponse: baseResponse(optListing, nil),
			Strike:              o.StrikePrice,
			OptionType:          string(o.OptionType),
			SettlementDate:      o.SettlementDate,
			ImpliedVolatility:   o.ImpliedVolatility,
			OpenInterest:        o.OpenInterest,
		}
	}

	return &dto.StockDetailedResponse{
		StockResponse: stockResp,
		History:       history,
		Options:       optionResponses,
	}, nil
}

// --- Futures ---

func (s *ListingService) GetFutures(ctx context.Context, q dto.ListingQuery) (*dto.PaginatedFuturesResponse, error) {
	filter, err := toFilter(q)
	if err != nil {
		return nil, commonErrors.BadRequestErr("invalid settlement_date format")
	}

	listings, total, err := s.listingRepo.FindFutures(ctx, filter)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	assetIDs := make([]uint, len(listings))
	for i, l := range listings {
		assetIDs[i] = l.AssetID
	}

	contracts, err := s.futuresRepo.FindByAssetIDs(ctx, assetIDs)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	contractMap := make(map[uint]model.FuturesContract)
	for _, fc := range contracts {
		contractMap[fc.AssetID] = fc
	}

	data := make([]dto.FuturesResponse, len(listings))
	for i, l := range listings {
		daily := latestDaily(l.DailyPriceInfos)
		fc := contractMap[l.AssetID]
		data[i] = dto.FuturesResponse{
			BaseListingResponse: baseResponse(l, daily),
			SettlementDate:      fc.SettlementDate,
			ContractSize:        fc.ContractSize,
			ContractUnit:        fc.ContractUnit,
		}
	}

	return &dto.PaginatedFuturesResponse{
		Data:     data,
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
	}, nil
}

func (s *ListingService) GetFutureDetails(ctx context.Context, listingID uint, daysBack int) (*dto.FutureDetailedResponse, error) {
	l, err := s.listingRepo.FindByID(ctx, listingID, daysBack)
	if err != nil || l == nil || l.Asset == nil || l.Asset.AssetType != model.AssetTypeFuture {
		return nil, commonErrors.NotFoundErr("future not found")
	}

	fc, err := s.futuresRepo.FindByAssetIDs(ctx, []uint{l.AssetID})
	if err != nil || len(fc) == 0 {
		return nil, commonErrors.NotFoundErr("contract details not found")
	}

	return &dto.FutureDetailedResponse{
		FuturesResponse: dto.FuturesResponse{
			BaseListingResponse: baseResponse(*l, latestDaily(l.DailyPriceInfos)),
			SettlementDate:      fc[0].SettlementDate,
			ContractSize:        fc[0].ContractSize,
			ContractUnit:        fc[0].ContractUnit,
		},
		History: mapHistory(l.DailyPriceInfos),
	}, nil
}

// --- Forex ---

func (s *ListingService) GetForex(ctx context.Context, q dto.ListingQuery) (*dto.PaginatedForexResponse, error) {
	filter, err := toFilter(q)
	if err != nil {
		return nil, commonErrors.BadRequestErr("invalid settlement_date format")
	}

	pairs, total, err := s.forexRepo.FindAll(ctx, filter)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	data := make([]dto.ForexResponse, len(pairs))
	for i, p := range pairs {
		var listing model.Listing
		if p.Listing != nil {
			listing = *p.Listing
		}
		daily := latestDaily(listing.DailyPriceInfos)
		base := baseResponse(listing, daily)
		base.Ticker = p.Base + "/" + p.Quote
		base.Name = p.Base + "/" + p.Quote
		base.Exchange = listing.ExchangeMIC
		base.Price = p.Rate

		data[i] = dto.ForexResponse{
			BaseListingResponse:  base,
			Base:                 p.Base,
			Quote:                p.Quote,
			ProviderUpdatedAt:    p.ProviderUpdatedAt,
			ProviderNextUpdateAt: p.ProviderNextUpdateAt,
		}
	}

	return &dto.PaginatedForexResponse{
		Data:     data,
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
	}, nil
}

func (s *ListingService) GetForexDetails(ctx context.Context, listingID uint, daysBack int) (*dto.ForexDetailedResponse, error) {
	l, err := s.listingRepo.FindByID(ctx, listingID, daysBack)
	if err != nil || l == nil || l.Asset == nil || l.Asset.AssetType != model.AssetTypeForexPair {
		return nil, commonErrors.NotFoundErr("forex not found")
	}

	pairs, err := s.forexRepo.FindByAssetIDs(ctx, []uint{l.AssetID})
	if err != nil || len(pairs) == 0 {
		return nil, commonErrors.NotFoundErr("forex pair details not found")
	}
	p := pairs[0]

	return &dto.ForexDetailedResponse{
		ForexResponse: dto.ForexResponse{
			BaseListingResponse:  baseResponse(*l, latestDaily(l.DailyPriceInfos)),
			Base:                 p.Base,
			Quote:                p.Quote,
			ProviderUpdatedAt:    p.ProviderUpdatedAt,
			ProviderNextUpdateAt: p.ProviderNextUpdateAt,
		},
		History: mapHistory(l.DailyPriceInfos),
	}, nil
}

func (s *ListingService) GetOptions(ctx context.Context, q dto.ListingQuery) (*dto.PaginatedOptionResponse, error) {
	filter, err := toFilter(q)
	if err != nil {
		return nil, commonErrors.BadRequestErr("invalid settlement_date format")
	}

	listings, total, err := s.listingRepo.FindOptions(ctx, filter)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	assetIDs := make([]uint, len(listings))
	for i, l := range listings {
		assetIDs[i] = l.AssetID
	}

	options, err := s.optionRepo.FindByAssetIDs(ctx, assetIDs)
	if err != nil {
		return nil, commonErrors.InternalErr(err)
	}

	optionMap := make(map[uint]model.Option)
	for _, o := range options {
		optionMap[o.AssetID] = o
	}

	data := make([]dto.OptionResponse, len(listings))
	for i, l := range listings {
		daily := latestDaily(l.DailyPriceInfos)
		o := optionMap[l.AssetID]
		data[i] = dto.OptionResponse{
			BaseListingResponse: baseResponse(l, daily),
			Strike:              o.StrikePrice,
			OptionType:          string(o.OptionType),
			SettlementDate:      o.SettlementDate,
			ImpliedVolatility:   o.ImpliedVolatility,
			OpenInterest:        o.OpenInterest,
		}
	}

	result := &dto.PaginatedOptionResponse{
		Data:     data,
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
	}
	return result, nil
}

func (s *ListingService) GetOptionDetails(ctx context.Context, listingID uint, daysBack int) (*dto.OptionDetailedResponse, error) {
	l, err := s.listingRepo.FindByID(ctx, listingID, daysBack)
	if err != nil || l == nil || l.Asset == nil || l.Asset.AssetType != model.AssetTypeOption {
		return nil, commonErrors.NotFoundErr("option not found")
	}

	opt, err := s.optionRepo.FindByAssetIDs(ctx, []uint{l.AssetID})
	if err != nil || len(opt) == 0 {
		return nil, commonErrors.NotFoundErr("option details not found")
	}

	return &dto.OptionDetailedResponse{
		OptionResponse: dto.OptionResponse{
			BaseListingResponse: baseResponse(*l, latestDaily(l.DailyPriceInfos)),
			Strike:              opt[0].StrikePrice,
			OptionType:          string(opt[0].OptionType),
			SettlementDate:      opt[0].SettlementDate,
			ImpliedVolatility:   opt[0].ImpliedVolatility,
			OpenInterest:        opt[0].OpenInterest,
		},
		History: mapHistory(l.DailyPriceInfos),
	}, nil
}
