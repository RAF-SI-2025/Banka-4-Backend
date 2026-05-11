package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/auth"
	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/permission"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- in-memory fakes ---

type fakeOtcOfferRepo struct {
	offers map[uint]*model.OtcOffer
	nextID uint
}

func newFakeOtcOfferRepo() *fakeOtcOfferRepo {
	return &fakeOtcOfferRepo{offers: make(map[uint]*model.OtcOffer), nextID: 1}
}

func (r *fakeOtcOfferRepo) Create(_ context.Context, o *model.OtcOffer) error {
	o.OtcOfferID = r.nextID
	r.nextID++
	cp := *o
	r.offers[o.OtcOfferID] = &cp
	return nil
}

func (r *fakeOtcOfferRepo) Save(_ context.Context, o *model.OtcOffer) error {
	cp := *o
	r.offers[o.OtcOfferID] = &cp
	return nil
}

func (r *fakeOtcOfferRepo) FindByID(_ context.Context, id uint) (*model.OtcOffer, error) {
	o, ok := r.offers[id]
	if !ok {
		return nil, nil
	}
	cp := *o
	return &cp, nil
}

func (r *fakeOtcOfferRepo) FindByIDForUpdate(ctx context.Context, id uint) (*model.OtcOffer, error) {
	return r.FindByID(ctx, id)
}

func (r *fakeOtcOfferRepo) FindActiveForUser(_ context.Context, userID uint) ([]model.OtcOffer, error) {
	var out []model.OtcOffer
	for _, o := range r.offers {
		if o.Status == model.OtcOfferStatusActive && (o.BuyerID == userID || o.SellerID == userID) {
			out = append(out, *o)
		}
	}
	return out, nil
}

func (r *fakeOtcOfferRepo) FindActiveBySellerAndStock(_ context.Context, sellerID, stockID uint, excludeID *uint) ([]model.OtcOffer, error) {
	var out []model.OtcOffer
	for _, o := range r.offers {
		if o.Status != model.OtcOfferStatusActive || o.SellerID != sellerID || o.StockAssetID != stockID {
			continue
		}
		if excludeID != nil && o.OtcOfferID == *excludeID {
			continue
		}
		out = append(out, *o)
	}
	return out, nil
}

type fakeOtcContractRepo struct {
	contracts map[uint]*model.OtcOptionContract
	nextID    uint
}

func newFakeOtcContractRepo() *fakeOtcContractRepo {
	return &fakeOtcContractRepo{contracts: make(map[uint]*model.OtcOptionContract), nextID: 1}
}

func (r *fakeOtcContractRepo) Create(_ context.Context, c *model.OtcOptionContract) error {
	c.OtcOptionContractID = r.nextID
	r.nextID++
	cp := *c
	r.contracts[c.OtcOptionContractID] = &cp
	return nil
}

func (r *fakeOtcContractRepo) Save(_ context.Context, c *model.OtcOptionContract) error {
	cp := *c
	r.contracts[c.OtcOptionContractID] = &cp
	return nil
}

func (r *fakeOtcContractRepo) FindByID(_ context.Context, id uint) (*model.OtcOptionContract, error) {
	c, ok := r.contracts[id]
	if !ok {
		return nil, nil
	}
	cp := *c
	return &cp, nil
}
func (r *fakeOtcContractRepo) FindByIDForUpdate(ctx context.Context, id uint) (*model.OtcOptionContract, error) {
	return r.FindByID(ctx, id)
}
func (r *fakeOtcContractRepo) FindByOfferID(_ context.Context, offerID uint) (*model.OtcOptionContract, error) {
	for _, contract := range r.contracts {
		if contract.OtcOfferID == offerID {
			copy := *contract
			return &copy, nil
		}
	}
	return nil, nil
}

func (r *fakeOtcContractRepo) FindForUser(_ context.Context, userID uint) ([]model.OtcOptionContract, error) {
	var out []model.OtcOptionContract
	for _, c := range r.contracts {
		if c.BuyerID == userID || c.SellerID == userID {
			out = append(out, *c)
		}
	}
	return out, nil
}

func (r *fakeOtcContractRepo) FindActiveBySellerAndStock(_ context.Context, sellerID, stockID uint, now time.Time) ([]model.OtcOptionContract, error) {
	var out []model.OtcOptionContract
	for _, c := range r.contracts {
		if c.SellerID == sellerID && c.StockAssetID == stockID && c.Status == model.OtcOptionContractStatusActive && c.SettlementDate.After(now) {
			out = append(out, *c)
		}
	}
	return out, nil
}

