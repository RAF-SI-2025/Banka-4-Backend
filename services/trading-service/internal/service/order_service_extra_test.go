package service

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/dto"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

// ── resolveDailyVolume Tests ──────────────────────────────────────

func TestResolveDailyVolume_ReturnsVolume(t *testing.T) {
	dailyInfo := &model.ListingDailyPriceInfo{Volume: 5000}
	listingRepo := &fakeListingRepo{dailyPriceInfo: dailyInfo}
	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, listingRepo,
		&fakeUserServiceClient{
			identityResp: &pb.GetIdentityByUserIdResponse{
				IdentityId: 5,
			},
		},
		&fakeOrderBankingClient{}, &fakeTaxRecorder{})

	vol := svc.resolveDailyVolume(context.Background(), 1)
	require.Equal(t, uint(5000), vol)
}

func TestResolveDailyVolume_NilDailyInfo_ReturnsZero(t *testing.T) {
	listingRepo := &fakeListingRepo{dailyPriceInfo: nil}
	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, listingRepo,
		&fakeUserServiceClient{
			identityResp: &pb.GetIdentityByUserIdResponse{
				IdentityId: 5,
			},
		},
		&fakeOrderBankingClient{}, &fakeTaxRecorder{})

	vol := svc.resolveDailyVolume(context.Background(), 1)
	require.Equal(t, uint(0), vol)
}

func TestResolveDailyVolume_ZeroVolume_ReturnsZero(t *testing.T) {
	dailyInfo := &model.ListingDailyPriceInfo{Volume: 0}
	listingRepo := &fakeListingRepo{dailyPriceInfo: dailyInfo}
	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, listingRepo,
		&fakeUserServiceClient{
			identityResp: &pb.GetIdentityByUserIdResponse{
				IdentityId: 5,
			},
		},
		&fakeOrderBankingClient{}, &fakeTaxRecorder{})

	vol := svc.resolveDailyVolume(context.Background(), 1)
	require.Equal(t, uint(0), vol)
}

func TestResolveDailyVolume_RepoError_ReturnsZero(t *testing.T) {
	listingRepo := &fakeListingRepo{dailyPriceErr: errors.New("db error")}
	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, listingRepo,
		&fakeUserServiceClient{
			identityResp: &pb.GetIdentityByUserIdResponse{
				IdentityId: 5,
			},
		},
		&fakeOrderBankingClient{}, &fakeTaxRecorder{})

	vol := svc.resolveDailyVolume(context.Background(), 1)
	require.Equal(t, uint(0), vol)
}

// ── nextExecutionAt Tests ─────────────────────────────────────────

func TestNextExecutionAt_RemainingZero_ReturnsNow(t *testing.T) {
	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, &fakeListingRepo{},
		&fakeUserServiceClient{
			identityResp: &pb.GetIdentityByUserIdResponse{
				IdentityId: 5,
			},
		},
		&fakeOrderBankingClient{}, &fakeTaxRecorder{})

	order := &model.Order{Quantity: 5, FilledQty: 5}
	result := svc.nextExecutionAt(context.Background(), order)
	require.Equal(t, svc.now(), result)
}

func TestNextExecutionAt_WithDailyVolume_ReturnsFutureOrNowTime(t *testing.T) {
	dailyInfo := &model.ListingDailyPriceInfo{Volume: 1000}
	listingRepo := &fakeListingRepo{dailyPriceInfo: dailyInfo}
	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, listingRepo,
		&fakeUserServiceClient{
			identityResp: &pb.GetIdentityByUserIdResponse{
				IdentityId: 5,
			},
		},
		&fakeOrderBankingClient{}, &fakeTaxRecorder{})

	order := &model.Order{Quantity: 10, FilledQty: 5, ListingID: 1, AfterHours: false}
	result := svc.nextExecutionAt(context.Background(), order)
	require.True(t, !result.Before(svc.now()))
}

