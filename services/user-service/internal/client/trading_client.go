package client

import "context"

type TradingClient interface {
	TransferManagerFunds(ctx context.Context, fromManagerID uint, toManagerID uint) error
}
