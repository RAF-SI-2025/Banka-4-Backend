package dto

import "time"

type AssetType string

const (
	AssetTypeStock   AssetType = "STOCK"
	AssetTypeFutures AssetType = "FUTURES"
	AssetTypeOption  AssetType = "OPTION"
	AssetTypeForex   AssetType = "FOREX"
)

type PortfolioAssetResponse struct {
	AssetID         uint                     `json:"asset_id"`
	Type            AssetType                `json:"type"`
	Ticker          string                   `json:"ticker"`
	Amount          float64                  `json:"amount"`
	PricePerUnitRSD float64                  `json:"price_per_unit_rsd"`
	AvgBuyPriceRSD  float64                  `json:"avg_buy_price_rsd"`
	LastModified    time.Time                `json:"last_modified"`
	Profit          float64                  `json:"profit"`
	PublicAmount    float64                  `json:"public_amount"`
	OptionData      *OptionSpecificAssetData `json:"option_data"`
}

type OptionSpecificAssetData struct {
	StrikePrice    float64   `json:"strike_price"`
	OptionType     string    `json:"option_type"`
	SettlementDate time.Time `json:"settlement_date"`
}

type PortfolioProfitResponse struct {
	TotalProfitRSD float64 `json:"total_profit_rsd"`
}
