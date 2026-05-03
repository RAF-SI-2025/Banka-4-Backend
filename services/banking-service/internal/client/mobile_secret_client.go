package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/RAF-SI-2025/Banka-4-Backend/services/banking-service/internal/config"
)

type MobileSecretClient interface {
	GetMobileSecret(ctx context.Context, authorizationHeader string) (string, error)
}

type mobileSecretClient struct {
	httpClient *http.Client
	baseURL    string
}

type mobileSecretResponse struct {
	Secret string `json:"secret"`
}

func NewMobileSecretClient(cfg *config.Configuration) MobileSecretClient {
	return &mobileSecretClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    strings.TrimRight(cfg.UserServiceBaseURL, "/"),
	}
}

func (c *mobileSecretClient) GetMobileSecret(ctx context.Context, authorizationHeader string) (secret string, err error) {
	if strings.TrimSpace(authorizationHeader) == "" {
		return "", fmt.Errorf("missing authorization header")
	}

	url := c.baseURL + "/api/secret-mobile"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create secret request: %w", err)
	}
	req.Header.Set("Authorization", authorizationHeader)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request mobile secret: %w", err)
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close mobile secret response body: %w", closeErr)
			secret = ""
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("user-service returned status %d", resp.StatusCode)
	}

	var payload mobileSecretResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode mobile secret response: %w", err)
	}
	if payload.Secret == "" {
		return "", fmt.Errorf("empty mobile secret")
	}

	secret = payload.Secret
	return secret, nil
}
