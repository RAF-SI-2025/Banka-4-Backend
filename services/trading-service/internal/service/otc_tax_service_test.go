package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/model"
)

// otcTaxUserClient embeds fakeOtcUserClient (which always resolves clients) and
// lets a test force GetClientById to fail, simulating a participant that has no
// client record — i.e. a bank actuary.
type otcTaxUserClient struct {
	fakeOtcUserClient
	noClient bool
}

func (c *otcTaxUserClient) GetClientById(_ context.Context, id uint64) (*pb.GetClientByIdResponse, error) {
	if c.noClient {
		return nil, errors.New("client not found")
	}
	return &pb.GetClientByIdResponse{Id: id}, nil
}

func newOtcTaxServiceForTest(rec *fakeTaxRecorder, noClient bool, stocks []model.Stock) *OtcTaxService {
	return NewOtcTaxService(
		rec,
		&otcTaxUserClient{noClient: noClient},
		&fakeOtcStockRepo{stocks: stocks},
		&processingBankingClient{accountByNumber: map[string]uint64{}},
	)
}

func stockWithPrice(assetID uint, price float64, currency string) model.Stock {
	return model.Stock{
		AssetID: assetID,
		Listing: &model.Listing{
			Price:    price,
			Exchange: &model.Exchange{Currency: currency},
		},
	}
}

// ── Premium tax (seller) ───────────────────────────────────────────

func TestOtcTaxService_RecordPremiumTax_ClientIsTaxed(t *testing.T) {
	rec := &fakeTaxRecorder{}
	svc := newOtcTaxServiceForTest(rec, false, nil)

	// Spec primer: premija $1150 -> base 1150 (porez 15% = 172.50 računa TaxService).
	contract := &model.OtcOptionContract{
		SellerID:            7,
		SellerAccountNumber: "seller-acc",
		PremiumRSD:          1150,
	}

	require.NoError(t, svc.RecordPremiumTax(context.Background(), contract))

	require.True(t, rec.called)
	require.Equal(t, "seller-acc", rec.recordedAccountNumber)
	require.Nil(t, rec.recordedEmployeeID)
	require.Equal(t, 1150.0, rec.recordedProfit)
	require.Equal(t, "RSD", rec.recordedCurrency)
}

func TestOtcTaxService_RecordPremiumTax_ActuaryExempt(t *testing.T) {
	rec := &fakeTaxRecorder{}
	svc := newOtcTaxServiceForTest(rec, true, nil)

	contract := &model.OtcOptionContract{
		SellerID:            7,
		SellerAccountNumber: "seller-acc",
		PremiumRSD:          1150,
	}

	require.NoError(t, svc.RecordPremiumTax(context.Background(), contract))
	require.False(t, rec.called, "actuaries do not pay premium tax")
}

func TestOtcTaxService_RecordPremiumTax_ZeroPremiumNoop(t *testing.T) {
	rec := &fakeTaxRecorder{}
	svc := newOtcTaxServiceForTest(rec, false, nil)

	require.NoError(t, svc.RecordPremiumTax(context.Background(), &model.OtcOptionContract{PremiumRSD: 0}))
	require.False(t, rec.called)
}

// ── Exercise tax (buyer) ───────────────────────────────────────────

func TestOtcTaxService_RecordExerciseTax_ClientIsTaxed(t *testing.T) {
	rec := &fakeTaxRecorder{}
	// Spec primer: 50 akcija, strike 200, tržišna 250, premija 1150.
	// Oporezivi iznos = (250-200)*50 - 1150 = 1350 (porez 15% = 202.50).
	svc := newOtcTaxServiceForTest(rec, false, []model.Stock{stockWithPrice(42, 250, "RSD")})

	contract := &model.OtcOptionContract{
		BuyerID:            9,
		BuyerAccountNumber: "buyer-acc",
		StockAssetID:       42,
		Amount:             50,
		StrikePriceRSD:     200,
		PremiumRSD:         1150,
	}

	require.NoError(t, svc.RecordExerciseTax(context.Background(), contract))

	require.True(t, rec.called)
	require.Equal(t, "buyer-acc", rec.recordedAccountNumber)
	require.Nil(t, rec.recordedEmployeeID)
	require.InDelta(t, 1350.0, rec.recordedProfit, 1e-9)
	require.Equal(t, "RSD", rec.recordedCurrency)
}

