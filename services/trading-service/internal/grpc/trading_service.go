package grpc

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/repository"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/service"
)

// publicStocksPageSize is the page size used when paging through public
// holdings for §3.1. The full set is collected by repeated calls.
const publicStocksPageSize = 200

type TradingServiceServer struct {
	pb.UnimplementedTradingServiceServer
	investmentFundService *service.InvestmentFundService
	assetOwnershipRepo    repository.AssetOwnershipRepository
}

func NewTradingServiceServer(
	investmentFundService *service.InvestmentFundService,
	assetOwnershipRepo repository.AssetOwnershipRepository,
) *TradingServiceServer {
	return &TradingServiceServer{
		investmentFundService: investmentFundService,
		assetOwnershipRepo:    assetOwnershipRepo,
	}
}

func (s *TradingServiceServer) TransferFunds(ctx context.Context, req *pb.TransferFundsRequest) (*pb.TransferFundsResponse, error) {
	count, err := s.investmentFundService.TransferFunds(ctx, uint(req.FromManagerId), uint(req.ToManagerId))
	if err != nil {
		return nil, err
	}
	return &pb.TransferFundsResponse{FundsTransferred: uint64(count)}, nil
}

// ListPublicStocks aggregates every AssetOwnership with public_amount > 0
// into a per-ticker entry, with one (seller_id, amount) row per owner.
// Pages through the repository to avoid loading the full table in memory
// when the catalogue grows.
func (s *TradingServiceServer) ListPublicStocks(ctx context.Context, _ *pb.ListPublicStocksRequest) (*pb.ListPublicStocksResponse, error) {
	byTicker := make(map[string][]*pb.PublicStockSeller)

	page := 1
	for {
		ownerships, total, err := s.assetOwnershipRepo.FindAllPublic(ctx, page, publicStocksPageSize)
		if err != nil {
			return nil, err
		}

		for i := range ownerships {
			row := &ownerships[i]
			ticker := row.Asset.Ticker
			if ticker == "" {
				continue
			}
			byTicker[ticker] = append(byTicker[ticker], &pb.PublicStockSeller{
				SellerId: uint64(row.UserId),
				Amount:   row.PublicAmount,
			})
		}

		if int64(page*publicStocksPageSize) >= total {
			break
		}
		page++
	}

	stocks := make([]*pb.PublicStockEntry, 0, len(byTicker))
	for ticker, sellers := range byTicker {
		stocks = append(stocks, &pb.PublicStockEntry{
			Ticker:  ticker,
			Sellers: sellers,
		})
	}

	return &pb.ListPublicStocksResponse{Stocks: stocks}, nil
}
