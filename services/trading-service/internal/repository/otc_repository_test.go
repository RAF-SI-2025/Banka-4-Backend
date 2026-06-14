package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

func setupOtcRepoDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(
		&model.Asset{},
		&model.Exchange{},
		&model.Listing{},
		&model.Stock{},
		&model.OtcOffer{},
		&model.OtcOptionContract{},
		&model.OtcExecutionSaga{},
		&model.OtcShareReservation{},
		&model.Order{},
		&model.OrderTransaction{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	return db
}

func createRepoContract(t *testing.T, db *gorm.DB, stock model.Stock, offerID uint, status model.OtcOptionContractStatus, settlementDate time.Time) model.OtcOptionContract {
	t.Helper()

	contract := model.OtcOptionContract{
		OtcOfferID:          offerID,
		BuyerID:             10 + offerID,
		SellerID:            20,
		StockAssetID:        stock.AssetID,
		Amount:              5,
		StrikePriceRSD:      100,
		PremiumRSD:          10,
		SettlementDate:      settlementDate,
		BuyerAccountNumber:  "buyer",
		SellerAccountNumber: "seller",
		Status:              status,
	}
	if err := db.Create(&contract).Error; err != nil {
		t.Fatalf("create contract: %v", err)
	}
	return contract
}

func createRepoStock(t *testing.T, db *gorm.DB, ticker string) model.Stock {
	t.Helper()

	asset := model.Asset{Ticker: ticker, Name: ticker + " Inc", AssetType: model.AssetTypeStock}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}

	stock := model.Stock{AssetID: asset.AssetID, OutstandingShares: 1000}
	if err := db.Create(&stock).Error; err != nil {
		t.Fatalf("create stock: %v", err)
	}
	stock.Asset = asset
	return stock
}

func TestOtcOfferRepositoryCreateFindAndSave(t *testing.T) {
	t.Parallel()

	db := setupOtcRepoDB(t)
	stock := createRepoStock(t, db, "AAPL")
	repo := NewOtcOfferRepository(db)
	ctx := context.Background()
	now := time.Now()
	sellerAccount := "444000100000000001"

	offer := &model.OtcOffer{
		BuyerID:             10,
		SellerID:            20,
		StockAssetID:        stock.AssetID,
		Amount:              5,
		PricePerStockRSD:    100,
		PremiumRSD:          10,
		SettlementDate:      now.Add(24 * time.Hour),
		BuyerAccountNumber:  "444000100000000000",
		SellerAccountNumber: &sellerAccount,
		Status:              model.OtcOfferStatusActive,
		LastModified:        now,
		ModifiedBy:          10,
	}

	if err := repo.Create(ctx, offer); err != nil {
		t.Fatalf("create offer: %v", err)
	}

	found, err := repo.FindByID(ctx, offer.OtcOfferID)
	if err != nil {
		t.Fatalf("find offer: %v", err)
	}
	if found == nil || found.Stock.Asset.Ticker != "AAPL" {
		t.Fatalf("unexpected found offer %#v", found)
	}

	found.Status = model.OtcOfferStatusAccepted
	if err := repo.Save(ctx, found); err != nil {
		t.Fatalf("save offer: %v", err)
	}

	updated, err := repo.FindByIDForUpdate(ctx, offer.OtcOfferID)
	if err != nil {
		t.Fatalf("find offer for update: %v", err)
	}
	if updated.Status != model.OtcOfferStatusAccepted {
		t.Fatalf("status = %q, want accepted", updated.Status)
	}

	missing, err := repo.FindByID(ctx, 9999)
	if err != nil {
		t.Fatalf("find missing offer: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected missing offer nil, got %#v", missing)
	}
}