func TestNextExecutionAt_AfterHours_AddsDelay(t *testing.T) {
	dailyInfo := &model.ListingDailyPriceInfo{Volume: 1000}
	listingRepo := &fakeListingRepo{dailyPriceInfo: dailyInfo}
	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, listingRepo,
		&fakeUserServiceClient{
			identityResp: &pb.GetIdentityByUserIdResponse{
				IdentityId: 5,
			},
		},
		&fakeOrderBankingClient{}, &fakeTaxRecorder{})

	order := &model.Order{Quantity: 10, FilledQty: 5, ListingID: 1, AfterHours: true}
	result := svc.nextExecutionAt(context.Background(), order)
	minExpected := svc.now().Add(afterHoursExecutionDelay)
	require.True(t, !result.Before(minExpected))
}

func TestNextExecutionAt_ZeroVolume_StillReturnsValidTime(t *testing.T) {
	listingRepo := &fakeListingRepo{dailyPriceInfo: nil}
	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{}, listingRepo,
		&fakeUserServiceClient{
			identityResp: &pb.GetIdentityByUserIdResponse{
				IdentityId: 5,
			},
		},
		&fakeOrderBankingClient{}, &fakeTaxRecorder{})

	order := &model.Order{Quantity: 10, FilledQty: 0, ListingID: 1, AfterHours: false}
	result := svc.nextExecutionAt(context.Background(), order)
	require.True(t, !result.Before(svc.now()))
}

// ── resolveExecutionPrice additional paths ────────────────────────

func TestResolveExecutionPrice_StopBuy(t *testing.T) {
	listing := &model.Listing{Price: 150.0, Ask: 151.0}
	p, ok := resolveExecutionPrice(&model.Order{OrderType: model.OrderTypeStop, Direction: model.OrderDirectionBuy}, listing)
	require.True(t, ok)
	require.Equal(t, 151.0, p)
}

func TestResolveExecutionPrice_StopSell(t *testing.T) {
	listing := &model.Listing{Price: 150.0, Ask: 151.0}
	p, ok := resolveExecutionPrice(&model.Order{OrderType: model.OrderTypeStop, Direction: model.OrderDirectionSell}, listing)
	require.True(t, ok)
	require.Equal(t, 150.0, p)
}

func TestResolveExecutionPrice_StopLimitBuy_CanExecute(t *testing.T) {
	listing := &model.Listing{Price: 150.0, Ask: 151.0}
	lv := 155.0
	p, ok := resolveExecutionPrice(&model.Order{OrderType: model.OrderTypeStopLimit, Direction: model.OrderDirectionBuy, LimitValue: &lv}, listing)
	require.True(t, ok)
	require.Equal(t, 151.0, p)
}

func TestResolveExecutionPrice_StopLimitBuy_CannotExecute(t *testing.T) {
	listing := &model.Listing{Price: 150.0, Ask: 160.0}
	lv := 155.0
	_, ok := resolveExecutionPrice(&model.Order{OrderType: model.OrderTypeStopLimit, Direction: model.OrderDirectionBuy, LimitValue: &lv}, listing)
	require.False(t, ok)
}

func TestResolveExecutionPrice_StopLimitSell_CanExecute(t *testing.T) {
	listing := &model.Listing{Price: 150.0, Ask: 151.0}
	lv := 140.0
	p, ok := resolveExecutionPrice(&model.Order{OrderType: model.OrderTypeStopLimit, Direction: model.OrderDirectionSell, LimitValue: &lv}, listing)
	require.True(t, ok)
	require.Equal(t, 150.0, p)
}

func TestResolveExecutionPrice_UnknownType_ReturnsFalse(t *testing.T) {
	listing := &model.Listing{Price: 150.0, Ask: 151.0}
	_, ok := resolveExecutionPrice(&model.Order{OrderType: "UNKNOWN"}, listing)
	require.False(t, ok)
}

// ── nextTradingOpen: weekend skipping ────────────────────────────

func TestNextTradingOpen_WeekdayReturnsItself(t *testing.T) {
	mon := time.Date(2025, 6, 9, 9, 0, 0, 0, time.UTC)
	result := nextTradingOpen(mon)
	require.Equal(t, mon, result)
}

func TestNextTradingOpen_SaturdaySkipsToMonday(t *testing.T) {
	sat := time.Date(2025, 6, 7, 9, 0, 0, 0, time.UTC)
	result := nextTradingOpen(sat)
	require.Equal(t, time.Monday, result.Weekday())
}

func TestNextTradingOpen_SundaySkipsToMonday(t *testing.T) {
	sun := time.Date(2025, 6, 8, 9, 0, 0, 0, time.UTC)
	result := nextTradingOpen(sun)
	require.Equal(t, time.Monday, result.Weekday())
}

