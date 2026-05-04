package client

import (
	"context"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
)

type BankingClient interface {
	GetAccountByNumber(ctx context.Context, accountNumber string) (*pb.GetAccountByNumberResponse, error)
	HasActiveLoan(ctx context.Context, clientID uint64) (*pb.HasActiveLoanResponse, error)
	CreatePaymentWithoutVerification(ctx context.Context, req *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error)
	GetAccountsByClientID(ctx context.Context, clientID uint64) (*pb.GetAccountsByClientIDResponse, error)
	ConvertCurrency(ctx context.Context, amount float64, fromCode, toCode string) (float64, error)
	ExecuteTradeSettlement(ctx context.Context, accountNumber, currencyCode string, direction pb.TradeSettlementDirection, amount float64) (*pb.ExecuteTradeSettlementResponse, error)
	GetAccountCurrency(ctx context.Context, accountNumber string) (string, error)
	CreateFundAccount(ctx context.Context, fundName string, managerID uint64) (string, error)
}
