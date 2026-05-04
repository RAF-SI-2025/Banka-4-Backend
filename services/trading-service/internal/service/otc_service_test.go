package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/stretchr/testify/require"
)

// --- Fake user client ---

type fakeOTCUserClient struct{}

func (f *fakeOTCUserClient) GetClientById(_ context.Context, id uint64) (*pb.GetClientByIdResponse, error) {
	return &pb.GetClientByIdResponse{Id: id, FullName: "Test Client"}, nil
}

func (f *fakeOTCUserClient) GetEmployeeById(_ context.Context, id uint64) (*pb.GetEmployeeByIdResponse, error) {
	return &pb.GetEmployeeByIdResponse{Id: id, FullName: "Test Employee"}, nil
}

func (f *fakeOTCUserClient) GetClientByIdentityId(_ context.Context, id uint64) (*pb.GetClientByIdResponse, error) {
	return &pb.GetClientByIdResponse{Id: id, FullName: "Test Client"}, nil
}

func (f *fakeOTCUserClient) GetEmployeeByIdentityId(_ context.Context, id uint64) (*pb.GetEmployeeByIdResponse, error) {
	return &pb.GetEmployeeByIdResponse{Id: id, FullName: "Test Employee"}, nil
}

func (f *fakeOTCUserClient) GetAllClients(_ context.Context, _, _ int32, _, _ string) (*pb.GetAllClientsResponse, error) {
	return &pb.GetAllClientsResponse{}, nil
}

func (f *fakeOTCUserClient) GetAllActuaries(_ context.Context, _, _ int32, _, _ string) (*pb.GetAllActuariesResponse, error) {
	return &pb.GetAllActuariesResponse{}, nil
}

func (f *fakeOTCUserClient) GetIdentityByUserId(_ context.Context, id uint64, _ string) (*pb.GetIdentityByUserIdResponse, error) {
	return nil, nil
}

// --- Helpers ---

func makeOwnershipForOTC(id, identityID, assetID uint, ownerType model.OwnerType, amount, reserved float64) *model.AssetOwnership {
	return &model.AssetOwnership{
		AssetOwnershipID: id,
		UserId:           identityID,
		OwnerType:        ownerType,
		AssetID:          assetID,
		Asset:            model.Asset{AssetID: assetID, AssetType: model.AssetTypeStock},
		Amount:           amount,
		ReservedAmount:   reserved,
		UpdatedAt:        time.Now(),
	}
}

func newTestOTCService(ownershipRepo *fakeAssetOwnershipRepo, listingRepo *fakeListingRepo) *OTCService {
	return NewOTCService(ownershipRepo, listingRepo, &fakeOTCUserClient{})
}

// --- PublishAsset tests ---