func TestOtcTaxService_RecordExerciseTax_ActuaryExempt(t *testing.T) {
	rec := &fakeTaxRecorder{}
	svc := newOtcTaxServiceForTest(rec, true, []model.Stock{stockWithPrice(42, 250, "RSD")})

	contract := &model.OtcOptionContract{
		BuyerID:            9,
		BuyerAccountNumber: "buyer-acc",
		StockAssetID:       42,
		Amount:             50,
		StrikePriceRSD:     200,
		PremiumRSD:         1150,
	}

	require.NoError(t, svc.RecordExerciseTax(context.Background(), contract))
	require.False(t, rec.called, "actuaries do not pay exercise tax")
}

func TestOtcTaxService_RecordExerciseTax_NoGainNoTax(t *testing.T) {
	rec := &fakeTaxRecorder{}
	// Market below strike -> negative base -> nothing to tax.
	svc := newOtcTaxServiceForTest(rec, false, []model.Stock{stockWithPrice(42, 180, "RSD")})

	contract := &model.OtcOptionContract{
		BuyerID:            9,
		BuyerAccountNumber: "buyer-acc",
		StockAssetID:       42,
		Amount:             50,
		StrikePriceRSD:     200,
		PremiumRSD:         1150,
	}

	require.NoError(t, svc.RecordExerciseTax(context.Background(), contract))
	require.False(t, rec.called)
}

func TestOtcTaxService_RecordExerciseTax_PremiumWipesGainNoTax(t *testing.T) {
	rec := &fakeTaxRecorder{}
	// Gross gain (250-200)*50 = 2500 but premium 3000 exceeds it -> base negative.
	svc := newOtcTaxServiceForTest(rec, false, []model.Stock{stockWithPrice(42, 250, "RSD")})

	contract := &model.OtcOptionContract{
		BuyerID:            9,
		BuyerAccountNumber: "buyer-acc",
		StockAssetID:       42,
		Amount:             50,
		StrikePriceRSD:     200,
		PremiumRSD:         3000,
	}

	require.NoError(t, svc.RecordExerciseTax(context.Background(), contract))
	require.False(t, rec.called)
}

// ── Expiry loss (buyer) ────────────────────────────────────────────

func TestOtcTaxService_RecordExpiryLoss_ClientGetsRelief(t *testing.T) {
	rec := &fakeTaxRecorder{}
	svc := newOtcTaxServiceForTest(rec, false, nil)

	// Spec primer: kupac platio premiju $1150, opcija istekla -> gubitak 1150
	// umanjuje porez (ReduceTax dobija osnovicu 1150; -15% = 172.50 od akumuliranog).
	contract := &model.OtcOptionContract{
		BuyerID:            9,
		BuyerAccountNumber: "buyer-acc",
		PremiumRSD:         1150,
	}

	require.NoError(t, svc.RecordExpiryLoss(context.Background(), contract))

	require.True(t, rec.reduceCalled)
	require.Equal(t, "buyer-acc", rec.reducedAccountNumber)
	require.Equal(t, 1150.0, rec.reducedLossBase)
}

func TestOtcTaxService_RecordExpiryLoss_ActuaryUnaffected(t *testing.T) {
	rec := &fakeTaxRecorder{}
	svc := newOtcTaxServiceForTest(rec, true, nil)

	contract := &model.OtcOptionContract{
		BuyerID:            9,
		BuyerAccountNumber: "buyer-acc",
		PremiumRSD:         1150,
	}

	require.NoError(t, svc.RecordExpiryLoss(context.Background(), contract))
	require.False(t, rec.reduceCalled, "actuaries pay no tax, so nothing to reduce")
}

func TestOtcTaxService_RecordExpiryLoss_ZeroPremiumNoop(t *testing.T) {
	rec := &fakeTaxRecorder{}
	svc := newOtcTaxServiceForTest(rec, false, nil)

	require.NoError(t, svc.RecordExpiryLoss(context.Background(), &model.OtcOptionContract{PremiumRSD: 0}))
	require.False(t, rec.reduceCalled)
}

func TestOtcTaxService_RecordExerciseTax_ConvertsForeignCurrency(t *testing.T) {
	rec := &fakeTaxRecorder{}
	// processingBankingClient.ConvertCurrency is 1:1, so a USD-priced listing of
	// 250 still yields a 250 RSD market price; the test confirms the foreign path
	// runs without error and still taxes the realized gain.
	svc := newOtcTaxServiceForTest(rec, false, []model.Stock{stockWithPrice(42, 250, "USD")})

	contract := &model.OtcOptionContract{
		BuyerID:            9,
		BuyerAccountNumber: "buyer-acc",
		StockAssetID:       42,
		Amount:             50,
		StrikePriceRSD:     200,
		PremiumRSD:         1150,
	}

	require.NoError(t, svc.RecordExerciseTax(context.Background(), contract))
	require.True(t, rec.called)
	require.InDelta(t, 1350.0, rec.recordedProfit, 1e-9)
}
