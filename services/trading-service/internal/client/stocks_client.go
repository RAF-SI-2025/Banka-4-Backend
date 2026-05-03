package client

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const finnhubBaseURL = "https://finnhub.io/api/v1"

type Symbol struct {
	Symbol      string `json:"symbol"`
	Description string `json:"description"`
	MIC         string `json:"mic"`
}

type Profile struct {
	Name             string  `json:"name"`
	Exchange         string  `json:"exchange"`
	ShareOutstanding float64 `json:"shareOutstanding"`
	Ticker           string  `json:"ticker"`
}

type Quote struct {
	CurrentPrice float64 `json:"c"`
	High         float64 `json:"h"`
	Change       float64 `json:"d"`
	Volume       float64 `json:"v"`
}

type BasicFinancials struct {
	Metric struct {
		DividendYieldIndicatedAnnual float64 `json:"dividendYieldIndicatedAnnual"`
	} `json:"metric"`
}

type StockClient struct {
	apiKey     string
	httpClient *http.Client
}

func NewStockClient(apiKey string) *StockClient {
	return &StockClient{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

func (c *StockClient) get(path string, out interface{}) (err error) {
	separator := "?"
	for _, ch := range path {
		if ch == '?' {
			separator = "&"
			break
		}
	}
	url := fmt.Sprintf("%s%s%stoken=%s", finnhubBaseURL, path, separator, c.apiKey)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("closing response body: %w", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("finnhub returned status %d", resp.StatusCode)
	}

	err = json.NewDecoder(resp.Body).Decode(out)
	return err
}

func (c *StockClient) GetSymbols(exchange string) ([]Symbol, error) {
	var symbols []Symbol
	if err := c.get(fmt.Sprintf("/stock/symbol?exchange=%s", exchange), &symbols); err != nil {
		return nil, err
	}
	return symbols, nil
}

func (c *StockClient) GetProfile(ticker string) (*Profile, error) {
	var p Profile
	if err := c.get(fmt.Sprintf("/stock/profile2?symbol=%s", ticker), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (c *StockClient) GetQuote(ticker string) (*Quote, error) {
	var q Quote
	if err := c.get(fmt.Sprintf("/quote?symbol=%s", ticker), &q); err != nil {
		return nil, err
	}
	return &q, nil
}

func (c *StockClient) GetBasicFinancials(ticker string) (*BasicFinancials, error) {
	var f BasicFinancials
	if err := c.get(fmt.Sprintf("/stock/metric?symbol=%s&metric=all", ticker), &f); err != nil {
		return nil, err
	}
	return &f, nil
}
