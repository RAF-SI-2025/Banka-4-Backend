package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

type peerShareRepoFake struct {
	reservations map[string]*model.PeerOtcShareReservation
	credits      map[string]*model.PeerOtcShareCredit

	findReservationErr error
	saveReservationErr error
	createCreditErr    error
	saveReservationCnt int
}

func newPeerShareRepoFake() *peerShareRepoFake {
	return &peerShareRepoFake{
		reservations: make(map[string]*model.PeerOtcShareReservation),
		credits:      make(map[string]*model.PeerOtcShareCredit),
	}
}

func (r *peerShareRepoFake) CreateReservation(_ context.Context, reservation *model.PeerOtcShareReservation) error {
	cp := *reservation
	r.reservations[reservation.ContractID] = &cp
	return nil
}

func (r *peerShareRepoFake) FindReservationByContractID(_ context.Context, contractID string) (*model.PeerOtcShareReservation, error) {
	return r.findReservation(contractID)
}

func (r *peerShareRepoFake) FindReservationByContractIDForUpdate(_ context.Context, contractID string) (*model.PeerOtcShareReservation, error) {
	return r.findReservation(contractID)
}

func (r *peerShareRepoFake) SaveReservation(_ context.Context, reservation *model.PeerOtcShareReservation) error {
	if r.saveReservationErr != nil {
		return r.saveReservationErr
	}
	r.saveReservationCnt++
	cp := *reservation
	r.reservations[reservation.ContractID] = &cp
	return nil
}

func (r *peerShareRepoFake) CreateCredit(_ context.Context, credit *model.PeerOtcShareCredit) error {
	if r.createCreditErr != nil {
		return r.createCreditErr
	}
	cp := *credit
	r.credits[credit.ContractID] = &cp
	return nil
}

func (r *peerShareRepoFake) FindCreditByContractID(_ context.Context, contractID string) (*model.PeerOtcShareCredit, error) {
	if credit, ok := r.credits[contractID]; ok {
		cp := *credit
		return &cp, nil
	}
	return nil, nil
}

func (r *peerShareRepoFake) findReservation(contractID string) (*model.PeerOtcShareReservation, error) {
	if r.findReservationErr != nil {
		return nil, r.findReservationErr
	}
	if reservation, ok := r.reservations[contractID]; ok {
		return reservation, nil
	}
	return nil, nil
}

type peerAssetOwnershipRepoFake struct {
	ownerships map[string]*model.AssetOwnership
	findErr    error
	upsertErr  error
	upsertCnt  int
}

func newPeerAssetOwnershipRepoFake(ownerships ...*model.AssetOwnership) *peerAssetOwnershipRepoFake {
	r := &peerAssetOwnershipRepoFake{ownerships: make(map[string]*model.AssetOwnership)}
	for _, ownership := range ownerships {
		cp := *ownership
		r.ownerships[peerOwnershipKey(ownership.UserId, ownership.OwnerType, ownership.AssetID)] = &cp
	}
	return r
}

func (r *peerAssetOwnershipRepoFake) FindByUserId(context.Context, uint, model.OwnerType) ([]model.AssetOwnership, error) {
	return nil, nil
}

func (r *peerAssetOwnershipRepoFake) FindByOwnerType(context.Context, model.OwnerType) ([]model.AssetOwnership, error) {
	return nil, nil
}

func (r *peerAssetOwnershipRepoFake) FindByID(context.Context, uint) (*model.AssetOwnership, error) {
	return nil, nil
}

func (r *peerAssetOwnershipRepoFake) FindByUserAndAsset(_ context.Context, userID uint, ownerType model.OwnerType, assetID uint) (*model.AssetOwnership, error) {
	return r.find(userID, ownerType, assetID)
}

func (r *peerAssetOwnershipRepoFake) FindByUserAndAssetForUpdate(_ context.Context, userID uint, ownerType model.OwnerType, assetID uint) (*model.AssetOwnership, error) {
	return r.find(userID, ownerType, assetID)
}

func (r *peerAssetOwnershipRepoFake) Upsert(_ context.Context, ownership *model.AssetOwnership) error {
	if r.upsertErr != nil {
		return r.upsertErr
	}
	r.upsertCnt++
	cp := *ownership
	r.ownerships[peerOwnershipKey(ownership.UserId, ownership.OwnerType, ownership.AssetID)] = &cp
	return nil
}

func (r *peerAssetOwnershipRepoFake) IncreaseReservedAmount(context.Context, uint, model.OwnerType, uint, float64) error {
	return nil
}

