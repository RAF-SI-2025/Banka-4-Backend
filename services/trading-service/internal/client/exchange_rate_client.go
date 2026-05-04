package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const exchangeRateBaseURL = "https://v6.exchangerate-api.com/v6"

type ExchangeRateAPIResponse struct {
	Result             string             `json:"result"`
	TimeLastUpdateUnix int64              `json:"time_last_update_unix"`
	TimeNextUpdateUnix int64              `json:"time_next_update_unix"`
	BaseCode           string             `json:"base_code"`
	ConversionRates    map[string]float64 `json:"conversion_rates"`
}

type ExchangeRateClient interface {
	FetchRates(ctx context.Context) (*ExchangeRateAPIResponse, error)
}

type exchangeRateClient struct {
	httpClient *http.Client
	apiURL     string
}

func NewExchangeRateClient(apiKey string) ExchangeRateClient {
	return &exchangeRateClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		apiURL:     fmt.Sprintf("%s/%s/latest/RSD", exchangeRateBaseURL, apiKey),
	}
}

func (c *exchangeRateClient) FetchRates(ctx context.Context) (apiResp *ExchangeRateAPIResponse, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching exchange rates: %w", err)
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("closing response body: %w", closeErr)
			apiResp = nil
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("exchange rate API returned status %d", resp.StatusCode)
	}

	var decodedResp ExchangeRateAPIResponse
	if err = json.NewDecoder(resp.Body).Decode(&decodedResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if decodedResp.Result != "success" {
		return nil, fmt.Errorf("exchange rate API returned result: %s", decodedResp.Result)
	}

	apiResp = &decodedResp
	return apiResp, nil
}