func (r *fakeOtcContractRepo) FindExpiredActive(_ context.Context, before time.Time, limit int) ([]model.OtcOptionContract, error) {
	out := make([]model.OtcOptionContract, 0)
	for _, c := range r.contracts {
		if c.Status == model.OtcOptionContractStatusActive && !c.SettlementDate.After(before) {
			out = append(out, *c)
		}
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

type fakeOtcAssetOwnershipRepo struct {
	ownerships     map[string]*model.AssetOwnership
	reservedDeltas map[uint]float64 // assetID -> cumulative delta
}

func newFakeOtcAssetOwnershipRepo() *fakeOtcAssetOwnershipRepo {
	return &fakeOtcAssetOwnershipRepo{
		ownerships:     make(map[string]*model.AssetOwnership),
		reservedDeltas: make(map[uint]float64),
	}
}

func otcOwnershipKey(id uint, ot model.OwnerType, assetID uint) string {
	return fmt.Sprintf("%d:%s:%d", id, ot, assetID)
}

func (r *fakeOtcAssetOwnershipRepo) seedOwnership(o model.AssetOwnership) {
	r.ownerships[otcOwnershipKey(o.UserId, o.OwnerType, o.AssetID)] = &o
}

func (r *fakeOtcAssetOwnershipRepo) FindByUserId(_ context.Context, id uint, ot model.OwnerType) ([]model.AssetOwnership, error) {
	var out []model.AssetOwnership
	for _, v := range r.ownerships {
		if v.UserId == id && v.OwnerType == ot {
			out = append(out, *v)
		}
	}
	return out, nil
}

func (r *fakeOtcAssetOwnershipRepo) FindByOwnerType(_ context.Context, ot model.OwnerType) ([]model.AssetOwnership, error) {
	var out []model.AssetOwnership
	for _, v := range r.ownerships {
		if v.OwnerType == ot {
			out = append(out, *v)
		}
	}
	return out, nil
}

func (r *fakeOtcAssetOwnershipRepo) Upsert(_ context.Context, o *model.AssetOwnership) error {
	r.ownerships[otcOwnershipKey(o.UserId, o.OwnerType, o.AssetID)] = o
	return nil
}

func (r *fakeOtcAssetOwnershipRepo) IncreaseReservedAmount(_ context.Context, id uint, ot model.OwnerType, assetID uint, delta float64) error {
	r.reservedDeltas[assetID] += delta
	key := otcOwnershipKey(id, ot, assetID)
	if o, ok := r.ownerships[key]; ok {
		o.ReservedAmount += delta
	}
	return nil
}

func (r *fakeOtcAssetOwnershipRepo) FindByID(_ context.Context, id uint) (*model.AssetOwnership, error) {
	for _, v := range r.ownerships {
		if v.AssetID == id {
			return v, nil
		}
	}
	return nil, nil
}
func (r *fakeOtcAssetOwnershipRepo) FindByUserAndAsset(_ context.Context, id uint, ot model.OwnerType, assetID uint) (*model.AssetOwnership, error) {
	key := otcOwnershipKey(id, ot, assetID)
	if ownership, ok := r.ownerships[key]; ok {
		return new(*ownership), nil
	}
	return nil, nil
}

func (r *fakeOtcAssetOwnershipRepo) FindByUserAndAssetForUpdate(ctx context.Context, id uint, ot model.OwnerType, assetID uint) (*model.AssetOwnership, error) {
	return r.FindByUserAndAsset(ctx, id, ot, assetID)
}

func (r *fakeOtcAssetOwnershipRepo) FindAllPublic(_ context.Context, page, pageSize int) ([]model.AssetOwnership, int64, error) {
	return nil, 0, nil
}

func (r *fakeOtcAssetOwnershipRepo) UpdateOTCFields(_ context.Context, ownershipID uint, publicAmount, reservedAmount float64) error {
	return nil
}

type fakeOtcStockRepo struct {
	stocks []model.Stock
}

func (r *fakeOtcStockRepo) Upsert(_ context.Context, _ *model.Stock) error   { return nil }
func (r *fakeOtcStockRepo) FindAll(_ context.Context) ([]model.Stock, error) { return r.stocks, nil }
func (r *fakeOtcStockRepo) Count(_ context.Context) (int64, error)           { return int64(len(r.stocks)), nil }
func (r *fakeOtcStockRepo) FindByAssetIDs(_ context.Context, ids []uint) ([]model.Stock, error) {
	idSet := make(map[uint]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}
	var out []model.Stock
	for _, s := range r.stocks {
		if idSet[s.AssetID] {
			out = append(out, s)
		}
	}
	return out, nil
}

// --- test constants ---

const (
	otcBuyerID       uint = 10
	otcSellerID      uint = 20
	otcAssetID       uint = 1
	assetOwnershipID uint = 1
)

var otcFutureDate = time.Now().Add(30 * 24 * time.Hour)

// ctxForOtcUser creates an authenticated context for the given identity ID.
func ctxForOtcUser(id uint) context.Context {
	cid := id
	return auth.SetAuthOnContext(context.Background(), &auth.AuthContext{
		IdentityID:   id,
		IdentityType: auth.IdentityClient,
		ClientID:     &cid,
		Permissions:  []permission.Permission{permission.Trading},
	})
}

// --- test setup ---

func newOtcTestService(t *testing.T) (*OtcOfferService, *fakeOtcOfferRepo, *fakeOtcContractRepo, *fakeOtcAssetOwnershipRepo, *fakeBankingClient) {
	t.Helper()

	offerRepo := newFakeOtcOfferRepo()
	contractRepo := newFakeOtcContractRepo()
	ownershipRepo := newFakeOtcAssetOwnershipRepo()
	stockRepo := &fakeOtcStockRepo{
		stocks: []model.Stock{
			{AssetID: otcAssetID},
		},
	}
	banking := &fakeBankingClient{
		accountByNumber: map[string]uint64{
			"seller-acc": uint64(otcSellerID),
			"buyer-acc":  uint64(otcBuyerID),
		},
	}

	// Seller has 100 public shares.
	ownershipRepo.seedOwnership(model.AssetOwnership{
		UserId:       otcSellerID,
		OwnerType:    model.OwnerTypeClient,
		AssetID:      otcAssetID,
		Amount:       100,
		PublicAmount: 100,
		Asset:        model.Asset{AssetType: model.AssetTypeStock},
	})

	processingSvc := NewOtcDealProcessingService(
		offerRepo,
		contractRepo,
		newProcessingShareReservationRepo(),
		newProcessingExecutionRepo(),
		ownershipRepo,
		&processingTxManager{},
		banking,
	)

	svc := NewOtcOfferService(offerRepo, contractRepo, ownershipRepo, stockRepo, banking, &fakeUserClient{}, processingSvc)
	return svc, offerRepo, contractRepo, ownershipRepo, banking
}

func otcCreateOffer(t *testing.T, svc *OtcOfferService, amount int) *model.OtcOffer {
	t.Helper()
	ctx := ctxForOtcUser(otcBuyerID)
	offer, err := svc.CreateOffer(ctx, dto.CreateOtcOfferRequest{
		AssetOwnershipID:   assetOwnershipID,
		Amount:             amount,
		PricePerStockRSD:   50.0,
		PremiumRSD:         5.0,
		SettlementDate:     otcFutureDate,
		BuyerAccountNumber: "buyer-acc",
	})
	require.NoError(t, err)
	return offer
}

// --- tests ---

func TestOtcCreateOffer_Success(t *testing.T) {
	svc, offerRepo, _, _, _ := newOtcTestService(t)

	offer := otcCreateOffer(t, svc, 10)

	assert.Equal(t, otcBuyerID, offer.BuyerID)
	assert.Equal(t, otcSellerID, offer.SellerID)
	assert.Equal(t, 10, offer.Amount)
	assert.Equal(t, model.OtcOfferStatusActive, offer.Status)
	assert.Equal(t, otcBuyerID, offer.ModifiedBy)
	assert.Len(t, offerRepo.offers, 1)
}

func TestOtcCreateOffer_SelfOffer_ReturnsError(t *testing.T) {
	svc, _, _, _, _ := newOtcTestService(t)
	ctx := ctxForOtcUser(otcSellerID)

	_, err := svc.CreateOffer(ctx, dto.CreateOtcOfferRequest{
		AssetOwnershipID:   assetOwnershipID,
		Amount:             10,
		PricePerStockRSD:   50,
		PremiumRSD:         5,
		SettlementDate:     otcFutureDate,
		BuyerAccountNumber: "acc",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "yourself")
}

func TestOtcCreateOffer_PastSettlementDate_ReturnsError(t *testing.T) {
	svc, _, _, _, _ := newOtcTestService(t)
	ctx := ctxForOtcUser(otcBuyerID)

	_, err := svc.CreateOffer(ctx, dto.CreateOtcOfferRequest{
		AssetOwnershipID:   assetOwnershipID,
		Amount:             10,
		PricePerStockRSD:   50,
		PremiumRSD:         5,
		SettlementDate:     time.Now().Add(-time.Hour),
		BuyerAccountNumber: "acc",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "future")
}

func TestOtcCreateOffer_ExceedsSellerCapacity_ReturnsError(t *testing.T) {
	svc, _, _, _, _ := newOtcTestService(t)
	ctx := ctxForOtcUser(otcBuyerID)

	_, err := svc.CreateOffer(ctx, dto.CreateOtcOfferRequest{
		AssetOwnershipID:   assetOwnershipID,
		Amount:             200, // seller only has 100 public
		PricePerStockRSD:   50,
		PremiumRSD:         5,
		SettlementDate:     otcFutureDate,
		BuyerAccountNumber: "acc",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "enough")
}

func TestOtcSendCounterOffer_Success(t *testing.T) {
	svc, offerRepo, _, _, _ := newOtcTestService(t)
	offer := otcCreateOffer(t, svc, 10)

	sellerAcc := "seller-acc"
	ctx := ctxForOtcUser(otcSellerID)
	updated, err := svc.SendCounterOffer(ctx, offer.OtcOfferID, dto.CounterOfferRequest{
		Amount:           8,
		PricePerStockRSD: 55,
		PremiumRSD:       6,
		SettlementDate:   otcFutureDate,
		AccountNumber:    &sellerAcc,
	})

	require.NoError(t, err)
	assert.Equal(t, 8, updated.Amount)
	assert.Equal(t, otcSellerID, updated.ModifiedBy)
	assert.Equal(t, &sellerAcc, offerRepo.offers[offer.OtcOfferID].SellerAccountNumber)
}

func TestOtcSendCounterOffer_SameUserTwice_ReturnsError(t *testing.T) {
	svc, _, _, _, _ := newOtcTestService(t)
	offer := otcCreateOffer(t, svc, 10)

	// buyer just created the offer (ModifiedBy=buyer) — buyer tries again
	ctx := ctxForOtcUser(otcBuyerID)
	_, err := svc.SendCounterOffer(ctx, offer.OtcOfferID, dto.CounterOfferRequest{
		Amount:           5,
		PricePerStockRSD: 50,
		PremiumRSD:       5,
		SettlementDate:   otcFutureDate,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "other party")
}

func TestOtcSendCounterOffer_NonParticipant_ReturnsError(t *testing.T) {
	svc, _, _, _, _ := newOtcTestService(t)
	offer := otcCreateOffer(t, svc, 10)

	ctx := ctxForOtcUser(99)
	_, err := svc.SendCounterOffer(ctx, offer.OtcOfferID, dto.CounterOfferRequest{
		Amount:           5,
		PricePerStockRSD: 50,
		PremiumRSD:       5,
		SettlementDate:   otcFutureDate,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "participant")
}

func TestOtcSendCounterOffer_OfferNotFound_ReturnsError(t *testing.T) {
	svc, _, _, _, _ := newOtcTestService(t)
	ctx := ctxForOtcUser(otcSellerID)

	_, err := svc.SendCounterOffer(ctx, 9999, dto.CounterOfferRequest{
		Amount:           5,
		PricePerStockRSD: 50,
		PremiumRSD:       5,
		SettlementDate:   otcFutureDate,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestOtcAcceptOffer_Success_CreatesContractAndIncreasesReservedAmount(t *testing.T) {
	svc, _, contractRepo, ownershipRepo, _ := newOtcTestService(t)
	offer := otcCreateOffer(t, svc, 10)

	sellerAcc := "seller-acc"
	ctx := ctxForOtcUser(otcSellerID) // seller accepts buyer's offer
	contract, err := svc.AcceptOffer(ctx, offer.OtcOfferID, dto.AcceptOfferRequest{
		AccountNumber: &sellerAcc,
	})

	require.NoError(t, err)
	assert.Len(t, contractRepo.contracts, 1)
	assert.Equal(t, 10, contract.Amount)
	assert.Equal(t, float64(50), contract.StrikePriceRSD)
	sellerOwnership, findErr := ownershipRepo.FindByUserAndAsset(context.Background(), otcSellerID, model.OwnerTypeClient, otcAssetID)
	require.NoError(t, findErr)
	require.NotNil(t, sellerOwnership)
	assert.Equal(t, float64(10), sellerOwnership.ReservedAmount, "reserved amount should increase by contract amount")
}

func TestOtcAcceptOffer_CallerIsModifiedBy_ReturnsError(t *testing.T) {
	svc, _, _, _, _ := newOtcTestService(t)
	offer := otcCreateOffer(t, svc, 10)

	// buyer created the offer (ModifiedBy=buyer) — buyer cannot accept their own
	ctx := ctxForOtcUser(otcBuyerID)
	_, err := svc.AcceptOffer(ctx, offer.OtcOfferID, dto.AcceptOfferRequest{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "own offer")
}

func TestOtcAcceptOffer_MissingSellerAccount_ReturnsError(t *testing.T) {
	svc, _, _, _, _ := newOtcTestService(t)
	offer := otcCreateOffer(t, svc, 10)

	ctx := ctxForOtcUser(otcSellerID)
	_, err := svc.AcceptOffer(ctx, offer.OtcOfferID, dto.AcceptOfferRequest{AccountNumber: nil})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "seller_account_number")
}

func TestOtcRejectOffer_Success(t *testing.T) {
	svc, offerRepo, _, _, _ := newOtcTestService(t)
	offer := otcCreateOffer(t, svc, 10)

	ctx := ctxForOtcUser(otcSellerID)
	rejected, err := svc.RejectOffer(ctx, offer.OtcOfferID, dto.RejectOfferRequest{})

	require.NoError(t, err)
	assert.Equal(t, model.OtcOfferStatusRejected, rejected.Status)
	assert.Equal(t, model.OtcOfferStatusRejected, offerRepo.offers[offer.OtcOfferID].Status)
}

func TestOtcRejectOffer_InactiveOffer_ReturnsError(t *testing.T) {
	svc, _, _, _, _ := newOtcTestService(t)
	offer := otcCreateOffer(t, svc, 10)

	// first rejection
	ctx := ctxForOtcUser(otcSellerID)
	_, err := svc.RejectOffer(ctx, offer.OtcOfferID, dto.RejectOfferRequest{})
	require.NoError(t, err)

	// second rejection on an already-rejected offer
	_, err = svc.RejectOffer(ctx, offer.OtcOfferID, dto.RejectOfferRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not active")
}

func TestOtcExerciseContract_Success(t *testing.T) {
	svc, _, _, _, _ := newOtcTestService(t)
	offer := otcCreateOffer(t, svc, 10)

	sellerAcc := "seller-acc"
	contract, err := svc.AcceptOffer(ctxForOtcUser(otcSellerID), offer.OtcOfferID, dto.AcceptOfferRequest{
		AccountNumber: &sellerAcc,
	})
	require.NoError(t, err)

	execution, err := svc.ExerciseContract(ctxForOtcUser(otcBuyerID), contract.OtcOptionContractID)
	require.NoError(t, err)
	require.NotNil(t, execution)
	assert.Equal(t, model.OtcExecutionStatusCompleted, execution.Status)
	assert.Equal(t, model.OtcExecutionStepCompleted, execution.CurrentStep)
}

func TestOtcExerciseContract_OnlyBuyerMayExercise(t *testing.T) {
	svc, _, _, _, _ := newOtcTestService(t)
	offer := otcCreateOffer(t, svc, 10)

	sellerAcc := "seller-acc"
	contract, err := svc.AcceptOffer(ctxForOtcUser(otcSellerID), offer.OtcOfferID, dto.AcceptOfferRequest{
		AccountNumber: &sellerAcc,
	})
	require.NoError(t, err)

	execution, err := svc.ExerciseContract(ctxForOtcUser(otcSellerID), contract.OtcOptionContractID)
	require.Nil(t, execution)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only the buyer")
}

func TestOtcGetActiveOffersForUser_ReturnsOnlyActiveOffers(t *testing.T) {
	svc, _, _, _, _ := newOtcTestService(t)
	otcCreateOffer(t, svc, 10)
	otcCreateOffer(t, svc, 5)

	offers, err := svc.GetActiveOffersForUser(context.Background(), otcBuyerID)
	require.NoError(t, err)
	assert.Len(t, offers, 2)
}

func TestOtcValidateSellerCapacity_MultipleConcurrentOffers(t *testing.T) {
	svc, _, _, _, _ := newOtcTestService(t)

	// seller has 100 public shares; create two offers summing to 90
	otcCreateOffer(t, svc, 50)
	otcCreateOffer(t, svc, 40)

	// third offer requesting 20 should fail (50+40+20=110 > 100)
	ctx := ctxForOtcUser(otcBuyerID)
	_, err := svc.CreateOffer(ctx, dto.CreateOtcOfferRequest{
		AssetOwnershipID:   assetOwnershipID,
		Amount:             20,
		PricePerStockRSD:   50,
		PremiumRSD:         5,
		SettlementDate:     otcFutureDate,
		BuyerAccountNumber: "acc",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "enough")
}
