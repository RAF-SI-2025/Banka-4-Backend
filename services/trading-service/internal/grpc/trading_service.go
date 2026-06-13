package grpc

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/client"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
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
	peerOtcShareService   *service.PeerOtcShareService
	userClient            client.UserServiceClient
}

func NewTradingServiceServer(
	investmentFundService *service.InvestmentFundService,
	assetOwnershipRepo repository.AssetOwnershipRepository,
	peerOtcShareService *service.PeerOtcShareService,
	userClient client.UserServiceClient,
) *TradingServiceServer {
	return &TradingServiceServer{
		investmentFundService: investmentFundService,
		assetOwnershipRepo:    assetOwnershipRepo,
		peerOtcShareService:   peerOtcShareService,
		userClient:            userClient,
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
// seller_id is the owner's global Identity.ID — resolved from the role-scoped
// UserId via user-service — so it is unique across owner types and unambiguous
// to peer banks on the wire. Pages through the repository to avoid loading the
// full table in memory when the catalogue grows.
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

			userType := "CLIENT"
			if row.OwnerType == model.OwnerTypeBank {
				userType = "ACTUARY"
			}

			resp, err := s.userClient.GetIdentityByUserId(ctx, uint64(row.UserId), userType)
			if err != nil {
				return nil, err
			}

			available := row.PublicAmount - row.ReservedAmount
			if available <= 0 {
				continue
			}
			byTicker[ticker] = append(byTicker[ticker], &pb.PublicStockSeller{
				SellerId: resp.GetIdentityId(),
				Amount:   available,
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

func (s *TradingServiceServer) ReservePeerOtcShares(ctx context.Context, req *pb.ReservePeerOtcSharesRequest) (*pb.PeerOtcSharesResponse, error) {
	statusValue, err := s.peerOtcShareService.Reserve(ctx, req.GetContractId(), uint(req.GetSellerId()), req.GetTicker(), req.GetAmount(), req.GetUserType())
	if err != nil {
		return nil, err
	}
	return &pb.PeerOtcSharesResponse{ContractId: req.GetContractId(), Status: statusValue}, nil
}

func (s *TradingServiceServer) ReleasePeerOtcShares(ctx context.Context, req *pb.ReleasePeerOtcSharesRequest) (*pb.PeerOtcSharesResponse, error) {
	statusValue, err := s.peerOtcShareService.Release(ctx, req.GetContractId())
	if err != nil {
		return nil, err
	}
	return &pb.PeerOtcSharesResponse{ContractId: req.GetContractId(), Status: statusValue}, nil
}

func (s *TradingServiceServer) ConsumePeerOtcShares(ctx context.Context, req *pb.ConsumePeerOtcSharesRequest) (*pb.PeerOtcSharesResponse, error) {
	statusValue, err := s.peerOtcShareService.Consume(ctx, req.GetContractId())
	if err != nil {
		return nil, err
	}
	return &pb.PeerOtcSharesResponse{ContractId: req.GetContractId(), Status: statusValue}, nil
}

func (s *TradingServiceServer) CreditPeerOtcShares(ctx context.Context, req *pb.CreditPeerOtcSharesRequest) (*pb.PeerOtcSharesResponse, error) {
	statusValue, err := s.peerOtcShareService.Credit(ctx, req.GetContractId(), uint(req.GetBuyerId()), req.GetTicker(), req.GetAmount(), req.GetPricePerUnitRsd(), req.GetUserType())
	if err != nil {
		return nil, err
	}
	return &pb.PeerOtcSharesResponse{ContractId: req.GetContractId(), Status: statusValue}, nil
}