func (r *peerAssetOwnershipRepoFake) FindAllPublic(context.Context, int, int) ([]model.AssetOwnership, int64, error) {
	return nil, 0, nil
}

func (r *peerAssetOwnershipRepoFake) UpdateOTCFields(context.Context, uint, float64, float64) error {
	return nil
}

func (r *peerAssetOwnershipRepoFake) FindAllByAssetIDs(context.Context, []uint) ([]model.AssetOwnership, error) {
	return nil, nil
}

func (r *peerAssetOwnershipRepoFake) find(userID uint, ownerType model.OwnerType, assetID uint) (*model.AssetOwnership, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	if ownership, ok := r.ownerships[peerOwnershipKey(userID, ownerType, assetID)]; ok {
		return ownership, nil
	}
	return nil, nil
}

func peerOwnershipKey(userID uint, ownerType model.OwnerType, assetID uint) string {
	return fmt.Sprintf("%d:%s:%d", userID, ownerType, assetID)
}

type peerStockRepoFake struct {
	stocks  []model.Stock
	findErr error
}

func (r *peerStockRepoFake) Upsert(context.Context, *model.Stock) error { return nil }

func (r *peerStockRepoFake) FindByAssetIDs(context.Context, []uint) ([]model.Stock, error) {
	return nil, nil
}

func (r *peerStockRepoFake) FindAll(context.Context) ([]model.Stock, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.stocks, nil
}

func (r *peerStockRepoFake) Count(context.Context) (int64, error) { return int64(len(r.stocks)), nil }

type peerTxManagerFake struct {
	err   error
	count int
}

func (m *peerTxManagerFake) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	m.count++
	if m.err != nil {
		return m.err
	}
	return fn(ctx)
}

func newPeerShareServiceForTest(
	shareRepo *peerShareRepoFake,
	ownershipRepo *peerAssetOwnershipRepoFake,
	stockRepo *peerStockRepoFake,
	txManager *peerTxManagerFake,
) *PeerOtcShareService {
	return NewPeerOtcShareService(shareRepo, ownershipRepo, stockRepo, txManager)
}

func peerStock(assetID uint, ticker string) model.Stock {
	return model.Stock{
		AssetID: assetID,
		Asset: model.Asset{
			AssetID:   assetID,
			Ticker:    ticker,
			Name:      ticker,
			AssetType: model.AssetTypeStock,
		},
	}
}

func TestPeerOtcShareServiceReserveCreatesReservationAndIncrementsReserved(t *testing.T) {
	shareRepo := newPeerShareRepoFake()
	ownershipRepo := newPeerAssetOwnershipRepoFake(&model.AssetOwnership{
		UserId:         7,
		OwnerType:      model.OwnerTypeBank,
		AssetID:        42,
		Amount:         100,
		PublicAmount:   80,
		ReservedAmount: 15,
	})
	txManager := &peerTxManagerFake{}
	svc := newPeerShareServiceForTest(
		shareRepo,
		ownershipRepo,
		&peerStockRepoFake{stocks: []model.Stock{peerStock(42, "AAPL")}},
		txManager,
	)

	status, err := svc.Reserve(context.Background(), " contract-1 ", 7, " aapl ", 25, "EMPLOYEE")
	if err != nil {
		t.Fatalf("Reserve returned error: %v", err)
	}
	if status != string(model.PeerOtcShareReservationActive) {
		t.Fatalf("status = %q", status)
	}

	reservation := shareRepo.reservations["contract-1"]
	if reservation == nil {
		t.Fatal("expected reservation to be created")
	}
	if reservation.SellerID != 7 || reservation.OwnerType != model.OwnerTypeBank || reservation.StockAssetID != 42 || reservation.ReservedAmount != 25 {
		t.Fatalf("unexpected reservation %#v", reservation)
	}
	ownership := ownershipRepo.ownerships[peerOwnershipKey(7, model.OwnerTypeBank, 42)]
	if ownership.ReservedAmount != 40 {
		t.Fatalf("reserved amount = %v, want 40", ownership.ReservedAmount)
	}
	if txManager.count != 1 || ownershipRepo.upsertCnt != 1 {
		t.Fatalf("tx count=%d upsert count=%d", txManager.count, ownershipRepo.upsertCnt)
	}
}

