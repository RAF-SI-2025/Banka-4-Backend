package service

import (
	"banking-service/internal/client"
	"banking-service/internal/model"
	"banking-service/internal/repository"
	"common/pkg/errors"
	"context"
	"log"
	"time"
)

const refreshInterval = 2 * time.Hour

type ExchangeService struct {
	repo   repository.ExchangeRateRepository
	client client.ExchangeRateClient
}

func NewExchangeService(repo repository.ExchangeRateRepository, apiClient client.ExchangeRateClient) *ExchangeService {
	return &ExchangeService{repo: repo, client: apiClient}
}

func (s *ExchangeService) Initialize(ctx context.Context) {
	rates, err := s.repo.GetAll(ctx)
	if err == nil && len(rates) > 0 {
		log.Println("exchange rates loaded from DB")
		return
	}

	if err := s.refreshFromAPI(ctx); err != nil {
		log.Printf("exchange rates initialization failed: %v", err)
	}
}

func (s *ExchangeService) StartBackgroundRefresh(ctx context.Context) {
	ticker := time.NewTicker(refreshInterval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				if err := s.refreshFromAPI(context.Background()); err != nil {
					log.Printf("exchange rate refresh failed: %v", err)
				}
			}
		}
	}()
}

func (s *ExchangeService) refreshFromAPI(ctx context.Context) error {
	apiResp, err := s.client.FetchRates(ctx)
	if err != nil {
		return err
	}

	providerUpdatedAt := time.Unix(apiResp.TimeLastUpdateUnix, 0)
	providerNextUpdateAt := time.Unix(apiResp.TimeNextUpdateUnix, 0)

	var rates []model.ExchangeRate
	for code := range model.AllowedForeignCurrencies {
		apiRate, ok := apiResp.ConversionRates[string(code)]
		if !ok || apiRate == 0 {
			continue
		}

		middleRate := 1.0 / apiRate
		rates = append(rates, model.ExchangeRate{
			CurrencyCode:         code,
			BaseCurrency:         model.RSD,
			BuyRate:              middleRate * (1 - model.BankCommission),
			MiddleRate:           middleRate,
			SellRate:             middleRate * (1 + model.BankCommission),
			ProviderUpdatedAt:    providerUpdatedAt,
			ProviderNextUpdateAt: providerNextUpdateAt,
		})
	}

	if len(rates) == 0 {
		return nil
	}

	if err := s.repo.UpsertAll(ctx, rates); err != nil {
		return err
	}

	log.Println("exchange rates refreshed from API")
	return nil
}

func (s *ExchangeService) GetRates(ctx context.Context) ([]model.ExchangeRate, error) {
	rates, err := s.repo.GetAll(ctx)
	if err != nil {
		return nil, errors.InternalErr(err)
	}

	if len(rates) == 0 {
		return nil, errors.ServiceUnavailableErr(nil)
	}

	return rates, nil
}

func (s *ExchangeService) Convert(ctx context.Context, amount float64, from, to model.CurrencyCode) (float64, error) {
	if from == to {
		return amount, nil
	}

	rates, err := s.GetRates(ctx)
	if err != nil {
		return 0, err
	}

	rateMap := make(map[model.CurrencyCode]model.ExchangeRate, len(rates))
	for _, rate := range rates {
		rateMap[rate.CurrencyCode] = rate
	}

	rsdAmount, err := s.toRSD(amount, from, rateMap)
	if err != nil {
		return 0, err
	}

	return s.fromRSD(rsdAmount, to, rateMap)
}

func (s *ExchangeService) toRSD(amount float64, currency model.CurrencyCode, rates map[model.CurrencyCode]model.ExchangeRate) (float64, error) {
	if currency == model.RSD {
		return amount, nil
	}

	rate, ok := rates[currency]
	if !ok {
		return 0, errors.ServiceUnavailableErr(nil)
	}

	return amount * rate.SellRate, nil
}

func (s *ExchangeService) fromRSD(amount float64, currency model.CurrencyCode, rates map[model.CurrencyCode]model.ExchangeRate) (float64, error) {
	if currency == model.RSD {
		return amount, nil
	}

	rate, ok := rates[currency]
	if !ok {
		return 0, errors.ServiceUnavailableErr(nil)
	}

	return amount / rate.BuyRate, nil
}
