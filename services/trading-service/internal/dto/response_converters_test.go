package dto

import (
	"testing"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

func testListing() model.Listing {
	return model.Listing{
		ListingID: 3,
		AssetID:   2,
		Asset:     &model.Asset{AssetID: 2, Ticker: "AAPL", Name: "Apple Inc", AssetType: model.AssetTypeStock},
	}
}

func TestOrderResponses(t *testing.T) {
	t.Parallel()

	price := 123.45
	order := model.Order{
		OrderID:          1,
		OrderOwnerUserID: 7,
		OrderOwnerType:   model.OwnerTypeClient,
		AccountNumber:    "444000100000000001",
		ListingID:        3,
		Listing:          testListing(),
		OrderType:        model.OrderTypeLimit,
		Direction:        model.OrderDirectionBuy,
		Quantity:         10,
		FilledQty:        4,
		ContractSize:     1,
		PricePerUnit:     &price,
		Status:           model.OrderStatusApproved,
	}

	resp := ToOrderResponse(order)
	if resp.Ticker != "AAPL" || resp.RemainingPortions != 6 {
		t.Fatalf("unexpected order response %#v", resp)
	}
	if len(ToOrderResponseList([]model.Order{order})) != 1 {
		t.Fatal("expected order response list")
	}

	summary := ToOrderSummaryResponse(order)
	if summary.AssetType != model.AssetTypeStock || summary.RemainingPortions != 6 {
		t.Fatalf("unexpected order summary %#v", summary)
	}
	if len(ToOrderSummaryResponseList([]model.Order{order})) != 1 {
		t.Fatal("expected order summary list")
	}

	noAsset := ToOrderResponse(model.Order{Quantity: 1})
	if noAsset.Ticker != "" || noAsset.ListingName != "" {
		t.Fatalf("expected empty asset fields, got %#v", noAsset)
	}
}

func TestTradingCollectionResponses(t *testing.T) {
	t.Parallel()

	exchanges := ToExchangeResponseList([]model.Exchange{{ExchangeID: 1, Name: "NYSE", MicCode: "XNYS", TradingEnabled: true}})
	if len(exchanges) != 1 || exchanges[0].MicCode != "XNYS" {
		t.Fatalf("unexpected exchanges %#v", exchanges)
	}

	recurring := ToRecurringOrderResponseList([]model.RecurringOrder{{
		RecurringOrderID: 4,
		UserID:           7,
		OwnerType:        model.OwnerTypeClient,
		ListingID:        3,
		Listing:          testListing(),
		Direction:        model.OrderDirectionBuy,
		Mode:             model.RecurringOrderModeByAmount,
		Value:            1000,
		AccountNumber:    "444",
		Cadence:          model.RecurringOrderCadenceMonthly,
		NextRun:          time.Now(),
		Active:           true,
	}})
	if len(recurring) != 1 || recurring[0].Ticker != "AAPL" {
		t.Fatalf("unexpected recurring orders %#v", recurring)
	}
}

func TestOtcResponses(t *testing.T) {
	t.Parallel()

	now := time.Now()
	stock := model.Stock{AssetID: 9, Asset: model.Asset{AssetID: 9, Ticker: "TSLA", Name: "Tesla", AssetType: model.AssetTypeStock}}
	offer := ToOtcOfferResponse(model.OtcOffer{
		OtcOfferID:       1,
		BuyerID:          2,
		SellerID:         3,
		StockAssetID:     9,
		Stock:            stock,
		Amount:           5,
		PricePerStockRSD: 100,
		PremiumRSD:       10,
		SettlementDate:   now,
		Status:           model.OtcOfferStatusActive,
		LastModified:     now,
		ModifiedBy:       2,
	})
	if offer.Ticker != "TSLA" || offer.StockName != "Tesla" {
		t.Fatalf("unexpected offer response %#v", offer)
	}

	contract := ToOtcOptionContractResponse(model.OtcOptionContract{
		OtcOptionContractID: 10,
		OtcOfferID:          1,
		BuyerID:             2,
		SellerID:            3,
		StockAssetID:        9,
		Stock:               stock,
		Amount:              5,
		StrikePriceRSD:      100,
		PremiumRSD:          10,
		SettlementDate:      now,
		Status:              model.OtcOptionContractStatusActive,
	})
	if contract.Ticker != "TSLA" || contract.Status != model.OtcOptionContractStatusActive {
		t.Fatalf("unexpected contract response %#v", contract)
	}
	if len(ToOtcOptionContractResponseList([]model.OtcOptionContract{{Stock: stock}})) != 1 {
		t.Fatal("expected contract response list")
	}

	saga := ToOtcExecutionSagaResponse(model.OtcExecutionSaga{
		OtcExecutionSagaID: 4,
		ContractID:         10,
		ExecutionKey:       "exec",
		CurrentStep:        model.OtcExecutionStepFundsReserved,
		Status:             model.OtcExecutionStatusInProgress,
		RetryCount:         2,
		LastError:          "retry",
	})
	if saga.ExecutionKey != "exec" || saga.RetryCount != 2 {
		t.Fatalf("unexpected saga response %#v", saga)
	}
}
