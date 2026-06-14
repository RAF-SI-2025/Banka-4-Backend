package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/config"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
)

func newOtcSvcWithStores(peers *PeerResolver) (*PeerOtcService, *fakeNegotiations, *fakeContracts) {
	negs := newFakeNegotiations()
	contracts := newFakeContracts()
	svc := NewPeerOtcService(
		negs, contracts, peers, NewPeerOtcClient(peers),
		nil, nil, nil, nil, newFakeOutbound(), fakeTxManager{},
	)
	return svc, negs, contracts
}

func activePeerContract(authority int, id string, buyerRouting int, buyerID string, sellerRouting int, sellerID string) *model.PeerContract {
	now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	buyerAccount := "444000000000000011"
	return &model.PeerContract{
		AuthorityRoutingNumber: authority,
		ID:                     id,
		NegotiationID:          id,
		BuyerRoutingNumber:     buyerRouting,
		BuyerID:                buyerID,
		SellerRoutingNumber:    sellerRouting,
		SellerID:               sellerID,
		Ticker:                 "AAPL",
		Amount:                 10,
		StrikePrice:            100,
		StrikeCurrency:         "RSD",
		Premium:                5,
		PremiumCurrency:        "RSD",
		SettlementDate:         "2030-01-01",
		BuyerAccountNumber:     &buyerAccount,
		Status:                 model.PeerContractActive,
		IsAuthoritative:        authority == ourRouting,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
}

func TestPeerOtcServiceListLocalPublicStocksMapsTradingResponse(t *testing.T) {
	trading := &fakeTrading{
		publicStocks: &pb.ListPublicStocksResponse{
			Stocks: []*pb.PublicStockEntry{
				{
					Ticker: "AAPL",
					Sellers: []*pb.PublicStockSeller{
						{SellerId: 7, Amount: 12},
						{SellerId: 9, Amount: 3},
					},
				},
			},
		},
	}
	svc := NewPeerOtcService(nil, nil, testResolver(ourRouting), nil, trading, nil, nil, nil, nil, nil)

	stocks, err := svc.ListLocalPublicStocks(context.Background())
	require.NoError(t, err)
	require.Len(t, stocks, 1)
	require.Equal(t, "AAPL", stocks[0].Stock.Ticker)
	require.Equal(t, dto.ForeignBankId{RoutingNumber: ourRouting, ID: "7"}, stocks[0].Sellers[0].Seller)
	require.Equal(t, 12, stocks[0].Sellers[0].Amount)
	require.Equal(t, dto.ForeignBankId{RoutingNumber: ourRouting, ID: "9"}, stocks[0].Sellers[1].Seller)
	require.Equal(t, 3, stocks[0].Sellers[1].Amount)
}

func TestPeerOtcServiceListAllPeerPublicStocksSkipsUnavailablePeers(t *testing.T) {
	goodPeer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/public-stock", r.URL.Path)
		require.Equal(t, "to-good", r.Header.Get("X-Api-Key"))
		_ = json.NewEncoder(w).Encode([]dto.PublicStock{
			{
				Stock: dto.StockDescription{Ticker: "MSFT"},
				Sellers: []dto.PublicStockSeller{
					{Seller: dto.ForeignBankId{RoutingNumber: 111, ID: "seller-1"}, Amount: 4},
				},
			},
		})
	}))
	defer goodPeer.Close()

	downPeer := httptest.NewServer(http.NotFoundHandler())
	downURL := downPeer.URL
	downPeer.Close()

	peers := testResolver(ourRouting,
		config.Peer{RoutingNumber: 111, BaseURL: goodPeer.URL, OurAPIKey: "to-good", TheirAPIKey: "from-good"},
		config.Peer{RoutingNumber: 222, BaseURL: downURL, OurAPIKey: "to-down", TheirAPIKey: "from-down"},
	)
	svc := NewPeerOtcService(nil, nil, peers, NewPeerOtcClient(peers), nil, nil, nil, nil, nil, nil)

	stocks, err := svc.ListAllPeerPublicStocks(context.Background())
	require.NoError(t, err)
	require.Equal(t, []dto.PublicStock{
		{
			Stock: dto.StockDescription{Ticker: "MSFT"},
			Sellers: []dto.PublicStockSeller{
				{Seller: dto.ForeignBankId{RoutingNumber: 111, ID: "seller-1"}, Amount: 4},
			},
		},
	}, stocks)
}

