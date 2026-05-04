package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type httpTradingClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewHTTPTradingClient(baseURL string) TradingClient {
	return &httpTradingClient{baseURL: baseURL, httpClient: &http.Client{}}
}

func (c *httpTradingClient) TransferManagerFunds(ctx context.Context, fromManagerID uint, toManagerID uint) error {
	body, _ := json.Marshal(map[string]uint{
		"from_manager_id": fromManagerID,
		"to_manager_id":   toManagerID,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/api/investment-funds/transfer-manager", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	if token, ok := ctx.Value("jwt_token").(string); ok && token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("trading-service returned %d", resp.StatusCode)
	}
	return nil
}
