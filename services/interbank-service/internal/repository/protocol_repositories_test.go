package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/interbank-service/internal/model"
)

func setupProtocolRepoDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	database, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := database.AutoMigrate(
		&model.InboundMessage{},
		&model.PreparedTransaction{},
		&model.OutboundMessage{},
		&model.PeerNegotiation{},
		&model.PeerContract{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	return database
}

func TestInboundMessageRepositorySaveFindAndUpdate(t *testing.T) {
	t.Parallel()

	repo := NewInboundMessageRepository(setupProtocolRepoDB(t))
	ctx := context.Background()

	missing, err := repo.FindByKey(ctx, 111, "missing")
	if err != nil {
		t.Fatalf("find missing: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil missing message, got %#v", missing)
	}

	msg := &model.InboundMessage{
		PeerRoutingNumber:   111,
		LocallyGeneratedKey: "key-1",
		MessageType:         "NEW_TX",
		RequestBody:         []byte(`{"request":1}`),
		ResponseStatus:      202,
		ResponseBody:        []byte(`{"pending":true}`),
		ProcessedAt:         time.Now(),
	}
	if err := repo.Save(ctx, msg); err != nil {
		t.Fatalf("save inbound: %v", err)
	}

	msg.ResponseStatus = 200
	msg.ResponseBody = []byte(`{"vote":"YES"}`)
	if err := repo.Save(ctx, msg); err != nil {
		t.Fatalf("update inbound: %v", err)
	}

	found, err := repo.FindByKey(ctx, 111, "key-1")
	if err != nil {
		t.Fatalf("find inbound: %v", err)
	}
	if found.ResponseStatus != 200 || string(found.ResponseBody) != `{"vote":"YES"}` {
		t.Fatalf("unexpected inbound message %#v", found)
	}
}

func TestPreparedTransactionRepositoryCreateFindAndUpdate(t *testing.T) {
	t.Parallel()

	repo := NewPreparedTransactionRepository(setupProtocolRepoDB(t))
	ctx := context.Background()

	tx := &model.PreparedTransaction{
		RoutingNumber: 111,
		ID:            "tx-1",
		Status:        model.PreparedTransactionPreparing,
		RequestBody:   []byte(`{"transaction":1}`),
	}
	if err := repo.Create(ctx, tx); err != nil {
		t.Fatalf("create prepared: %v", err)
	}

	found, err := repo.FindByID(ctx, 111, "tx-1")
	if err != nil {
		t.Fatalf("find prepared: %v", err)
	}
	if found == nil || found.Status != model.PreparedTransactionPreparing {
		t.Fatalf("unexpected prepared transaction %#v", found)
	}

	found.Status = model.PreparedTransactionPrepared
	if err := repo.Update(ctx, found); err != nil {
		t.Fatalf("update prepared: %v", err)
	}
	updated, err := repo.FindByID(ctx, 111, "tx-1")
	if err != nil {
		t.Fatalf("find updated prepared: %v", err)
	}
	if updated.Status != model.PreparedTransactionPrepared {
		t.Fatalf("status = %q, want prepared", updated.Status)
	}

	missing, err := repo.FindByID(ctx, 222, "tx-1")
	if err != nil {
		t.Fatalf("find missing prepared: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil missing prepared, got %#v", missing)
	}
}

func TestOutboundMessageRepositoryLifecycle(t *testing.T) {
	t.Parallel()

	database := setupProtocolRepoDB(t)
	repo := NewOutboundMessageRepository(database)
	ctx := context.Background()

	msg := &model.OutboundMessage{
		PeerRoutingNumber:   111,
		MessageType:         "NEW_TX",
		IdempotenceKeyLocal: "out-1",
		Payload:             []byte(`{"messageType":"NEW_TX"}`),
		FlowType:            model.FlowTypePayment,
		BankingTxID:         42,
	}
	if err := repo.Enqueue(ctx, msg); err != nil {
		t.Fatalf("enqueue outbound: %v", err)
	}
	if msg.ID == 0 || msg.Status != model.OutboundPending || msg.NextRetryAt.IsZero() {
		t.Fatalf("defaults not applied to outbound %#v", msg)
	}

	duplicate := *msg
	duplicate.ID = 0
	if err := repo.Enqueue(ctx, &duplicate); err != nil {
		t.Fatalf("enqueue duplicate outbound: %v", err)
	}

	var count int64
	if err := database.Model(&model.OutboundMessage{}).Count(&count).Error; err != nil {
		t.Fatalf("count outbound: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected duplicate enqueue to be ignored, got %d rows", count)
	}

	batch, err := repo.NextBatch(ctx, 10)
	if err != nil {
		t.Fatalf("next batch: %v", err)
	}
	if len(batch) != 1 || batch[0].IdempotenceKeyLocal != "out-1" {
		t.Fatalf("unexpected batch %#v", batch)
	}

	if err := repo.MarkSent(ctx, msg.ID, 204, []byte("ok")); err != nil {
		t.Fatalf("mark sent: %v", err)
	}
	var sent model.OutboundMessage
	if err := database.First(&sent, msg.ID).Error; err != nil {
		t.Fatalf("find sent: %v", err)
	}
	if sent.Status != model.OutboundSent || sent.LastResponseStatus != 204 || string(sent.LastResponseBody) != "ok" {
		t.Fatalf("unexpected sent row %#v", sent)
	}

	pending := &model.OutboundMessage{
		PeerRoutingNumber:   222,
		MessageType:         "ROLLBACK_TX",
		IdempotenceKeyLocal: "out-2",
		Payload:             []byte(`{}`),
		FlowType:            model.FlowTypeOTC,
	}
	if err := repo.Enqueue(ctx, pending); err != nil {
		t.Fatalf("enqueue second outbound: %v", err)
	}
	nextRetry := time.Now().Add(time.Hour)
	if err := repo.Reschedule(ctx, pending.ID, 3, "peer down", 503, []byte("down"), nextRetry); err != nil {
		t.Fatalf("reschedule: %v", err)
	}
	if err := repo.MarkFailed(ctx, pending.ID, "too many attempts"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	if err := repo.Cancel(ctx, pending.ID); err != nil {
		t.Fatalf("cancel failed row: %v", err)
	}
	var failed model.OutboundMessage
	if err := database.First(&failed, pending.ID).Error; err != nil {
		t.Fatalf("find failed: %v", err)
	}
	if failed.Status != model.OutboundFailed || failed.Attempts != 3 || failed.LastError != "too many attempts" {
		t.Fatalf("unexpected failed row %#v", failed)
	}

	cancelMe := &model.OutboundMessage{
		PeerRoutingNumber:   333,
		MessageType:         "COMMIT_TX",
		IdempotenceKeyLocal: "out-3",
		Payload:             []byte(`{}`),
		FlowType:            model.FlowTypeOTC,
	}
	if err := repo.Enqueue(ctx, cancelMe); err != nil {
		t.Fatalf("enqueue cancellable outbound: %v", err)
	}
	if err := repo.Cancel(ctx, cancelMe.ID); err != nil {
		t.Fatalf("cancel pending row: %v", err)
	}
	var canceled model.OutboundMessage
	if err := database.First(&canceled, cancelMe.ID).Error; err != nil {
		t.Fatalf("find canceled: %v", err)
	}
	if canceled.Status != model.OutboundCanceled {
		t.Fatalf("status = %q, want canceled", canceled.Status)
	}
}

func TestPeerNegotiationRepositoryQueries(t *testing.T) {
	t.Parallel()

	repo := NewPeerNegotiationRepository(setupProtocolRepoDB(t))
	ctx := context.Background()
	now := time.Now()

	rows := []model.PeerNegotiation{
		{ID: "neg-1", BuyerRoutingNumber: 444, BuyerID: "9", SellerRoutingNumber: 111, SellerID: "seller", Ticker: "AAPL", Amount: 2, PricePerStock: 100, PriceCurrency: "RSD", Premium: 5, PremiumCurrency: "RSD", SettlementDate: "2030-01-01", BuyerAccountNumber: "444000000000000011", LastModifiedByRouting: 444, LastModifiedByID: "9", Status: model.PeerNegotiationOngoing, UpdatedAt: now.Add(time.Hour)},
		{ID: "neg-2", BuyerRoutingNumber: 222, BuyerID: "buyer", SellerRoutingNumber: 444, SellerID: "7", Ticker: "MSFT", Amount: 3, PricePerStock: 200, PriceCurrency: "RSD", Premium: 8, PremiumCurrency: "RSD", SettlementDate: "2030-02-01", BuyerAccountNumber: "222000000000000011", LastModifiedByRouting: 222, LastModifiedByID: "buyer", Status: model.PeerNegotiationCancelled, UpdatedAt: now},
	}
	for i := range rows {
		if err := repo.Create(ctx, &rows[i]); err != nil {
			t.Fatalf("create negotiation %d: %v", i, err)
		}
	}

	found, err := repo.FindByID(ctx, 111, "neg-1")
	if err != nil {
		t.Fatalf("find negotiation: %v", err)
	}
	if found == nil || found.Ticker != "AAPL" {
		t.Fatalf("unexpected negotiation %#v", found)
	}
	found.PricePerStock = 90
	if err := repo.Update(ctx, found); err != nil {
		t.Fatalf("update negotiation: %v", err)
	}
	locked, err := repo.FindByIDForUpdate(ctx, 111, "neg-1")
	if err != nil {
		t.Fatalf("find for update: %v", err)
	}
	if locked.PricePerStock != 90 {
		t.Fatalf("price = %.2f, want 90", locked.PricePerStock)
	}

	partyRows, err := repo.ListByParty(ctx, 444, "9")
	if err != nil {
		t.Fatalf("list by party: %v", err)
	}
	if len(partyRows) != 1 || partyRows[0].ID != "neg-1" {
		t.Fatalf("unexpected party rows %#v", partyRows)
	}

	ongoing, err := repo.FindOngoing(ctx)
	if err != nil {
		t.Fatalf("find ongoing: %v", err)
	}
	if len(ongoing) != 1 || ongoing[0].ID != "neg-1" {
		t.Fatalf("unexpected ongoing rows %#v", ongoing)
	}
}

func TestPeerContractRepositoryQueries(t *testing.T) {
	t.Parallel()

	repo := NewPeerContractRepository(setupProtocolRepoDB(t))
	ctx := context.Background()
	now := time.Now()

	contracts := []model.PeerContract{
		{AuthorityRoutingNumber: 111, ID: "contract-1", NegotiationID: "neg-1", BuyerRoutingNumber: 444, BuyerID: "9", SellerRoutingNumber: 111, SellerID: "seller", Ticker: "AAPL", Amount: 2, StrikePrice: 100, StrikeCurrency: "RSD", Premium: 5, PremiumCurrency: "RSD", SettlementDate: "2030-01-01", Status: model.PeerContractActive, CreatedAt: now, UpdatedAt: now.Add(time.Hour)},
		{AuthorityRoutingNumber: 444, ID: "contract-2", NegotiationID: "neg-2", BuyerRoutingNumber: 222, BuyerID: "buyer", SellerRoutingNumber: 444, SellerID: "7", Ticker: "MSFT", Amount: 3, StrikePrice: 200, StrikeCurrency: "RSD", Premium: 8, PremiumCurrency: "RSD", SettlementDate: "2030-02-01", Status: model.PeerContractCancelled, CreatedAt: now, UpdatedAt: now},
	}
	for i := range contracts {
		if err := repo.Create(ctx, &contracts[i]); err != nil {
			t.Fatalf("create contract %d: %v", i, err)
		}
	}

	found, err := repo.FindByID(ctx, 111, "contract-1")
	if err != nil {
		t.Fatalf("find contract: %v", err)
	}
	if found == nil || found.Ticker != "AAPL" {
		t.Fatalf("unexpected contract %#v", found)
	}
	byNegotiation, err := repo.FindByNegotiationID(ctx, 444, "neg-2")
	if err != nil {
		t.Fatalf("find by negotiation: %v", err)
	}
	if byNegotiation == nil || byNegotiation.ID != "contract-2" {
		t.Fatalf("unexpected contract by negotiation %#v", byNegotiation)
	}

	forBuyer, err := repo.ListByParty(ctx, 444, "9")
	if err != nil {
		t.Fatalf("list by buyer: %v", err)
	}
	if len(forBuyer) != 1 || forBuyer[0].ID != "contract-1" {
		t.Fatalf("unexpected buyer contracts %#v", forBuyer)
	}

	found.Status = model.PeerContractExercised
	exercisedAt := now.Add(2 * time.Hour)
	found.ExercisedAt = &exercisedAt
	if err := repo.Update(ctx, found); err != nil {
		t.Fatalf("update contract: %v", err)
	}
	active, err := repo.FindActive(ctx)
	if err != nil {
		t.Fatalf("find active: %v", err)
	}
	if len(active) != 0 {
		t.Fatalf("expected no active contracts, got %#v", active)
	}

	missing, err := repo.FindByID(ctx, 999, "missing")
	if err != nil {
		t.Fatalf("find missing contract: %v", err)
	}
	if missing != nil {
		t.Fatalf("expected missing contract nil, got %#v", missing)
	}
}