func TestPeerOtcServiceCreateForLocalBuyerStoresMirror(t *testing.T) {
	bank := newMockBank()
	defer bank.close()
	svc, negs, _ := newOtcSvcWithStores(peersTo(bank))

	id, err := svc.CreateForLocalBuyer(context.Background(), 9, LocalCreateRequest{
		SellerID:           dto.ForeignBankId{RoutingNumber: 111, ID: "seller-1"},
		Ticker:             "AAPL",
		Amount:             10,
		PricePerStock:      100,
		PriceCurrency:      "RSD",
		Premium:            5,
		PremiumCurrency:    "RSD",
		SettlementDate:     "2030-01-01",
		BuyerAccountNumber: "444000000000000011",
	})
	require.NoError(t, err)
	require.Equal(t, &dto.ForeignBankId{RoutingNumber: 111, ID: "remote-neg"}, id)
	require.Equal(t, []string{"POST /negotiations"}, bank.otcGot())

	stored, err := negs.FindByID(context.Background(), 111, "remote-neg")
	require.NoError(t, err)
	require.False(t, stored.IsAuthoritative)
	require.Equal(t, ourRouting, stored.BuyerRoutingNumber)
	require.Equal(t, "9", stored.BuyerID)
	require.Equal(t, 111, stored.SellerRoutingNumber)
	require.Equal(t, "seller-1", stored.SellerID)
	require.Equal(t, "AAPL", stored.Ticker)
}

func TestPeerOtcServiceListMyNegotiationsAndContractsFilterByLocalParty(t *testing.T) {
	svc, negs, contracts := newOtcSvcWithStores(testResolver(ourRouting))
	seedMirrorNegotiation(negs, "neg-local-buyer")
	seedAuthoritativeNegotiation(negs, "neg-local-seller")
	negs.seed(model.PeerNegotiation{
		ID:                    "neg-other",
		BuyerRoutingNumber:    111,
		BuyerID:               "buyer-2",
		SellerRoutingNumber:   222,
		SellerID:              "seller-2",
		Ticker:                "GOOG",
		Amount:                1,
		PricePerStock:         50,
		PriceCurrency:         "RSD",
		Premium:               1,
		PremiumCurrency:       "RSD",
		SettlementDate:        "2030-01-01",
		BuyerAccountNumber:    "111000000000000011",
		LastModifiedByRouting: 111,
		LastModifiedByID:      "buyer-2",
		Status:                model.PeerNegotiationOngoing,
	})
	require.NoError(t, contracts.Create(context.Background(), activePeerContract(111, "contract-buyer", ourRouting, "9", 111, "seller-1")))
	require.NoError(t, contracts.Create(context.Background(), activePeerContract(ourRouting, "contract-seller", 111, "buyer-1", ourRouting, "7")))
	require.NoError(t, contracts.Create(context.Background(), activePeerContract(222, "contract-other", 111, "buyer-2", 222, "seller-2")))

	buyerNegotiations, err := svc.ListMyNegotiations(context.Background(), 9)
	require.NoError(t, err)
	require.Len(t, buyerNegotiations, 1)
	require.Equal(t, dto.ForeignBankId{RoutingNumber: 111, ID: "neg-local-buyer"}, buyerNegotiations[0].ID)
	require.Equal(t, "ongoing", buyerNegotiations[0].Status)

	sellerContracts, err := svc.ListMyContracts(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, sellerContracts, 1)
	require.Equal(t, dto.ForeignBankId{RoutingNumber: ourRouting, ID: "contract-seller"}, sellerContracts[0].ID)
	require.Equal(t, "active", sellerContracts[0].Status)
}

func TestPeerOtcServiceWithdrawAsLocalCancelsMirrorAndNotifiesSeller(t *testing.T) {
	bank := newMockBank()
	defer bank.close()
	svc, negs, _ := newOtcSvcWithStores(peersTo(bank))
	seedMirrorNegotiation(negs, "neg-withdraw")

	err := svc.WithdrawAsLocal(context.Background(), 9, dto.ForeignBankId{RoutingNumber: 111, ID: "neg-withdraw"})
	require.NoError(t, err)
	require.Equal(t, []string{"DELETE /negotiations/111/neg-withdraw"}, bank.otcGot())

	updated, err := negs.FindByID(context.Background(), 111, "neg-withdraw")
	require.NoError(t, err)
	require.Equal(t, model.PeerNegotiationCancelled, updated.Status)
}

func TestPeerOtcServiceLookupLocalUser(t *testing.T) {
	svc := NewPeerOtcService(nil, nil, testResolver(ourRouting), nil, nil, &fakeUserClient{
		byIdentity: map[uint64]*pb.GetUserByIdentityIdResponse{
			77: {FullName: "Jane Client", UserId: 7, UserType: "CLIENT"},
		},
	}, nil, nil, nil, nil)

	info, err := svc.LookupLocalUser(context.Background(), ourRouting, "77")
	require.NoError(t, err)
	require.Equal(t, "Banka 4", info.BankDisplayName)
	require.Equal(t, "Jane Client", info.DisplayName)

	_, err = svc.LookupLocalUser(context.Background(), 111, "77")
	require.Error(t, err)
	_, err = svc.LookupLocalUser(context.Background(), ourRouting, "not-a-number")
	require.Error(t, err)

	notFoundSvc := NewPeerOtcService(nil, nil, testResolver(ourRouting), nil, nil, &fakeUserClient{
		err: status.Error(codes.NotFound, "missing"),
	}, nil, nil, nil, nil)
	_, err = notFoundSvc.LookupLocalUser(context.Background(), ourRouting, "77")
	require.Error(t, err)
}

