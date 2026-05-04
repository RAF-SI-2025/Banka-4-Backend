package grpc

import (
	"context"
	"fmt"

	"github.com/RAF-SI-2025/Banka-4-Backend/common/pkg/pb"
	"github.com/RAF-SI-2025/Banka-4-Backend/services/trading-service/internal/client"
)

type BankingServiceClient struct {
	stub pb.BankingServiceClient
}

func NewBankingServiceClient(conn *client.BankingConn) *BankingServiceClient {
	return &BankingServiceClient{
		stub: pb.NewBankingServiceClient(conn.ClientConn),
	}
}

func (c *BankingServiceClient) GetAccountByNumber(ctx context.Context, accountNumber string) (*pb.GetAccountByNumberResponse, error) {
	resp, err := c.stub.GetAccountByNumber(ctx, &pb.GetAccountByNumberRequest{
		AccountNumber: accountNumber,
	})
	if err != nil {
		return nil, fmt.Errorf("banking client GetAccountByNumber: %w", err)
	}
	return resp, nil
}

func (c *BankingServiceClient) HasActiveLoan(ctx context.Context, clientID uint64) (*pb.HasActiveLoanResponse, error) {
	resp, err := c.stub.HasActiveLoan(ctx, &pb.HasActiveLoanRequest{
		ClientId: clientID,
	})

	if err != nil {
		return nil, fmt.Errorf("banking client HasActiveLoan: %w", err)
	}

	return resp, nil
}

func (c *BankingServiceClient) CreatePaymentWithoutVerification(ctx context.Context, req *pb.CreatePaymentRequest) (*pb.CreatePaymentResponse, error) {
	resp, err := c.stub.CreatePaymentWithoutVerification(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("banking client CreatePayment: %w", err)
	}
	return resp, nil
}

func (c *BankingServiceClient) GetAccountsByClientID(ctx context.Context, clientID uint64) (*pb.GetAccountsByClientIDResponse, error) {
	resp, err := c.stub.GetAccountsByClientID(ctx, &pb.GetAccountsByClientIDRequest{
		ClientId: clientID,
	})
	if err != nil {
		return nil, fmt.Errorf("banking client GetAccountsByClientID: %w", err)
	}
	return resp, nil
}

func (c *BankingServiceClient) ConvertCurrency(ctx context.Context, amount float64, fromCode, toCode string) (float64, error) {
	resp, err := c.stub.ConvertCurrency(ctx, &pb.ConvertCurrencyRequest{
		Amount:   amount,
		FromCode: fromCode,
		ToCode:   toCode,
	})
	if err != nil {
		return 0, fmt.Errorf("banking client ConvertCurrency: %w", err)
	}
	return resp.ConvertedAmount, nil
}

func (c *BankingServiceClient) ExecuteTradeSettlement(ctx context.Context, accountNumber, currencyCode string, direction pb.TradeSettlementDirection, amount float64) (*pb.ExecuteTradeSettlementResponse, error) {
	resp, err := c.stub.ExecuteTradeSettlement(ctx, &pb.ExecuteTradeSettlementRequest{
		AccountNumber:     accountNumber,
		TradeCurrencyCode: currencyCode,
		Direction:         direction,
		Amount:            amount,
	})
	if err != nil {
		return nil, fmt.Errorf("banking client ExecuteTradeSettlement: %w", err)
	}
	return resp, nil
}
func (c *BankingServiceClient) GetAccountCurrency(ctx context.Context, accountNumber string) (string, error) {
	resp, err := c.stub.GetAccountByNumber(ctx, &pb.GetAccountByNumberRequest{
		AccountNumber: accountNumber,
	})
	if err != nil {
		return "", fmt.Errorf("banking client GetAccountCurrency: %w", err)
	}
	return resp.CurrencyCode, nil
}

func (c *BankingServiceClient) CreateFundAccount(ctx context.Context, fundName string, managerID uint64) (string, error) {
	resp, err := c.stub.CreateFundAccount(ctx, &pb.CreateFundAccountRequest{
		FundName:  fundName,
		ManagerId: managerID,
	})
	if err != nil {
		return "", fmt.Errorf("banking client CreateFundAccount: %w", err)
	}
	return resp.AccountNumber, nil
}
