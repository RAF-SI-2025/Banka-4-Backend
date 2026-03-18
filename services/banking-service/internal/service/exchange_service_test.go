package service

import (
	"banking-service/internal/model"
	"common/pkg/errors"
	"context"
	stderrors "errors"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeExchangeRateRepo struct {
	rates []model.ExchangeRate
	err   error
}

func (r *fakeExchangeRateRepo) UpsertAll(_ context.Context, _ []model.ExchangeRate) error {
	return nil
}

func (r *fakeExchangeRateRepo) GetAll(_ context.Context) ([]model.ExchangeRate, error) {
	return r.rates, r.err
}

func buildTestRates() []model.ExchangeRate {
	// Simulate: API returns 1 RSD = 0.008457 EUR, 1 RSD = 0.009756 USD
	eurMiddle := 1.0 / 0.008457
	usdMiddle := 1.0 / 0.009756

	return []model.ExchangeRate{
		{
			CurrencyCode: model.EUR,
			BaseCurrency: model.RSD,
			BuyRate:      eurMiddle * (1 - model.BankCommission),
			MiddleRate:   eurMiddle,
			SellRate:     eurMiddle * (1 + model.BankCommission),
		},
		{
			CurrencyCode: model.USD,
			BaseCurrency: model.RSD,
			BuyRate:      usdMiddle * (1 - model.BankCommission),
			MiddleRate:   usdMiddle,
			SellRate:     usdMiddle * (1 + model.BankCommission),
		},
	}
}

func newTestService(rates []model.ExchangeRate) *ExchangeService {
	return &ExchangeService{
		repo: &fakeExchangeRateRepo{rates: rates},
	}
}

func roundTo2(v float64) float64 {
	return math.Round(v*100) / 100
}

func TestConvert(t *testing.T) {
	t.Parallel()

	rates := buildTestRates()
	svc := newTestService(rates)

	eurBuy := rates[0].BuyRate
	eurSell := rates[0].SellRate
	usdBuy := rates[1].BuyRate
	usdSell := rates[1].SellRate

	tests := []struct {
		name    string
		amount  float64
		from    model.CurrencyCode
		to      model.CurrencyCode
		wantAmt float64
		wantErr bool
	}{
		{
			name:    "RSD to EUR",
			amount:  10000,
			from:    model.RSD,
			to:      model.EUR,
			wantAmt: roundTo2(10000 / eurBuy),
		},
		{
			name:    "EUR to RSD",
			amount:  100,
			from:    model.EUR,
			to:      model.RSD,
			wantAmt: roundTo2(100 * eurSell),
		},
		{
			name:    "USD to RSD",
			amount:  100,
			from:    model.USD,
			to:      model.RSD,
			wantAmt: roundTo2(100 * usdSell),
		},
		{
			name:    "EUR to USD",
			amount:  100,
			from:    model.EUR,
			to:      model.USD,
			wantAmt: roundTo2((100 * eurSell) / usdBuy),
		},
		{
			name:    "same currency EUR to EUR",
			amount:  500,
			from:    model.EUR,
			to:      model.EUR,
			wantAmt: 500,
		},
		{
			name:    "same currency RSD to RSD",
			amount:  1000,
			from:    model.RSD,
			to:      model.RSD,
			wantAmt: 1000,
		},
		{
			name:    "unavailable from currency GBP",
			amount:  100,
			from:    model.GBP,
			to:      model.EUR,
			wantErr: true,
		},
		{
			name:    "unavailable to currency GBP",
			amount:  100,
			from:    model.EUR,
			to:      model.GBP,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := svc.Convert(context.Background(), tt.amount, tt.from, tt.to)

			if tt.wantErr {
				require.Error(t, err)
				var appErr *errors.AppError
				require.True(t, stderrors.As(err, &appErr))
				require.Equal(t, 503, appErr.Code)
				return
			}

			require.NoError(t, err)
			require.InDelta(t, tt.wantAmt, roundTo2(result), 0.01)
		})
	}
}

func TestConvert_AmountPreservation(t *testing.T) {
	t.Parallel()

	svc := newTestService(buildTestRates())
	ctx := context.Background()

	eurToRSD, err := svc.Convert(ctx, 100, model.EUR, model.RSD)
	require.NoError(t, err)

	rsdToEUR, err := svc.Convert(ctx, eurToRSD, model.RSD, model.EUR)
	require.NoError(t, err)

	// Due to buy/sell spread the roundtrip loses ~3%
	require.InDelta(t, 100, rsdToEUR, 4.0)
}

func TestConvert_EmptyDB(t *testing.T) {
	t.Parallel()

	svc := newTestService(nil)

	_, err := svc.Convert(context.Background(), 100, model.EUR, model.RSD)
	require.Error(t, err)

	var appErr *errors.AppError
	require.True(t, stderrors.As(err, &appErr))
	require.Equal(t, 503, appErr.Code)
}

func TestGetRates_EmptyDB(t *testing.T) {
	t.Parallel()

	svc := newTestService(nil)

	_, err := svc.GetRates(context.Background())
	require.Error(t, err)

	var appErr *errors.AppError
	require.True(t, stderrors.As(err, &appErr))
	require.Equal(t, 503, appErr.Code)
}

func TestGetRates_Success(t *testing.T) {
	t.Parallel()

	rates := buildTestRates()
	svc := newTestService(rates)

	result, err := svc.GetRates(context.Background())
	require.NoError(t, err)
	require.Len(t, result, 2)
}
