package dto

import (
	"banking-service/internal/model"
	"math"
	"time"
)

type RateItem struct {
	Currency   string  `json:"currency"`
	BuyRate    float64 `json:"buy_rate"`
	MiddleRate float64 `json:"middle_rate"`
	SellRate   float64 `json:"sell_rate"`
}

type ExchangeRatesResponse struct {
	BaseCurrency string     `json:"base_currency"`
	UpdatedAt    time.Time  `json:"updated_at"`
	NextUpdateAt time.Time  `json:"next_update_at"`
	Rates        []RateItem `json:"rates"`
}

type ConvertResponse struct {
	Amount       float64 `json:"amount"`
	FromCurrency string  `json:"from_currency"`
	ToCurrency   string  `json:"to_currency"`
	Total        float64 `json:"total"`
}

func ToExchangeRatesResponse(rates []model.ExchangeRate) *ExchangeRatesResponse {
	items := make([]RateItem, 0, len(rates))
	for _, r := range rates {
		items = append(items, RateItem{
			Currency:   string(r.CurrencyCode),
			BuyRate:    round2(r.BuyRate),
			MiddleRate: round2(r.MiddleRate),
			SellRate:   round2(r.SellRate),
		})
	}

	return &ExchangeRatesResponse{
		BaseCurrency: string(rates[0].BaseCurrency),
		UpdatedAt:    rates[0].ProviderUpdatedAt,
		NextUpdateAt: rates[0].ProviderNextUpdateAt,
		Rates:        items,
	}
}

func ToConvertResponse(amount float64, from, to model.CurrencyCode, total float64) *ConvertResponse {
	return &ConvertResponse{
		Amount:       amount,
		FromCurrency: string(from),
		ToCurrency:   string(to),
		Total:        round2(total),
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