func TestOtcOfferRepositoryFiltersActiveOffers(t *testing.T) {
	t.Parallel()

	db := setupOtcRepoDB(t)
	stock := createRepoStock(t, db, "MSFT")
	repo := NewOtcOfferRepository(db)
	ctx := context.Background()
	now := time.Now()

	offers := []model.OtcOffer{
		{BuyerID: 1, SellerID: 2, StockAssetID: stock.AssetID, Amount: 1, PricePerStockRSD: 100, PremiumRSD: 1, SettlementDate: now, BuyerAccountNumber: "b1", Status: model.OtcOfferStatusActive, LastModified: now.Add(2 * time.Hour), ModifiedBy: 1},
		{BuyerID: 3, SellerID: 2, StockAssetID: stock.AssetID, Amount: 1, PricePerStockRSD: 100, PremiumRSD: 1, SettlementDate: now, BuyerAccountNumber: "b2", Status: model.OtcOfferStatusActive, LastModified: now.Add(time.Hour), ModifiedBy: 3},
		{BuyerID: 1, SellerID: 4, StockAssetID: stock.AssetID, Amount: 1, PricePerStockRSD: 100, PremiumRSD: 1, SettlementDate: now, BuyerAccountNumber: "b3", Status: model.OtcOfferStatusRejected, LastModified: now, ModifiedBy: 1},
	}
	for i := range offers {
		if err := db.Create(&offers[i]).Error; err != nil {
			t.Fatalf("create offer %d: %v", i, err)
		}
	}

	activeForUser, err := repo.FindActiveForUser(ctx, 1)
	if err != nil {
		t.Fatalf("find active for user: %v", err)
	}
	if len(activeForUser) != 1 || activeForUser[0].BuyerID != 1 {
		t.Fatalf("unexpected active offers %#v", activeForUser)
	}

	exclude := offers[0].OtcOfferID
	activeBySeller, err := repo.FindActiveBySellerAndStock(ctx, 2, stock.AssetID, &exclude)
	if err != nil {
		t.Fatalf("find active by seller and stock: %v", err)
	}
	if len(activeBySeller) != 1 || activeBySeller[0].OtcOfferID == exclude {
		t.Fatalf("unexpected seller offers %#v", activeBySeller)
	}
}

func TestOtcOptionContractRepositoryQueries(t *testing.T) {
	t.Parallel()

	db := setupOtcRepoDB(t)
	stock := createRepoStock(t, db, "GOOG")
	repo := NewOtcOptionContractRepository(db)
	ctx := context.Background()
	now := time.Now()

	contracts := []model.OtcOptionContract{
		{OtcOfferID: 1, BuyerID: 10, SellerID: 20, StockAssetID: stock.AssetID, Amount: 5, StrikePriceRSD: 100, PremiumRSD: 10, SettlementDate: now.Add(-time.Hour), BuyerAccountNumber: "b1", SellerAccountNumber: "s1", Status: model.OtcOptionContractStatusActive},
		{OtcOfferID: 2, BuyerID: 11, SellerID: 20, StockAssetID: stock.AssetID, Amount: 3, StrikePriceRSD: 120, PremiumRSD: 12, SettlementDate: now.Add(24 * time.Hour), BuyerAccountNumber: "b2", SellerAccountNumber: "s2", Status: model.OtcOptionContractStatusActive},
		{OtcOfferID: 3, BuyerID: 12, SellerID: 20, StockAssetID: stock.AssetID, Amount: 4, StrikePriceRSD: 130, PremiumRSD: 13, SettlementDate: now.Add(-2 * time.Hour), BuyerAccountNumber: "b3", SellerAccountNumber: "s3", Status: model.OtcOptionContractStatusExercised},
	}
	for i := range contracts {
		if err := repo.Create(ctx, &contracts[i]); err != nil {
			t.Fatalf("create contract %d: %v", i, err)
		}
	}

	byOffer, err := repo.FindByOfferID(ctx, 2)
	if err != nil {
		t.Fatalf("find by offer: %v", err)
	}
	if byOffer == nil || byOffer.BuyerID != 11 {
		t.Fatalf("unexpected contract by offer %#v", byOffer)
	}

	forUser, err := repo.FindForUser(ctx, 10)
	if err != nil {
		t.Fatalf("find for user: %v", err)
	}
	if len(forUser) != 1 || forUser[0].BuyerID != 10 {
		t.Fatalf("unexpected user contracts %#v", forUser)
	}

	active, err := repo.FindActiveBySellerAndStock(ctx, 20, stock.AssetID, now)
	if err != nil {
		t.Fatalf("find active by seller and stock: %v", err)
	}
	if len(active) != 1 || active[0].OtcOfferID != 2 {
		t.Fatalf("unexpected active contracts %#v", active)
	}

	expired, err := repo.FindExpiredActive(ctx, now, 10)
	if err != nil {
		t.Fatalf("find expired active: %v", err)
	}
	if len(expired) != 1 || expired[0].OtcOfferID != 1 {
		t.Fatalf("unexpected expired contracts %#v", expired)
	}

	alsoExpired, err := repo.FindExpiringContracts(ctx, now)
	if err != nil {
		t.Fatalf("find expiring contracts: %v", err)
	}
	if len(alsoExpired) != 1 || alsoExpired[0].OtcOfferID != 1 {
		t.Fatalf("unexpected expiring contracts %#v", alsoExpired)
	}

	missing, err := repo.FindByOfferID(ctx, 9999)
	if err != nil {
		t.Fatalf("find missing by offer: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil missing contract, got %#v", missing)
	}
}