func TestOTCService_PublishAsset(t *testing.T) {
	cases := []struct {
		name          string
		ownershipRepo *fakeAssetOwnershipRepo
		ownershipID   uint
		identityID    uint
		ownerType     model.OwnerType
		amount        float64
		wantErr       bool
		checkErr      func(t *testing.T, err error)
	}{
		{
			name: "happy path — no existing public amount",
			ownershipRepo: &fakeAssetOwnershipRepo{
				byID: makeOwnershipForOTC(1, 10, 5, model.OwnerTypeClient, 20, 0),
			},
			ownershipID: 1, identityID: 10, ownerType: model.OwnerTypeClient, amount: 5,
		},
		{
			name: "happy path — existing reserved amount respected",
			ownershipRepo: &fakeAssetOwnershipRepo{
				byID: makeOwnershipForOTC(1, 10, 5, model.OwnerTypeClient, 20, 2),
			},
			ownershipID: 1, identityID: 10, ownerType: model.OwnerTypeClient, amount: 10,
		},
		{
			name:          "ownership not found",
			ownershipRepo: &fakeAssetOwnershipRepo{byID: nil},
			ownershipID:   99, identityID: 10, ownerType: model.OwnerTypeClient, amount: 5,
			wantErr: true,
			checkErr: func(t *testing.T, err error) {
				require.Contains(t, err.Error(), "not found")
			},
		},
		{
			name: "identity mismatch",
			ownershipRepo: &fakeAssetOwnershipRepo{
				byID: makeOwnershipForOTC(1, 10, 5, model.OwnerTypeClient, 20, 0),
			},
			ownershipID: 1, identityID: 99, ownerType: model.OwnerTypeClient, amount: 5,
			wantErr: true,
			checkErr: func(t *testing.T, err error) {
				require.Contains(t, err.Error(), "do not own")
			},
		},
		{
			name: "owner type mismatch",
			ownershipRepo: &fakeAssetOwnershipRepo{
				byID: makeOwnershipForOTC(1, 10, 5, model.OwnerTypeClient, 20, 0),
			},
			ownershipID: 1, identityID: 10, ownerType: model.OwnerTypeActuary, amount: 5,
			wantErr: true,
			checkErr: func(t *testing.T, err error) {
				require.Contains(t, err.Error(), "do not own")
			},
		},
		{
			name: "amount < 0",
			ownershipRepo: &fakeAssetOwnershipRepo{
				byID: makeOwnershipForOTC(1, 10, 5, model.OwnerTypeClient, 20, 0),
			},
			ownershipID: 1, identityID: 10, ownerType: model.OwnerTypeClient, amount: -1,
			wantErr: true,
			checkErr: func(t *testing.T, err error) {
				require.Contains(t, err.Error(), "non-negative")
			},
		},
		{
			name: "amount exceeds available",
			ownershipRepo: &fakeAssetOwnershipRepo{
				byID: makeOwnershipForOTC(1, 10, 5, model.OwnerTypeClient, 10, 3),
			},
			ownershipID: 1, identityID: 10, ownerType: model.OwnerTypeClient, amount: 8,
			// available = 10 - 3 = 7, asking for 8
			wantErr: true,
			checkErr: func(t *testing.T, err error) {
				require.Contains(t, err.Error(), "exceeds available")
			},
		},
		{
			name: "update error",
			ownershipRepo: &fakeAssetOwnershipRepo{
				byID:         makeOwnershipForOTC(1, 10, 5, model.OwnerTypeClient, 20, 0),
				updateOTCErr: errors.New("db error"),
			},
			ownershipID: 1, identityID: 10, ownerType: model.OwnerTypeClient, amount: 5,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestOTCService(tc.ownershipRepo, &fakeListingRepo{})
			err := svc.PublishAsset(context.Background(), tc.ownershipID, tc.identityID, tc.ownerType, tc.amount)
			if tc.wantErr {
				require.Error(t, err)
				if tc.checkErr != nil {
					tc.checkErr(t, err)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

// --- GetPublicOTCAssets tests ---

func TestOTCService_GetPublicOTCAssets(t *testing.T) {
	asset := model.Asset{AssetID: 5, Ticker: "AAPL", Name: "Apple Inc.", AssetType: model.AssetTypeStock}
	exchange := &model.Exchange{MicCode: "XNYS", Currency: "USD"}
	listing := model.Listing{
		ListingID: 1,
		AssetID:   5,
		Price:     150.0,
		Exchange:  exchange,
	}

	ownership := model.AssetOwnership{
		AssetOwnershipID: 1,
		UserId:           10,
		OwnerType:        model.OwnerTypeClient,
		AssetID:          5,
		Asset:            asset,
		PublicAmount:     8,
		ReservedAmount:   2,
		UpdatedAt:        time.Now(),
	}

	cases := []struct {
		name          string
		ownershipRepo *fakeAssetOwnershipRepo
		listingRepo   *fakeListingRepo
		wantErr       bool
		check         func(t *testing.T, total int64)
	}{
		{
			name: "happy path with listings",
			ownershipRepo: &fakeAssetOwnershipRepo{
				allPublic:      []model.AssetOwnership{ownership},
				allPublicTotal: 1,
			},
			listingRepo: &fakeListingRepo{byAssetIDs: []model.Listing{listing}},
			check: func(t *testing.T, total int64) {
				require.Equal(t, int64(1), total)
			},
		},
		{
			name: "empty result",
			ownershipRepo: &fakeAssetOwnershipRepo{
				allPublic:      []model.AssetOwnership{},
				allPublicTotal: 0,
			},
			listingRepo: &fakeListingRepo{},
			check: func(t *testing.T, total int64) {
				require.Equal(t, int64(0), total)
			},
		},
		{
			name:          "repo error",
			ownershipRepo: &fakeAssetOwnershipRepo{allPublicErr: errors.New("db error")},
			listingRepo:   &fakeListingRepo{},
			wantErr:       true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestOTCService(tc.ownershipRepo, tc.listingRepo)
			results, total, err := svc.GetPublicOTCAssets(context.Background(), 1, 10)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			_ = results
			tc.check(t, total)
		})
	}
}

func TestOTCService_GetPublicOTCAssets_FieldMapping(t *testing.T) {
	asset := model.Asset{AssetID: 5, Ticker: "AAPL", Name: "Apple Inc.", AssetType: model.AssetTypeStock}
	exchange := &model.Exchange{MicCode: "XNYS", Currency: "USD"}
	listing := model.Listing{ListingID: 1, AssetID: 5, Price: 150.0, Exchange: exchange}
	ownership := model.AssetOwnership{
		AssetOwnershipID: 1,
		UserId:           10,
		OwnerType:        model.OwnerTypeClient,
		AssetID:          5,
		Asset:            asset,
		PublicAmount:     8,
		ReservedAmount:   2,
	}

	svc := newTestOTCService(
		&fakeAssetOwnershipRepo{allPublic: []model.AssetOwnership{ownership}, allPublicTotal: 1},
		&fakeListingRepo{byAssetIDs: []model.Listing{listing}},
	)

	results, total, err := svc.GetPublicOTCAssets(context.Background(), 1, 10)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, results, 1)

	r := results[0]
	require.Equal(t, "AAPL", r.Ticker)
	require.Equal(t, "Apple Inc.", r.Name)
	require.Equal(t, float64(150.0), r.Price)
	require.Equal(t, "USD", r.Currency)
	require.Equal(t, float64(6), r.AvailableAmount) // 8 - 2
	require.Equal(t, model.AssetTypeStock, r.SecurityType)
	require.Equal(t, "Test Client", r.OwnerName)
}