func TestPeerOtcShareServiceReserveIsIdempotentForSameContract(t *testing.T) {
	shareRepo := newPeerShareRepoFake()
	shareRepo.reservations["contract-1"] = &model.PeerOtcShareReservation{
		ContractID:     "contract-1",
		SellerID:       7,
		OwnerType:      model.OwnerTypeClient,
		StockAssetID:   42,
		ReservedAmount: 25,
		Status:         model.PeerOtcShareReservationReleased,
	}
	ownershipRepo := newPeerAssetOwnershipRepoFake(&model.AssetOwnership{
		UserId:       7,
		OwnerType:    model.OwnerTypeClient,
		AssetID:      42,
		Amount:       100,
		PublicAmount: 80,
	})
	svc := newPeerShareServiceForTest(
		shareRepo,
		ownershipRepo,
		&peerStockRepoFake{stocks: []model.Stock{peerStock(42, "AAPL")}},
		&peerTxManagerFake{},
	)

	status, err := svc.Reserve(context.Background(), "contract-1", 7, "AAPL", 25, "CLIENT")
	if err != nil {
		t.Fatalf("Reserve returned error: %v", err)
	}
	if status != string(model.PeerOtcShareReservationReleased) {
		t.Fatalf("status = %q", status)
	}
	if ownershipRepo.upsertCnt != 0 {
		t.Fatalf("idempotent reserve should not upsert ownership, got %d", ownershipRepo.upsertCnt)
	}

	_, err = svc.Reserve(context.Background(), "contract-1", 7, "AAPL", 26, "CLIENT")
	if err == nil {
		t.Fatal("expected conflict for different reservation details")
	}
}

func TestPeerOtcShareServiceReserveRejectsInvalidOrUnavailableShares(t *testing.T) {
	svc := newPeerShareServiceForTest(
		newPeerShareRepoFake(),
		newPeerAssetOwnershipRepoFake(),
		&peerStockRepoFake{stocks: []model.Stock{peerStock(42, "AAPL")}},
		&peerTxManagerFake{},
	)

	if _, err := svc.Reserve(context.Background(), "", 7, "AAPL", 1, "CLIENT"); err == nil {
		t.Fatal("expected validation error")
	}
	if _, err := svc.Reserve(context.Background(), "contract-1", 7, "MSFT", 1, "CLIENT"); err == nil {
		t.Fatal("expected missing stock error")
	}
	if _, err := svc.Reserve(context.Background(), "contract-1", 7, "AAPL", 1, "CLIENT"); err == nil {
		t.Fatal("expected missing ownership error")
	}

	ownershipRepo := newPeerAssetOwnershipRepoFake(&model.AssetOwnership{
		UserId:         7,
		OwnerType:      model.OwnerTypeClient,
		AssetID:        42,
		Amount:         30,
		PublicAmount:   20,
		ReservedAmount: 15,
	})
	svc = newPeerShareServiceForTest(
		newPeerShareRepoFake(),
		ownershipRepo,
		&peerStockRepoFake{stocks: []model.Stock{peerStock(42, "AAPL")}},
		&peerTxManagerFake{},
	)
	if _, err := svc.Reserve(context.Background(), "contract-1", 7, "AAPL", 10, "CLIENT"); err == nil {
		t.Fatal("expected insufficient public shares error")
	}
}

func TestPeerOtcShareServiceReleaseTransitionsActiveReservation(t *testing.T) {
	shareRepo := newPeerShareRepoFake()
	shareRepo.reservations["contract-1"] = &model.PeerOtcShareReservation{
		ContractID:     "contract-1",
		SellerID:       7,
		OwnerType:      model.OwnerTypeClient,
		StockAssetID:   42,
		ReservedAmount: 25,
		Status:         model.PeerOtcShareReservationActive,
	}
	ownershipRepo := newPeerAssetOwnershipRepoFake(&model.AssetOwnership{
		UserId:         7,
		OwnerType:      model.OwnerTypeClient,
		AssetID:        42,
		Amount:         100,
		PublicAmount:   80,
		ReservedAmount: 15,
	})
	svc := newPeerShareServiceForTest(shareRepo, ownershipRepo, &peerStockRepoFake{}, &peerTxManagerFake{})

	status, err := svc.Release(context.Background(), " contract-1 ")
	if err != nil {
		t.Fatalf("Release returned error: %v", err)
	}
	if status != string(model.PeerOtcShareReservationReleased) {
		t.Fatalf("status = %q", status)
	}
	if shareRepo.reservations["contract-1"].Status != model.PeerOtcShareReservationReleased {
		t.Fatalf("reservation status = %s", shareRepo.reservations["contract-1"].Status)
	}
	if ownershipRepo.ownerships[peerOwnershipKey(7, model.OwnerTypeClient, 42)].ReservedAmount != 0 {
		t.Fatalf("reserved amount should not go below zero")
	}
}