func TestOtcExecutionSagaRepositoryQueries(t *testing.T) {
	t.Parallel()

	db := setupOtcRepoDB(t)
	stock := createRepoStock(t, db, "TSLA")
	contract := createRepoContract(t, db, stock, 101, model.OtcOptionContractStatusActive, time.Now().Add(24*time.Hour))
	repo := NewOtcExecutionSagaRepository(db)
	ctx := context.Background()
	now := time.Now()
	retryAt := now.Add(-time.Minute)
	futureRetry := now.Add(time.Hour)

	sagas := []model.OtcExecutionSaga{
		{ContractID: contract.OtcOptionContractID, ExecutionKey: "exec-1", CurrentStep: model.OtcExecutionStepInit, Status: model.OtcExecutionStatusInProgress, NextRetryAt: &retryAt},
		{ContractID: contract.OtcOptionContractID + 100, ExecutionKey: "exec-2", CurrentStep: model.OtcExecutionStepFundsReserved, Status: model.OtcExecutionStatusCompleted},
		{ContractID: contract.OtcOptionContractID + 200, ExecutionKey: "exec-3", CurrentStep: model.OtcExecutionStepFundsReserved, Status: model.OtcExecutionStatusCompensating, NextRetryAt: &futureRetry},
	}
	for i := range sagas {
		if i > 0 {
			other := createRepoContract(t, db, stock, uint(102+i), model.OtcOptionContractStatusActive, now.Add(24*time.Hour))
			sagas[i].ContractID = other.OtcOptionContractID
		}
		if err := repo.Create(ctx, &sagas[i]); err != nil {
			t.Fatalf("create saga %d: %v", i, err)
		}
	}

	found, err := repo.FindByID(ctx, sagas[0].OtcExecutionSagaID)
	if err != nil {
		t.Fatalf("find saga: %v", err)
	}
	if found == nil || found.Contract.Stock.Asset.Ticker != "TSLA" {
		t.Fatalf("unexpected found saga %#v", found)
	}

	byContract, err := repo.FindByContractIDForUpdate(ctx, contract.OtcOptionContractID)
	if err != nil {
		t.Fatalf("find saga by contract: %v", err)
	}
	if byContract == nil || byContract.ExecutionKey != "exec-1" {
		t.Fatalf("unexpected saga by contract %#v", byContract)
	}

	pending, err := repo.FindPendingForExecution(ctx, now, 10)
	if err != nil {
		t.Fatalf("find pending sagas: %v", err)
	}
	if len(pending) != 1 || pending[0].ExecutionKey != "exec-1" {
		t.Fatalf("unexpected pending sagas %#v", pending)
	}

	found.Status = model.OtcExecutionStatusFailed
	found.LastError = "failed"
	if err := repo.Save(ctx, found); err != nil {
		t.Fatalf("save saga: %v", err)
	}
	updated, err := repo.FindByContractID(ctx, contract.OtcOptionContractID)
	if err != nil {
		t.Fatalf("find updated saga: %v", err)
	}
	if updated.Status != model.OtcExecutionStatusFailed || updated.LastError != "failed" {
		t.Fatalf("unexpected updated saga %#v", updated)
	}

	missing, err := repo.FindByID(ctx, 9999)
	if err != nil {
		t.Fatalf("find missing saga: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil missing saga, got %#v", missing)
	}
}

