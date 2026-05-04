package model

import "time"

type InvestmentFund struct {
	FundID              uint      `gorm:"primaryKey;autoIncrement"`
	Name                string    `gorm:"not null;size:255;uniqueIndex"`
	Description         string    `gorm:"not null;size:1000"`
	MinimumContribution float64   `gorm:"not null;default:0"`
	ManagerID           uint      `gorm:"not null"`
	AccountNumber       string    `gorm:"not null;size:50;uniqueIndex"`
	CreatedAt           time.Time `gorm:"not null"`

	Positions []ClientFundPosition `gorm:"foreignKey:FundID"`
}

type ClientFundPosition struct {
	PositionID          uint            `gorm:"primaryKey;autoIncrement"`
	FundID              uint            `gorm:"not null;uniqueIndex:idx_fund_client"`
	Fund                *InvestmentFund `gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	ClientID            uint            `gorm:"not null;uniqueIndex:idx_fund_client"`
	OwnerType           OwnerType       `gorm:"not null;size:10;uniqueIndex:idx_fund_client;default:'CLIENT'"`
	TotalInvestedAmount float64         `gorm:"not null;default:0"`
	UpdatedAt           time.Time       `gorm:"not null"`
}

type ClientFundInvestment struct {
	ClientFundInvestmentID uint      `gorm:"primaryKey;autoIncrement"`
	ClientID               uint      `gorm:"not null;index"`
	OwnerType              OwnerType `gorm:"not null;size:10"`
	FundID                 uint      `gorm:"not null;index"`
	Fund                   InvestmentFund
	AccountNumber          string  `gorm:"not null;size:50"`
	Amount                 float64 `gorm:"not null"`
	CurrencyCode           string  `gorm:"not null;size:10"`
	CreatedAt              time.Time
}

type FundRedemptionStatus string

const (
	FundRedemptionCompleted          FundRedemptionStatus = "COMPLETED"
	FundRedemptionPendingLiquidation FundRedemptionStatus = "PENDING_LIQUIDATION"
)

type ClientFundRedemption struct {
	ClientFundRedemptionID uint      `gorm:"primaryKey;autoIncrement"`
	ClientID               uint      `gorm:"not null;index"`
	OwnerType              OwnerType `gorm:"not null;size:10"`
	FundID                 uint      `gorm:"not null;index"`
	Fund                   InvestmentFund
	AccountNumber          string               `gorm:"not null;size:50"`
	Amount                 float64              `gorm:"not null"`
	CurrencyCode           string               `gorm:"not null;size:10"`
	Status                 FundRedemptionStatus `gorm:"not null;size:32;index"`
	CreatedAt              time.Time
	CompletedAt            *time.Time
}
