package model

import "time"

type OrderType string
type OrderDirection string
type OrderStatus string

const (
	OrderTypeMarket    OrderType = "MARKET"
	OrderTypeLimit     OrderType = "LIMIT"
	OrderTypeStop      OrderType = "STOP"
	OrderTypeStopLimit OrderType = "STOP_LIMIT"
)

const (
	OrderDirectionBuy  OrderDirection = "BUY"
	OrderDirectionSell OrderDirection = "SELL"
)

const (
	OrderStatusPending  OrderStatus = "PENDING"
	OrderStatusApproved OrderStatus = "APPROVED"
	OrderStatusDeclined OrderStatus = "DECLINED"
)

type Order struct {
	OrderID          uint      `gorm:"primaryKey;autoIncrement"`
	OrderOwnerUserID uint      `gorm:"not null;index"`
	OrderOwnerType   OwnerType `gorm:"not null;size:10;default:'CLIENT'"`
	AssetOwnerUserID uint      `gorm:"not null;default:0"`
	AssetOwnerType   OwnerType `gorm:"not null;size:10;default:''"`

	AccountNumber     string `gorm:"not null;size:18;index"`
	ListingID         uint   `gorm:"not null;index"`
	Listing           Listing
	OrderType         OrderType      `gorm:"not null;size:20"`
	Direction         OrderDirection `gorm:"not null;size:4"`
	Quantity          uint           `gorm:"not null"`
	FilledQty         uint           `gorm:"not null;default:0"`
	LimitValue        *float64
	StopValue         *float64
	ContractSize      float64 `gorm:"not null;default:1"`
	PricePerUnit      *float64
	AllOrNone         bool        `gorm:"not null;default:false"`
	Margin            bool        `gorm:"not null;default:false"`
	Status            OrderStatus `gorm:"not null;size:20"`
	ApprovedBy        *uint
	IsDone            bool `gorm:"not null;default:false"`
	AfterHours        bool `gorm:"not null;default:false"`
	Triggered         bool `gorm:"not null;default:false"`
	NextExecutionAt   *time.Time
	CommissionCharged bool      `gorm:"not null;default:false"`
	CommissionExempt  bool      `gorm:"not null;default:false"`
	OwnerType         OwnerType `gorm:"not null;size:10;default:'CLIENT'"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (o *Order) RemainingPortions() uint {
	return o.Quantity - o.FilledQty
}