func TestOtcShareReservationRepositoryQueries(t *testing.T) {
	t.Parallel()

	db := setupOtcRepoDB(t)
	stock := createRepoStock(t, db, "NVDA")
	contract1 := createRepoContract(t, db, stock, 201, model.OtcOptionContractStatusActive, time.Now().Add(24*time.Hour))
	contract2 := createRepoContract(t, db, stock, 202, model.OtcOptionContractStatusActive, time.Now().Add(24*time.Hour))
	repo := NewOtcShareReservationRepository(db)
	ctx := context.Background()

	reservations := []model.OtcShareReservation{
		{ContractID: contract1.OtcOptionContractID, SellerID: 20, OwnerType: model.OwnerTypeClient, StockAssetID: stock.AssetID, ReservedAmount: 12, Status: model.OtcShareReservationStatusActive},
		{ContractID: contract2.OtcOptionContractID, SellerID: 20, OwnerType: model.OwnerTypeClient, StockAssetID: stock.AssetID, ReservedAmount: 8, Status: model.OtcShareReservationStatusActive},
	}
	for i := range reservations {
		if err := repo.Create(ctx, &reservations[i]); err != nil {
			t.Fatalf("create reservation %d: %v", i, err)
		}
	}

	found, err := repo.FindByContractIDForUpdate(ctx, contract1.OtcOptionContractID)
	if err != nil {
		t.Fatalf("find reservation: %v", err)
	}
	if found == nil || found.ReservedAmount != 12 {
		t.Fatalf("unexpected reservation %#v", found)
	}

	exclude := contract1.OtcOptionContractID
	total, err := repo.SumActiveReservedBySellerAsset(ctx, 20, model.OwnerTypeClient, stock.AssetID, &exclude)
	if err != nil {
		t.Fatalf("sum active reservations: %v", err)
	}
	if total != 8 {
		t.Fatalf("sum active reservations = %.2f, want 8", total)
	}

	found.Status = model.OtcShareReservationStatusReleased
	if err := repo.Save(ctx, found); err != nil {
		t.Fatalf("save reservation: %v", err)
	}
	updated, err := repo.FindByContractID(ctx, contract1.OtcOptionContractID)
	if err != nil {
		t.Fatalf("find updated reservation: %v", err)
	}
	if updated.Status != model.OtcShareReservationStatusReleased {
		t.Fatalf("status = %q, want released", updated.Status)
	}

	missing, err := repo.FindByContractID(ctx, 9999)
	if err != nil {
		t.Fatalf("find missing reservation: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil missing reservation, got %#v", missing)
	}
}

func TestOrderTransactionRepositoryCreate(t *testing.T) {
	t.Parallel()

	db := setupOtcRepoDB(t)
	repo := NewOrderTransactionRepository(db)
	ctx := context.Background()

	executedAt := time.Now()
	tx := &model.OrderTransaction{
		OrderID:      42,
		Quantity:     3,
		PricePerUnit: 100,
		TotalPrice:   300,
		Commission:   1.5,
		ExecutedAt:   executedAt,
	}
	if err := repo.Create(ctx, tx); err != nil {
		t.Fatalf("create order transaction: %v", err)
	}
	if tx.OrderTransactionID == 0 {
		t.Fatal("expected generated order transaction id")
	}

	var stored model.OrderTransaction
	if err := db.First(&stored, tx.OrderTransactionID).Error; err != nil {
		t.Fatalf("load order transaction: %v", err)
	}
	if stored.TotalPrice != 300 || stored.Commission != 1.5 {
		t.Fatalf("unexpected stored transaction %#v", stored)
	}
}