func TestPeerOtcServiceAcceptExistingContracts(t *testing.T) {
	t.Run("from peer returns existing authoritative contract", func(t *testing.T) {
		svc, negs, contracts := newOtcSvcWithStores(testResolver(ourRouting))
		seedAuthoritativeNegotiation(negs, "neg-accepted")
		row, err := negs.FindByID(context.Background(), ourRouting, "neg-accepted")
		require.NoError(t, err)
		row.Status = model.PeerNegotiationAccepted
		require.NoError(t, negs.Update(context.Background(), row))
		require.NoError(t, contracts.Create(context.Background(), activePeerContract(ourRouting, "neg-accepted", 111, "buyer-1", ourRouting, "7")))

		contract, err := svc.AcceptFromPeer(context.Background(), 111, ourRouting, "neg-accepted")
		require.NoError(t, err)
		require.Equal(t, dto.ForeignBankId{RoutingNumber: ourRouting, ID: "neg-accepted"}, contract.ID)
		require.Equal(t, dto.ForeignBankId{RoutingNumber: ourRouting, ID: "neg-accepted"}, contract.NegotiationID)
	})

	t.Run("as local buyer returns existing mirror contract", func(t *testing.T) {
		svc, negs, contracts := newOtcSvcWithStores(testResolver(ourRouting))
		seedMirrorNegotiation(negs, "neg-mirror-accepted")
		row, err := negs.FindByID(context.Background(), 111, "neg-mirror-accepted")
		require.NoError(t, err)
		row.Status = model.PeerNegotiationAccepted
		require.NoError(t, negs.Update(context.Background(), row))
		require.NoError(t, contracts.Create(context.Background(), activePeerContract(111, "neg-mirror-accepted", ourRouting, "9", 111, "seller-1")))

		contract, err := svc.AcceptAsLocal(context.Background(), 9, dto.ForeignBankId{RoutingNumber: 111, ID: "neg-mirror-accepted"}, LocalAcceptRequest{})
		require.NoError(t, err)
		require.Equal(t, dto.ForeignBankId{RoutingNumber: 111, ID: "neg-mirror-accepted"}, contract.ID)
		require.Equal(t, dto.ForeignBankId{RoutingNumber: ourRouting, ID: "9"}, contract.BuyerID)
	})
}

func TestPeerOtcServiceExerciseAsLocalValidation(t *testing.T) {
	_, _, contracts := newOtcSvcWithStores(testResolver(ourRouting))
	require.NoError(t, contracts.Create(context.Background(), activePeerContract(111, "contract-active", ourRouting, "9", 111, "seller-1")))
	require.NoError(t, contracts.Create(context.Background(), activePeerContract(111, "contract-foreign-buyer", 222, "10", 111, "seller-1")))
	expired := activePeerContract(111, "contract-expired", ourRouting, "9", 111, "seller-1")
	expired.SettlementDate = "2000-01-01"
	require.NoError(t, contracts.Create(context.Background(), expired))
	inactive := activePeerContract(111, "contract-inactive", ourRouting, "9", 111, "seller-1")
	inactive.Status = model.PeerContractCancelled
	require.NoError(t, contracts.Create(context.Background(), inactive))

	svc := NewPeerOtcService(nil, contracts, testResolver(ourRouting), nil, nil, nil, nil, nil, nil, nil)

	_, err := svc.ExerciseAsLocal(context.Background(), 9, dto.ForeignBankId{RoutingNumber: 111, ID: "missing"}, "444000000000000011")
	require.Error(t, err)
	_, err = svc.ExerciseAsLocal(context.Background(), 9, dto.ForeignBankId{RoutingNumber: 111, ID: "contract-foreign-buyer"}, "444000000000000011")
	require.Error(t, err)
	_, err = svc.ExerciseAsLocal(context.Background(), 9, dto.ForeignBankId{RoutingNumber: 111, ID: "contract-inactive"}, "444000000000000011")
	require.Error(t, err)
	_, err = svc.ExerciseAsLocal(context.Background(), 9, dto.ForeignBankId{RoutingNumber: 111, ID: "contract-expired"}, "444000000000000011")
	require.Error(t, err)
	_, err = svc.ExerciseAsLocal(context.Background(), 9, dto.ForeignBankId{RoutingNumber: 111, ID: "contract-active"}, " ")
	require.Error(t, err)
}