func TestPeerOtcShareServiceReleaseRejectsMissingOrConsumedReservation(t *testing.T) {
	svc := newPeerShareServiceForTest(newPeerShareRepoFake(), newPeerAssetOwnershipRepoFake(), &peerStockRepoFake{}, &peerTxManagerFake{})
	if _, err := svc.Release(context.Background(), ""); err == nil {
		t.Fatal("expected validation error")
	}
	if _, err := svc.Release(context.Background(), "missing"); err == nil {
		t.Fatal("expected not found error")
	}

	shareRepo := newPeerShareRepoFake()
	shareRepo.reservations["contract-1"] = &model.PeerOtcShareReservation{
		ContractID: "contract-1",
		Status:     model.PeerOtcShareReservationConsumed,
	}
	svc = newPeerShareServiceForTest(shareRepo, newPeerAssetOwnershipRepoFake(), &peerStockRepoFake{}, &peerTxManagerFake{})
	if _, err := svc.Release(context.Background(), "contract-1"); err == nil {
		t.Fatal("expected consumed release error")
	}
}

func TestPeerOtcShareServiceConsumeTransitionsActiveReservation(t *testing.T) {
	shareRepo := newPeerShareRepoFake()
	shareRepo.reservations["contract-1"] = &model.PeerOtcShareReservation{
		ContractID:     "contract-1",
		SellerID:       7,
		OwnerType:      model.OwnerTypeClient,
		StockAssetID:   42,
		ReservedAmount: 25,
		Status:         model.PeerOtcShareReservationActive,
	}
	ownershipRepo := newPeerAssetOwnershipRepoFake(&model.AssetOwnership{
		UserId:         7,
		OwnerType:      model.OwnerTypeClient,
		AssetID:        42,
		Amount:         100,
		PublicAmount:   80,
		ReservedAmount: 40,
	})
	svc := newPeerShareServiceForTest(shareRepo, ownershipRepo, &peerStockRepoFake{}, &peerTxManagerFake{})

	status, err := svc.Consume(context.Background(), "contract-1")
	if err != nil {
		t.Fatalf("Consume returned error: %v", err)
	}
	if status != string(model.PeerOtcShareReservationConsumed) {
		t.Fatalf("status = %q", status)
	}
	ownership := ownershipRepo.ownerships[peerOwnershipKey(7, model.OwnerTypeClient, 42)]
	if ownership.Amount != 75 || ownership.PublicAmount != 55 || ownership.ReservedAmount != 15 {
		t.Fatalf("unexpected ownership after consume %#v", ownership)
	}
}

func TestPeerOtcShareServiceConsumeRejectsReleasedOrInsufficientReservation(t *testing.T) {
	shareRepo := newPeerShareRepoFake()
	shareRepo.reservations["released"] = &model.PeerOtcShareReservation{
		ContractID: "released",
		Status:     model.PeerOtcShareReservationReleased,
	}
	svc := newPeerShareServiceForTest(shareRepo, newPeerAssetOwnershipRepoFake(), &peerStockRepoFake{}, &peerTxManagerFake{})
	if _, err := svc.Consume(context.Background(), "released"); err == nil {
		t.Fatal("expected released consume error")
	}

	shareRepo.reservations["missing-ownership"] = &model.PeerOtcShareReservation{
		ContractID:     "missing-ownership",
		SellerID:       7,
		OwnerType:      model.OwnerTypeClient,
		StockAssetID:   42,
		ReservedAmount: 25,
		Status:         model.PeerOtcShareReservationActive,
	}
	if _, err := svc.Consume(context.Background(), "missing-ownership"); err == nil {
		t.Fatal("expected missing ownership error")
	}

	ownershipRepo := newPeerAssetOwnershipRepoFake(&model.AssetOwnership{
		UserId:         7,
		OwnerType:      model.OwnerTypeClient,
		AssetID:        42,
		Amount:         100,
		PublicAmount:   80,
		ReservedAmount: 10,
	})
	svc = newPeerShareServiceForTest(shareRepo, ownershipRepo, &peerStockRepoFake{}, &peerTxManagerFake{})
	if _, err := svc.Consume(context.Background(), "missing-ownership"); err == nil {
		t.Fatal("expected insufficient reserved shares error")
	}
}