// ── processOrder: stop condition not met -> reschedules ──────────

func TestProcessOrder_StopNotMet_Reschedules(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()
	orderRepo := &fakeOrderRepo{}
	listingRepo := &fakeListingRepo{listing: listing}
	exchangeRepo := &fakeExchangeRepo{exchange: exchange}
	svc := newTestOrderService(orderRepo, &fakeOrderTransactionRepo{}, exchangeRepo, listingRepo, &fakeUserServiceClient{}, &fakeOrderBankingClient{}, &fakeTaxRecorder{})

	stopVal := 200.0
	order := &model.Order{
		OrderID:      1,
		ListingID:    1,
		OrderType:    model.OrderTypeStop,
		Direction:    model.OrderDirectionBuy,
		Quantity:     5,
		ContractSize: 1,
		Triggered:    false,
		StopValue:    &stopVal,
		Status:       model.OrderStatusApproved,
	}

	err := svc.processOrder(context.Background(), order)
	require.NoError(t, err)
	require.False(t, order.IsDone)
	require.NotNil(t, order.NextExecutionAt)
	require.False(t, order.Triggered)
}

// ── processOrder: limit price not met -> reschedules ─────────────

func TestProcessOrder_LimitNotMet_Reschedules(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()
	orderRepo := &fakeOrderRepo{}
	listingRepo := &fakeListingRepo{listing: listing}
	exchangeRepo := &fakeExchangeRepo{exchange: exchange}
	svc := newTestOrderService(orderRepo, &fakeOrderTransactionRepo{}, exchangeRepo, listingRepo,
		&fakeUserServiceClient{
			identityResp: &pb.GetIdentityByUserIdResponse{
				IdentityId: 5,
			},
		},
		&fakeOrderBankingClient{}, &fakeTaxRecorder{})

	limitVal := 100.0
	order := &model.Order{
		OrderID:      1,
		ListingID:    1,
		OrderType:    model.OrderTypeLimit,
		Direction:    model.OrderDirectionBuy,
		Quantity:     5,
		ContractSize: 1,
		Triggered:    true,
		LimitValue:   &limitVal,
		Status:       model.OrderStatusApproved,
	}

	err := svc.processOrder(context.Background(), order)
	require.NoError(t, err)
	require.False(t, order.IsDone)
	require.NotNil(t, order.NextExecutionAt)
}

// ── processOrder: settlement transient error -> retries ──────────

func TestProcessOrder_SettlementTransientError_Reschedules(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()
	orderRepo := &fakeOrderRepo{}
	listingRepo := &fakeListingRepo{listing: listing}
	exchangeRepo := &fakeExchangeRepo{exchange: exchange}
	txRepo := &fakeOrderTransactionRepo{}
	bankingClient := &fakeOrderBankingClient{
		settlementErr: errors.New("network timeout"),
	}
	svc := newTestOrderService(orderRepo, txRepo, exchangeRepo, listingRepo,
		&fakeUserServiceClient{
			identityResp: &pb.GetIdentityByUserIdResponse{
				IdentityId: 5,
			},
		},
		bankingClient, &fakeTaxRecorder{})

	order := &model.Order{
		OrderID:          1,
		ListingID:        1,
		OrderType:        model.OrderTypeMarket,
		Direction:        model.OrderDirectionBuy,
		Quantity:         1,
		ContractSize:     1,
		Triggered:        true,
		AllOrNone:        true,
		Status:           model.OrderStatusApproved,
		AccountNumber:    "444000100000000110",
		CommissionExempt: true,
	}

	err := svc.processOrder(context.Background(), order)
	require.Error(t, err)
	require.False(t, order.IsDone)
	require.NotNil(t, order.NextExecutionAt)
}

// ── processOrder: partial fill -> next execution scheduled ───────

