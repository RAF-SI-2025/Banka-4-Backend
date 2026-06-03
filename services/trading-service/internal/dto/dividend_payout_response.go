package dto

import "time"

type DividendPayoutResponse struct {
	DividendPayoutID uint      `json:"dividendPayoutId"`
	AssetOwnershipID uint      `json:"assetOwnershipId"`
	Stock            string    `json:"stock"`
	Quantity         float64   `json:"quantity"`
	GrossAmount      float64   `json:"grossAmount"`
	TaxAmount        float64   `json:"taxAmount"`
	NetAmount        float64   `json:"netAmount"`
	CurrencyCode     string    `json:"currencyCode"`
	AccountNumber    string    `json:"accountNumber"`
	PaymentDate      time.Time `json:"paymentDate"`
}

type ListDividendPayoutsResponse struct {
	Data []DividendPayoutResponse `json:"data"`
}