func TestPeerOtcShareServiceCreditCreatesOrUpdatesOwnership(t *testing.T) {
	shareRepo := newPeerShareRepoFake()
	ownershipRepo := newPeerAssetOwnershipRepoFake(&model.AssetOwnership{
		UserId:         9,
		OwnerType:      model.OwnerTypeClient,
		AssetID:        42,
		Amount:         10,
		AvgBuyPriceRSD: 100,
		PublicAmount:   0,
	})
	svc := newPeerShareServiceForTest(
		shareRepo,
		ownershipRepo,
		&peerStockRepoFake{stocks: []model.Stock{peerStock(42, "AAPL")}},
		&peerTxManagerFake{},
	)

	status, err := svc.Credit(context.Background(), "contract-1", 9, "AAPL", 5, 130, "CLIENT")
	if err != nil {
		t.Fatalf("Credit returned error: %v", err)
	}
	if status != "CREDITED" {
		t.Fatalf("status = %q", status)
	}
	ownership := ownershipRepo.ownerships[peerOwnershipKey(9, model.OwnerTypeClient, 42)]
	if ownership.Amount != 15 || ownership.AvgBuyPriceRSD != 110 {
		t.Fatalf("unexpected weighted ownership %#v", ownership)
	}
	if credit := shareRepo.credits["contract-1"]; credit == nil || credit.BuyerID != 9 || credit.Amount != 5 {
		t.Fatalf("unexpected credit %#v", credit)
	}

	status, err = svc.Credit(context.Background(), "contract-2", 10, "AAPL", 2, 200, "EMPLOYEE")
	if err != nil {
		t.Fatalf("Credit new ownership returned error: %v", err)
	}
	if status != "CREDITED" {
		t.Fatalf("status = %q", status)
	}
	newOwnership := ownershipRepo.ownerships[peerOwnershipKey(10, model.OwnerTypeBank, 42)]
	if newOwnership == nil || newOwnership.Amount != 2 || newOwnership.AvgBuyPriceRSD != 200 {
		t.Fatalf("unexpected new ownership %#v", newOwnership)
	}
}

func TestPeerOtcShareServiceCreditIsIdempotentAndRejectsDifferentDetails(t *testing.T) {
	shareRepo := newPeerShareRepoFake()
	shareRepo.credits["contract-1"] = &model.PeerOtcShareCredit{
		ContractID:   "contract-1",
		BuyerID:      9,
		StockAssetID: 42,
		Amount:       5,
	}
	ownershipRepo := newPeerAssetOwnershipRepoFake()
	svc := newPeerShareServiceForTest(
		shareRepo,
		ownershipRepo,
		&peerStockRepoFake{stocks: []model.Stock{peerStock(42, "AAPL")}},
		&peerTxManagerFake{},
	)

	status, err := svc.Credit(context.Background(), "contract-1", 9, "AAPL", 5, 130, "CLIENT")
	if err != nil {
		t.Fatalf("Credit returned error: %v", err)
	}
	if status != "CREDITED" {
		t.Fatalf("status = %q", status)
	}
	if ownershipRepo.upsertCnt != 0 {
		t.Fatalf("idempotent credit should not upsert ownership, got %d", ownershipRepo.upsertCnt)
	}

	_, err = svc.Credit(context.Background(), "contract-1", 9, "AAPL", 6, 130, "CLIENT")
	if err == nil {
		t.Fatal("expected credit conflict")
	}
}

func TestPeerOtcShareServiceWrapsRepositoryAndTransactionErrors(t *testing.T) {
	stockRepo := &peerStockRepoFake{stocks: []model.Stock{peerStock(42, "AAPL")}}
	shareRepo := newPeerShareRepoFake()
	shareRepo.findReservationErr = errors.New("db down")
	svc := newPeerShareServiceForTest(
		shareRepo,
		newPeerAssetOwnershipRepoFake(),
		stockRepo,
		&peerTxManagerFake{},
	)
	if _, err := svc.Reserve(context.Background(), "contract-1", 7, "AAPL", 1, "CLIENT"); err == nil {
		t.Fatal("expected reservation lookup error")
	}

	svc = newPeerShareServiceForTest(
		newPeerShareRepoFake(),
		newPeerAssetOwnershipRepoFake(),
		stockRepo,
		&peerTxManagerFake{err: errors.New("tx failed")},
	)
	if _, err := svc.Credit(context.Background(), "contract-1", 7, "AAPL", 1, 100, "CLIENT"); err == nil {
		t.Fatal("expected transaction error")
	}
}