func TestProcessOrder_MarketOrder_PartialFill_SchedulesNext(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()
	orderRepo := &fakeOrderRepo{}
	listingRepo := &fakeListingRepo{
		listing:        listing,
		dailyPriceInfo: &model.ListingDailyPriceInfo{Volume: 1000},
	}
	exchangeRepo := &fakeExchangeRepo{exchange: exchange}
	txRepo := &fakeOrderTransactionRepo{}
	bankingClient := &fakeOrderBankingClient{
		settlementResp: &pb.ExecuteTradeSettlementResponse{
			SourceAmount:            151.0,
			SourceCurrencyCode:      "USD",
			DestinationAmount:       151.0,
			DestinationCurrencyCode: "USD",
		},
	}
	svc := newTestOrderService(orderRepo, txRepo, exchangeRepo, listingRepo,
		&fakeUserServiceClient{
			identityResp: &pb.GetIdentityByUserIdResponse{
				IdentityId: 5,
			},
		},
		bankingClient, &fakeTaxRecorder{})
	svc.rng = rand.New(rand.NewSource(42))

	order := &model.Order{
		OrderID:          1,
		ListingID:        1,
		OrderType:        model.OrderTypeMarket,
		Direction:        model.OrderDirectionBuy,
		Quantity:         100,
		FilledQty:        0,
		ContractSize:     1,
		Triggered:        true,
		AllOrNone:        false,
		Status:           model.OrderStatusApproved,
		AccountNumber:    "444000100000000110",
		CommissionExempt: true,
	}

	err := svc.processOrder(context.Background(), order)
	require.NoError(t, err)
	if !order.IsDone {
		require.NotNil(t, order.NextExecutionAt)
	}
}

// ── calculateInitialPricePerUnit: StopLimit path ─────────────────

func TestCalculateInitialPricePerUnit_StopLimit(t *testing.T) {
	listing := &model.Listing{Price: 150.0, Ask: 151.0}
	lv := 155.0
	p := calculateInitialPricePerUnit(placeOrderParams{OrderType: model.OrderTypeStopLimit, LimitValue: &lv}, listing)
	require.NotNil(t, p)
	require.Equal(t, 155.0, *p)
}

func TestCalculateInitialPricePerUnit_Unknown(t *testing.T) {
	listing := &model.Listing{Price: 150.0, Ask: 151.0}
	p := calculateInitialPricePerUnit(placeOrderParams{OrderType: "UNKNOWN"}, listing)
	require.Nil(t, p)
}

// ── calculateCommission: StopLimit and Unknown paths ─────────────

func TestCalculateCommission_StopLimit(t *testing.T) {
	require.InDelta(t, 0.24*10, calculateCommission(model.OrderTypeStopLimit, 10), 0.001)
	require.Equal(t, 12.0, calculateCommission(model.OrderTypeStopLimit, 100))
}

func TestCalculateCommission_UnknownType(t *testing.T) {
	require.Equal(t, 0.0, calculateCommission("UNKNOWN", 1000))
}

// ── isStopConditionMet: default direction ────────────────────────

func TestIsStopConditionMet_UnknownDirection(t *testing.T) {
	listing := &model.Listing{Price: 150.0, Ask: 151.0}
	sv := 150.0
	order := &model.Order{Direction: "UNKNOWN", StopValue: &sv}
	require.False(t, isStopConditionMet(order, listing))
}

// ── processOrder: fillQty 0 -> reschedules ───────────────────────

func TestProcessOrder_FillQtyZero_Reschedules(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()
	orderRepo := &fakeOrderRepo{}
	listingRepo := &fakeListingRepo{listing: listing, dailyPriceInfo: nil}
	exchangeRepo := &fakeExchangeRepo{exchange: exchange}
	svc := newTestOrderService(orderRepo, &fakeOrderTransactionRepo{}, exchangeRepo, listingRepo,
		&fakeUserServiceClient{
			identityResp: &pb.GetIdentityByUserIdResponse{
				IdentityId: 5,
			},
		},
		&fakeOrderBankingClient{}, &fakeTaxRecorder{})

	order := &model.Order{
		OrderID:      1,
		ListingID:    1,
		OrderType:    model.OrderTypeMarket,
		Direction:    model.OrderDirectionBuy,
		Quantity:     5,
		FilledQty:    5,
		ContractSize: 1,
		Triggered:    true,
		Status:       model.OrderStatusApproved,
	}

	err := svc.processOrder(context.Background(), order)
	require.NoError(t, err)
	require.NotNil(t, order.NextExecutionAt)
}

// ── processOrder: commission sell path ───────────────────────────

func TestProcessOrder_MarketSell_WithCommission(t *testing.T) {
	listing := defaultListing()
	exchange := defaultExchange()
	orderRepo := &fakeOrderRepo{}
	listingRepo := &fakeListingRepo{
		listing:        listing,
		dailyPriceInfo: &model.ListingDailyPriceInfo{Volume: 1000},
	}
	exchangeRepo := &fakeExchangeRepo{exchange: exchange}
	txRepo := &fakeOrderTransactionRepo{}
	bankingClient := &fakeOrderBankingClient{
		settlementResp: &pb.ExecuteTradeSettlementResponse{
			SourceAmount:            150.0,
			SourceCurrencyCode:      "USD",
			DestinationAmount:       150.0,
			DestinationCurrencyCode: "USD",
		},
	}
	svc := newTestOrderService(orderRepo, txRepo, exchangeRepo, listingRepo,
		&fakeUserServiceClient{
			identityResp: &pb.GetIdentityByUserIdResponse{
				IdentityId: 5,
			},
		},
		bankingClient, &fakeTaxRecorder{})
	svc.assetOwnershipRepo = &fakeAssetOwnershipRepo{
		ownerships: []model.AssetOwnership{
			{AssetID: listing.AssetID, Amount: 10},
		},
	}
	svc.now = func() time.Time {
		return time.Date(2025, 6, 4, 10, 0, 0, 0, time.UTC)
	}

	order := &model.Order{
		OrderID:           1,
		ListingID:         1,
		OrderType:         model.OrderTypeMarket,
		Direction:         model.OrderDirectionSell,
		Quantity:          1,
		FilledQty:         0,
		ContractSize:      1,
		Triggered:         true,
		AllOrNone:         true,
		Status:            model.OrderStatusApproved,
		AccountNumber:     "444000100000000110",
		CommissionExempt:  false,
		CommissionCharged: false,
	}

	err := svc.processOrder(context.Background(), order)
	require.NoError(t, err)
}

// ── CreateOrder: expired asset paths ─────────────────────────────

func TestCreateOrder_ExpiredFuture_ReturnsError(t *testing.T) {
	listing := defaultListing()
	listing.AssetID = 1
	listing.Asset.AssetType = model.AssetTypeFuture

	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{exchange: defaultExchange()}, &fakeListingRepo{listing: listing},
		&fakeUserServiceClient{
			identityResp: &pb.GetIdentityByUserIdResponse{
				IdentityId: 5,
			},
		},
		&fakeOrderBankingClient{accountResp: defaultAccountResp(10)}, &fakeTaxRecorder{})
	svc.futuresRepo = &fakeFuturesRepo{futures: []model.FuturesContract{{SettlementDate: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}}}

	order, err := svc.CreateOrder(clientAuthCtx(), dto.CreateOrderRequest{ListingID: 1, AccountNumber: "444000100000000110", OrderType: model.OrderTypeMarket, Direction: model.OrderDirectionBuy, Quantity: 10})
	require.Error(t, err)
	require.Nil(t, order)
	require.Contains(t, err.Error(), "expired futures contract")
}

func TestCreateOrder_ExpiredOption_ReturnsError(t *testing.T) {
	listing := defaultListing()
	listing.AssetID = 1
	listing.Asset.AssetType = model.AssetTypeOption

	svc := newTestOrderService(&fakeOrderRepo{}, &fakeOrderTransactionRepo{}, &fakeExchangeRepo{exchange: defaultExchange()}, &fakeListingRepo{listing: listing},
		&fakeUserServiceClient{
			identityResp: &pb.GetIdentityByUserIdResponse{
				IdentityId: 5,
			},
		},
		&fakeOrderBankingClient{accountResp: defaultAccountResp(10)}, &fakeTaxRecorder{})
	svc.optionRepo = &fakeOptionRepo{options: []model.Option{{SettlementDate: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}}}

	order, err := svc.CreateOrder(clientAuthCtx(), dto.CreateOrderRequest{ListingID: 1, AccountNumber: "444000100000000110", OrderType: model.OrderTypeMarket, Direction: model.OrderDirectionBuy, Quantity: 10})
	require.Error(t, err)
	require.Nil(t, order)
	require.Contains(t, err.Error(), "expired option")
}
